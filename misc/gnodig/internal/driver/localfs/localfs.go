package localfs

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/gnolang/gno/misc/gnodig/internal/driver"
)

type Source struct {
	path string
	file *os.File
}

func New(path string) (*Source, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", path, err)
	}
	return &Source{path: path, file: f}, nil
}

func NewFromURI(uri string) (driver.LogSource, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("parse URI %q: %w", uri, err)
	}

	if u.Scheme != "" && u.Scheme != "file" {
		return nil, fmt.Errorf("expected file:// URI, got %q", uri)
	}

	// Reconstruct the path from the parsed URL components.
	// file://relative/path → Host="relative", Path="/path"
	// file:///absolute/path → Host="", Path="/absolute/path"
	// bare/path → Scheme="", Opaque="", Path="bare/path"
	path := u.Path
	if u.Host != "" {
		path = u.Host + path
	}
	if path == "" {
		path = u.Opaque // handles edge cases
	}
	if path == "" {
		return nil, fmt.Errorf("empty path in URI %q", uri)
	}

	return New(path)
}

func (s *Source) Reader(_ context.Context) (io.ReaderAt, int64, error) {
	info, err := s.file.Stat()
	if err != nil {
		return nil, 0, err
	}
	return s.file, info.Size(), nil
}

func (s *Source) URI() string {
	return "file://" + s.path
}

func (s *Source) Close() error {
	return s.file.Close()
}
