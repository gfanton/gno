package chainrpc

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tidwall/gjson"
)

const (
	defaultTimeout   = 30 * time.Second
	maxResponseBytes = 64 << 20 // 64 MB
)

// Client is a lightweight TM2 RPC HTTP client that returns raw gjson results.
type Client struct {
	rpcURL    string
	http      *http.Client
	genesisMu sync.Mutex // serializes genesis downloads
}

// New creates a new Client targeting the given TM2 RPC base URL (e.g. "http://localhost:26657").
func New(rpcURL string) *Client {
	return &Client{
		rpcURL: strings.TrimRight(rpcURL, "/"),
		http:   &http.Client{Timeout: defaultTimeout},
	}
}

// ---- request helpers

func (c *Client) get(ctx context.Context, path string, params url.Values) (gjson.Result, error) {
	u := c.rpcURL + "/" + strings.TrimLeft(path, "/")
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return gjson.Result{}, fmt.Errorf("rpc %s: %w", path, err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return gjson.Result{}, fmt.Errorf("rpc %s: %w", path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return gjson.Result{}, fmt.Errorf("rpc %s: read body: %w", path, err)
	}

	// Surface any JSON-RPC error returned in the response body.
	parsed := gjson.ParseBytes(raw)
	if errMsg := parsed.Get("error"); errMsg.Exists() && errMsg.Raw != "null" && errMsg.Raw != "" {
		return gjson.Result{}, fmt.Errorf("rpc %s: %s", path, errMsg.String())
	}

	return parsed.Get("result"), nil
}

// ---- public methods

// Status returns the node status (sync info, node info, validator info).
func (c *Client) Status(ctx context.Context) (gjson.Result, error) {
	return c.get(ctx, "status", nil)
}

// Block returns the block at the given height. Pass height=0 for latest.
func (c *Client) Block(ctx context.Context, height int64) (gjson.Result, error) {
	p := url.Values{}
	if height > 0 {
		p.Set("height", strconv.FormatInt(height, 10))
	}
	return c.get(ctx, "block", p)
}

// BlockResults returns the ABCI results for the block at the given height.
func (c *Client) BlockResults(ctx context.Context, height int64) (gjson.Result, error) {
	p := url.Values{}
	if height > 0 {
		p.Set("height", strconv.FormatInt(height, 10))
	}
	return c.get(ctx, "block_results", p)
}

// BlockRange returns block headers for minHeight..maxHeight (blockchain endpoint).
func (c *Client) BlockRange(ctx context.Context, minHeight, maxHeight int64) (gjson.Result, error) {
	p := url.Values{}
	p.Set("minHeight", strconv.FormatInt(minHeight, 10))
	p.Set("maxHeight", strconv.FormatInt(maxHeight, 10))
	return c.get(ctx, "blockchain", p)
}

// Tx returns a transaction by its hash (hex-encoded).
func (c *Client) Tx(ctx context.Context, hash []byte) (gjson.Result, error) {
	p := url.Values{}
	p.Set("hash", "0x"+hex.EncodeToString(hash))
	return c.get(ctx, "tx", p)
}

// Validators returns the validator set at the given height. Pass height=0 for latest.
func (c *Client) Validators(ctx context.Context, height int64) (gjson.Result, error) {
	p := url.Values{}
	if height > 0 {
		p.Set("height", strconv.FormatInt(height, 10))
	}
	return c.get(ctx, "validators", p)
}

// ConsensusState returns the current consensus state.
func (c *Client) ConsensusState(ctx context.Context) (gjson.Result, error) {
	return c.get(ctx, "consensus_state", nil)
}

// DumpConsensusState returns a detailed dump of the consensus state.
func (c *Client) DumpConsensusState(ctx context.Context) (gjson.Result, error) {
	return c.get(ctx, "dump_consensus_state", nil)
}

// NetInfo returns network information (peers, etc.).
func (c *Client) NetInfo(ctx context.Context) (gjson.Result, error) {
	return c.get(ctx, "net_info", nil)
}

// Health returns the node health status.
func (c *Client) Health(ctx context.Context) (gjson.Result, error) {
	return c.get(ctx, "health", nil)
}

// UnconfirmedTxs returns unconfirmed transactions. Pass limit=0 for the server default.
func (c *Client) UnconfirmedTxs(ctx context.Context, limit int) (gjson.Result, error) {
	p := url.Values{}
	if limit > 0 {
		p.Set("limit", strconv.Itoa(limit))
	}
	return c.get(ctx, "unconfirmed_txs", p)
}

// NumUnconfirmedTxs returns the count of unconfirmed transactions.
func (c *Client) NumUnconfirmedTxs(ctx context.Context) (gjson.Result, error) {
	return c.get(ctx, "num_unconfirmed_txs", nil)
}

// ABCIQuery performs an ABCI query. Pass height=0 for latest.
func (c *Client) ABCIQuery(ctx context.Context, path string, data []byte, height int64, prove bool) (gjson.Result, error) {
	p := url.Values{}
	p.Set("path", strconv.Quote(path))
	if len(data) > 0 {
		p.Set("data", "0x"+hex.EncodeToString(data))
	}
	if height > 0 {
		p.Set("height", strconv.FormatInt(height, 10))
	}
	if prove {
		p.Set("prove", "true")
	}
	return c.get(ctx, "abci_query", p)
}

// Genesis returns the genesis document.
func (c *Client) Genesis(ctx context.Context) (gjson.Result, error) {
	return c.get(ctx, "genesis", nil)
}

// ConsensusParams returns the consensus parameters at the given height. Pass height=0 for latest.
func (c *Client) ConsensusParams(ctx context.Context, height int64) (gjson.Result, error) {
	p := url.Values{}
	if height > 0 {
		p.Set("height", strconv.FormatInt(height, 10))
	}
	return c.get(ctx, "consensus_params", p)
}
