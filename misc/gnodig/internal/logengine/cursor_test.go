package logengine_test

import (
	"context"
	"testing"

	"github.com/gnolang/gno/misc/gnodig/internal/logengine"
)

func TestCursor_ReadForward(t *testing.T) {
	src, idx := buildTestIndex(t, 20)
	defer src.Close()

	r, _, err := src.Reader(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	cur := logengine.NewCursor(r, idx, 0)

	entries, err := cur.Read(5)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 5 {
		t.Fatalf("Read(5) returned %d entries, want 5", len(entries))
	}

	entries2, err := cur.Read(5)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries2) != 5 {
		t.Fatalf("Read(5) returned %d entries, want 5", len(entries2))
	}

	if entries[0].Line == entries2[0].Line {
		t.Error("cursor did not advance — same first line after two reads")
	}
}

func TestCursor_SeekToOffset(t *testing.T) {
	src, idx := buildTestIndex(t, 20)
	defer src.Close()

	r, _, err := src.Reader(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	cur := logengine.NewCursor(r, idx, 0)

	entries, _ := cur.Read(1)
	firstLine := entries[0].Line

	cur.Read(5) //nolint

	cur.SeekOffset(0)
	entries, _ = cur.Read(1)
	if entries[0].Line != firstLine {
		t.Error("seek back to offset 0 did not return to first line")
	}
}

func TestCursor_ReadAtEOF(t *testing.T) {
	src, idx := buildTestIndex(t, 20)
	defer src.Close()

	r, size, err := src.Reader(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	cur := logengine.NewCursor(r, idx, size)
	entries, err := cur.Read(5)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries at EOF, got %d", len(entries))
	}
}
