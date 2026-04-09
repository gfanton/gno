package probeserver

import (
	"log/slog"
	"time"
)

// AuditLogger records structured logs for tool call activity.
type AuditLogger struct {
	logger *slog.Logger
}

// NewAuditLogger creates an AuditLogger backed by the given slog.Logger.
func NewAuditLogger(logger *slog.Logger) *AuditLogger {
	return &AuditLogger{logger: logger}
}

// LogToolCall emits a structured log entry for a single tool invocation.
func (a *AuditLogger) LogToolCall(tool, identity string, duration time.Duration, hasError bool) {
	a.logger.Info("tool_call",
		"tool", tool,
		"identity", identity,
		"duration_ms", duration.Milliseconds(),
		"error", hasError,
	)
}
