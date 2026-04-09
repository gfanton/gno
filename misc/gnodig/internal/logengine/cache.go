package logengine

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/singleflight"

	"github.com/gnolang/gno/misc/gnodig/internal/driver"
)

type Cache struct {
	mu      sync.Mutex
	entries map[string]*Index
	group   singleflight.Group
}

func NewCache() *Cache {
	return &Cache{entries: make(map[string]*Index)}
}

func (c *Cache) GetOrBuild(ctx context.Context, src driver.LogSource, cfg ScanConfig) (*Index, error) {
	uri := src.URI()

	// Check cache — also verify staleness via source size.
	_, currentSize, err := src.Reader(ctx)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	if idx, ok := c.entries[uri]; ok && idx.SourceSize == currentSize {
		c.mu.Unlock()
		return idx, nil
	}
	c.mu.Unlock()

	// Build via singleflight — only one goroutine builds per URI+size.
	key := fmt.Sprintf("%s@%d", uri, currentSize)
	v, err, _ := c.group.Do(key, func() (any, error) {
		idx, err := BuildIndex(ctx, src, cfg)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		c.entries[uri] = idx
		c.mu.Unlock()
		return idx, nil
	})
	if err != nil {
		return nil, err
	}

	return v.(*Index), nil
}
