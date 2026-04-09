package logengine_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gnolang/gno/misc/gnodig/internal/driver/localfs"
	"github.com/gnolang/gno/misc/gnodig/internal/logengine"
)

// buildTestIndex creates a temp file with testLines(n) content, opens it as a
// LogSource, builds an index, and returns the source and index. The caller is
// responsible for calling src.Close().
func buildTestIndex(t *testing.T, n int) (*localfs.Source, *logengine.Index) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	content := testLines(n)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	src, err := localfs.New(path)
	if err != nil {
		t.Fatal(err)
	}

	idx, err := logengine.BuildIndex(context.Background(), src, logengine.ScanConfig{
		BlockSize:   512,
		Concurrency: 2,
	})
	if err != nil {
		src.Close()
		t.Fatal(err)
	}

	return src, idx
}

func TestQuery_SearchByLevel(t *testing.T) {
	src, idx := buildTestIndex(t, 100)
	defer src.Close()

	q := logengine.Query{Level: logengine.LevelError}
	results, err := logengine.Search(context.Background(), src, idx, q)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) == 0 {
		t.Fatal("expected error-level results, got none")
	}

	for _, entry := range results {
		if entry.Level != "error" {
			t.Errorf("expected level=error, got %q (line: %s)", entry.Level, entry.Line)
		}
	}

	// testLines generates error for every 5th line (0, 5, 10, ..., 95): 20 errors in 100 lines.
	if len(results) != 20 {
		t.Errorf("expected 20 error results, got %d", len(results))
	}
}

func TestQuery_SearchByText(t *testing.T) {
	src, idx := buildTestIndex(t, 100)
	defer src.Close()

	q := logengine.Query{Text: "line 42"}
	results, err := logengine.Search(context.Background(), src, idx, q)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'line 42', got %d", len(results))
	}

	if results[0].Level != "info" {
		t.Errorf("expected level=info for line 42, got %q", results[0].Level)
	}
}

func TestQuery_NoMatches(t *testing.T) {
	src, idx := buildTestIndex(t, 100)
	defer src.Close()

	q := logengine.Query{Text: "this text does not exist in any log line"}
	results, err := logengine.Search(context.Background(), src, idx, q)
	if err != nil {
		t.Fatal(err)
	}

	if results == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchModuleFilter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mod.jsonl")

	lines := `{"ts":"2025-01-01T00:00:00Z","level":"info","module":"consensus","msg":"round started"}
{"ts":"2025-01-01T00:00:01Z","level":"info","module":"p2p","msg":"peer connected"}
{"ts":"2025-01-01T00:00:02Z","level":"info","module":"consensus","msg":"block committed"}
{"ts":"2025-01-01T00:00:03Z","level":"info","module":"mempool","msg":"tx added"}
`

	if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}

	src, err := localfs.New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	ctx := context.Background()
	idx, err := logengine.BuildIndex(ctx, src, logengine.ScanConfig{
		BlockSize:   4096,
		Concurrency: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("include module", func(t *testing.T) {
		q := logengine.Query{Module: "consensus"}
		results, err := logengine.Search(ctx, src, idx, q)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 2 {
			t.Errorf("expected 2 consensus results, got %d", len(results))
		}
	})

	t.Run("exclude module", func(t *testing.T) {
		q := logengine.Query{ExcludeModule: "consensus"}
		results, err := logengine.Search(ctx, src, idx, q)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 2 {
			t.Errorf("expected 2 non-consensus results, got %d", len(results))
		}
		for _, entry := range results {
			if entry.Line == "" {
				continue
			}
			mod := logengine.ExtractModule([]byte(entry.Line))
			if mod == "consensus" {
				t.Errorf("expected consensus to be excluded, got line: %s", entry.Line)
			}
		}
	})
}

func TestQuery_Summarize(t *testing.T) {
	src, idx := buildTestIndex(t, 100)
	defer src.Close()

	s := logengine.Summarize(idx)

	if s.TotalBytes != idx.SourceSize {
		t.Errorf("TotalBytes = %d, want %d", s.TotalBytes, idx.SourceSize)
	}

	if s.BlockCount != len(idx.Blocks) {
		t.Errorf("BlockCount = %d, want %d", s.BlockCount, len(idx.Blocks))
	}

	if s.TimeMin == "" {
		t.Error("expected non-empty TimeMin")
	}

	// Every block has info-level lines, so BlocksWithLevel["info"] == BlockCount.
	if s.BlocksWithLevel["info"] != s.BlockCount {
		t.Errorf("BlocksWithLevel[info] = %d, want %d", s.BlocksWithLevel["info"], s.BlockCount)
	}
}
