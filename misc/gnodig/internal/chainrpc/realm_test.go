package chainrpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFormatEvalData(t *testing.T) {
	data := formatEvalData("gno.land/r/demo/counter", "Counter()")
	require.Equal(t, "gno.land/r/demo/counter.Counter()", string(data))
}

func TestFormatEvalData_Render(t *testing.T) {
	data := formatEvalData("gno.land/r/demo/boards", `Render("")`)
	require.Equal(t, `gno.land/r/demo/boards.Render("")`, string(data))
}

func TestFormatFileData(t *testing.T) {
	// qfile listing: just pkgPath
	data := formatFileData("gno.land/r/demo/boards", "")
	require.Equal(t, "gno.land/r/demo/boards", string(data))

	// qfile content: pkgPath/fileName
	data = formatFileData("gno.land/r/demo/boards", "boards.gno")
	require.Equal(t, "gno.land/r/demo/boards/boards.gno", string(data))
}

func TestFormatAccountPath(t *testing.T) {
	path := formatAccountPath("g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5")
	require.Equal(t, "auth/accounts/g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5", path)
}

// ---- Integration Tests

const testRPC = "https://rpc.gno.land"

func skipIfNoRPC(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c := New(testRPC)
	if _, err := c.Health(ctx); err != nil {
		t.Skipf("RPC not available: %v", err)
	}
}

func TestEvalExpression_Integration(t *testing.T) {
	skipIfNoRPC(t)
	c := New(testRPC)
	ctx := context.Background()

	result, err := c.EvalExpression(ctx, "gno.land/r/gnoland/blog", `Render("")`)
	require.NoError(t, err)
	require.NotEmpty(t, result.Result)
	require.Equal(t, "gno.land/r/gnoland/blog", result.PkgPath)
	t.Logf("result (first 200): %s", result.Result[:min(200, len(result.Result))])
}

func TestInspectPackage_Integration(t *testing.T) {
	skipIfNoRPC(t)
	c := New(testRPC)
	ctx := context.Background()

	result, err := c.InspectPackage(ctx, "gno.land/r/gnoland/blog")
	require.NoError(t, err)
	require.NotEmpty(t, result.Files)
	t.Logf("files: %d", len(result.Files))
	if result.Functions != "" {
		t.Logf("functions (first 200): %s", result.Functions[:min(200, len(result.Functions))])
	}
}

func TestQueryAccount_Integration(t *testing.T) {
	skipIfNoRPC(t)
	c := New(testRPC)
	ctx := context.Background()

	result, err := c.QueryAccount(ctx, "g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5")
	require.NoError(t, err)
	require.True(t, result.Exists)
	require.NotEmpty(t, result.Coins)
	t.Logf("coins: %s, seq: %d", result.Coins, result.Sequence)
}

func TestQueryAccount_NonExistent(t *testing.T) {
	skipIfNoRPC(t)
	c := New(testRPC)
	ctx := context.Background()

	result, err := c.QueryAccount(ctx, "g1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq")
	// May error or return exists=false depending on how the node handles it
	if err == nil {
		require.False(t, result.Exists)
	}
}
