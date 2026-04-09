package doctor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeverity_String(t *testing.T) {
	assert.Equal(t, "info", Info.String())
	assert.Equal(t, "warning", Warning.String())
	assert.Equal(t, "critical", Critical.String())
}
