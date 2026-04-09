package chainrpc

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

// ---- Types

// PeerConsensusResult holds the parsed dump_consensus_state with peer details.
type PeerConsensusResult struct {
	Local   LocalState       `json:"local"`
	Peers   []PeerState      `json:"peers"`
	Summary ConsensusSummary `json:"summary"`
}

// LocalState holds the local node's consensus round state.
type LocalState struct {
	Height            int64  `json:"height"`
	Round             int    `json:"round"`
	Step              string `json:"step"`
	ProposalBlockHash string `json:"proposal_block_hash,omitempty"`
	LockedBlockHash   string `json:"locked_block_hash,omitempty"`
	ValidBlockHash    string `json:"valid_block_hash,omitempty"`
}

// VoteInfo holds expanded bitarray information for prevotes/precommits.
type VoteInfo struct {
	BitArray string   `json:"bitarray"`
	Count    int      `json:"count"`
	Total    int      `json:"total"`
	Indices  []int    `json:"indices"`
	Voters   []string `json:"voters"`
}

// PeerState holds a single peer's consensus state.
type PeerState struct {
	Moniker        string   `json:"moniker"`
	Address        string   `json:"address"`
	RemoteIP       string   `json:"remote_ip"`
	Height         int64    `json:"height"`
	Round          int      `json:"round"`
	Step           int      `json:"step"`
	HasProposal    bool     `json:"has_proposal"`
	Prevotes       VoteInfo `json:"prevotes"`
	Precommits     VoteInfo `json:"precommits"`
	LastCommit     VoteInfo `json:"last_commit"`
	ValidatorIndex *int     `json:"validator_index,omitempty"`
}

// ConsensusSummary holds aggregate peer height stats.
type ConsensusSummary struct {
	TotalPeers        int            `json:"total_peers"`
	PeersAtSameHeight int            `json:"peers_at_same_height"`
	PeersBehind       int            `json:"peers_behind"`
	Heights           map[string]int `json:"heights"`
}

// ---- BitArray Parsing

// bitArrayRe matches "BA{N:bits}" where N is total and bits is x/_ characters.
var bitArrayRe = regexp.MustCompile(`^BA\{(\d+):([x_]*)\}$`)

// parseBitArray parses a TM2 bitarray string like "BA{7:x_x__x_}".
// Returns a VoteInfo with count, total, and indices of set bits.
// Handles empty, "null", and "nil-BitArray" as zero-value results.
func parseBitArray(s string) VoteInfo {
	info := VoteInfo{
		BitArray: s,
		Indices:  []int{},
		Voters:   []string{},
	}

	m := bitArrayRe.FindStringSubmatch(s)
	if m == nil {
		return info
	}

	total, _ := strconv.Atoi(m[1]) // safe: regex guarantees \d+
	bits := m[2]
	info.Total = total

	for i, ch := range bits {
		if ch == 'x' {
			info.Count++
			info.Indices = append(info.Indices, i)
		}
	}

	return info
}

// ---- Node Address Parsing

// parseNodeAddress extracts the peer ID and IP from "id@ip:port".
func parseNodeAddress(addr string) (id, ip string) {
	id, hostPort, ok := strings.Cut(addr, "@")
	if !ok {
		return addr, ""
	}
	colonIdx := strings.LastIndex(hostPort, ":")
	if colonIdx < 0 {
		return id, hostPort
	}
	return id, hostPort[:colonIdx]
}

// ---- Local State Parsing

// parseLocalState extracts height/round/step and block hashes from the round_state object.
func parseLocalState(rs gjson.Result) LocalState {
	local := LocalState{
		ProposalBlockHash: rs.Get("proposal_block_hash").String(),
		LockedBlockHash:   rs.Get("locked_block_hash").String(),
		ValidBlockHash:    rs.Get("valid_block_hash").String(),
	}

	hrs := rs.Get("height/round/step").String()
	parts := strings.SplitN(hrs, "/", 3)
	if len(parts) == 3 {
		// Height/round may be non-numeric in malformed RPC responses;
		// zero is an acceptable fallback for diagnostic display.
		local.Height, _ = strconv.ParseInt(parts[0], 10, 64)
		local.Round, _ = strconv.Atoi(parts[1])
		local.Step = parts[2]
	}

	return local
}

// ---- Peer State Parsing

// parsePeerState parses a single peer's state from the amino JSON string.
func parsePeerState(nodeAddr string, peerJSON gjson.Result, monikers map[string]monikerInfo) PeerState {
	id, ip := parseNodeAddress(nodeAddr)

	ps := PeerState{
		Address:  id,
		RemoteIP: ip,
	}

	// Apply moniker/remote_ip from net_info if available
	if info, ok := monikers[id]; ok {
		ps.Moniker = info.moniker
		if info.remoteIP != "" {
			ps.RemoteIP = info.remoteIP
		}
	}

	rs := peerJSON.Get("round_state")
	ps.Height = rs.Get("height").Int()
	ps.Round = int(rs.Get("round").Int())
	ps.Step = int(rs.Get("step").Int())

	ps.HasProposal = rs.Get("proposal").Bool()
	ps.Prevotes = parseBitArray(rs.Get("prevotes").String())
	ps.Precommits = parseBitArray(rs.Get("precommits").String())
	ps.LastCommit = parseBitArray(rs.Get("last_commit").String())

	return ps
}

// ---- Summary

// buildSummary computes aggregate peer height statistics.
func buildSummary(localHeight int64, peers []PeerState) ConsensusSummary {
	s := ConsensusSummary{
		TotalPeers: len(peers),
		Heights:    make(map[string]int),
	}

	for _, p := range peers {
		hStr := strconv.FormatInt(p.Height, 10)
		s.Heights[hStr]++

		switch {
		case p.Height == localHeight:
			s.PeersAtSameHeight++
		case p.Height < localHeight:
			s.PeersBehind++
		}
	}

	return s
}

// ---- Moniker Lookup

type monikerInfo struct {
	moniker  string
	remoteIP string
}

// buildMonikerMap creates a peer ID → moniker+IP mapping from net_info.
func buildMonikerMap(netInfo gjson.Result) map[string]monikerInfo {
	m := make(map[string]monikerInfo)
	netInfo.Get("peers").ForEach(func(_, v gjson.Result) bool {
		netAddr := v.Get("node_info.net_address").String()
		id, _ := parseNodeAddress(netAddr)
		if id != "" {
			m[id] = monikerInfo{
				moniker:  v.Get("node_info.moniker").String(),
				remoteIP: v.Get("remote_ip").String(),
			}
		}
		return true
	})
	return m
}

// ---- Public API

// FetchPeerConsensus fetches and parses the consensus state with peer details.
// Calls dump_consensus_state for the raw dump and net_info for moniker mapping.
// net_info failure is non-fatal — peers will lack monikers.
func FetchPeerConsensus(ctx context.Context, c *Client) (*PeerConsensusResult, error) {
	dump, err := c.DumpConsensusState(ctx)
	if err != nil {
		return nil, fmt.Errorf("dump_consensus_state: %w", err)
	}

	// Best-effort moniker mapping from net_info
	var monikers map[string]monikerInfo
	if netInfo, err := c.NetInfo(ctx); err == nil {
		monikers = buildMonikerMap(netInfo)
	}
	if monikers == nil {
		monikers = make(map[string]monikerInfo)
	}

	local := parseLocalState(dump.Get("round_state"))

	var peers []PeerState
	dump.Get("peers").ForEach(func(_, v gjson.Result) bool {
		nodeAddr := v.Get("node_address").String()
		peerStateStr := v.Get("peer_state").String()
		peerJSON := gjson.Parse(peerStateStr)
		peers = append(peers, parsePeerState(nodeAddr, peerJSON, monikers))
		return true
	})

	if peers == nil {
		peers = []PeerState{}
	}

	return &PeerConsensusResult{
		Local:   local,
		Peers:   peers,
		Summary: buildSummary(local.Height, peers),
	}, nil
}
