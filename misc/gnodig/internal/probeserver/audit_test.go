package probeserver

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditLog_Format(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	audit := &AuditLogger{logger: logger}
	audit.LogToolCall("node_overview", "alice@example", 150*time.Millisecond, false)

	var entry map[string]any
	err := json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err)
	assert.Equal(t, "tool_call", entry["msg"])
	assert.Equal(t, "node_overview", entry["tool"])
	assert.Equal(t, "alice@example", entry["identity"])
	assert.Equal(t, float64(150), entry["duration_ms"])
	assert.Equal(t, false, entry["error"])
}

func TestAuditLog_WithError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	audit := &AuditLogger{logger: logger}
	audit.LogToolCall("block_inspect", "bob@example", 50*time.Millisecond, true)

	var entry map[string]any
	err := json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err)
	assert.Equal(t, true, entry["error"])
}
