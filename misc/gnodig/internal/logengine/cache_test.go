package logengine_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gnolang/gno/misc/gnodig/internal/driver/localfs"
	"github.com/gnolang/gno/misc/gnodig/internal/logengine"
)

func TestCache_HitAndMiss(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	content := testLines(20)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	src, err := localfs.New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	cache := logengine.NewCache()
	cfg := logengine.ScanConfig{BlockSize: 512, Concurrency: 1}

	idx1, err := cache.GetOrBuild(context.Background(), src, cfg)
	if err != nil {
		t.Fatal(err)
	}

	idx2, err := cache.GetOrBuild(context.Background(), src, cfg)
	if err != nil {
		t.Fatal(err)
	}

	if idx1 != idx2 {
		t.Error("expected cache hit — got different index pointers")
	}
}

func TestCache_ConcurrentSameURI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	content := testLines(50)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cache := logengine.NewCache()
	cfg := logengine.ScanConfig{BlockSize: 512, Concurrency: 1}

	const goroutines = 10
	var wg sync.WaitGroup
	indexes := make([]*logengine.Index, goroutines)
	var errors atomic.Int32

	for i := range goroutines {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			src, err := localfs.New(path)
			if err != nil {
				errors.Add(1)
				return
			}
			defer src.Close()

			idx, err := cache.GetOrBuild(context.Background(), src, cfg)
			if err != nil {
				errors.Add(1)
				return
			}
			indexes[i] = idx
		}(i)
	}

	wg.Wait()

	if errors.Load() > 0 {
		t.Fatalf("%d goroutines failed", errors.Load())
	}

	for i := 1; i < goroutines; i++ {
		if indexes[i] != indexes[0] {
			t.Errorf("goroutine %d got different index pointer", i)
		}
	}
}

func TestCache_StalenessCheck(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	content := testLines(10)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cache := logengine.NewCache()
	cfg := logengine.ScanConfig{BlockSize: 4096, Concurrency: 1}

	src1, _ := localfs.New(path)
	idx1, err := cache.GetOrBuild(context.Background(), src1, cfg)
	src1.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Append more data — file size changes.
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString(testLines(10))
	f.Close()

	src2, _ := localfs.New(path)
	idx2, err := cache.GetOrBuild(context.Background(), src2, cfg)
	src2.Close()
	if err != nil {
		t.Fatal(err)
	}

	if idx1 == idx2 {
		t.Error("expected cache miss after file size change — got same pointer")
	}
	if idx2.SourceSize <= idx1.SourceSize {
		t.Error("rebuilt index should have larger SourceSize")
	}
}
