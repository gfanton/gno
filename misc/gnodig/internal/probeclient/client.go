package probeclient

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gnolang/gno/misc/gnodig/internal/probeapi"
	"github.com/gnolang/gno/misc/gnodig/internal/probeauth"
)

// Config holds the parameters for creating a Client.
type Config struct {
	Address    string
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
}

// Client authenticates with a probe server and calls tools over HTTP.
type Client struct {
	addr  string
	priv  ed25519.PrivateKey
	pub   ed25519.PublicKey
	http  *http.Client
	mu    sync.RWMutex
	token string
}

// New creates a Client from cfg. Returns an error if Address is empty.
func New(cfg Config) (*Client, error) {
	if cfg.Address == "" {
		return nil, errors.New("probeclient: address is required")
	}
	return &Client{
		addr: cfg.Address,
		priv: cfg.PrivateKey,
		pub:  cfg.PublicKey,
		http: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Authenticate performs a two-phase HTTP handshake with the probe server and
// stores the resulting bearer token for use by CallTool.
func (c *Client) Authenticate(ctx context.Context) error {
	// Phase 1: obtain a challenge.
	req1, err := http.NewRequestWithContext(ctx, http.MethodPost, c.addr+"/v1/auth", nil)
	if err != nil {
		return fmt.Errorf("probeclient: build challenge request: %w", err)
	}

	resp1, err := c.http.Do(req1)
	if err != nil {
		return fmt.Errorf("probeclient: challenge request: %w", err)
	}
	defer resp1.Body.Close()

	if resp1.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp1.Body, 4096))
		return fmt.Errorf("probeclient: challenge failed (status %d): %s", resp1.StatusCode, body)
	}

	var hs probeapi.Handshake
	if err := json.NewDecoder(resp1.Body).Decode(&hs); err != nil {
		return fmt.Errorf("probeclient: decode challenge: %w", err)
	}

	// Sign the challenge.
	sig := probeauth.SignChallenge(c.priv, hs.Challenge)

	reply := probeapi.Handshake{
		Version:   probeapi.ProtocolVersion,
		Challenge: hs.Challenge,
		Signature: sig,
		PubKey:    []byte(c.pub),
	}
	replyBody, err := json.Marshal(reply)
	if err != nil {
		return fmt.Errorf("probeclient: marshal verify request: %w", err)
	}

	// Phase 2: submit signature, get token.
	req2, err := http.NewRequestWithContext(ctx, http.MethodPost, c.addr+"/v1/auth", bytes.NewReader(replyBody))
	if err != nil {
		return fmt.Errorf("probeclient: build verify request: %w", err)
	}
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Probe-Phase", "verify")

	resp2, err := c.http.Do(req2)
	if err != nil {
		return fmt.Errorf("probeclient: verify request: %w", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp2.Body, 4096))
		return fmt.Errorf("probeclient: verify failed (status %d): %s", resp2.StatusCode, body)
	}

	var authResp probeapi.AuthResponse
	if err := json.NewDecoder(resp2.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("probeclient: decode auth response: %w", err)
	}

	c.mu.Lock()
	c.token = authResp.Token
	c.mu.Unlock()

	return nil
}

// CallTool calls the named tool on the probe server with the given params.
// Returns the raw JSON result, or a *probeapi.ToolError if the server reports
// a tool-level error. Returns an error if the client is not yet authenticated.
func (c *Client) CallTool(ctx context.Context, tool string, params json.RawMessage) (json.RawMessage, error) {
	c.mu.RLock()
	token := c.token
	c.mu.RUnlock()

	if token == "" {
		return nil, errors.New("probeclient: not authenticated")
	}

	reqBody, err := json.Marshal(probeapi.ToolRequest{Tool: tool, Params: params})
	if err != nil {
		return nil, fmt.Errorf("probeclient: marshal tool request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.addr+"/v1/tool/"+tool, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("probeclient: build tool request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("probeclient: tool request: %w", err)
	}
	defer resp.Body.Close()

	var toolResp probeapi.ToolResponse
	if err := json.NewDecoder(resp.Body).Decode(&toolResp); err != nil {
		return nil, fmt.Errorf("probeclient: decode tool response: %w", err)
	}

	if toolResp.Error != nil {
		return nil, toolResp.Error
	}

	return toolResp.Result, nil
}

// IsAuthenticated reports whether the client holds a non-empty token.
func (c *Client) IsAuthenticated() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token != ""
}

// Close zeroes sensitive key material and closes idle HTTP connections.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.priv {
		c.priv[i] = 0
	}
	c.token = ""
	c.http.CloseIdleConnections()
}
