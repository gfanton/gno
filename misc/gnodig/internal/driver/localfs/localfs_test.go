package localfs_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gnolang/gno/misc/gnodig/internal/driver/localfs"
)

func TestSource_ReaderAt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	content := "{\"ts\":\"2025-01-01T00:00:00Z\",\"level\":\"info\",\"msg\":\"hello\"}\n{\"ts\":\"2025-01-01T00:00:01Z\",\"level\":\"error\",\"msg\":\"boom\"}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	src, err := localfs.New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	if got := src.URI(); got != "file://"+path {
		t.Errorf("URI = %q, want %q", got, "file://"+path)
	}

	r, size, err := src.Reader(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if size != int64(len(content)) {
		t.Errorf("size = %d, want %d", size, len(content))
	}

	buf := make([]byte, 10)
	n, err := r.ReadAt(buf, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(buf[:n]); got != "{\"ts\":\"202" {
		t.Errorf("ReadAt(0) = %q, want %q", got, "{\"ts\":\"202")
	}
}

func TestSource_FromURI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	if err := os.WriteFile(path, []byte("{\"msg\":\"ok\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	src, err := localfs.NewFromURI("file://" + path)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	_, size, err := src.Reader(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if size == 0 {
		t.Error("expected non-zero size")
	}
}

func TestSource_FromURI_InvalidScheme(t *testing.T) {
	_, err := localfs.NewFromURI("http://example.com")
	if err == nil {
		t.Error("expected error for non-file URI")
	}
}
