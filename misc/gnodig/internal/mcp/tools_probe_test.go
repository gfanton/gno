package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsProbeTarget(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   bool
	}{
		{"rpc url", "http://localhost:26657", false},
		{"data dir", "/path/to/gnoland-data", false},
		{"probe address", "probe://val01.internal:9090", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isProbeTarget(tt.target))
		})
	}
}
