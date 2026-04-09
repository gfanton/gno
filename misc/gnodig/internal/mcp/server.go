package mcp

import (
	"fmt"
	"net/url"
	"sync"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gnolang/gno/misc/gnodig/internal/chainrpc"
	"github.com/gnolang/gno/misc/gnodig/internal/driver"
	"github.com/gnolang/gno/misc/gnodig/internal/driver/localfs"
	"github.com/gnolang/gno/misc/gnodig/internal/logengine"
)

type ServerConfig struct {
	ScanConfig logengine.ScanConfig
}

func LogResolvers() map[string]driver.Resolver {
	return map[string]driver.Resolver{
		"file": localfs.NewFromURI,
	}
}

type chainClients struct {
	mu      sync.Mutex
	clients map[string]*chainrpc.Client
}

func newChainClients() *chainClients {
	return &chainClients{clients: make(map[string]*chainrpc.Client)}
}

const maxCachedClients = 100

func (cc *chainClients) get(target string) (*chainrpc.Client, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("invalid target URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("target must use http or https scheme, got %q", u.Scheme)
	}

	cc.mu.Lock()
	defer cc.mu.Unlock()
	if c, ok := cc.clients[target]; ok {
		return c, nil
	}
	// Evict all if at capacity (simple strategy).
	if len(cc.clients) >= maxCachedClients {
		clear(cc.clients)
	}
	c := chainrpc.New(target)
	cc.clients[target] = c
	return c, nil
}

// Server wraps the MCP server and provides cleanup on shutdown.
type Server struct {
	*sdkmcp.Server
	cleanup func()
}

// Close releases resources held by the server.
func (s *Server) Close() {
	if s.cleanup != nil {
		s.cleanup()
	}
}

func NewServer(cfg ServerConfig) *Server {
	srv := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "gnodig", Version: "v0.1.0"},
		&sdkmcp.ServerOptions{
			Instructions: desc("instructions"),
		},
	)

	cache := logengine.NewCache()
	resolvers := LogResolvers()
	clients := newChainClients()
	ddCache := newDataDirCache()

	registerRPCTools(srv, clients)
	registerRealmTools(srv, clients, ".debug")
	registerLogTools(srv, cache, resolvers, cfg)
	registerNodeDataTools(srv, ddCache)
	registerDoctorTools(srv, clients, ddCache)

	return &Server{Server: srv, cleanup: ddCache.closeAll}
}
