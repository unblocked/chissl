// Package session issues and resolves opaque, server-side login sessions.
//
// A session token is 32 bytes from crypto/rand, base64url encoded. Only the
// SHA-256 of the token is persisted, so a database read does not yield usable
// cookies. Sessions are stored in the database when one is configured and in
// memory otherwise.
package session

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/NextChapterSoftware/chissl/share/database"
)

// CookieName is the name of the dashboard session cookie.
const CookieName = "chissl_session"

// ErrNotFound is returned when a token does not resolve to a live session.
var ErrNotFound = errors.New("session not found or expired")

// Store persists sessions.
type Store interface {
	// Issue creates a session for username and returns the opaque token.
	Issue(username, ip string, ttl time.Duration) (string, error)
	// Lookup resolves a token to its username. It returns ErrNotFound for
	// unknown or expired tokens.
	Lookup(token string) (string, error)
	// Revoke deletes the session for token. Revoking an unknown token is not an error.
	Revoke(token string) error
}

// NewStore returns a database-backed store when db is non-nil and an in-memory store otherwise.
func NewStore(db database.Database) Store {
	if db == nil {
		return NewMemoryStore()
	}
	return &dbStore{db: db}
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashToken derives the stored identifier for a token.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

type dbStore struct {
	db database.Database
}

func (s *dbStore) Issue(username, ip string, ttl time.Duration) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	sess := &database.Session{
		ID:        hashToken(token),
		Username:  username,
		ExpiresAt: time.Now().Add(ttl),
		IPAddress: ip,
	}
	if err := s.db.CreateSession(sess); err != nil {
		return "", err
	}
	return token, nil
}

func (s *dbStore) Lookup(token string) (string, error) {
	if token == "" {
		return "", ErrNotFound
	}
	sess, err := s.db.GetSession(hashToken(token))
	if err != nil || sess == nil {
		return "", ErrNotFound
	}
	if time.Now().After(sess.ExpiresAt) {
		return "", ErrNotFound
	}
	return sess.Username, nil
}

func (s *dbStore) Revoke(token string) error {
	if token == "" {
		return nil
	}
	return s.db.DeleteSession(hashToken(token))
}

type memoryEntry struct {
	username string
	expires  time.Time
}

// MemoryStore keeps sessions in process memory.
type MemoryStore struct {
	mu       sync.Mutex
	sessions map[string]memoryEntry
}

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{sessions: map[string]memoryEntry{}}
}

func (m *MemoryStore) Issue(username, _ string, ttl time.Duration) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked()
	m.sessions[hashToken(token)] = memoryEntry{username: username, expires: time.Now().Add(ttl)}
	return token, nil
}

func (m *MemoryStore) Lookup(token string) (string, error) {
	if token == "" {
		return "", ErrNotFound
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.sessions[hashToken(token)]
	if !ok || time.Now().After(e.expires) {
		return "", ErrNotFound
	}
	return e.username, nil
}

func (m *MemoryStore) Revoke(token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, hashToken(token))
	return nil
}

// pruneLocked drops expired entries; the caller must hold mu.
func (m *MemoryStore) pruneLocked() {
	now := time.Now()
	for k, e := range m.sessions {
		if now.After(e.expires) {
			delete(m.sessions, k)
		}
	}
}
