package doctor

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

const (
	haltedThreshold         = 5 * time.Minute
	consensusStuckThreshold = 3
	minPeers                = 3
)

func checkChainHalted(ctx *Context) ([]Finding, error) {
	ov, err := ctx.status.Get()
	if err != nil {
		return nil, err
	}

	status := gjson.GetBytes(ov.Status, "sync_info")
	if !status.Exists() {
		return nil, nil
	}

	if status.Get("catching_up").Bool() {
		return nil, nil
	}

	blockTimeStr := status.Get("latest_block_time").String()
	blockTime, err := time.Parse(time.RFC3339Nano, blockTimeStr)
	if err != nil {
		return nil, fmt.Errorf("parse block time %q: %w", blockTimeStr, err)
	}

	age := time.Since(blockTime)
	if age <= haltedThreshold {
		return nil, nil
	}

	height := status.Get("latest_block_height").String()
	return []Finding{{
		ID:       "chain_halted",
		Severity: Critical,
		Detail:   fmt.Sprintf("No new blocks for %s (height %s, last block %s)", formatDuration(age), height, blockTimeStr),
		Source:   "rpc",
	}}, nil
}

func checkConsensusStuck(ctx *Context) ([]Finding, error) {
	ov, err := ctx.status.Get()
	if err != nil {
		return nil, err
	}

	hrs := gjson.GetBytes(ov.ConsensusState, "round_state.height/round/step").String()
	if hrs == "" {
		return nil, nil
	}

	parts := strings.SplitN(hrs, "/", 3)
	if len(parts) != 3 {
		return nil, nil
	}

	round, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("parse consensus round %q: %w", parts[1], err)
	}

	if round < consensusStuckThreshold {
		return nil, nil
	}

	return []Finding{{
		ID:       "consensus_stuck",
		Severity: Critical,
		Detail:   fmt.Sprintf("Consensus at height %s round %d step %s — not advancing", parts[0], round, parts[2]),
		Source:   "rpc",
	}}, nil
}

func checkLowPeers(ctx *Context) ([]Finding, error) {
	ov, err := ctx.status.Get()
	if err != nil {
		return nil, err
	}
	peersStr := gjson.GetBytes(ov.NetInfo, "n_peers").String()
	if peersStr == "" {
		return nil, nil
	}
	peers, err := strconv.Atoi(peersStr)
	if err != nil {
		return nil, fmt.Errorf("parse peer count %q: %w", peersStr, err)
	}
	if peers >= minPeers {
		return nil, nil
	}
	return []Finding{{
		ID:       "low_peers",
		Severity: Warning,
		Detail:   fmt.Sprintf("Only %d peers connected (expected >= %d)", peers, minPeers),
		Source:   "rpc",
	}}, nil
}

func checkMempoolBacklog(ctx *Context) ([]Finding, error) {
	ov, err := ctx.status.Get()
	if err != nil {
		return nil, err
	}
	txsStr := gjson.GetBytes(ov.UnconfirmedCount, "n_txs").String()
	if txsStr == "" || txsStr == "0" {
		return nil, nil
	}
	txs, err := strconv.Atoi(txsStr)
	if err != nil {
		return nil, fmt.Errorf("parse unconfirmed tx count %q: %w", txsStr, err)
	}
	if txs == 0 {
		return nil, nil
	}
	// Only flag if chain is also halted.
	blockTimeStr := gjson.GetBytes(ov.Status, "sync_info.latest_block_time").String()
	blockTime, err := time.Parse(time.RFC3339Nano, blockTimeStr)
	if err != nil {
		return nil, fmt.Errorf("parse block time %q: %w", blockTimeStr, err)
	}
	if time.Since(blockTime) <= haltedThreshold {
		return nil, nil
	}
	return []Finding{{
		ID:       "mempool_backlog",
		Severity: Info,
		Detail:   fmt.Sprintf("%d unconfirmed txs queued while chain is halted", txs),
		Source:   "rpc",
	}}, nil
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	hours := int(d.Hours())
	if hours < 24 {
		return fmt.Sprintf("%dh%dm", hours, int(d.Minutes())%60)
	}
	days := hours / 24
	return fmt.Sprintf("%dd%dh", days, hours%24)
}
