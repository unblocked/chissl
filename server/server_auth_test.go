package chserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NextChapterSoftware/chissl/share/auth"
	"github.com/NextChapterSoftware/chissl/share/ccrypto"
	"github.com/NextChapterSoftware/chissl/share/database"
	"github.com/NextChapterSoftware/chissl/share/session"
)

// newAuthTestServer returns a server backed by a fresh SQLite database with a
// CLI admin ("admin"/"adminpass"), a database admin ("dbadmin"/"dbadminpass"),
// a regular user ("alice"/"alicepass") whose password is stored as legacy
// plaintext, and an SSO-provisioned user ("sso@example.com") without a password.
func newAuthTestServer(t *testing.T, cfg *Config) (*Server, database.Database) {
	t.Helper()
	dbConfig := &database.DatabaseConfig{Type: "sqlite", FilePath: filepath.Join(t.TempDir(), "test.db")}
	db := database.NewDatabase(dbConfig)
	if err := db.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	for _, u := range []*database.User{
		{Username: "dbadmin", Password: "dbadminpass", IsAdmin: true},
		{Username: "alice", Password: "alicepass"},
		{Username: "bob", Password: "bobpass"},
		{Username: "sso@example.com", Password: ""},
	} {
		if err := db.CreateUser(u); err != nil {
			t.Fatal(err)
		}
	}
	if cfg == nil {
		cfg = &Config{}
	}
	cfg.Database = dbConfig
	cfg.Dashboard.Enabled = true
	if cfg.Auth == "" {
		cfg.Auth = "admin:adminpass"
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Close both the server's handle and this seeding handle so Windows can
	// remove the TempDir database file (open files cannot be unlinked there).
	t.Cleanup(func() {
		_ = srv.Shutdown()
		_ = db.Close()
	})
	return srv, db
}

func do(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// login submits the dashboard form and returns the issued session cookie.
func login(t *testing.T, h http.Handler, username, password string) *http.Cookie {
	t.Helper()
	form := url.Values{"username": {username}, "password": {password}}
	req := httptest.NewRequest(http.MethodPost, "/dashboard", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := do(h, req)
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/dashboard" {
		t.Fatalf("login %s: expected redirect to /dashboard, got %d %s", username, rr.Code, rr.Header().Get("Location"))
	}
	for _, c := range rr.Result().Cookies() {
		if c.Name == session.CookieName {
			return c
		}
	}
	t.Fatalf("login %s: no session cookie set", username)
	return nil
}

func TestSessionCookieIsOpaqueAndCannotBeForged(t *testing.T) {
	srv, _ := newAuthTestServer(t, nil)
	h := srv.HTTPHandler()

	for _, forged := range []string{"admin", "dbadmin", "alice"} {
		req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
		req.AddCookie(&http.Cookie{Name: session.CookieName, Value: forged})
		if rr := do(h, req); rr.Code != http.StatusUnauthorized {
			t.Fatalf("forged cookie %q accepted with %d", forged, rr.Code)
		}
	}

	c := login(t, h, "admin", "adminpass")
	if c.Value == "admin" || len(c.Value) < 40 || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie is not an opaque hardened token: %+v", c)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.AddCookie(c)
	if rr := do(h, req); rr.Code != http.StatusOK {
		t.Fatalf("valid session rejected: %d %s", rr.Code, rr.Body.String())
	}

	// Logout revokes the session server-side.
	req = httptest.NewRequest(http.MethodGet, "/dashboard/logout", nil)
	req.AddCookie(c)
	if rr := do(h, req); rr.Code != http.StatusSeeOther {
		t.Fatalf("logout: %d", rr.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.AddCookie(c)
	if rr := do(h, req); rr.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session still accepted: %d", rr.Code)
	}
}

func TestLoginRejectsBadPasswordAndSSOPlaceholders(t *testing.T) {
	srv, _ := newAuthTestServer(t, nil)
	h := srv.HTTPHandler()
	for _, tc := range [][2]string{{"admin", "wrong"}, {"sso@example.com", ""}, {"sso@example.com", "SSO_USER"}, {"nobody", "x"}} {
		form := url.Values{"username": {tc[0]}, "password": {tc[1]}}
		req := httptest.NewRequest(http.MethodPost, "/dashboard", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := do(h, req)
		if !strings.Contains(rr.Header().Get("Location"), "error=") || len(rr.Result().Cookies()) != 0 {
			t.Fatalf("login %q/%q should fail, got %d %s cookies=%d", tc[0], tc[1], rr.Code, rr.Header().Get("Location"), len(rr.Result().Cookies()))
		}
		req = httptest.NewRequest(http.MethodGet, "/api/user/info", nil)
		req.SetBasicAuth(tc[0], tc[1])
		if rr := do(h, req); rr.Code != http.StatusUnauthorized {
			t.Fatalf("basic auth %q/%q accepted with %d", tc[0], tc[1], rr.Code)
		}
	}
}

func TestNonAdminSessionCannotUseAdminRoutes(t *testing.T) {
	srv, _ := newAuthTestServer(t, nil)
	h := srv.HTTPHandler()
	c := login(t, h, "alice", "alicepass")
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.AddCookie(c)
	if rr := do(h, req); rr.Code != http.StatusForbidden {
		t.Fatalf("non-admin session reached admin route: %d", rr.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/user/info", nil)
	req.AddCookie(c)
	if rr := do(h, req); rr.Code != http.StatusOK {
		t.Fatalf("non-admin session rejected from user route: %d", rr.Code)
	}
}

func TestPasswordsAreHashedAndNeverReturned(t *testing.T) {
	srv, db := newAuthTestServer(t, nil)
	h := srv.HTTPHandler()

	// A successful login with a legacy plaintext row upgrades it to a hash.
	req := httptest.NewRequest(http.MethodGet, "/api/user/info", nil)
	req.SetBasicAuth("alice", "alicepass")
	if rr := do(h, req); rr.Code != http.StatusOK {
		t.Fatalf("legacy plaintext login failed: %d", rr.Code)
	}
	u, err := db.GetUser("alice")
	if err != nil {
		t.Fatal(err)
	}
	if !ccrypto.IsPasswordHash(u.Password) {
		t.Fatalf("password not upgraded to a hash: %q", u.Password)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/user/info", nil)
	req.SetBasicAuth("alice", "alicepass")
	if rr := do(h, req); rr.Code != http.StatusOK {
		t.Fatalf("login with hashed password failed: %d", rr.Code)
	}

	// Users created through the API are stored hashed.
	body := `{"username":"carol","password":"carolpass","is_admin":false}`
	req = httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(body))
	req.SetBasicAuth("admin", "adminpass")
	if rr := do(h, req); rr.Code != http.StatusCreated {
		t.Fatalf("create user: %d %s", rr.Code, rr.Body.String())
	}
	if u, _ := db.GetUser("carol"); u == nil || !ccrypto.IsPasswordHash(u.Password) {
		t.Fatalf("created user stored without hash: %+v", u)
	}

	// No listing or lookup ever includes password material.
	for _, p := range []string{"/api/users", "/users", "/user/bob"} {
		req = httptest.NewRequest(http.MethodGet, p, nil)
		req.SetBasicAuth("admin", "adminpass")
		rr := do(h, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: %d", p, rr.Code)
		}
		if strings.Contains(rr.Body.String(), "password") || strings.Contains(rr.Body.String(), "bobpass") || strings.Contains(rr.Body.String(), "$2") {
			t.Fatalf("%s leaks password material: %s", p, rr.Body.String())
		}
	}
}

func TestTunnelRoutesEnforceOwnership(t *testing.T) {
	srv, db := newAuthTestServer(t, nil)
	h := srv.HTTPHandler()
	if err := db.CreateTunnel(&database.Tunnel{ID: "sess-1", Username: "alice", Status: "open"}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		user, pass, method string
		want               int
	}{
		{"bob", "bobpass", http.MethodGet, http.StatusForbidden},
		{"bob", "bobpass", http.MethodDelete, http.StatusForbidden},
		{"alice", "alicepass", http.MethodGet, http.StatusOK},
		{"admin", "adminpass", http.MethodGet, http.StatusOK},
	} {
		req := httptest.NewRequest(tc.method, "/api/tunnels/sess-1", nil)
		req.SetBasicAuth(tc.user, tc.pass)
		if rr := do(h, req); rr.Code != tc.want {
			t.Fatalf("%s %s as %s: got %d want %d", tc.method, "/api/tunnels/sess-1", tc.user, rr.Code, tc.want)
		}
	}
}

func TestDashboardAssetsNeverReadFromDisk(t *testing.T) {
	srv, _ := newAuthTestServer(t, nil)
	h := srv.HTTPHandler()
	// Relative to the test working directory (server/), "../go.mod" exists on disk.
	for _, p := range []string{"/dashboard/../../../go.mod", "/dashboard/..%2F..%2F..%2Fgo.mod", "/dashboard/static/../../../../go.mod"} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rr := do(h, req)
		if rr.Code != http.StatusNotFound || strings.Contains(rr.Body.String(), "module ") {
			t.Fatalf("%s: got %d body %q", p, rr.Code, rr.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/dashboard/static/js/core/app.js", nil)
	if rr := do(h, req); rr.Code != http.StatusOK || rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("embedded asset not served: %d", rr.Code)
	}
}

func TestCookieAuthRejectsCrossSiteMutations(t *testing.T) {
	srv, _ := newAuthTestServer(t, nil)
	h := srv.HTTPHandler()
	c := login(t, h, "admin", "adminpass")

	body := `{"username":"evil","password":"evilpass","is_admin":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(body))
	req.AddCookie(c)
	req.Header.Set("Origin", "https://attacker.example")
	if rr := do(h, req); rr.Code != http.StatusForbidden {
		t.Fatalf("cross-origin POST accepted: %d", rr.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(body))
	req.AddCookie(c)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	if rr := do(h, req); rr.Code != http.StatusForbidden {
		t.Fatalf("cross-site POST accepted: %d", rr.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(body))
	req.AddCookie(c)
	req.Header.Set("Origin", "http://"+req.Host)
	if rr := do(h, req); rr.Code != http.StatusCreated {
		t.Fatalf("same-origin POST rejected: %d %s", rr.Code, rr.Body.String())
	}
}

func TestBearerTokensAndAuth0(t *testing.T) {
	srv, db := newAuthTestServer(t, &Config{Auth0: &auth.Auth0Config{Enabled: true, Domain: "example.invalid", Audience: "chissl"}})
	h := srv.HTTPHandler()

	// Garbage Bearer tokens are rejected even with Auth0 enabled.
	for _, tok := range []string{"x", "eyJhbGciOiJub25lIn0.e30."} {
		req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		if rr := do(h, req); rr.Code != http.StatusUnauthorized {
			t.Fatalf("bearer %q accepted: %d", tok, rr.Code)
		}
	}

	// A database API token authenticates as its owner.
	if err := db.CreateUserToken(&database.UserToken{ID: "tok-1", Username: "alice", Name: "ci", Token: "secrettoken", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/user/info", nil)
	req.Header.Set("Authorization", "Bearer secrettoken")
	rr := do(h, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("api token rejected: %d", rr.Code)
	}
	var info map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &info)
	if info["username"] != "alice" || info["auth_method"] != "api_token" {
		t.Fatalf("unexpected identity: %v", info)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Authorization", "Bearer secrettoken")
	if rr := do(h, req); rr.Code != http.StatusForbidden {
		t.Fatalf("non-admin token reached admin route: %d", rr.Code)
	}
}
