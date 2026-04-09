package mcp

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gnolang/gno/misc/gnodig/internal/chainrpc"
)

// ---- Input Structs

type rpcTargetInput struct {
	Target string `json:"target" jsonschema:"RPC endpoint URL (e.g. http://node:26657),required"`
}

type blockInspectInput struct {
	Target string `json:"target" jsonschema:"RPC endpoint URL (e.g. http://node:26657),required"`
	Height int64  `json:"height,omitempty" jsonschema:"Block height (0 = latest)"`
}

type chainQueryInput struct {
	Target string `json:"target" jsonschema:"RPC endpoint URL (e.g. http://node:26657),required"`
	Method string `json:"method" jsonschema:"RPC method: tx, genesis, abci_query, blockchain, dump_consensus_state,required,enum=tx,enum=genesis,enum=abci_query,enum=blockchain,enum=dump_consensus_state"`
	Params any    `json:"params,omitempty" jsonschema:"Method-specific parameters"`
}

// ---- Registration

func registerRPCTools(srv *sdkmcp.Server, clients *chainClients) {
	// ---- node_overview
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "node_overview",
		Description: desc("node_overview"),
		Annotations: readOnlyAnnotation,
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in rpcTargetInput) (*sdkmcp.CallToolResult, any, error) {
		c, err := clients.get(in.Target)
		if err != nil {
			return nil, nil, err
		}

		return textResult(chainrpc.FetchOverview(ctx, c))
	})

	// ---- block_inspect
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "block_inspect",
		Description: desc("block_inspect"),
		Annotations: readOnlyAnnotation,
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in blockInspectInput) (*sdkmcp.CallToolResult, any, error) {
		c, err := clients.get(in.Target)
		if err != nil {
			return nil, nil, err
		}

		return textResult(chainrpc.FetchBlockInspect(ctx, c, in.Height))
	})

	// ---- chain_query
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "chain_query",
		Description: desc("chain_query"),
		Annotations: readOnlyAnnotation,
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in chainQueryInput) (*sdkmcp.CallToolResult, any, error) {
		c, err := clients.get(in.Target)
		if err != nil {
			return nil, nil, err
		}

		// Marshal params to a generic map for field extraction.
		params, err := toStringMap(in.Params)
		if err != nil {
			return nil, nil, fmt.Errorf("chain_query: invalid params: %w", err)
		}

		switch in.Method {
		case "tx":
			hashStr := strings.TrimPrefix(params["hash"], "0x")
			hashBytes, err := hex.DecodeString(hashStr)
			if err != nil {
				return nil, nil, fmt.Errorf("chain_query tx: invalid hash %q: %w", params["hash"], err)
			}
			result, err := c.Tx(ctx, hashBytes)
			if err != nil {
				return nil, nil, fmt.Errorf("chain_query tx: %w", err)
			}
			return rawResult(result.Raw)

		case "genesis":
			result, err := c.Genesis(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("chain_query genesis: %w", err)
			}
			return rawResult(result.Raw)

		case "abci_query":
			path := params["path"]
			var data []byte
			if d := params["data"]; d != "" {
				data, err = hex.DecodeString(strings.TrimPrefix(d, "0x"))
				if err != nil {
					return nil, nil, fmt.Errorf("chain_query abci_query: invalid data %q: %w", d, err)
				}
			}
			var height int64
			if h := params["height"]; h != "" {
				v, err := strconv.ParseInt(h, 10, 64)
				if err != nil {
					return nil, nil, fmt.Errorf("chain_query abci_query: invalid height %q: %w", h, err)
				}
				height = v
			}
			result, err := c.ABCIQuery(ctx, path, data, height, false)
			if err != nil {
				return nil, nil, fmt.Errorf("chain_query abci_query: %w", err)
			}
			return rawResult(result.Raw)

		case "blockchain":
			var minHeight, maxHeight int64
			if h := params["min_height"]; h != "" {
				v, err := strconv.ParseInt(h, 10, 64)
				if err != nil {
					return nil, nil, fmt.Errorf("chain_query blockchain: invalid min_height %q: %w", h, err)
				}
				minHeight = v
			}
			if h := params["max_height"]; h != "" {
				v, err := strconv.ParseInt(h, 10, 64)
				if err != nil {
					return nil, nil, fmt.Errorf("chain_query blockchain: invalid max_height %q: %w", h, err)
				}
				maxHeight = v
			}
			result, err := c.BlockRange(ctx, minHeight, maxHeight)
			if err != nil {
				return nil, nil, fmt.Errorf("chain_query blockchain: %w", err)
			}
			return rawResult(result.Raw)

		case "dump_consensus_state":
			result, err := c.DumpConsensusState(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("chain_query dump_consensus_state: %w", err)
			}
			return rawResult(result.Raw)

		default:
			return nil, nil, fmt.Errorf("chain_query: unsupported method %q (use tx, genesis, abci_query, blockchain, or dump_consensus_state)", in.Method)
		}
	})

	// ---- peer_consensus
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "peer_consensus",
		Description: desc("peer_consensus"),
		Annotations: readOnlyAnnotation,
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in rpcTargetInput) (*sdkmcp.CallToolResult, any, error) {
		c, err := clients.get(in.Target)
		if err != nil {
			return nil, nil, err
		}

		result, err := chainrpc.FetchPeerConsensus(ctx, c)
		if err != nil {
			return nil, nil, fmt.Errorf("peer_consensus: %w", err)
		}
		return textResult(result)
	})
}

// toStringMap converts an arbitrary params value to map[string]string.
func toStringMap(v any) (map[string]string, error) {
	if v == nil {
		return map[string]string{}, nil
	}

	// If it's already the right type (unlikely from JSON, but defensive).
	if m, ok := v.(map[string]string); ok {
		return m, nil
	}

	// JSON deserialization typically yields map[string]any.
	if m, ok := v.(map[string]any); ok {
		out := make(map[string]string, len(m))
		for k, val := range m {
			out[k] = fmt.Sprint(val)
		}
		return out, nil
	}

	// Fall back: re-marshal then unmarshal.
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out map[string]string
	if err := json.Unmarshal(data, &out); err != nil {
		// Try map[string]any as fallback.
		var m map[string]any
		if err2 := json.Unmarshal(data, &m); err2 != nil {
			return nil, err
		}
		out = make(map[string]string, len(m))
		for k, val := range m {
			out[k] = fmt.Sprint(val)
		}
	}
	return out, nil
}
