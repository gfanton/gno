package mcp

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gnolang/gno/misc/gnodig/internal/chainrpc"
	"github.com/gnolang/gno/misc/gnodig/internal/doctor"
	"github.com/gnolang/gno/misc/gnodig/internal/nodedata"
)

type doctorInput struct {
	Target string `json:"target" jsonschema:"RPC endpoint URL or data directory path,required"`
}

func registerDoctorTools(srv *sdkmcp.Server, clients *chainClients, ddCache *dataDirCache) {
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "node_doctor",
		Description: desc("node_doctor"),
		Annotations: readOnlyAnnotation,
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in doctorInput) (*sdkmcp.CallToolResult, any, error) {
		var rpcClient *chainrpc.Client
		var dataDir *nodedata.DataDir

		switch doctor.DetectTargetType(in.Target) {
		case doctor.TargetRPC:
			c, err := clients.get(in.Target)
			if err != nil {
				return nil, nil, err
			}
			rpcClient = c
		case doctor.TargetDataDir:
			dd, err := ddCache.get(in.Target)
			if err != nil {
				return nil, nil, err
			}
			dataDir = dd
		}

		report := doctor.RunDoctor(ctx, in.Target, rpcClient, dataDir)
		return textResult(report)
	})
}
