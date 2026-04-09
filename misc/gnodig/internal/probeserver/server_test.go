package probeserver

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gnolang/gno/misc/gnodig/internal/probeapi"
	"github.com/gnolang/gno/misc/gnodig/internal/probeauth"
)

// testServer creates a server with one authorized keypair for testing.
func testServer(t *testing.T) (*Server, ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	keysPath := filepath.Join(dir, "authorized_keys")
	line := fmt.Sprintf("ssh-ed25519 %s test-key\n", base64.StdEncoding.EncodeToString(pub))
	if err := os.WriteFile(keysPath, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}

	srv, err := New(Config{
		AuthorizedKeysPath: keysPath,
		MaxConcurrent:      10,
	})
	if err != nil {
		t.Fatal(err)
	}

	return srv, pub, priv
}

func TestServer_ToolCall(t *testing.T) {
	srv, pub, priv := testServer(t)

	// Register an echo tool that returns its params as-is.
	srv.HandleTool("echo", func(_ context.Context, params json.RawMessage) (json.RawMessage, error) {
		return params, nil
	})

	// Authenticate to get a token.
	token, err := srv.Authenticate(pub, priv)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}

	// Call the echo tool.
	body := `{"tool":"echo","params":{"msg":"hello"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/tool/echo", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp probeapi.ToolResponse
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if string(resp.Result) != `{"msg":"hello"}` {
		t.Fatalf("unexpected result: %s", resp.Result)
	}
}

func TestServer_UnknownTool(t *testing.T) {
	srv, pub, priv := testServer(t)

	token, err := srv.Authenticate(pub, priv)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}

	body := `{"tool":"nonexistent","params":{}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/tool/nonexistent", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp probeapi.ToolResponse
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response")
	}
	if resp.Error.Code != probeapi.ErrNotFound {
		t.Fatalf("expected error code %q, got %q", probeapi.ErrNotFound, resp.Error.Code)
	}
}

func TestServer_Unauthenticated(t *testing.T) {
	srv, _, _ := testServer(t)

	srv.HandleTool("echo", func(_ context.Context, params json.RawMessage) (json.RawMessage, error) {
		return params, nil
	})

	body := `{"tool":"echo","params":{}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/tool/echo", strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp probeapi.ToolResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response")
	}
	if resp.Error.Code != probeapi.ErrAuthFailed {
		t.Fatalf("expected error code %q, got %q", probeapi.ErrAuthFailed, resp.Error.Code)
	}
}

func TestServer_AuthFlow(t *testing.T) {
	srv, pub, priv := testServer(t)

	srv.HandleTool("echo", func(_ context.Context, params json.RawMessage) (json.RawMessage, error) {
		return params, nil
	})

	// Phase 1: get challenge.
	req := httptest.NewRequest(http.MethodPost, "/v1/auth", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var hs probeapi.Handshake
	err := json.Unmarshal(rec.Body.Bytes(), &hs)
	require.NoError(t, err)
	require.Len(t, hs.Challenge, probeauth.ChallengeSize)

	// Sign challenge.
	sig := probeauth.SignChallenge(priv, hs.Challenge)

	reply := probeapi.Handshake{
		Version:   probeapi.ProtocolVersion,
		Challenge: hs.Challenge,
		Signature: sig,
		PubKey:    []byte(pub),
	}
	replyBody, err := json.Marshal(reply)
	require.NoError(t, err)

	// Phase 2: verify and get token.
	req = httptest.NewRequest(http.MethodPost, "/v1/auth", bytes.NewReader(replyBody))
	req.Header.Set("X-Probe-Phase", "verify")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var authResp probeapi.AuthResponse
	err = json.Unmarshal(rec.Body.Bytes(), &authResp)
	require.NoError(t, err)
	require.NotEmpty(t, authResp.Token)

	// Use token to call tool.
	toolBody := `{"tool":"echo","params":{"msg":"hello"}}`
	req = httptest.NewRequest(http.MethodPost, "/v1/tool/echo", strings.NewReader(toolBody))
	req.Header.Set("Authorization", "Bearer "+authResp.Token)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var toolResp probeapi.ToolResponse
	err = json.Unmarshal(rec.Body.Bytes(), &toolResp)
	require.NoError(t, err)
	require.Nil(t, toolResp.Error)
	require.Equal(t, `{"msg":"hello"}`, string(toolResp.Result))
}

func TestServer_AuthFlow_UnknownKey(t *testing.T) {
	srv, _, _ := testServer(t)

	// Generate a keypair that is NOT in the server's authorized_keys.
	unknownPub, unknownPriv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	// Phase 1: get challenge.
	req := httptest.NewRequest(http.MethodPost, "/v1/auth", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var hs probeapi.Handshake
	err = json.Unmarshal(rec.Body.Bytes(), &hs)
	require.NoError(t, err)

	// Sign with unknown key.
	sig := probeauth.SignChallenge(unknownPriv, hs.Challenge)

	reply := probeapi.Handshake{
		Version:   probeapi.ProtocolVersion,
		Challenge: hs.Challenge,
		Signature: sig,
		PubKey:    []byte(unknownPub),
	}
	replyBody, err := json.Marshal(reply)
	require.NoError(t, err)

	// Phase 2: should be rejected.
	req = httptest.NewRequest(http.MethodPost, "/v1/auth", bytes.NewReader(replyBody))
	req.Header.Set("X-Probe-Phase", "verify")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestServer_Healthz(t *testing.T) {
	srv, _, _ := testServer(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != `{"status":"ok"}` {
		t.Fatalf("unexpected body: %q", got)
	}
}

func TestServer_Healthz_NoAuth(t *testing.T) {
	srv, _, _ := testServer(t)

	// /healthz must be accessible without any Authorization header.
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 without auth, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestServer_ConcurrencyLimit(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	dir := t.TempDir()
	keysPath := filepath.Join(dir, "authorized_keys")
	line := fmt.Sprintf("ssh-ed25519 %s test-key\n", base64.StdEncoding.EncodeToString(pub))
	err = os.WriteFile(keysPath, []byte(line), 0o600)
	require.NoError(t, err)

	limitSrv, err := New(Config{
		AuthorizedKeysPath: keysPath,
		MaxConcurrent:      1,
	})
	require.NoError(t, err)

	blocked := make(chan struct{})
	unblock := make(chan struct{})

	limitSrv.HandleTool("slow", func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
		close(blocked)
		<-unblock
		return json.RawMessage(`{}`), nil
	})

	token, err := limitSrv.Authenticate(pub, priv)
	require.NoError(t, err)

	// Start first request in background; it will block inside the slow tool.
	go func() {
		body, _ := json.Marshal(probeapi.ToolRequest{Tool: "slow", Params: json.RawMessage(`{}`)}) //nolint
		req := httptest.NewRequest(http.MethodPost, "/v1/tool/slow", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		limitSrv.ServeHTTP(rec, req)
	}()

	// Wait until the first request has acquired the semaphore slot.
	<-blocked

	// Second request should be rejected with 429.
	body2, err := json.Marshal(probeapi.ToolRequest{Tool: "slow", Params: json.RawMessage(`{}`)})
	require.NoError(t, err)
	req2 := httptest.NewRequest(http.MethodPost, "/v1/tool/slow", bytes.NewReader(body2))
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	limitSrv.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusTooManyRequests, rec2.Code)

	// Unblock the first request.
	close(unblock)
}
