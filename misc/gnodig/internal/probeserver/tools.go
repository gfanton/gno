package probeserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gnolang/gno/misc/gnodig/internal/chainrpc"
	"github.com/gnolang/gno/misc/gnodig/internal/nodedata"
)

// ---- RPC Tools

// rpcBlockInspectParams holds parameters for the block_inspect tool.
type rpcBlockInspectParams struct {
	Height int64 `json:"height"`
}

// RegisterRPCTools registers live-node tools (node_overview, block_inspect)
// onto the server. The client points at the fixed RPC endpoint configured at
// startup — request params cannot override the target (prevents SSRF).
func RegisterRPCTools(srv *Server, client *chainrpc.Client) {
	srv.HandleTool("node_overview", func(ctx context.Context, _ json.RawMessage) (json.RawMessage, error) {
		ov := chainrpc.FetchOverview(ctx, client)
		return json.Marshal(ov)
	})

	srv.HandleTool("block_inspect", func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
		var p rpcBlockInspectParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("block_inspect: invalid params: %w", err)
		}
		bi := chainrpc.FetchBlockInspect(ctx, client, p.Height)
		return json.Marshal(bi)
	})
}

// ---- NodeData Tools

// nodeDataBlockParams holds parameters for the node_data_block tool.
type nodeDataBlockParams struct {
	Height int64 `json:"height"`
}

// nodeDataWALParams holds parameters for the node_data_wal tool.
type nodeDataWALParams struct {
	Height int64  `json:"height"`
	Mode   string `json:"mode"`
	Round  *int   `json:"round"`
	Type   string `json:"type"`
	Limit  int    `json:"limit"`
}

// RegisterNodeDataTools registers offline data-directory tools
// (node_data_open, node_data_block, node_data_wal) onto the server.
// dd must be a valid, open DataDir.
func RegisterNodeDataTools(srv *Server, dd *nodedata.DataDir) {
	srv.HandleTool("node_data_open", func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
		ov, err := dd.Overview()
		if err != nil {
			return nil, fmt.Errorf("node_data_open: %w", err)
		}
		return json.Marshal(ov)
	})

	srv.HandleTool("node_data_block", func(_ context.Context, params json.RawMessage) (json.RawMessage, error) {
		var p nodeDataBlockParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("node_data_block: invalid params: %w", err)
		}
		height := p.Height
		if height == 0 {
			height = dd.BlockStore().Height()
		}
		detail, err := dd.Block(height)
		if err != nil {
			return nil, fmt.Errorf("node_data_block: %w", err)
		}
		return json.Marshal(detail)
	})

	srv.HandleTool("node_data_wal", func(_ context.Context, params json.RawMessage) (json.RawMessage, error) {
		var p nodeDataWALParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("node_data_wal: invalid params: %w", err)
		}

		switch p.Mode {
		case "raw":
			if p.Round == nil && p.Type == "" {
				return nil, fmt.Errorf("node_data_wal: raw mode requires at least one of round or type filter")
			}
			limit := p.Limit
			if limit <= 0 {
				limit = 50
			}
			detail, err := dd.WALFiltered(p.Height, p.Round, p.Type, limit)
			if err != nil {
				return nil, fmt.Errorf("node_data_wal: %w", err)
			}
			return json.Marshal(detail)

		case "summary", "":
			summary, err := dd.WALSummary(p.Height)
			if err != nil {
				return nil, fmt.Errorf("node_data_wal: %w", err)
			}
			return json.Marshal(summary)

		default:
			return nil, fmt.Errorf("node_data_wal: unknown mode %q: use summary or raw", p.Mode)
		}
	})
}
