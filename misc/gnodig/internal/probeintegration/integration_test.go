package probeintegration_test

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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnolang/gno/misc/gnodig/internal/probeapi"
	"github.com/gnolang/gno/misc/gnodig/internal/probeclient"
	"github.com/gnolang/gno/misc/gnodig/internal/probeserver"
)

func TestProbe_EndToEnd(t *testing.T) {
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

	ctx := context.Background()

	require.NoError(t, client.Authenticate(ctx))
	require.True(t, client.IsAuthenticated())

	params := json.RawMessage(`{"key":"value"}`)
	result, err := client.CallTool(ctx, "echo", params)
	require.NoError(t, err)
	assert.JSONEq(t, `{"key":"value"}`, string(result))

	_, err = client.CallTool(ctx, "nonexistent", params)
	require.Error(t, err)

	var toolErr *probeapi.ToolError
	require.True(t, errors.As(err, &toolErr))
	assert.Equal(t, probeapi.ErrNotFound, toolErr.Code)
}
