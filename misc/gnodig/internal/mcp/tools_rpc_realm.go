package mcp

import (
	"context"
	"fmt"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ---- Target Validation

// requireRPCTarget checks that the target is an HTTP(S) URL and returns
// a helpful error message suggesting the offline alternative when it's not.
func requireRPCTarget(target, tool, offlineHint string) error {
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return nil
	}
	return fmt.Errorf("%s requires an RPC target (e.g. http://node:26657), got %q. %s", tool, target, offlineHint)
}

// ---- Input Structs

type realmEvalInput struct {
	Target     string `json:"target" jsonschema:"RPC endpoint URL (e.g. http://node:26657),required"`
	PkgPath    string `json:"pkg_path" jsonschema:"Package path (e.g. gno.land/r/demo/boards),required"`
	Expression string `json:"expression" jsonschema:"Gno expression to evaluate (e.g. Render(\"\") or Counter()),required"`
}

type realmInspectInput struct {
	Target  string `json:"target" jsonschema:"RPC endpoint URL (e.g. http://node:26657),required"`
	PkgPath string `json:"pkg_path" jsonschema:"Package path (e.g. gno.land/r/demo/boards),required"`
}

type realmSourceInput struct {
	Target  string `json:"target" jsonschema:"RPC endpoint URL (e.g. http://node:26657),required"`
	PkgPath string `json:"pkg_path" jsonschema:"Package path (e.g. gno.land/r/demo/boards),required"`
	File    string `json:"file" jsonschema:"File name to fetch (e.g. boards.gno),required"`
}

type accountInfoInput struct {
	Target  string `json:"target" jsonschema:"RPC endpoint URL (e.g. http://node:26657),required"`
	Address string `json:"address" jsonschema:"Account address (e.g. g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5),required"`
}

type genesisInfoInput struct {
	Target  string `json:"target" jsonschema:"RPC endpoint URL (e.g. http://node:26657),required"`
	Mode    string `json:"mode,omitempty" jsonschema:"Mode: summary (default) or balance"`
	Address string `json:"address,omitempty" jsonschema:"Account address for balance lookup (required in balance mode)"`
}

// ---- Registration

func registerRealmTools(srv *sdkmcp.Server, clients *chainClients, cacheDir string) {
	// ---- realm_eval
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "realm_eval",
		Description: desc("realm_eval"),
		Annotations: readOnlyAnnotation,
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in realmEvalInput) (*sdkmcp.CallToolResult, any, error) {
		if err := requireRPCTarget(in.Target, "realm_eval", "Use node_data_state to query stored state offline."); err != nil {
			return nil, nil, err
		}
		c, err := clients.get(in.Target)
		if err != nil {
			return nil, nil, err
		}
		result, err := c.EvalExpression(ctx, in.PkgPath, in.Expression)
		if err != nil {
			return nil, nil, err
		}
		return textResult(result)
	})

	// ---- realm_inspect
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "realm_inspect",
		Description: desc("realm_inspect"),
		Annotations: readOnlyAnnotation,
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in realmInspectInput) (*sdkmcp.CallToolResult, any, error) {
		if err := requireRPCTarget(in.Target, "realm_inspect", "Use node_data_state --path to get package metadata offline."); err != nil {
			return nil, nil, err
		}
		c, err := clients.get(in.Target)
		if err != nil {
			return nil, nil, err
		}
		result, err := c.InspectPackage(ctx, in.PkgPath)
		if err != nil {
			return nil, nil, err
		}
		return textResult(result)
	})

	// ---- realm_source
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "realm_source",
		Description: desc("realm_source"),
		Annotations: readOnlyAnnotation,
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in realmSourceInput) (*sdkmcp.CallToolResult, any, error) {
		if err := requireRPCTarget(in.Target, "realm_source", "Use node_data_state --path to query package files offline."); err != nil {
			return nil, nil, err
		}
		c, err := clients.get(in.Target)
		if err != nil {
			return nil, nil, err
		}
		result, err := c.FetchSource(ctx, in.PkgPath, in.File)
		if err != nil {
			return nil, nil, err
		}
		return textResult(result)
	})

	// ---- account_info
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "account_info",
		Description: desc("account_info"),
		Annotations: readOnlyAnnotation,
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in accountInfoInput) (*sdkmcp.CallToolResult, any, error) {
		if err := requireRPCTarget(in.Target, "account_info", "Use node_data_state --account to query account state offline."); err != nil {
			return nil, nil, err
		}
		c, err := clients.get(in.Target)
		if err != nil {
			return nil, nil, err
		}
		result, err := c.QueryAccount(ctx, in.Address)
		if err != nil {
			return nil, nil, err
		}
		return textResult(result)
	})

	// ---- genesis_info
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "genesis_info",
		Description: desc("genesis_info"),
		Annotations: readOnlyAnnotation,
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in genesisInfoInput) (*sdkmcp.CallToolResult, any, error) {
		if err := requireRPCTarget(in.Target, "genesis_info", "If genesis was previously cached, it can be read offline."); err != nil {
			return nil, nil, err
		}
		c, err := clients.get(in.Target)
		if err != nil {
			return nil, nil, err
		}

		mode := in.Mode
		if mode == "" {
			mode = "summary"
		}

		switch mode {
		case "summary":
			result, err := c.FetchGenesisSummary(ctx, cacheDir)
			if err != nil {
				return nil, nil, err
			}
			return textResult(result)

		case "balance":
			if in.Address == "" {
				return nil, nil, fmt.Errorf("address is required for balance mode")
			}
			result, err := c.LookupGenesisBalance(ctx, cacheDir, in.Address)
			if err != nil {
				return nil, nil, err
			}
			return textResult(result)

		default:
			return nil, nil, fmt.Errorf("unknown mode %q: use summary or balance", mode)
		}
	})
}
