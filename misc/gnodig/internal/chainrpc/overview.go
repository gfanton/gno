package chainrpc

import (
	"context"
	"encoding/json"

	"github.com/tidwall/gjson"
)

// Overview aggregates results from several node RPC calls.
type Overview struct {
	Status           json.RawMessage `json:"status,omitempty"`
	StatusError      string          `json:"status_error,omitempty"`
	NetInfo          json.RawMessage `json:"net_info,omitempty"`
	NetInfoError     string          `json:"net_info_error,omitempty"`
	ConsensusState   json.RawMessage `json:"consensus_state,omitempty"`
	ConsensusError   string          `json:"consensus_state_error,omitempty"`
	UnconfirmedCount json.RawMessage `json:"num_unconfirmed_txs,omitempty"`
	UnconfirmedError string          `json:"num_unconfirmed_txs_error,omitempty"`
}

// FetchOverview calls status, net_info, consensus_state, and
// num_unconfirmed_txs, collecting partial errors into the result fields.
func FetchOverview(ctx context.Context, c *Client) *Overview {
	out := &Overview{}

	if status, err := c.Status(ctx); err != nil {
		out.StatusError = err.Error()
	} else {
		out.Status = json.RawMessage(status.Raw)
	}

	if net, err := c.NetInfo(ctx); err != nil {
		out.NetInfoError = err.Error()
	} else {
		out.NetInfo = json.RawMessage(net.Raw)
	}

	if cs, err := c.ConsensusState(ctx); err != nil {
		out.ConsensusError = err.Error()
	} else {
		out.ConsensusState = json.RawMessage(cs.Raw)
	}

	if unconf, err := c.NumUnconfirmedTxs(ctx); err != nil {
		out.UnconfirmedError = err.Error()
	} else {
		out.UnconfirmedCount = json.RawMessage(unconf.Raw)
	}

	return out
}

// BlockInspect aggregates block, block_results, and validators for a height.
type BlockInspect struct {
	Block           json.RawMessage `json:"block,omitempty"`
	BlockError      string          `json:"block_error,omitempty"`
	BlockResults    json.RawMessage `json:"block_results,omitempty"`
	BlockResultsErr string          `json:"block_results_error,omitempty"`
	Validators      json.RawMessage `json:"validators,omitempty"`
	ValidatorsError string          `json:"validators_error,omitempty"`
}

// FetchBlockInspect loads block, block_results, and validators for height.
// Height 0 means latest; the actual height is resolved from the block response.
func FetchBlockInspect(ctx context.Context, c *Client, height int64) *BlockInspect {
	out := &BlockInspect{}

	block, blockErr := c.Block(ctx, height)
	if blockErr != nil {
		out.BlockError = blockErr.Error()
	} else {
		out.Block = json.RawMessage(block.Raw)
		if height == 0 {
			height = resolveHeight(block)
		}
	}

	if br, err := c.BlockResults(ctx, height); err != nil {
		out.BlockResultsErr = err.Error()
	} else {
		out.BlockResults = json.RawMessage(br.Raw)
	}

	if v, err := c.Validators(ctx, height); err != nil {
		out.ValidatorsError = err.Error()
	} else {
		out.Validators = json.RawMessage(v.Raw)
	}

	return out
}

// resolveHeight extracts the block height from a block response.
func resolveHeight(block gjson.Result) int64 {
	if h := block.Get("block_meta.header.height"); h.Exists() {
		return h.Int()
	}
	if h := block.Get("block.header.height"); h.Exists() {
		return h.Int()
	}
	return 0
}
