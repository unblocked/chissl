package session

import (
	"testing"
	"time"
)

func TestMemoryStoreLifecycle(t *testing.T) {
	s := NewMemoryStore()
	tok, err := s.Issue("alice", "127.0.0.1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) < 40 {
		t.Fatalf("token too short: %q", tok)
	}
	if u, err := s.Lookup(tok); err != nil || u != "alice" {
		t.Fatalf("lookup: %q %v", u, err)
	}
	if _, err := s.Lookup("alice"); err != ErrNotFound {
		t.Fatalf("a bare username must not be a valid session, got %v", err)
	}
	if err := s.Revoke(tok); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Lookup(tok); err != ErrNotFound {
		t.Fatal("revoked session still resolves")
	}
}

func TestMemoryStoreExpiry(t *testing.T) {
	s := NewMemoryStore()
	tok, _ := s.Issue("bob", "", -time.Second)
	if _, err := s.Lookup(tok); err != ErrNotFound {
		t.Fatal("expired session resolved")
	}
}
