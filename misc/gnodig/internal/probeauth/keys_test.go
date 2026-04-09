package probeauth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

// ---- Test helpers

func testKeypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return pub, priv
}

func writeKeysFile(t *testing.T, lines []string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "authorized_keys")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		t.Fatalf("write keys file: %v", err)
	}
	return f.Name()
}

func keyLine(pub ed25519.PublicKey, comment string) string {
	encoded := base64.StdEncoding.EncodeToString(pub)
	if comment != "" {
		return "ssh-ed25519 " + encoded + " " + comment
	}
	return "ssh-ed25519 " + encoded
}

// ---- Tests

func TestParseAuthorizedKeys(t *testing.T) {
	pub1, _ := testKeypair(t)
	pub2, _ := testKeypair(t)

	path := writeKeysFile(t, []string{
		keyLine(pub1, "alice@host"),
		keyLine(pub2, "bob@host"),
	})

	ks, err := ParseAuthorizedKeys(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ks.Len() != 2 {
		t.Fatalf("expected 2 keys, got %d", ks.Len())
	}
	if !ks.Contains(pub1) {
		t.Error("expected pub1 to be contained")
	}
	if !ks.Contains(pub2) {
		t.Error("expected pub2 to be contained")
	}
	if got := ks.Comment(pub1); got != "alice@host" {
		t.Errorf("expected comment 'alice@host', got %q", got)
	}
	if got := ks.Comment(pub2); got != "bob@host" {
		t.Errorf("expected comment 'bob@host', got %q", got)
	}
}

func TestParseAuthorizedKeys_EmptyFile(t *testing.T) {
	path := writeKeysFile(t, []string{})

	ks, err := ParseAuthorizedKeys(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ks.Len() != 0 {
		t.Fatalf("expected 0 keys, got %d", ks.Len())
	}
}

func TestParseAuthorizedKeys_BadAlgorithm(t *testing.T) {
	pub, _ := testKeypair(t)
	encoded := base64.StdEncoding.EncodeToString(pub)
	path := writeKeysFile(t, []string{
		"ssh-rsa " + encoded + " comment",
	})

	_, err := ParseAuthorizedKeys(path)
	if err == nil {
		t.Fatal("expected error for bad algorithm, got nil")
	}
}

func TestParseAuthorizedKeys_BadBase64(t *testing.T) {
	path := writeKeysFile(t, []string{
		"ssh-ed25519 not-valid-base64!! comment",
	})

	_, err := ParseAuthorizedKeys(path)
	if err == nil {
		t.Fatal("expected error for bad base64, got nil")
	}
}

func TestParseAuthorizedKeys_WrongKeyLength(t *testing.T) {
	// encode something that is valid base64 but not 32 bytes
	tooShort := base64.StdEncoding.EncodeToString([]byte("tooshort"))
	path := writeKeysFile(t, []string{
		"ssh-ed25519 " + tooShort + " comment",
	})

	_, err := ParseAuthorizedKeys(path)
	if err == nil {
		t.Fatal("expected error for wrong key length, got nil")
	}
}

func TestParseAuthorizedKeys_UnknownKeyNotContained(t *testing.T) {
	pub1, _ := testKeypair(t)
	pub2, _ := testKeypair(t) // never written to file

	path := writeKeysFile(t, []string{
		keyLine(pub1, "alice@host"),
	})

	ks, err := ParseAuthorizedKeys(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ks.Contains(pub2) {
		t.Error("expected pub2 NOT to be contained")
	}
}

// ---- KeyStore tests

func TestKeyStore_Reload(t *testing.T) {
	pub1, _ := testKeypair(t)
	pub2, _ := testKeypair(t)

	path := writeKeysFile(t, []string{
		keyLine(pub1, "alice@host"),
	})

	store, err := NewKeyStore(path)
	if err != nil {
		t.Fatalf("NewKeyStore: %v", err)
	}

	if !store.Contains(pub1) {
		t.Error("expected pub1 before reload")
	}
	if store.Contains(pub2) {
		t.Error("expected pub2 absent before reload")
	}

	// Rewrite file with pub2 added
	if err := os.WriteFile(path, []byte(keyLine(pub1, "alice@host")+"\n"+keyLine(pub2, "bob@host")+"\n"), 0o600); err != nil {
		t.Fatalf("rewrite keys file: %v", err)
	}

	if err := store.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	if !store.Contains(pub1) {
		t.Error("expected pub1 after reload")
	}
	if !store.Contains(pub2) {
		t.Error("expected pub2 after reload")
	}
}

func TestKeyStore_ReloadBadFile_KeepsOld(t *testing.T) {
	pub1, _ := testKeypair(t)

	path := writeKeysFile(t, []string{
		keyLine(pub1, "alice@host"),
	})

	store, err := NewKeyStore(path)
	if err != nil {
		t.Fatalf("NewKeyStore: %v", err)
	}

	// Overwrite file with invalid content
	if err := os.WriteFile(path, []byte("ssh-rsa badkey comment\n"), 0o600); err != nil {
		t.Fatalf("rewrite keys file: %v", err)
	}

	if err := store.Reload(); err == nil {
		t.Fatal("expected error on bad file reload, got nil")
	}

	// Original key must still be accessible
	if !store.Contains(pub1) {
		t.Error("expected old key to remain after failed reload")
	}
}
