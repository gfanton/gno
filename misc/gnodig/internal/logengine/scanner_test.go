package logengine_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gnolang/gno/misc/gnodig/internal/driver/localfs"
	"github.com/gnolang/gno/misc/gnodig/internal/logengine"
)

func testLines(n int) string {
	var b strings.Builder
	for i := range n {
		level := "info"
		if i%5 == 0 {
			level = "error"
		}
		fmt.Fprintf(&b, `{"ts":"2025-01-01T00:00:%02d.%09dZ","level":"%s","msg":"line %d"}`+"\n",
			i/1000000000, i%1000000000, level, i)
	}
	return b.String()
}

func TestScanner_BuildIndex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	content := testLines(100)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	src, err := localfs.New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	idx, err := logengine.BuildIndex(context.Background(), src, logengine.ScanConfig{
		BlockSize:   512,
		Concurrency: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(idx.Blocks) == 0 {
		t.Fatal("expected at least one block")
	}

	var totalSize uint64
	for i, b := range idx.Blocks {
		if b.Offset != totalSize {
			t.Errorf("block %d: offset = %d, want %d", i, b.Offset, totalSize)
		}
		totalSize += uint64(b.Size)
		if b.TsMin == 0 || b.TsMax == 0 {
			t.Errorf("block %d: missing timestamps", i)
		}
		if b.TsMin > b.TsMax {
			t.Errorf("block %d: TsMin (%d) > TsMax (%d)", i, b.TsMin, b.TsMax)
		}
		if !logengine.HasLevelOrAbove(b.LevelFlags, logengine.LevelInfo) {
			t.Errorf("block %d: expected LevelInfo flag", i)
		}
	}

	if int64(totalSize) != idx.SourceSize {
		t.Errorf("total block size %d != source size %d", totalSize, idx.SourceSize)
	}
}

func TestScanner_CarryOver(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	longMsg := strings.Repeat("x", 300)
	content := fmt.Sprintf(`{"ts":"2025-01-01T00:00:00Z","level":"info","msg":"%s"}`+"\n", longMsg)
	content += `{"ts":"2025-01-01T00:00:01Z","level":"error","msg":"short"}` + "\n"

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	src, err := localfs.New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	idx, err := logengine.BuildIndex(context.Background(), src, logengine.ScanConfig{
		BlockSize:   128,
		Concurrency: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	var totalSize uint64
	for _, b := range idx.Blocks {
		totalSize += uint64(b.Size)
	}
	if int64(totalSize) != idx.SourceSize {
		t.Errorf("total block size %d != source size %d", totalSize, idx.SourceSize)
	}

	var combinedFlags uint16
	for _, b := range idx.Blocks {
		combinedFlags |= b.LevelFlags
	}
	if combinedFlags&logengine.LevelInfo == 0 {
		t.Error("expected LevelInfo across blocks")
	}
	if combinedFlags&logengine.LevelError == 0 {
		t.Error("expected LevelError across blocks")
	}
}

func TestScanner_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	src, err := localfs.New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	idx, err := logengine.BuildIndex(context.Background(), src, logengine.ScanConfig{
		BlockSize: 512, Concurrency: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Blocks) != 0 {
		t.Errorf("expected 0 blocks for empty file, got %d", len(idx.Blocks))
	}
}

func TestScanner_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.jsonl")
	content := "not json at all\n" +
		`{"ts":"2025-01-01T00:00:00Z","level":"info","msg":"ok"}` + "\n" +
		"also not json\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	src, err := localfs.New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	idx, err := logengine.BuildIndex(context.Background(), src, logengine.ScanConfig{
		BlockSize: 4096, Concurrency: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Blocks) == 0 {
		t.Fatal("expected at least one block")
	}
}

func TestScanner_SingleLineNoNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "single.jsonl")
	content := `{"ts":"2025-01-01T00:00:00Z","level":"info","msg":"no trailing newline"}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	src, err := localfs.New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	idx, err := logengine.BuildIndex(context.Background(), src, logengine.ScanConfig{
		BlockSize: 4096, Concurrency: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Blocks) == 0 {
		t.Fatal("expected at least one block")
	}
	if idx.Blocks[0].LevelFlags&logengine.LevelInfo == 0 {
		t.Error("expected LevelInfo flag")
	}
}
