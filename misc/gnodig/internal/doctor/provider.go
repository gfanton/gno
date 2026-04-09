package doctor

import (
	"errors"
	"sync"
)

// ErrProviderNotAvailable is returned when a provider has no fetch function,
// meaning the underlying data source (RPC or data directory) was not configured.
var ErrProviderNotAvailable = errors.New("provider not available for this target")

// provider must not be copied after first use (contains sync.Once).
// Always store in structs and access via pointer receiver.
type provider[T any] struct {
	fetch func() (T, error)
	val   T
	err   error
	once  sync.Once
}

func newProvider[T any](fetch func() (T, error)) provider[T] {
	return provider[T]{fetch: fetch}
}

func (p *provider[T]) Get() (T, error) {
	p.once.Do(func() {
		if p.fetch == nil {
			p.err = ErrProviderNotAvailable
			return
		}
		p.val, p.err = p.fetch()
	})
	return p.val, p.err
}
