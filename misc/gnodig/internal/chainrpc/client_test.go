package chainrpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Status(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"jsonrpc": "2.0",
			"id": "",
			"result": {
				"node_info": {
					"network": "test-chain"
				},
				"sync_info": {
					"latest_block_height": "12345"
				}
			}
		}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	result, err := c.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}

	height := result.Get("sync_info.latest_block_height").String()
	if height != "12345" {
		t.Errorf("expected height=12345, got %q", height)
	}

	network := result.Get("node_info.network").String()
	if network != "test-chain" {
		t.Errorf("expected network=test-chain, got %q", network)
	}
}

func TestClient_Block(t *testing.T) {
	const wantHeight = int64(100)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/block" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if h := r.URL.Query().Get("height"); h != "100" {
			t.Errorf("expected height query param=100, got %q", h)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"jsonrpc": "2.0",
			"id": "",
			"result": {
				"block": {
					"header": {
						"height": "100",
						"chain_id": "test-chain"
					}
				}
			}
		}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	result, err := c.Block(context.Background(), wantHeight)
	if err != nil {
		t.Fatalf("Block() error: %v", err)
	}

	height := result.Get("block.header.height").String()
	if height != "100" {
		t.Errorf("expected block height=100, got %q", height)
	}
}

func TestClient_RPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"jsonrpc": "2.0",
			"id": "",
			"error": {
				"code": -32600,
				"message": "block not found",
				"data": ""
			}
		}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.Block(context.Background(), 999999)
	if err == nil {
		t.Fatal("expected error for RPC error response, got nil")
	}
}
