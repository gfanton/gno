package logengine_test

import (
	"path/filepath"
	"testing"

	"github.com/gnolang/gno/misc/gnodig/internal/logengine"
)

func TestIndex_RoundTrip(t *testing.T) {
	blocks := []logengine.BlockMeta{
		{Offset: 0, Size: 1024, TsMin: 1000, TsMax: 2000, LevelFlags: logengine.LevelInfo, Hash: 0xdeadbeef},
		{Offset: 1024, Size: 2048, TsMin: 2001, TsMax: 3000, LevelFlags: logengine.LevelError | logengine.LevelWarn, Hash: 0xcafebabe},
	}
	idx := &logengine.Index{SourceSize: 3072, Blocks: blocks}

	dir := t.TempDir()
	path := filepath.Join(dir, "test.gdx")

	if err := idx.WriteTo(path); err != nil {
		t.Fatal(err)
	}

	got, err := logengine.ReadIndex(path)
	if err != nil {
		t.Fatal(err)
	}

	if got.SourceSize != idx.SourceSize {
		t.Errorf("SourceSize = %d, want %d", got.SourceSize, idx.SourceSize)
	}
	if len(got.Blocks) != len(idx.Blocks) {
		t.Fatalf("len(Blocks) = %d, want %d", len(got.Blocks), len(idx.Blocks))
	}
	for i, b := range got.Blocks {
		if b != idx.Blocks[i] {
			t.Errorf("Blocks[%d] = %+v, want %+v", i, b, idx.Blocks[i])
		}
	}
}

func TestIndex_FileNotFound(t *testing.T) {
	_, err := logengine.ReadIndex("/nonexistent/path.gdx")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestIndex_EmptyBlocks(t *testing.T) {
	idx := &logengine.Index{SourceSize: 0, Blocks: nil}
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.gdx")

	if err := idx.WriteTo(path); err != nil {
		t.Fatal(err)
	}
	got, err := logengine.ReadIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Blocks) != 0 {
		t.Errorf("expected 0 blocks, got %d", len(got.Blocks))
	}
}
