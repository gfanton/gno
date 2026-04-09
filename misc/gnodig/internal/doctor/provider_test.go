package doctor

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvider_LazyFetch(t *testing.T) {
	calls := 0
	p := newProvider(func() (string, error) {
		calls++
		return "hello", nil
	})

	val, err := p.Get()
	require.NoError(t, err)
	assert.Equal(t, "hello", val)
	assert.Equal(t, 1, calls)

	val, err = p.Get()
	require.NoError(t, err)
	assert.Equal(t, "hello", val)
	assert.Equal(t, 1, calls)
}

func TestProvider_ErrorCached(t *testing.T) {
	p := newProvider(func() (string, error) {
		return "", errors.New("boom")
	})

	_, err := p.Get()
	assert.EqualError(t, err, "boom")

	_, err = p.Get()
	assert.EqualError(t, err, "boom")
}

func TestProvider_Nil(t *testing.T) {
	var p provider[string]
	_, err := p.Get()
	assert.Error(t, err)
}
