package nodedata

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSummarizeWAL(t *testing.T) {
	msgs := []WALMessage{
		{Time: "2026-03-29T14:17:00.000Z", Type: "/tm.msgInfo",
			Data: json.RawMessage(`{"msg":{"@type":"/tm.ProposalMessage","Proposal":{"height":"100","round":"0","block_id":{"hash":"qrs="}}},"peer_key":""}`)},
		{Time: "2026-03-29T14:17:01.000Z", Type: "/tm.msgInfo",
			Data: json.RawMessage(`{"msg":{"@type":"/tm.VoteMessage","Vote":{"type":1,"height":"100","round":"0","block_id":{"hash":"qrs="},"validator_address":"g1val1"}},"peer_key":""}`)},
		{Time: "2026-03-29T14:17:01.100Z", Type: "/tm.msgInfo",
			Data: json.RawMessage(`{"msg":{"@type":"/tm.VoteMessage","Vote":{"type":1,"height":"100","round":"0","block_id":{"hash":null},"validator_address":"g1val2"}},"peer_key":""}`)},
		{Time: "2026-03-29T14:17:02.000Z", Type: "/tm.timeoutInfo",
			Data: json.RawMessage(`{"duration":"1000000000","height":"100","round":"0","step":"RoundStepPrecommit"}`)},
	}

	summary := SummarizeWAL(100, msgs)

	require.Equal(t, int64(100), summary.Height)
	require.Len(t, summary.Rounds, 1)

	r := summary.Rounds[0]
	require.Equal(t, 0, r.Round)
	require.Equal(t, 2, r.Prevotes.Total)
	require.Equal(t, 1, r.Prevotes.Nil)
	require.Equal(t, "timeout", r.Outcome)
}

func TestSummarizeWAL_MultiRound(t *testing.T) {
	msgs := []WALMessage{
		{Time: "t1", Type: "/tm.msgInfo",
			Data: json.RawMessage(`{"msg":{"@type":"/tm.ProposalMessage","Proposal":{"height":"200","round":"0","block_id":{"hash":"qQ=="}}},"peer_key":""}`)},
		{Time: "t2", Type: "/tm.timeoutInfo",
			Data: json.RawMessage(`{"duration":"1000000000","height":"200","round":"0","step":"RoundStepPrevote"}`)},
		{Time: "t3", Type: "/tm.msgInfo",
			Data: json.RawMessage(`{"msg":{"@type":"/tm.ProposalMessage","Proposal":{"height":"200","round":"1","block_id":{"hash":"uw=="}}},"peer_key":""}`)},
		{Time: "t4", Type: "/tm.msgInfo",
			Data: json.RawMessage(`{"msg":{"@type":"/tm.VoteMessage","Vote":{"type":2,"height":"200","round":"1","block_id":{"hash":"uw=="},"validator_address":"g1v1"}},"peer_key":""}`)},
		{Time: "t5", Type: "/tm.msgInfo",
			Data: json.RawMessage(`{"msg":{"@type":"/tm.VoteMessage","Vote":{"type":2,"height":"200","round":"1","block_id":{"hash":"uw=="},"validator_address":"g1v2"}},"peer_key":""}`)},
	}

	summary := SummarizeWAL(200, msgs)

	require.Len(t, summary.Rounds, 2)
	require.Equal(t, "timeout", summary.Rounds[0].Outcome)
	require.Equal(t, 2, summary.Rounds[1].Precommits.Block["uw=="])
}

func TestFilterWALMessages(t *testing.T) {
	msgs := []WALMessage{
		// Round 0: proposal + prevote
		{Time: "t1", Type: "/tm.msgInfo",
			Data: json.RawMessage(`{"msg":{"@type":"/tm.ProposalMessage","Proposal":{"height":"100","round":"0","block_id":{"hash":"qrs="}}},"peer_key":""}`)},
		{Time: "t2", Type: "/tm.msgInfo",
			Data: json.RawMessage(`{"msg":{"@type":"/tm.VoteMessage","Vote":{"type":1,"height":"100","round":"0","block_id":{"hash":"qrs="},"validator_address":"g1val1"}},"peer_key":""}`)},
		// Round 0: timeout
		{Time: "t3", Type: "/tm.timeoutInfo",
			Data: json.RawMessage(`{"duration":"1000000000","height":"100","round":"0","step":"RoundStepPrecommit"}`)},
		// Round 1: proposal + precommit
		{Time: "t4", Type: "/tm.msgInfo",
			Data: json.RawMessage(`{"msg":{"@type":"/tm.ProposalMessage","Proposal":{"height":"100","round":"1","block_id":{"hash":"uw=="}}},"peer_key":""}`)},
		{Time: "t5", Type: "/tm.msgInfo",
			Data: json.RawMessage(`{"msg":{"@type":"/tm.VoteMessage","Vote":{"type":2,"height":"100","round":"1","block_id":{"hash":"uw=="},"validator_address":"g1val2"}},"peer_key":""}`)},
	}

	t.Run("filter by round 0", func(t *testing.T) {
		round := 0
		got := FilterWALMessages(msgs, &round, "", 100)
		require.Len(t, got, 3) // proposal + prevote + timeout
	})

	t.Run("filter by round 1", func(t *testing.T) {
		round := 1
		got := FilterWALMessages(msgs, &round, "", 100)
		require.Len(t, got, 2) // proposal + precommit
	})

	t.Run("filter by type prevote", func(t *testing.T) {
		got := FilterWALMessages(msgs, nil, "prevote", 100)
		require.Len(t, got, 1)
	})

	t.Run("filter by type precommit", func(t *testing.T) {
		got := FilterWALMessages(msgs, nil, "precommit", 100)
		require.Len(t, got, 1)
	})

	t.Run("filter by type proposal", func(t *testing.T) {
		got := FilterWALMessages(msgs, nil, "proposal", 100)
		require.Len(t, got, 2) // both rounds have proposals
	})

	t.Run("filter by type timeout", func(t *testing.T) {
		got := FilterWALMessages(msgs, nil, "timeout", 100)
		require.Len(t, got, 1)
	})

	t.Run("filter by round 0 and type prevote", func(t *testing.T) {
		round := 0
		got := FilterWALMessages(msgs, &round, "prevote", 100)
		require.Len(t, got, 1)
	})

	t.Run("no filters returns all", func(t *testing.T) {
		got := FilterWALMessages(msgs, nil, "", 100)
		require.Len(t, got, len(msgs))
	})
}

func TestSummarizeWAL_DedupGossip(t *testing.T) {
	// Same vote from the same validator appears 3 times (gossip duplicates)
	vote := `{"msg":{"@type":"/tm.VoteMessage","Vote":{"type":1,"height":"100","round":"0","block_id":{"hash":"qrs="},"validator_address":"g1val1"}},"peer_key":""}`
	msgs := []WALMessage{
		{Time: "t1", Type: "/tm.msgInfo", Data: json.RawMessage(vote)},
		{Time: "t2", Type: "/tm.msgInfo", Data: json.RawMessage(vote)},
		{Time: "t3", Type: "/tm.msgInfo", Data: json.RawMessage(vote)},
	}

	summary := SummarizeWAL(100, msgs)
	require.Len(t, summary.Rounds, 1)
	// Should count only 1 prevote, not 3
	require.Equal(t, 1, summary.Rounds[0].Prevotes.Total)
	require.Equal(t, 1, summary.Rounds[0].Prevotes.Block["qrs="])
}

func TestSummarizeWAL_StaleVoteFiltered(t *testing.T) {
	// Vote for height 99 mixed into height 100 messages
	staleVote := `{"msg":{"@type":"/tm.VoteMessage","Vote":{"type":2,"height":"99","round":"0","block_id":{"hash":"old="},"validator_address":"g1stale"}},"peer_key":""}`
	goodVote := `{"msg":{"@type":"/tm.VoteMessage","Vote":{"type":1,"height":"100","round":"0","block_id":{"hash":"qrs="},"validator_address":"g1good"}},"peer_key":""}`
	msgs := []WALMessage{
		{Time: "t1", Type: "/tm.msgInfo", Data: json.RawMessage(staleVote)},
		{Time: "t2", Type: "/tm.msgInfo", Data: json.RawMessage(goodVote)},
	}

	summary := SummarizeWAL(100, msgs)
	require.Len(t, summary.Rounds, 1)
	// Only the good vote should be counted
	require.Equal(t, 1, summary.Rounds[0].Prevotes.Total)
	require.Equal(t, 0, summary.Rounds[0].Precommits.Total)
}

func TestSummarizeWAL_PerValidatorVoters(t *testing.T) {
	msgs := []WALMessage{
		{Time: "t1", Type: "/tm.msgInfo",
			Data: json.RawMessage(`{"msg":{"@type":"/tm.VoteMessage","Vote":{"type":1,"height":"100","round":"0","block_id":{"hash":"AA"},"validator_address":"g1alice"}},"peer_key":""}`)},
		{Time: "t2", Type: "/tm.msgInfo",
			Data: json.RawMessage(`{"msg":{"@type":"/tm.VoteMessage","Vote":{"type":1,"height":"100","round":"0","block_id":{"hash":""},"validator_address":"g1bob"}},"peer_key":""}`)},
		{Time: "t3", Type: "/tm.msgInfo",
			Data: json.RawMessage(`{"msg":{"@type":"/tm.VoteMessage","Vote":{"type":1,"height":"100","round":"0","block_id":{"hash":"AA"},"validator_address":"g1charlie"}},"peer_key":""}`)},
	}

	summary := SummarizeWAL(100, msgs)
	r := summary.Rounds[0]
	require.Equal(t, 3, r.Prevotes.Total)
	require.Equal(t, 2, r.Prevotes.Block["AA"])
	require.ElementsMatch(t, []string{"g1alice", "g1charlie"}, r.Prevotes.Voters["AA"])
	require.Equal(t, 1, r.Prevotes.Nil)
	require.ElementsMatch(t, []string{"g1bob"}, r.Prevotes.NilVoters)
}

func TestFilterWALMessages_DedupAndStale(t *testing.T) {
	vote1 := `{"msg":{"@type":"/tm.VoteMessage","Vote":{"type":1,"height":"100","round":"0","block_id":{"hash":"AA"},"validator_address":"g1val1"}},"peer_key":""}`
	stale := `{"msg":{"@type":"/tm.VoteMessage","Vote":{"type":1,"height":"99","round":"0","block_id":{"hash":"old"},"validator_address":"g1stale"}},"peer_key":""}`
	msgs := []WALMessage{
		{Time: "t1", Type: "/tm.msgInfo", Data: json.RawMessage(vote1)},
		{Time: "t2", Type: "/tm.msgInfo", Data: json.RawMessage(vote1)}, // gossip dup
		{Time: "t3", Type: "/tm.msgInfo", Data: json.RawMessage(vote1)}, // gossip dup
		{Time: "t4", Type: "/tm.msgInfo", Data: json.RawMessage(stale)}, // stale
	}

	got := FilterWALMessages(msgs, nil, "prevote", 100)
	require.Len(t, got, 1) // 1 unique vote, stale filtered
}

func TestSlimVote(t *testing.T) {
	innerJSON := `{"@type":"/tm.VoteMessage","Vote":{"type":1,"height":"100","round":"0","block_id":{"hash":"qrs=","parts":{"total":"1","hash":"uw=="}},"timestamp":"2026-03-30T14:17:19.123Z","validator_address":"g1val1","validator_index":"3","signature":"AAAAAAAAAA=="}}`
	slim := slimVote(innerJSON)

	var result slimVoteData
	require.NoError(t, json.Unmarshal(slim, &result))
	require.Equal(t, int64(1), result.Type)
	require.Equal(t, int64(0), result.Round)
	require.Equal(t, "qrs=", result.BlockHash)
	require.Equal(t, "g1val1", result.ValidatorAddress)
	require.Equal(t, int64(3), result.ValidatorIndex)

	// Slim should not contain signature or parts hash
	require.NotContains(t, string(slim), "signature")
	require.NotContains(t, string(slim), "parts")
}

func TestSlimProposal(t *testing.T) {
	innerJSON := `{"@type":"/tm.ProposalMessage","Proposal":{"height":"100","round":"0","block_id":{"hash":"qrs=","parts":{"total":"1","hash":"uw=="}},"pol_round":"-1","timestamp":"2026-03-30T14:17:19.123Z","signature":"BBBBBBBBBB=="}}`
	slim := slimProposal(innerJSON)

	var result slimProposalWrapper
	require.NoError(t, json.Unmarshal(slim, &result))
	require.Equal(t, int64(0), result.Proposal.Round)
	require.Equal(t, "qrs=", result.Proposal.BlockHash)
}
