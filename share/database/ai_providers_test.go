package database

import (
	"bytes"
	"encoding/base64"
	"sync"
	"testing"
)

// resetEncryptionKeyForTest forces the package back to its pre-init state so
// each test can configure the key deterministically.
func resetEncryptionKeyForTest(t *testing.T) {
	t.Helper()
	encryptionKeyMu.Lock()
	encryptionKey = nil
	encryptionKeyMu.Unlock()
	t.Setenv(EncryptionKeyEnvVar, "")
}

func TestSetEncryptionKeyValidation(t *testing.T) {
	resetEncryptionKeyForTest(t)

	if err := SetEncryptionKey([]byte("too-short")); err == nil {
		t.Fatal("expected error for short key")
	}

	exact := bytes.Repeat([]byte("k"), 32)
	if err := SetEncryptionKey(exact); err != nil {
		t.Fatalf("32-byte key rejected: %v", err)
	}
	if !bytes.Equal(currentEncryptionKey(), exact) {
		t.Fatal("32-byte key was not used verbatim")
	}

	long := []byte("a much longer passphrase that is certainly more than thirty-two bytes")
	if err := SetEncryptionKey(long); err != nil {
		t.Fatalf("long passphrase rejected: %v", err)
	}
	if got := currentEncryptionKey(); len(got) != 32 || bytes.Equal(got, long[:32]) {
		t.Fatalf("long passphrase should be hashed to 32 bytes, got %d bytes", len(got))
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	resetEncryptionKeyForTest(t)
	if err := SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")); err != nil {
		t.Fatal(err)
	}

	const secret = "sk-test-round-trip-secret"
	enc, err := EncryptAPIKey(secret)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if enc == secret {
		t.Fatal("ciphertext equals plaintext")
	}
	if _, err := base64.StdEncoding.DecodeString(enc); err != nil {
		t.Fatalf("ciphertext is not base64: %v", err)
	}

	dec, err := DecryptAPIKey(enc)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if dec != secret {
		t.Fatalf("round trip mismatch: got %q want %q", dec, secret)
	}

	// Must not be decryptable with the legacy key alone.
	data, _ := base64.StdEncoding.DecodeString(enc)
	if _, err := aesGCMOpen(legacyEncryptionKey, data); err == nil {
		t.Fatal("ciphertext produced with configured key decrypted with legacy key")
	}
}

func TestDecryptLegacyFallback(t *testing.T) {
	resetEncryptionKeyForTest(t)

	// Simulate a row written by an old build using the hard-coded key.
	const secret = "sk-ant-legacy-row"
	legacyCipher, err := aesGCMSeal(legacyEncryptionKey, secret)
	if err != nil {
		t.Fatal(err)
	}

	// Configure a different key; legacy rows must still decrypt.
	if err := SetEncryptionKey([]byte("fedcba9876543210fedcba9876543210")); err != nil {
		t.Fatal(err)
	}
	dec, err := DecryptAPIKey(legacyCipher)
	if err != nil {
		t.Fatalf("legacy fallback decrypt failed: %v", err)
	}
	if dec != secret {
		t.Fatalf("legacy fallback mismatch: got %q want %q", dec, secret)
	}

	// Garbage must still fail.
	if _, err := DecryptAPIKey(base64.StdEncoding.EncodeToString([]byte("not a valid ciphertext at all"))); err == nil {
		t.Fatal("expected error for invalid ciphertext")
	}
}

func TestEncryptionKeyFromEnv(t *testing.T) {
	resetEncryptionKeyForTest(t)
	t.Setenv(EncryptionKeyEnvVar, "env-passphrase-that-is-longer-than-thirty-two-bytes!")

	// Reset the once so the env var is consulted again.
	encryptionKeyOnce = sync.Once{}

	key := currentEncryptionKey()
	if len(key) != 32 {
		t.Fatalf("expected 32-byte derived key, got %d", len(key))
	}
	if bytes.Equal(key, legacyEncryptionKey) {
		t.Fatal("env key was ignored; legacy key in use")
	}

	enc, err := EncryptAPIKey("x")
	if err != nil {
		t.Fatal(err)
	}
	dec, err := DecryptAPIKey(enc)
	if err != nil || dec != "x" {
		t.Fatalf("round trip with env key failed: %v %q", err, dec)
	}
}
