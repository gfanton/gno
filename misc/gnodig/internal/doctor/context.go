package doctor

import (
	"strings"

	"github.com/gnolang/gno/misc/gnodig/internal/chainrpc"
	"github.com/gnolang/gno/misc/gnodig/internal/nodedata"
)

type TargetType int

const (
	TargetRPC TargetType = iota
	TargetDataDir
)

func DetectTargetType(target string) TargetType {
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return TargetRPC
	}
	return TargetDataDir
}

// ---- Context

type Context struct {
	target string
	ttype  TargetType

	// RPC providers
	status provider[*chainrpc.Overview]

	// Data dir providers
	dataOverview provider[*nodedata.Overview]
	walSummary   provider[*nodedata.WALSummary]
}

// ---- Test helpers

type contextOption func(*Context)

func newTestContext(opts ...contextOption) *Context {
	ctx := &Context{target: "test", ttype: TargetRPC}
	for _, opt := range opts {
		opt(ctx)
	}
	return ctx
}

func withStatus(v *chainrpc.Overview) contextOption {
	return func(ctx *Context) {
		ctx.status = newProvider(func() (*chainrpc.Overview, error) { return v, nil })
	}
}

func withDataOverview(v *nodedata.Overview) contextOption {
	return func(ctx *Context) {
		ctx.dataOverview = newProvider(func() (*nodedata.Overview, error) { return v, nil })
	}
}

func withWALSummary(v *nodedata.WALSummary) contextOption {
	return func(ctx *Context) {
		ctx.walSummary = newProvider(func() (*nodedata.WALSummary, error) { return v, nil })
	}
}
