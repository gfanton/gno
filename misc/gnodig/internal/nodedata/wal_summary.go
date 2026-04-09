package nodedata

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

// TM2 consensus vote type constants.
const (
	voteTypePrevote   = 1
	voteTypePrecommit = 2
)

// WALSummary holds a per-round digest of WAL activity at a height.
type WALSummary struct {
	Height int64          `json:"height"`
	Rounds []RoundSummary `json:"rounds"`
}

// RoundSummary aggregates votes and outcomes for a single consensus round.
type RoundSummary struct {
	Round           int       `json:"round"`
	Proposer        string    `json:"proposer,omitempty"`
	BlockHash       string    `json:"block_hash,omitempty"`
	Prevotes        VoteTally `json:"prevotes"`
	Precommits      VoteTally `json:"precommits"`
	Outcome         string    `json:"outcome"`                    // "commit", "timeout", "wal_end"
	TimeoutDuration string    `json:"timeout_duration,omitempty"` // e.g. "1s"
}

// VoteTally counts votes by block hash, with per-validator breakdown.
type VoteTally struct {
	Block     map[string]int      `json:"block,omitempty"`  // hash -> count
	Voters    map[string][]string `json:"voters,omitempty"` // hash -> list of validator addresses
	Nil       int                 `json:"nil"`
	NilVoters []string            `json:"nil_voters,omitempty"` // validators who voted nil
	Total     int                 `json:"total"`
}

// ---- Vote info extraction (shared between summary and filter)

// voteInfo holds the parsed fields from a WAL vote message.
type voteInfo struct {
	voteType int64
	height   int64
	round    int
	hash     string
	addr     string
}

// extractVoteInfo parses vote fields from the inner MsgInfo JSON.
// Returns nil if the message is not a vote.
func extractVoteInfo(innerJSON string) *voteInfo {
	innerType := gjson.Get(innerJSON, "@type").String()
	if !isVoteMsg(innerType) {
		return nil
	}
	return &voteInfo{
		voteType: gjson.Get(innerJSON, "Vote.type").Int(),
		height:   gjson.Get(innerJSON, "Vote.height").Int(),
		round:    int(gjson.Get(innerJSON, "Vote.round").Int()),
		hash:     gjson.Get(innerJSON, "Vote.block_id.hash").String(),
		addr:     gjson.Get(innerJSON, "Vote.validator_address").String(),
	}
}

// voteDedupKey returns a unique key for deduplication: "voteType:round:addr".
func voteDedupKey(voteType int64, round int, addr string) string {
	return fmt.Sprintf("%d:%d:%s", voteType, round, addr)
}

// isStaleVote returns true if the vote's height doesn't match the target.
func isStaleVote(vi *voteInfo, targetHeight int64) bool {
	return targetHeight > 0 && vi.height != 0 && vi.height != targetHeight
}

// ---- Summary

// SummarizeWAL aggregates raw WAL messages into a per-round digest.
// Deduplicates votes by validator and filters out stale votes from other heights.
func SummarizeWAL(height int64, msgs []WALMessage) *WALSummary {
	rounds := make(map[int]*RoundSummary)
	var roundOrder []int

	getOrCreate := func(r int) *RoundSummary {
		if rs, ok := rounds[r]; ok {
			return rs
		}
		rs := &RoundSummary{
			Round: r,
			Prevotes: VoteTally{
				Block:  make(map[string]int),
				Voters: make(map[string][]string),
			},
			Precommits: VoteTally{
				Block:  make(map[string]int),
				Voters: make(map[string][]string),
			},
			Outcome: "wal_end",
		}
		rounds[r] = rs
		roundOrder = append(roundOrder, r)
		return rs
	}

	seen := make(map[string]bool)

	for _, m := range msgs {
		data := string(m.Data)

		if isTimeoutMsg(m.Type) {
			round := int(gjson.Get(data, "round").Int())
			rs := getOrCreate(round)
			rs.Outcome = "timeout"
			if dur := gjson.Get(data, "duration"); dur.Exists() {
				rs.TimeoutDuration = formatDuration(dur.Int())
			}
			continue
		}

		innerMsg := gjson.Get(data, "msg")
		if !innerMsg.Exists() {
			continue
		}
		innerStr := innerMsg.Raw
		innerType := gjson.Get(innerStr, "@type").String()

		switch {
		case isProposalMsg(innerType):
			round := int(gjson.Get(innerStr, "Proposal.round").Int())
			hash := gjson.Get(innerStr, "Proposal.block_id.hash").String()
			rs := getOrCreate(round)
			rs.BlockHash = hash

		case isVoteMsg(innerType):
			vi := extractVoteInfo(innerStr)
			if vi == nil {
				continue
			}
			if isStaleVote(vi, height) {
				continue
			}

			key := voteDedupKey(vi.voteType, vi.round, vi.addr)
			if seen[key] {
				continue
			}
			seen[key] = true

			rs := getOrCreate(vi.round)
			tally := &rs.Prevotes
			if vi.voteType == voteTypePrecommit {
				tally = &rs.Precommits
			}

			tally.Total++
			if vi.hash == "" {
				tally.Nil++
				if vi.addr != "" {
					tally.NilVoters = append(tally.NilVoters, vi.addr)
				}
			} else {
				tally.Block[vi.hash]++
				if vi.addr != "" {
					tally.Voters[vi.hash] = append(tally.Voters[vi.hash], vi.addr)
				}
			}
		}
	}

	result := &WALSummary{Height: height, Rounds: make([]RoundSummary, 0, len(roundOrder))}
	for _, r := range roundOrder {
		result.Rounds = append(result.Rounds, *rounds[r])
	}
	return result
}

// ---- Type detection (case-insensitive)

func isTimeoutMsg(typeURL string) bool {
	return strings.Contains(strings.ToLower(typeURL), "timeout")
}

func isProposalMsg(innerType string) bool {
	return strings.Contains(strings.ToLower(innerType), "proposal")
}

func isVoteMsg(innerType string) bool {
	return strings.Contains(strings.ToLower(innerType), "vote")
}

func formatDuration(nanos int64) string {
	return time.Duration(nanos).String()
}

// ---- Filtering

// FilterWALMessages returns the subset of msgs matching the given round
// and/or message type filter. Deduplicates votes by validator and filters
// out stale votes from previous heights that leak through gossip.
func FilterWALMessages(msgs []WALMessage, round *int, msgType string, height int64) []WALMessage {
	var out []WALMessage
	seen := make(map[string]bool)

	for _, m := range msgs {
		data := string(m.Data)

		if round != nil && extractRound(m.Type, data) != *round {
			continue
		}
		if msgType != "" && !matchesMsgType(m.Type, data, msgType) {
			continue
		}

		// For vote messages: dedup by validator and filter stale heights
		inner := gjson.Get(data, "msg")
		if inner.Exists() {
			vi := extractVoteInfo(inner.Raw)
			if vi != nil {
				if isStaleVote(vi, height) {
					continue
				}
				key := voteDedupKey(vi.voteType, int(vi.height), vi.addr)
				if seen[key] {
					continue
				}
				seen[key] = true
			}
		}

		out = append(out, m)
	}
	return out
}

// extractRound returns the consensus round from a WAL message.
func extractRound(typeURL, data string) int {
	if isTimeoutMsg(typeURL) {
		return int(gjson.Get(data, "round").Int())
	}

	innerMsg := gjson.Get(data, "msg")
	if !innerMsg.Exists() {
		return -1
	}
	innerStr := innerMsg.Raw

	innerType := gjson.Get(innerStr, "@type").String()
	switch {
	case isProposalMsg(innerType):
		return int(gjson.Get(innerStr, "Proposal.round").Int())
	case isVoteMsg(innerType):
		return int(gjson.Get(innerStr, "Vote.round").Int())
	}
	return -1
}

// matchesMsgType checks if a WAL message matches the requested type filter.
func matchesMsgType(typeURL, data, filter string) bool {
	switch strings.ToLower(filter) {
	case "timeout":
		return isTimeoutMsg(typeURL)
	case "proposal":
		innerType := gjson.Get(data, "msg.@type").String()
		return isProposalMsg(innerType)
	case "prevote":
		innerType := gjson.Get(data, "msg.@type").String()
		if !isVoteMsg(innerType) {
			return false
		}
		return gjson.Get(data, "msg.Vote.type").Int() == voteTypePrevote
	case "precommit":
		innerType := gjson.Get(data, "msg.@type").String()
		if !isVoteMsg(innerType) {
			return false
		}
		return gjson.Get(data, "msg.Vote.type").Int() == voteTypePrecommit
	}
	return false
}

// ---- Raw message slimming

// slimWALMessages strips heavy fields (signatures, block part bytes, proofs)
// from WAL messages to keep raw mode output within context limits.
func slimWALMessages(msgs []WALMessage) []WALMessage {
	if len(msgs) == 0 {
		return nil
	}
	out := make([]WALMessage, len(msgs))
	for i, m := range msgs {
		out[i] = WALMessage{
			Time: m.Time,
			Type: m.Type,
			Data: slimMessageData(m.Type, m.Data),
		}
	}
	return out
}

func slimMessageData(typeURL string, data json.RawMessage) json.RawMessage {
	raw := string(data)

	if isTimeoutMsg(typeURL) {
		return data // already small
	}

	inner := gjson.Get(raw, "msg")
	if !inner.Exists() {
		return data
	}

	innerType := gjson.Get(inner.Raw, "@type").String()

	switch {
	case isVoteMsg(innerType):
		return slimVote(inner.Raw)
	case isProposalMsg(innerType):
		return slimProposal(inner.Raw)
	default:
		return data
	}
}

// ---- Slim data types (typed structs ensure json.Marshal cannot fail)

type slimVoteData struct {
	Type             int64  `json:"type"`
	Round            int64  `json:"round"`
	BlockHash        string `json:"block_hash"`
	ValidatorAddress string `json:"validator_address"`
	ValidatorIndex   int64  `json:"validator_index"`
	Timestamp        string `json:"timestamp"`
}

type slimProposalWrapper struct {
	Proposal slimProposalData `json:"proposal"`
}

type slimProposalData struct {
	Round     int64  `json:"round"`
	BlockHash string `json:"block_hash"`
	POLRound  int64  `json:"pol_round"`
	Timestamp string `json:"timestamp"`
}

func slimVote(innerJSON string) json.RawMessage {
	v := gjson.Get(innerJSON, "Vote")
	b, _ := json.Marshal(slimVoteData{
		Type:             v.Get("type").Int(),
		Round:            v.Get("round").Int(),
		BlockHash:        v.Get("block_id.hash").String(),
		ValidatorAddress: v.Get("validator_address").String(),
		ValidatorIndex:   v.Get("validator_index").Int(),
		Timestamp:        v.Get("timestamp").String(),
	})
	return json.RawMessage(b)
}

func slimProposal(innerJSON string) json.RawMessage {
	p := gjson.Get(innerJSON, "Proposal")
	b, _ := json.Marshal(slimProposalWrapper{Proposal: slimProposalData{
		Round:     p.Get("round").Int(),
		BlockHash: p.Get("block_id.hash").String(),
		POLRound:  p.Get("pol_round").Int(),
		Timestamp: p.Get("timestamp").String(),
	}})
	return json.RawMessage(b)
}
