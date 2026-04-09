package probeauth

import (
	"bufio"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
)

const keyAlgorithm = "ssh-ed25519"

// KeyEntry holds a parsed authorized key and its comment.
type KeyEntry struct {
	PubKey  ed25519.PublicKey
	Comment string
}

// KeySet is an immutable set of authorized public keys.
type KeySet struct {
	entries []KeyEntry
	byKey   map[string]int // string(pubkey raw bytes) → index
}

// Len returns the number of authorized keys.
func (ks *KeySet) Len() int { return len(ks.entries) }

// Contains reports whether pub is in the set.
func (ks *KeySet) Contains(pub ed25519.PublicKey) bool {
	_, ok := ks.byKey[string(pub)]
	return ok
}

// Comment returns the comment associated with pub, or "" if not found.
func (ks *KeySet) Comment(pub ed25519.PublicKey) string {
	idx, ok := ks.byKey[string(pub)]
	if !ok {
		return ""
	}
	return ks.entries[idx].Comment
}

// Entries returns a copy of all key entries.
func (ks *KeySet) Entries() []KeyEntry {
	out := make([]KeyEntry, len(ks.entries))
	copy(out, ks.entries)
	return out
}

// ParseAuthorizedKeys reads a gnodig key file and returns a KeySet. The format
// resembles SSH authorized_keys but the base64 payload is the raw 32-byte
// ed25519 public key, NOT the SSH wire format. Use "gnodig probe keys add" to
// generate correctly formatted entries.
// Each non-empty line must be of the form: ssh-ed25519 <base64-pubkey> [comment]
// Lines starting with '#' are silently skipped.
func ParseAuthorizedKeys(path string) (*KeySet, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open authorized_keys: %w", err)
	}
	defer f.Close()

	ks := &KeySet{
		byKey: make(map[string]int),
	}

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		entry, err := parseKeyLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}

		idx := len(ks.entries)
		ks.entries = append(ks.entries, entry)
		ks.byKey[string(entry.PubKey)] = idx
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read authorized_keys: %w", err)
	}
	return ks, nil
}

func parseKeyLine(line string) (KeyEntry, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return KeyEntry{}, fmt.Errorf("invalid key line: expected at least 2 fields")
	}

	algo, encoded := fields[0], fields[1]
	if algo != keyAlgorithm {
		return KeyEntry{}, fmt.Errorf("unsupported algorithm %q: only %s is supported", algo, keyAlgorithm)
	}

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return KeyEntry{}, fmt.Errorf("base64 decode: %w", err)
	}

	if len(raw) != ed25519.PublicKeySize {
		return KeyEntry{}, fmt.Errorf("invalid key length: expected %d bytes, got %d", ed25519.PublicKeySize, len(raw))
	}

	var comment string
	if len(fields) > 2 {
		comment = strings.Join(fields[2:], " ")
	}

	return KeyEntry{
		PubKey:  ed25519.PublicKey(raw),
		Comment: comment,
	}, nil
}

// ---- KeyStore: hot-reloadable authorized keys

// KeyStore wraps a KeySet with atomic hot-reload from a file.
type KeyStore struct {
	path string
	keys atomic.Pointer[KeySet]
}

// NewKeyStore loads the keys file and returns a KeyStore.
// Call Reload to refresh from disk.
func NewKeyStore(path string) (*KeyStore, error) {
	ks, err := ParseAuthorizedKeys(path)
	if err != nil {
		return nil, err
	}
	s := &KeyStore{path: path}
	s.keys.Store(ks)
	return s, nil
}

// Contains reports whether pub is in the current key set.
func (s *KeyStore) Contains(pub ed25519.PublicKey) bool {
	return s.keys.Load().Contains(pub)
}

// Comment returns the comment for pub in the current key set.
func (s *KeyStore) Comment(pub ed25519.PublicKey) string {
	return s.keys.Load().Comment(pub)
}

// Reload re-reads the keys file and atomically replaces the current set.
// On error, the existing set is preserved.
func (s *KeyStore) Reload() error {
	ks, err := ParseAuthorizedKeys(s.path)
	if err != nil {
		return err
	}
	s.keys.Store(ks)
	return nil
}
