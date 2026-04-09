package nodedata

// Tests for PebbleDB secondary (read-only) mode behaviour.
//
// PebbleDB (github.com/cockroachdb/pebble) supports opening a database in
// read-only mode via pebble.Options{ReadOnly: true}. This test suite probes
// the actual locking semantics:
//
// Findings:
//   - ReadOnly: true still acquires the directory lock file.
//   - Two handles pointing at the same path CANNOT co-exist in the same OS
//     process regardless of ReadOnly: the second Open returns
//     "lock held by current process".
//   - ReadOnly succeeds after the primary handle is closed (lock released).
//   - Implication for gnodig: a live node holds the PebbleDB lock exclusively.
//     gnodig can only open the node's data directory when the node is stopped,
//     or it must use a separate process (cross-process locking not tested here).

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/gnolang/gno/tm2/pkg/db/pebbledb"
)

// TestPebbleDB_SecondaryReadsWhilePrimaryWrites probes whether a read-only
// PebbleDB handle can co-exist with an open primary handle in the same process.
//
// Finding: it cannot — PebbleDB uses a single OS lock per directory regardless
// of ReadOnly. Opening a second handle (read-only or not) while the primary is
// open in the same process fails with "lock held by current process".
func TestPebbleDB_SecondaryReadsWhilePrimaryWrites(t *testing.T) {
	dir := t.TempDir()
	const dbName = "probe"
	const nKeys = 100

	// Open primary and write 100 keys with Sync so they are durable.
	primary, err := pebbledb.NewPebbleDB(dbName, dir)
	if err != nil {
		t.Fatalf("open primary: %v", err)
	}
	defer primary.Close()

	for i := range nKeys {
		key := []byte(fmt.Sprintf("key-%04d", i))
		val := []byte(fmt.Sprintf("val-%04d", i))
		if err := primary.SetSync(key, val); err != nil {
			t.Fatalf("primary SetSync key %d: %v", i, err)
		}
	}

	// Attempt to open read-only while primary is still open.
	_, openErr := pebbledb.NewPebbleDBWithOpts(dbName, dir, &pebble.Options{ReadOnly: true})
	if openErr == nil {
		t.Fatal("expected error opening read-only while primary is open, but Open succeeded")
	}

	// Document the exact error for future reference.
	t.Logf("FINDING: concurrent read-only open rejected: %v", openErr)
	t.Logf("IMPLICATION: gnodig cannot open a live node's PebbleDB in the same process")
}

// TestPebbleDB_SecondaryAfterPrimaryCloses verifies the baseline (and only
// viable) case: open read-only after the primary handle is closed.
//
// Finding: this works correctly — all 50 keys are readable.
func TestPebbleDB_SecondaryAfterPrimaryCloses(t *testing.T) {
	dir := t.TempDir()
	const dbName = "probe"
	const nKeys = 50

	// Write via primary, then close to release the lock.
	{
		db, err := pebbledb.NewPebbleDB(dbName, dir)
		if err != nil {
			t.Fatalf("open primary: %v", err)
		}
		for i := range nKeys {
			key := []byte(fmt.Sprintf("key-%04d", i))
			val := []byte(fmt.Sprintf("val-%04d", i))
			if err := db.SetSync(key, val); err != nil {
				db.Close()
				t.Fatalf("Set key %d: %v", i, err)
			}
		}
		if err := db.Close(); err != nil {
			t.Fatalf("close primary: %v", err)
		}
	}

	// Open read-only; lock is now free.
	dbPath := filepath.Join(dir, dbName+".db")
	ro, err := pebble.Open(dbPath, &pebble.Options{ReadOnly: true})
	if err != nil {
		t.Fatalf("open read-only after primary closed: %v", err)
	}
	defer ro.Close()

	missing := 0
	for i := range nKeys {
		key := []byte(fmt.Sprintf("key-%04d", i))
		got, closer, err := ro.Get(key)
		if err != nil {
			t.Errorf("Get key %d: %v", i, err)
			missing++
			continue
		}
		want := fmt.Sprintf("val-%04d", i)
		if string(got) != want {
			t.Errorf("key %d: want %q got %q", i, want, string(got))
			missing++
		}
		closer.Close()
	}

	if missing > 0 {
		t.Errorf("%d/%d keys missing or wrong in read-only handle", missing, nKeys)
	} else {
		t.Logf("PASS: read-only handle after primary close read all %d keys correctly", nKeys)
	}
}

// TestPebbleDB_ConcurrentPrimaryWriteSecondaryRead documents that opening a
// read-only secondary while the primary is open fails in the same process.
// This mirrors the real-world gnodig scenario: trying to inspect a running
// node's data directory without stopping the node.
//
// Finding: PebbleDB v1.1.5 does NOT support a secondary/snapshot reader pattern
// for same-process concurrent access. Each directory may have only one open
// pebble.DB handle at a time within the same process.
func TestPebbleDB_ConcurrentPrimaryWriteSecondaryRead(t *testing.T) {
	dir := t.TempDir()
	const dbName = "probe"
	const phase1Keys = 50
	const phase2Keys = 50

	// Phase 1: write first batch via primary.
	primary, err := pebbledb.NewPebbleDB(dbName, dir)
	if err != nil {
		t.Fatalf("open primary: %v", err)
	}
	defer primary.Close()

	for i := range phase1Keys {
		key := []byte(fmt.Sprintf("key-%04d", i))
		val := []byte(fmt.Sprintf("val-%04d", i))
		if err := primary.SetSync(key, val); err != nil {
			t.Fatalf("primary SetSync phase1 key %d: %v", i, err)
		}
	}

	// Attempt to open secondary read-only while primary is open — must fail.
	_, secondErr := pebbledb.NewPebbleDBWithOpts(dbName, dir, &pebble.Options{ReadOnly: true})
	if secondErr == nil {
		t.Fatal("expected secondary open to fail while primary is open")
	}
	t.Logf("FINDING: secondary open while primary holds lock: %v", secondErr)

	// Phase 2: primary continues writing without issue.
	for i := phase1Keys; i < phase1Keys+phase2Keys; i++ {
		key := []byte(fmt.Sprintf("key-%04d", i))
		val := []byte(fmt.Sprintf("val-%04d", i))
		if err := primary.SetSync(key, val); err != nil {
			t.Fatalf("primary SetSync phase2 key %d: %v", i, err)
		}
	}

	// Verify the primary still has all keys after the failed secondary open.
	for i := range phase1Keys + phase2Keys {
		key := []byte(fmt.Sprintf("key-%04d", i))
		got, err := primary.Get(key)
		if err != nil || got == nil {
			t.Errorf("primary missing key %d after secondary open attempt", i)
		}
	}
	t.Logf("PASS: primary unaffected by failed secondary open; all %d keys intact", phase1Keys+phase2Keys)
}
