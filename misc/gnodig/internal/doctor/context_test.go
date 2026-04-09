package doctor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectTargetType(t *testing.T) {
	tests := []struct {
		input    string
		wantType TargetType
	}{
		{"https://rpc.gno.land", TargetRPC},
		{"http://localhost:26657", TargetRPC},
		{"/path/to/gnoland-data", TargetDataDir},
		{"./relative/data", TargetDataDir},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := DetectTargetType(tt.input)
			assert.Equal(t, tt.wantType, got)
		})
	}
}
