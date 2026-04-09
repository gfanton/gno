package doctor

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/gnolang/gno/misc/gnodig/internal/chainrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeOverview(statusJSON string) *chainrpc.Overview {
	return &chainrpc.Overview{
		Status: json.RawMessage(statusJSON),
	}
}

func TestCheckChainHalted_Halted(t *testing.T) {
	ov := makeOverview(`{
		"sync_info": {
			"latest_block_height": "352922",
			"latest_block_time": "2026-03-30T14:17:13Z",
			"catching_up": false
		}
	}`)
	ctx := newTestContext(withStatus(ov))
	findings, err := checkChainHalted(ctx)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, "chain_halted", findings[0].ID)
	assert.Equal(t, Critical, findings[0].Severity)
}

func TestCheckChainHalted_Healthy(t *testing.T) {
	recentTime := time.Now().Add(-10 * time.Second).UTC().Format(time.RFC3339Nano)
	ov := makeOverview(fmt.Sprintf(`{
		"sync_info": {
			"latest_block_height": "100",
			"latest_block_time": %q,
			"catching_up": false
		}
	}`, recentTime))
	ctx := newTestContext(withStatus(ov))
	findings, err := checkChainHalted(ctx)
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func TestCheckChainHalted_CatchingUp(t *testing.T) {
	ov := makeOverview(`{
		"sync_info": {
			"latest_block_height": "100",
			"latest_block_time": "2020-01-01T00:00:00Z",
			"catching_up": true
		}
	}`)
	ctx := newTestContext(withStatus(ov))
	findings, err := checkChainHalted(ctx)
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func TestCheckConsensusStuck_HighRound(t *testing.T) {
	ov := &chainrpc.Overview{
		Status:         json.RawMessage(`{"sync_info": {"latest_block_height": "352922"}}`),
		ConsensusState: json.RawMessage(`{"round_state": {"height/round/step": "352923/5/6"}}`),
	}
	ctx := newTestContext(withStatus(ov))
	findings, err := checkConsensusStuck(ctx)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, "consensus_stuck", findings[0].ID)
	assert.Equal(t, Critical, findings[0].Severity)
	assert.Contains(t, findings[0].Detail, "round 5")
}

func TestCheckConsensusStuck_LowRound(t *testing.T) {
	ov := &chainrpc.Overview{
		Status:         json.RawMessage(`{"sync_info": {"latest_block_height": "100"}}`),
		ConsensusState: json.RawMessage(`{"round_state": {"height/round/step": "101/1/3"}}`),
	}
	ctx := newTestContext(withStatus(ov))
	findings, err := checkConsensusStuck(ctx)
	require.NoError(t, err)
	assert.Empty(t, findings) // round 1 is transient, not stuck
}

func TestCheckConsensusStuck_Healthy(t *testing.T) {
	ov := &chainrpc.Overview{
		Status:         json.RawMessage(`{"sync_info": {"latest_block_height": "100"}}`),
		ConsensusState: json.RawMessage(`{"round_state": {"height/round/step": "101/0/1"}}`),
	}
	ctx := newTestContext(withStatus(ov))
	findings, err := checkConsensusStuck(ctx)
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func TestCheckLowPeers_Low(t *testing.T) {
	ov := &chainrpc.Overview{
		NetInfo: json.RawMessage(`{"n_peers": "1"}`),
	}
	ctx := newTestContext(withStatus(ov))
	findings, err := checkLowPeers(ctx)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, "low_peers", findings[0].ID)
	assert.Equal(t, Warning, findings[0].Severity)
	assert.Contains(t, findings[0].Detail, "1 peers")
}

func TestCheckLowPeers_Healthy(t *testing.T) {
	ov := &chainrpc.Overview{
		NetInfo: json.RawMessage(`{"n_peers": "10"}`),
	}
	ctx := newTestContext(withStatus(ov))
	findings, err := checkLowPeers(ctx)
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func TestCheckMempoolBacklog_WithBacklog(t *testing.T) {
	oldTime := time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339Nano)
	ov := &chainrpc.Overview{
		UnconfirmedCount: json.RawMessage(`{"n_txs": "5"}`),
		Status:           json.RawMessage(fmt.Sprintf(`{"sync_info": {"latest_block_time": %q}}`, oldTime)),
	}
	ctx := newTestContext(withStatus(ov))
	findings, err := checkMempoolBacklog(ctx)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, "mempool_backlog", findings[0].ID)
	assert.Equal(t, Info, findings[0].Severity)
	assert.Contains(t, findings[0].Detail, "5 unconfirmed txs")
}

func TestCheckMempoolBacklog_NoBacklog(t *testing.T) {
	ov := &chainrpc.Overview{
		UnconfirmedCount: json.RawMessage(`{"n_txs": "0"}`),
	}
	ctx := newTestContext(withStatus(ov))
	findings, err := checkMempoolBacklog(ctx)
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{2*time.Hour + 30*time.Minute, "2h30m"},
		{25 * time.Hour, "1d1h"},
		{48 * time.Hour, "2d0h"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, formatDuration(tt.d))
		})
	}
}
