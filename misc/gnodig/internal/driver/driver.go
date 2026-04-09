package driver

import (
	"context"
	"fmt"
	"io"
	"net/url"
)

type LogSource interface {
	Reader(ctx context.Context) (io.ReaderAt, int64, error)
	URI() string
	Close() error
}

type Resolver func(uri string) (LogSource, error)

func DefaultResolvers() map[string]Resolver {
	return make(map[string]Resolver)
}

func ResolveURI(uri string, resolvers map[string]Resolver) (LogSource, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid target URI %q: %w", uri, err)
	}

	scheme := u.Scheme
	// Bare paths (no scheme) default to "file".
	if scheme == "" {
		scheme = "file"
	}

	r, ok := resolvers[scheme]
	if !ok {
		return nil, fmt.Errorf("unsupported URI scheme %q (from %q)", scheme, uri)
	}
	return r(uri)
}
