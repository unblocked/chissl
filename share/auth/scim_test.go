package auth

import (
	"path/filepath"
	"testing"

	"github.com/NextChapterSoftware/chissl/share/database"
)

func newSCIMTestDB(t *testing.T) database.Database {
	t.Helper()
	cfg := &database.DatabaseConfig{Type: "sqlite", FilePath: filepath.Join(t.TempDir(), "scim.db")}
	db := database.NewDatabase(cfg)
	if err := db.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// SCIM login must bind to the IdP subject (sub), not the mutable
// preferred_username/email claims. Otherwise an IdP user could take over a
// local account by choosing a colliding username (NEBU-00156 / NEBU-00166 /
// NEBU-00009) or mint the reserved --auth admin name (NEBU-00175).
func TestSCIMBindsToExternalSubject(t *testing.T) {
	db := newSCIMTestDB(t)
	m := NewSCIMMiddleware(db, &database.SCIMConfig{}, "cliadmin")

	// First login provisions a non-admin account with no usable password,
	// bound to subject S1.
	u, err := m.createOrUpdateUser(&SCIMUserInfo{ID: "S1", Username: "carol", Email: "carol@x"})
	if err != nil || u == nil {
		t.Fatalf("first login failed: %v", err)
	}
	if u.IsAdmin || u.Password != "" {
		t.Fatalf("SSO user must be non-admin with no password: %+v", u)
	}

	// The same subject may log in again and refresh its profile.
	if _, err := m.createOrUpdateUser(&SCIMUserInfo{ID: "S1", Username: "carol", Email: "new@x"}); err != nil {
		t.Fatalf("same-subject relogin rejected: %v", err)
	}

	// A different subject presenting the same username is refused (takeover).
	if _, err := m.createOrUpdateUser(&SCIMUserInfo{ID: "S2", Username: "carol"}); err == nil {
		t.Fatalf("different subject adopted existing account")
	}

	// A login without a subject is refused outright.
	if _, err := m.createOrUpdateUser(&SCIMUserInfo{ID: "", Username: "dave"}); err == nil {
		t.Fatalf("login without a subject accepted")
	}

	// The reserved --auth admin name cannot be provisioned via SSO.
	if _, err := m.createOrUpdateUser(&SCIMUserInfo{ID: "S9", Username: "cliadmin"}); err == nil {
		t.Fatalf("SSO provisioned the reserved CLI admin name")
	}

	// A pre-existing local-password account cannot be adopted by SSO.
	if err := db.CreateUser(&database.User{Username: "localpw", Password: "secret"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.createOrUpdateUser(&SCIMUserInfo{ID: "S3", Username: "localpw"}); err == nil {
		t.Fatalf("SSO adopted a local-password account")
	}
}
