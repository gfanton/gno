package chainrpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dumpConsensusResponse is a realistic TM2 dump_consensus_state response.
// The peer_state field is amino JSON encoded as a string inside the JSON.
const dumpConsensusResponse = `{
	"jsonrpc": "2.0",
	"id": "",
	"result": {
		"round_state": {
			"height/round/step": "500/0/RoundStepPrevote",
			"proposal_block_hash": "ABCD1234",
			"locked_block_hash": "",
			"valid_block_hash": "ABCD1234"
		},
		"peers": [
			{
				"node_address": "g1peer1@192.168.1.10:26656",
				"peer_state": "{\"round_state\":{\"height\":500,\"round\":0,\"step\":3,\"proposal\":true,\"prevotes\":\"BA{4:xx__}\",\"precommits\":\"BA{4:x___}\",\"last_commit\":\"BA{4:xxxx}\"}}"
			},
			{
				"node_address": "g1peer2@192.168.1.11:26656",
				"peer_state": "{\"round_state\":{\"height\":499,\"round\":0,\"step\":1,\"proposal\":false,\"prevotes\":\"BA{4:____}\",\"precommits\":\"BA{4:____}\",\"last_commit\":\"BA{4:xxx_}\"}}"
			},
			{
				"node_address": "g1peer3@192.168.1.12:26656",
				"peer_state": "{\"round_state\":{\"height\":500,\"round\":0,\"step\":3,\"proposal\":true,\"prevotes\":\"BA{4:xxx_}\",\"precommits\":\"BA{4:xx__}\",\"last_commit\":\"BA{4:xxxx}\"}}"
			}
		]
	}
}`

// netInfoResponse provides moniker mapping for peers.
const netInfoResponse = `{
	"jsonrpc": "2.0",
	"id": "",
	"result": {
		"n_peers": "3",
		"peers": [
			{
				"node_info": {
					"net_address": "g1peer1@192.168.1.10:26656",
					"moniker": "validator-alpha",
					"listen_addr": "192.168.1.10:26656"
				},
				"remote_ip": "192.168.1.10"
			},
			{
				"node_info": {
					"net_address": "g1peer2@192.168.1.11:26656",
					"moniker": "validator-beta",
					"listen_addr": "192.168.1.11:26656"
				},
				"remote_ip": "192.168.1.11"
			},
			{
				"node_info": {
					"net_address": "g1peer3@192.168.1.12:26656",
					"moniker": "validator-gamma",
					"listen_addr": "192.168.1.12:26656"
				},
				"remote_ip": "192.168.1.12"
			}
		]
	}
}`

func TestFetchPeerConsensus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/dump_consensus_state":
			_, _ = w.Write([]byte(dumpConsensusResponse))
		case "/net_info":
			_, _ = w.Write([]byte(netInfoResponse))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := New(srv.URL)
	result, err := FetchPeerConsensus(context.Background(), c)
	require.NoError(t, err)

	// ---- Local state
	assert.Equal(t, int64(500), result.Local.Height)
	assert.Equal(t, 0, result.Local.Round)
	assert.Equal(t, "RoundStepPrevote", result.Local.Step)
	assert.Equal(t, "ABCD1234", result.Local.ProposalBlockHash)
	assert.Equal(t, "", result.Local.LockedBlockHash)
	assert.Equal(t, "ABCD1234", result.Local.ValidBlockHash)

	// ---- Peer count
	require.Len(t, result.Peers, 3)

	// ---- Peer 0 (at height 500)
	p0 := result.Peers[0]
	assert.Equal(t, "validator-alpha", p0.Moniker)
	assert.Equal(t, "g1peer1", p0.Address)
	assert.Equal(t, "192.168.1.10", p0.RemoteIP)
	assert.Equal(t, int64(500), p0.Height)
	assert.Equal(t, 0, p0.Round)
	assert.Equal(t, 3, p0.Step)
	assert.True(t, p0.HasProposal)

	// Prevotes: BA{4:xx__} → 2 of 4, indices [0,1]
	assert.Equal(t, 2, p0.Prevotes.Count)
	assert.Equal(t, 4, p0.Prevotes.Total)
	assert.Equal(t, []int{0, 1}, p0.Prevotes.Indices)
	assert.Equal(t, "BA{4:xx__}", p0.Prevotes.BitArray)
	assert.NotNil(t, p0.Prevotes.Voters) // always non-nil

	// Precommits: BA{4:x___} → 1 of 4, indices [0]
	assert.Equal(t, 1, p0.Precommits.Count)
	assert.Equal(t, 4, p0.Precommits.Total)
	assert.Equal(t, []int{0}, p0.Precommits.Indices)

	// LastCommit: BA{4:xxxx} → 4 of 4
	assert.Equal(t, 4, p0.LastCommit.Count)
	assert.Equal(t, 4, p0.LastCommit.Total)
	assert.Equal(t, []int{0, 1, 2, 3}, p0.LastCommit.Indices)

	// ---- Peer 1 (behind at height 499)
	p1 := result.Peers[1]
	assert.Equal(t, "validator-beta", p1.Moniker)
	assert.Equal(t, int64(499), p1.Height)
	assert.Equal(t, 0, p1.Round)
	assert.Equal(t, 1, p1.Step)
	assert.False(t, p1.HasProposal)

	// Prevotes: BA{4:____} → 0 of 4
	assert.Equal(t, 0, p1.Prevotes.Count)
	assert.Equal(t, 4, p1.Prevotes.Total)
	assert.Equal(t, []int{}, p1.Prevotes.Indices) // empty, never nil

	// LastCommit: BA{4:xxx_} → 3 of 4
	assert.Equal(t, 3, p1.LastCommit.Count)
	assert.Equal(t, []int{0, 1, 2}, p1.LastCommit.Indices)

	// ---- Peer 2
	p2 := result.Peers[2]
	assert.Equal(t, "validator-gamma", p2.Moniker)
	assert.Equal(t, int64(500), p2.Height)

	// ---- Summary
	assert.Equal(t, 3, result.Summary.TotalPeers)
	assert.Equal(t, 2, result.Summary.PeersAtSameHeight) // peer0 and peer2 at 500
	assert.Equal(t, 1, result.Summary.PeersBehind)       // peer1 at 499
	assert.Equal(t, 2, result.Summary.Heights["500"])
	assert.Equal(t, 1, result.Summary.Heights["499"])
}

func TestFetchPeerConsensus_NetInfoFailure(t *testing.T) {
	// net_info fails but dump_consensus_state works — should still succeed
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/dump_consensus_state":
			_, _ = w.Write([]byte(dumpConsensusResponse))
		case "/net_info":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","error":{"code":-32600,"message":"internal error"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := New(srv.URL)
	result, err := FetchPeerConsensus(context.Background(), c)
	require.NoError(t, err)

	// Should still have peers, just no monikers
	require.Len(t, result.Peers, 3)
	assert.Equal(t, "", result.Peers[0].Moniker)
	assert.Equal(t, "g1peer1", result.Peers[0].Address)
}

func TestParseBitArray(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		count   int
		total   int
		indices []int
	}{
		{
			name:    "mixed set/unset",
			input:   "BA{7:x_x__x_}",
			count:   3,
			total:   7,
			indices: []int{0, 2, 5},
		},
		{
			name:    "none set",
			input:   "BA{7:_______}",
			count:   0,
			total:   7,
			indices: []int{},
		},
		{
			name:    "all set",
			input:   "BA{7:xxxxxxx}",
			count:   7,
			total:   7,
			indices: []int{0, 1, 2, 3, 4, 5, 6},
		},
		{
			name:    "single set",
			input:   "BA{1:x}",
			count:   1,
			total:   1,
			indices: []int{0},
		},
		{
			name:    "empty string",
			input:   "",
			count:   0,
			total:   0,
			indices: []int{},
		},
		{
			name:    "null string",
			input:   "null",
			count:   0,
			total:   0,
			indices: []int{},
		},
		{
			name:    "nil-BitArray",
			input:   "nil-BitArray",
			count:   0,
			total:   0,
			indices: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := parseBitArray(tt.input)
			assert.Equal(t, tt.count, info.Count, "count")
			assert.Equal(t, tt.total, info.Total, "total")
			assert.Equal(t, tt.indices, info.Indices, "indices")
			assert.Equal(t, tt.input, info.BitArray, "bitarray preserved")
			assert.NotNil(t, info.Indices, "indices must never be nil")
			assert.NotNil(t, info.Voters, "voters must never be nil")
		})
	}
}
