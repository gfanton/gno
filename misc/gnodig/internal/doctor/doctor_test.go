package doctor

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/gnolang/gno/misc/gnodig/internal/chainrpc"
	"github.com/gnolang/gno/misc/gnodig/internal/nodedata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func findingIDs(report *Report) map[string]bool {
	ids := make(map[string]bool)
	for _, f := range report.Findings {
		ids[f.ID] = true
	}
	return ids
}

func TestRunChecks_HaltedChain(t *testing.T) {
	// Simulate gnoland1-style halt: old block time, high round, vote split.
	oldTime := time.Now().Add(-30 * time.Minute).UTC().Format(time.RFC3339Nano)
	status := &chainrpc.Overview{
		Status: json.RawMessage(fmt.Sprintf(`{
			"sync_info": {
				"latest_block_height": "352922",
				"latest_block_time": %q,
				"catching_up": false
			}
		}`, oldTime)),
		ConsensusState: json.RawMessage(`{
			"round_state": {"height/round/step": "352923/5/6"}
		}`),
		NetInfo:          json.RawMessage(`{"n_peers": "4"}`),
		UnconfirmedCount: json.RawMessage(`{"n_txs": "5"}`),
	}

	walSummary := &nodedata.WALSummary{
		Height: 352923,
		Rounds: []nodedata.RoundSummary{{
			Round: 0,
			Prevotes: nodedata.VoteTally{
				Block: map[string]int{"AABB": 3, "CCDD": 2},
				Total: 5,
			},
		}},
	}

	ctx := newTestContext(
		withStatus(status),
		withWALSummary(walSummary),
	)
	report := runChecks(ctx)

	ids := findingIDs(report)
	assert.True(t, ids["chain_halted"], "expected chain_halted finding")
	assert.True(t, ids["consensus_stuck"], "expected consensus_stuck finding")
	assert.True(t, ids["vote_split"], "expected vote_split finding")
	assert.True(t, ids["apphash_divergence"], "expected apphash_divergence correlation")
	assert.True(t, ids["mempool_backlog"], "expected mempool_backlog finding")
	assert.False(t, report.Healthy, "halted chain should not be healthy")
}

func TestRunChecks_HealthyNode(t *testing.T) {
	recentTime := time.Now().Add(-2 * time.Second).UTC().Format(time.RFC3339Nano)
	status := &chainrpc.Overview{
		Status: json.RawMessage(fmt.Sprintf(`{
			"sync_info": {
				"latest_block_height": "100",
				"latest_block_time": %q,
				"catching_up": false
			}
		}`, recentTime)),
		ConsensusState:   json.RawMessage(`{"round_state": {"height/round/step": "101/0/1"}}`),
		NetInfo:          json.RawMessage(`{"n_peers": "10"}`),
		UnconfirmedCount: json.RawMessage(`{"n_txs": "0"}`),
	}

	dataOv := &nodedata.Overview{
		LatestHeight:     100,
		BlockStoreHeight: 100,
		WALHeight:        100,
		BlockIDMatch:     boolPtr(true),
	}

	walSummary := &nodedata.WALSummary{
		Height: 100,
		Rounds: []nodedata.RoundSummary{{
			Round: 0,
			Prevotes: nodedata.VoteTally{
				Block: map[string]int{"AABB": 5},
				Total: 5,
			},
		}},
	}

	ctx := newTestContext(
		withStatus(status),
		withDataOverview(dataOv),
		withWALSummary(walSummary),
	)
	report := runChecks(ctx)

	assert.Empty(t, report.Findings, "healthy node should have no findings")
	assert.Empty(t, report.Errors, "healthy node should have no errors")
	assert.True(t, report.Healthy, "healthy node should be marked healthy")
}

func TestRunChecks_DataDirOnly(t *testing.T) {
	// Only data dir providers wired — RPC checks should be silently skipped.
	dataOv := &nodedata.Overview{
		LatestHeight:     352922,
		BlockStoreHeight: 352922,
		WALHeight:        352923,
		BlockIDMatch:     boolPtr(false),
	}

	walSummary := &nodedata.WALSummary{
		Height: 352923,
		Rounds: []nodedata.RoundSummary{{
			Round: 0,
			Prevotes: nodedata.VoteTally{
				Block: map[string]int{"AABB": 5},
				Total: 5,
			},
		}},
	}

	ctx := newTestContext(
		withDataOverview(dataOv),
		withWALSummary(walSummary),
	)
	report := runChecks(ctx)

	ids := findingIDs(report)
	assert.True(t, ids["wal_ahead"], "expected wal_ahead finding")
	assert.True(t, ids["state_mismatch"], "expected state_mismatch finding")
	assert.True(t, ids["crash_recovery"], "expected crash_recovery correlation")

	// RPC checks should NOT produce errors — they should be silently skipped.
	for _, ce := range report.Errors {
		require.NotContains(t, ce.Error, "not available",
			"RPC check %q should be silently skipped, not produce an error", ce.CheckID)
	}

	assert.False(t, report.Healthy, "node with state_mismatch should not be healthy")
}
