package doctor

import (
	"testing"

	"github.com/gnolang/gno/misc/gnodig/internal/nodedata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func boolPtr(b bool) *bool { return &b }

func TestCheckWALAhead_Ahead(t *testing.T) {
	ov := &nodedata.Overview{
		LatestHeight:     352922,
		BlockStoreHeight: 352922,
		WALHeight:        352923,
	}
	ctx := newTestContext(withDataOverview(ov))
	findings, err := checkWALAhead(ctx)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, "wal_ahead", findings[0].ID)
	assert.Equal(t, Info, findings[0].Severity)
	assert.Contains(t, findings[0].Detail, "352923")
	assert.Contains(t, findings[0].Detail, "352922")
}

func TestCheckWALAhead_Normal(t *testing.T) {
	ov := &nodedata.Overview{
		LatestHeight:     352922,
		BlockStoreHeight: 352922,
		WALHeight:        352922,
	}
	ctx := newTestContext(withDataOverview(ov))
	findings, err := checkWALAhead(ctx)
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func TestCheckVoteSplit_Split(t *testing.T) {
	ws := &nodedata.WALSummary{
		Height: 352923,
		Rounds: []nodedata.RoundSummary{{
			Round: 0,
			Prevotes: nodedata.VoteTally{
				Block: map[string]int{
					"AABB": 3,
					"CCDD": 2,
				},
				Total: 5,
			},
		}},
	}
	ctx := newTestContext(withWALSummary(ws))
	findings, err := checkVoteSplit(ctx)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, "vote_split", findings[0].ID)
	assert.Equal(t, Critical, findings[0].Severity)
	assert.Contains(t, findings[0].Detail, "2 distinct prevote hashes")
}

func TestCheckVoteSplit_NoSplit(t *testing.T) {
	ws := &nodedata.WALSummary{
		Height: 352923,
		Rounds: []nodedata.RoundSummary{{
			Round: 0,
			Prevotes: nodedata.VoteTally{
				Block: map[string]int{
					"AABB": 5,
				},
				Total: 5,
			},
		}},
	}
	ctx := newTestContext(withWALSummary(ws))
	findings, err := checkVoteSplit(ctx)
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func TestCheckStateMismatch_Mismatch(t *testing.T) {
	ov := &nodedata.Overview{
		LatestHeight:     352922,
		BlockStoreHeight: 352922,
		BlockIDMatch:     boolPtr(false),
	}
	ctx := newTestContext(withDataOverview(ov))
	findings, err := checkStateMismatch(ctx)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, "state_mismatch", findings[0].ID)
	assert.Equal(t, Critical, findings[0].Severity)
}

func TestCheckStateMismatch_Healthy(t *testing.T) {
	ov := &nodedata.Overview{
		LatestHeight:     352922,
		BlockStoreHeight: 352922,
		BlockIDMatch:     boolPtr(true),
	}
	ctx := newTestContext(withDataOverview(ov))
	findings, err := checkStateMismatch(ctx)
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func TestCheckProposalRejected_Rejected(t *testing.T) {
	// Real pattern from gnoland1 halt: 2 voted for block, 9 voted nil
	ws := &nodedata.WALSummary{
		Height: 352923,
		Rounds: []nodedata.RoundSummary{{
			Round: 0,
			Prevotes: nodedata.VoteTally{
				Block: map[string]int{"gUxMqP1U...": 2},
				Nil:   9,
				Total: 11,
			},
		}},
	}
	ctx := newTestContext(withWALSummary(ws))
	findings, err := checkProposalRejected(ctx)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, "proposal_rejected", findings[0].ID)
	assert.Equal(t, Warning, findings[0].Severity)
	assert.Contains(t, findings[0].Detail, "9 of 11")
	assert.Contains(t, findings[0].Detail, "confidence: medium")
}

func TestCheckProposalRejected_Healthy(t *testing.T) {
	// Normal consensus: most voted for the block, few nil
	ws := &nodedata.WALSummary{
		Height: 100,
		Rounds: []nodedata.RoundSummary{{
			Round: 0,
			Prevotes: nodedata.VoteTally{
				Block: map[string]int{"hashA": 10},
				Nil:   1,
				Total: 11,
			},
		}},
	}
	ctx := newTestContext(withWALSummary(ws))
	findings, err := checkProposalRejected(ctx)
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func TestCheckProposalRejected_AllNil(t *testing.T) {
	// All nil, no block votes — proposer timeout, not rejection
	ws := &nodedata.WALSummary{
		Height: 100,
		Rounds: []nodedata.RoundSummary{{
			Round: 0,
			Prevotes: nodedata.VoteTally{
				Block: map[string]int{},
				Nil:   11,
				Total: 11,
			},
		}},
	}
	ctx := newTestContext(withWALSummary(ws))
	findings, err := checkProposalRejected(ctx)
	require.NoError(t, err)
	assert.Empty(t, findings) // no block votes = timeout, not rejection
}
