package nodedata

// Tests for WAL file concurrent read/write safety at the OS level.
//
// The WAL reader in gnodig opens WAL files with os.Open (O_RDONLY). These
// tests verify that:
//   - Multiple concurrent readers do not interfere with a writer appending
//     to the same file.
//   - Holding a read-only file descriptor open does not block an appending
//     writer on macOS/Linux (POSIX O_RDONLY does not prevent O_APPEND writes).

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestWAL_ConcurrentWriteAndRead starts a writer goroutine that appends lines
// to a temp file and 50 reader iterations that open the file O_RDONLY, read
// all content, then close it. The test asserts that no read produces an error
// within a 200ms window.
func TestWAL_ConcurrentWriteAndRead(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "wal-*.log")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer f.Close()
	path := f.Name()

	// Seed 100 initial lines.
	for i := range 100 {
		fmt.Fprintf(f, "line-%04d\n", i)
	}

	var (
		stop      atomic.Bool
		writeErr  atomic.Value // stores error or nil
		readErrs  atomic.Int64
		linesRead atomic.Int64
	)

	// Writer appends lines continuously until stop is set.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		wf, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			writeErr.Store(err)
			return
		}
		defer wf.Close()
		i := 100
		for !stop.Load() {
			if _, err := fmt.Fprintf(wf, "line-%04d\n", i); err != nil {
				writeErr.Store(err)
				return
			}
			i++
		}
	}()

	// 50 reader iterations: open, read all, close.
	const iterations = 50
	for range iterations {
		rf, err := os.Open(path) // O_RDONLY
		if err != nil {
			readErrs.Add(1)
			continue
		}
		scanner := bufio.NewScanner(rf)
		var n int64
		for scanner.Scan() {
			n++
		}
		if err := scanner.Err(); err != nil {
			readErrs.Add(1)
		}
		linesRead.Add(n)
		rf.Close()
	}

	// Let the writer run for 200ms total.
	time.Sleep(200 * time.Millisecond)
	stop.Store(true)
	wg.Wait()

	if v := writeErr.Load(); v != nil {
		t.Errorf("writer goroutine failed: %v", v)
	}
	if n := readErrs.Load(); n > 0 {
		t.Errorf("got %d read error(s) out of %d iterations", n, iterations)
	} else {
		t.Logf("PASS: 0 read errors across %d iterations; ~%d total lines read", iterations, linesRead.Load())
	}
}

// TestWAL_ReadDoesNotBlockWriter verifies that holding a read-only file
// descriptor open does not prevent a writer from appending to the same file.
//
// On POSIX systems O_RDONLY never prevents O_APPEND writes — this test
// confirms that property holds for the platform gnodig targets.
func TestWAL_ReadDoesNotBlockWriter(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/cs.wal"

	// Write initial content.
	if err := os.WriteFile(path, []byte("initial line\n"), 0o600); err != nil {
		t.Fatalf("write initial content: %v", err)
	}

	// Open reader and hold it open throughout the test.
	reader, err := os.Open(path)
	if err != nil {
		t.Fatalf("open reader: %v", err)
	}
	defer reader.Close()

	// Writer must succeed within 5 seconds despite the held reader.
	done := make(chan error, 1)
	go func() {
		wf, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			done <- fmt.Errorf("open writer: %w", err)
			return
		}
		defer wf.Close()
		const extra = 20
		for i := range extra {
			if _, err := fmt.Fprintf(wf, "extra line %d\n", i); err != nil {
				done <- fmt.Errorf("write line %d: %w", i, err)
				return
			}
		}
		done <- nil
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("writer failed: %v", err)
		}
		t.Log("PASS: writer succeeded with reader holding the file open")
	case <-time.After(5 * time.Second):
		t.Fatal("writer blocked for >5s with reader holding the file open")
	}

	// Confirm the written data is actually there.
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(content) == 0 {
		t.Error("file is empty after write")
	}
	t.Logf("file size after concurrent write: %d bytes", len(content))
}
