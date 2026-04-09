package probeclient_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gnolang/gno/misc/gnodig/internal/probeapi"
	"github.com/gnolang/gno/misc/gnodig/internal/probeclient"
	"github.com/gnolang/gno/misc/gnodig/internal/probeserver"
)

// testClientServer creates a probe server with one authorized keypair and a
// matching Client pointed at it. The httptest.Server is closed on t.Cleanup.
func testClientServer(t *testing.T) (*probeclient.Client, *httptest.Server) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	dir := t.TempDir()
	keysPath := filepath.Join(dir, "authorized_keys")
	line := fmt.Sprintf("ssh-ed25519 %s test-key\n", base64.StdEncoding.EncodeToString(pub))
	require.NoError(t, os.WriteFile(keysPath, []byte(line), 0o600))

	srv, err := probeserver.New(probeserver.Config{
		AuthorizedKeysPath: keysPath,
		MaxConcurrent:      10,
	})
	require.NoError(t, err)

	srv.HandleTool("echo", func(_ context.Context, params json.RawMessage) (json.RawMessage, error) {
		return params, nil
	})

	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	client, err := probeclient.New(probeclient.Config{
		Address:    ts.URL,
		PrivateKey: priv,
		PublicKey:  pub,
	})
	require.NoError(t, err)

	return client, ts
}

func TestClient_CallTool(t *testing.T) {
	client, _ := testClientServer(t)

	err := client.Authenticate(context.Background())
	require.NoError(t, err)
	require.True(t, client.IsAuthenticated())

	params := json.RawMessage(`{"msg":"hello"}`)
	result, err := client.CallTool(context.Background(), "echo", params)
	require.NoError(t, err)
	require.Equal(t, `{"msg":"hello"}`, string(result))
}

func TestClient_CallTool_UnknownTool(t *testing.T) {
	client, _ := testClientServer(t)

	err := client.Authenticate(context.Background())
	require.NoError(t, err)

	_, err = client.CallTool(context.Background(), "nonexistent", json.RawMessage(`{}`))
	require.Error(t, err)

	var toolErr *probeapi.ToolError
	require.True(t, errors.As(err, &toolErr), "expected *probeapi.ToolError, got %T: %v", err, err)
	require.Equal(t, probeapi.ErrNotFound, toolErr.Code)
}

func TestClient_CallTool_Unauthenticated(t *testing.T) {
	client, _ := testClientServer(t)

	// Do NOT call Authenticate — token should be empty.
	require.False(t, client.IsAuthenticated())

	_, err := client.CallTool(context.Background(), "echo", json.RawMessage(`{}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "not authenticated")
}
