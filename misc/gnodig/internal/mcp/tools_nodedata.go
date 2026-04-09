package mcp

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/sync/singleflight"

	"github.com/gnolang/gno/misc/gnodig/internal/nodedata"
)

// ---- DataDir Cache

type dataDirCache struct {
	mu    sync.Mutex
	dirs  map[string]*nodedata.DataDir
	group singleflight.Group
}

func newDataDirCache() *dataDirCache {
	return &dataDirCache{dirs: make(map[string]*nodedata.DataDir)}
}

func (c *dataDirCache) get(target string) (*nodedata.DataDir, error) {
	absPath, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	// Resolve symlinks to normalize the cache key and prevent path traversal.
	absPath, err = filepath.EvalSymlinks(absPath)
	if err != nil {
		return nil, fmt.Errorf("resolve path %q: %w", target, err)
	}

	v, err, _ := c.group.Do(absPath, func() (any, error) {
		c.mu.Lock()
		if dd, ok := c.dirs[absPath]; ok {
			c.mu.Unlock()
			return dd, nil
		}
		c.mu.Unlock()

		dd, err := nodedata.Open(absPath)
		if err != nil {
			return nil, err
		}

		c.mu.Lock()
		c.dirs[absPath] = dd
		c.mu.Unlock()
		return dd, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*nodedata.DataDir), nil
}

func (c *dataDirCache) closeAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, dd := range c.dirs {
		dd.Close()
	}
	clear(c.dirs)
}

// ---- Input Structs

type nodeDataTargetInput struct {
	Target string `json:"target" jsonschema:"Path to gnoland data directory,required"`
}

type nodeDataBlockInput struct {
	Target string `json:"target" jsonschema:"Path to gnoland data directory,required"`
	Height int64  `json:"height" jsonschema:"Block height. 0 = latest"`
}

type nodeDataWALInput struct {
	Target string `json:"target" jsonschema:"Path to gnoland data directory,required"`
	Height int64  `json:"height" jsonschema:"Block height to examine,required"`
	Mode   string `json:"mode"   jsonschema:"Output mode: summary (default) or raw"`
	Round  *int   `json:"round"  jsonschema:"Filter by consensus round number"`
	Type   string `json:"type"   jsonschema:"Filter by message type: proposal prevote precommit timeout"`
	Limit  int    `json:"limit"  jsonschema:"Max messages to return in raw mode (default 50)"`
}

type nodeDataStateInput struct {
	Target  string `json:"target"  jsonschema:"Path to gnoland data directory,required"`
	Height  int64  `json:"height"  jsonschema:"State version (block height). 0 = latest"`
	Path    string `json:"path,omitempty"    jsonschema:"Package/realm path (e.g. gno.land/r/demo/boards)"`
	Account string `json:"account,omitempty" jsonschema:"Account address (e.g. g1jg8...)"`
}

type nodeDataTxInput struct {
	Target string `json:"target" jsonschema:"Path to gnoland data directory,required"`
	Hash   string `json:"hash"   jsonschema:"Transaction hash (hex). Scans backward from tip — slower."`
	Height int64  `json:"height" jsonschema:"Block height (use with index)"`
	Index  int    `json:"index"  jsonschema:"Transaction index within the block (use with height)"`
}

type nodeCompareInput struct {
	Targets []string `json:"targets" jsonschema:"Paths to 2-5 gnoland data directories,required"`
	Height  int64    `json:"height"  jsonschema:"Block height to compare,required"`
}

// ---- Registration

func registerNodeDataTools(srv *sdkmcp.Server, ddCache *dataDirCache) {
	// ---- node_data_open
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "node_data_open",
		Description: desc("node_data_open"),
		Annotations: readOnlyAnnotation,
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in nodeDataTargetInput) (*sdkmcp.CallToolResult, any, error) {
		dd, err := ddCache.get(in.Target)
		if err != nil {
			return nil, nil, fmt.Errorf("open data dir: %w", err)
		}

		ov, err := dd.Overview()
		if err != nil {
			return nil, nil, fmt.Errorf("read overview: %w", err)
		}

		return textResult(ov)
	})

	// ---- node_data_block
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "node_data_block",
		Description: desc("node_data_block"),
		Annotations: readOnlyAnnotation,
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in nodeDataBlockInput) (*sdkmcp.CallToolResult, any, error) {
		dd, err := ddCache.get(in.Target)
		if err != nil {
			return nil, nil, fmt.Errorf("open data dir: %w", err)
		}

		height := in.Height
		if height == 0 {
			height = dd.BlockStore().Height()
		}

		detail, err := dd.Block(height)
		if err != nil {
			return nil, nil, err
		}

		return textResult(detail)
	})

	// ---- node_data_wal
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "node_data_wal",
		Description: desc("node_data_wal"),
		Annotations: readOnlyAnnotation,
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in nodeDataWALInput) (*sdkmcp.CallToolResult, any, error) {
		dd, err := ddCache.get(in.Target)
		if err != nil {
			return nil, nil, fmt.Errorf("open data dir: %w", err)
		}

		switch in.Mode {
		case "raw":
			if in.Round == nil && in.Type == "" {
				return nil, nil, fmt.Errorf("raw mode requires at least one of round or type filter")
			}
			limit := in.Limit
			if limit <= 0 {
				limit = 50
			}
			detail, err := dd.WALFiltered(in.Height, in.Round, in.Type, limit)
			if err != nil {
				return nil, nil, err
			}
			return textResult(detail)

		case "summary", "":
			summary, err := dd.WALSummary(in.Height)
			if err != nil {
				return nil, nil, err
			}
			return textResult(summary)

		default:
			return nil, nil, fmt.Errorf("unknown mode %q: use summary or raw", in.Mode)
		}
	})

	// ---- node_data_state
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "node_data_state",
		Description: desc("node_data_state"),
		Annotations: readOnlyAnnotation,
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in nodeDataStateInput) (*sdkmcp.CallToolResult, any, error) {
		hasPath := in.Path != ""
		hasAccount := in.Account != ""

		if hasPath == hasAccount {
			return nil, nil, fmt.Errorf("provide exactly one of path or account")
		}

		dd, err := ddCache.get(in.Target)
		if err != nil {
			return nil, nil, fmt.Errorf("open data dir: %w", err)
		}

		if hasAccount {
			result, err := dd.AccountQuery(in.Height, in.Account)
			if err != nil {
				return nil, nil, err
			}
			return textResult(result)
		}

		result, err := dd.PackageQuery(in.Height, in.Path)
		if err != nil {
			return nil, nil, err
		}
		return textResult(result)
	})

	// ---- node_data_tx
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "node_data_tx",
		Description: desc("node_data_tx"),
		Annotations: readOnlyAnnotation,
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in nodeDataTxInput) (*sdkmcp.CallToolResult, any, error) {
		dd, err := ddCache.get(in.Target)
		if err != nil {
			return nil, nil, fmt.Errorf("open data dir: %w", err)
		}

		hasHash := in.Hash != ""
		hasHeight := in.Height > 0

		if hasHash == hasHeight {
			return nil, nil, fmt.Errorf("provide either hash or height+index, not both")
		}

		if hasHash {
			detail, err := dd.TxByHash(in.Hash, nodedata.TxHashScanWindow)
			if err != nil {
				return nil, nil, err
			}
			return textResult(detail)
		}

		detail, err := dd.TxByIndex(in.Height, in.Index)
		if err != nil {
			return nil, nil, err
		}
		return textResult(detail)
	})

	// ---- node_compare
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "node_compare",
		Description: desc("node_compare"),
		Annotations: readOnlyAnnotation,
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in nodeCompareInput) (*sdkmcp.CallToolResult, any, error) {
		dirs := make([]*nodedata.DataDir, len(in.Targets))
		for i, target := range in.Targets {
			dd, err := ddCache.get(target)
			if err != nil {
				return nil, nil, fmt.Errorf("open data dir %q: %w", target, err)
			}
			dirs[i] = dd
		}

		result, err := nodedata.CompareBlocks(dirs, in.Height)
		if err != nil {
			return nil, nil, err
		}

		return textResult(result)
	})
}
