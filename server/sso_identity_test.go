package chserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NextChapterSoftware/chissl/share/database"
	"github.com/NextChapterSoftware/chissl/share/session"
)

// A session (or token) whose username merely collides with the --auth admin
// must not inherit admin rights: a database row for that name governs instead.
// This closes the SSO/token username-collision escalation (NEBU-00175 /
// NEBU-00147), while the genuine password-verified CLI admin keeps admin.
func TestUsernameCollisionWithCLIAdminDoesNotGrantAdmin(t *testing.T) {
	srv, db := newAuthTestServer(t, nil) // CLI admin is "admin"/"adminpass"

	// An SSO/DB account that collides with the CLI admin name, non-admin.
	if err := db.CreateUser(&database.User{Username: "admin", Password: "", IsAdmin: false}); err != nil {
		t.Fatal(err)
	}
	h := srv.HTTPHandler()

	// A session for that colliding account cannot reach admin routes.
	token, err := srv.sessionStore.Issue("admin", "127.0.0.1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: token})
	if rr := do(h, req); rr.Code != http.StatusForbidden {
		t.Fatalf("colliding non-admin session reached admin route: %d", rr.Code)
	}

	// The genuine CLI admin (password-verified) still has admin.
	req = httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.SetBasicAuth("admin", "adminpass")
	if rr := do(h, req); rr.Code != http.StatusOK {
		t.Fatalf("genuine CLI admin denied admin route: %d", rr.Code)
	}
}

// Without a shadowing account, the CLI admin's own dashboard session must still
// resolve to admin — the fix must not over-restrict the legitimate bootstrap
// admin.
func TestCLIAdminSessionRemainsAdmin(t *testing.T) {
	srv, _ := newAuthTestServer(t, nil)
	h := srv.HTTPHandler()
	c := login(t, h, "admin", "adminpass")
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.AddCookie(c)
	if rr := do(h, req); rr.Code != http.StatusOK {
		t.Fatalf("CLI admin session denied admin route: %d", rr.Code)
	}
}
