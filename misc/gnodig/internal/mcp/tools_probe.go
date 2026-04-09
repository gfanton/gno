package mcp

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"strings"
	"sync"

	"github.com/gnolang/gno/misc/gnodig/internal/probeclient"
	"golang.org/x/sync/singleflight"
)

// isProbeTarget returns true if the target string indicates a probe connection.
func isProbeTarget(target string) bool {
	return strings.HasPrefix(target, "probe://")
}

// probeClients caches authenticated probe clients by address.
type probeClients struct {
	mu      sync.Mutex
	clients map[string]*probeclient.Client
	flight  singleflight.Group
	priv    ed25519.PrivateKey
	pub     ed25519.PublicKey
}

func newProbeClients(pub ed25519.PublicKey, priv ed25519.PrivateKey) *probeClients {
	return &probeClients{
		clients: make(map[string]*probeclient.Client),
		pub:     pub,
		priv:    priv,
	}
}

// get returns an authenticated client for the given probe address.
// Uses singleflight to deduplicate concurrent connection attempts.
func (pc *probeClients) get(ctx context.Context, address string) (*probeclient.Client, error) {
	addr := normalizeProbeAddr(address)

	// Fast path: return cached client.
	pc.mu.Lock()
	if c, ok := pc.clients[addr]; ok {
		pc.mu.Unlock()
		return c, nil
	}
	pc.mu.Unlock()

	// Deduplicate concurrent connection attempts to the same address.
	v, err, _ := pc.flight.Do(addr, func() (any, error) {
		// Double-check cache (another caller in the flight group may have stored it).
		pc.mu.Lock()
		if c, ok := pc.clients[addr]; ok {
			pc.mu.Unlock()
			return c, nil
		}
		pc.mu.Unlock()

		c, err := probeclient.New(probeclient.Config{
			Address:    addr,
			PrivateKey: pc.priv,
			PublicKey:  pc.pub,
		})
		if err != nil {
			return nil, fmt.Errorf("create probe client for %q: %w", addr, err)
		}

		if err := c.Authenticate(ctx); err != nil {
			c.Close()
			return nil, fmt.Errorf("authenticate with probe %q: %w", addr, err)
		}

		pc.mu.Lock()
		pc.clients[addr] = c
		pc.mu.Unlock()
		return c, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*probeclient.Client), nil
}

// Close closes all cached probe clients, zeroes key material, and clears the cache.
func (pc *probeClients) Close() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	for _, c := range pc.clients {
		c.Close()
	}
	clear(pc.clients)
	for i := range pc.priv {
		pc.priv[i] = 0
	}
}

func normalizeProbeAddr(address string) string {
	addr := strings.TrimPrefix(address, "probe://")
	if !strings.Contains(addr, "://") {
		addr = "http://" + addr
	}
	return addr
}
