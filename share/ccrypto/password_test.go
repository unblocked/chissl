package ccrypto

import "testing"

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !IsPasswordHash(hash) {
		t.Fatalf("expected bcrypt hash, got %q", hash)
	}
	if ok, legacy := CheckPassword(hash, "correct horse"); !ok || legacy {
		t.Fatalf("hash should verify: ok=%v legacy=%v", ok, legacy)
	}
	if ok, _ := CheckPassword(hash, "wrong"); ok {
		t.Fatal("wrong password accepted")
	}
}

func TestCheckPasswordLegacyPlaintext(t *testing.T) {
	if ok, legacy := CheckPassword("plain", "plain"); !ok || !legacy {
		t.Fatalf("legacy plaintext should match and be flagged: ok=%v legacy=%v", ok, legacy)
	}
	if ok, _ := CheckPassword("plain", "other"); ok {
		t.Fatal("mismatched plaintext accepted")
	}
}

func TestCheckPasswordRejectsEmpty(t *testing.T) {
	if ok, _ := CheckPassword("", ""); ok {
		t.Fatal("empty stored and empty password must not match")
	}
	if ok, _ := CheckPassword("", "x"); ok {
		t.Fatal("empty stored must never match")
	}
	if ok, _ := CheckPassword("x", ""); ok {
		t.Fatal("empty password must never match")
	}
	if _, err := HashPassword(""); err == nil {
		t.Fatal("hashing an empty password must fail")
	}
}
