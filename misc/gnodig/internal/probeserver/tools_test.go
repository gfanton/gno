package probeserver

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gnolang/gno/misc/gnodig/internal/chainrpc"
)

func TestRegisterRPCTools(t *testing.T) {
	srv, _, _ := testServer(t)

	assert.False(t, srv.HasTool("node_overview"))
	assert.False(t, srv.HasTool("block_inspect"))

	RegisterRPCTools(srv, chainrpc.New("http://localhost:26657"))

	assert.True(t, srv.HasTool("node_overview"))
	assert.True(t, srv.HasTool("block_inspect"))
}

func TestRegisterNodeDataTools(t *testing.T) {
	srv, _, _ := testServer(t)

	assert.False(t, srv.HasTool("node_data_open"))
	assert.False(t, srv.HasTool("node_data_block"))
	assert.False(t, srv.HasTool("node_data_wal"))

	// RegisterNodeDataTools captures dd in closures, so registration
	// succeeds even with nil. We verify tools are registered;
	// calling them requires a real DataDir.
	RegisterNodeDataTools(srv, nil)

	assert.True(t, srv.HasTool("node_data_open"))
	assert.True(t, srv.HasTool("node_data_block"))
	assert.True(t, srv.HasTool("node_data_wal"))
}
