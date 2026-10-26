package chserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NextChapterSoftware/chissl/share/database"
)

// Regression: saving a new SSO client secret must persist (go-sqlite3 binds $N
// by order of appearance, which previously broke the UPDATE and silently kept
// the old secret). Also verifies an empty secret keeps the stored one.
func TestSSOSecretUpdatePersists(t *testing.T) {
	srv, db := newAuthTestServer(t, nil)
	h := srv.HTTPHandler()
	post := func(secret string) int {
		cfg, _ := json.Marshal(map[string]any{
			"client_id": "cid", "client_secret": secret,
			"auth_url": "https://idp/authorize", "token_url": "https://idp/token",
			"user_info_url": "https://idp/userinfo", "redirect_url": "https://x/cb",
			"scopes": []string{"openid"},
		})
		body, _ := json.Marshal(map[string]any{"provider": "okta", "enabled": true, "config": json.RawMessage(cfg)})
		req := httptest.NewRequest(http.MethodPost, "/api/sso/configs", strings.NewReader(string(body)))
		req.SetBasicAuth("admin", "adminpass")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}
	stored := func() string {
		c, err := db.GetSSOConfig(database.SSOProviderOkta)
		if err != nil || c == nil {
			t.Fatalf("read sso: %v", err)
		}
		return c.Config.(database.SCIMConfig).ClientSecret
	}
	if post("SECRET_AAA") != http.StatusCreated || stored() != "SECRET_AAA" {
		t.Fatalf("initial save failed, stored=%q", stored())
	}
	if post("SECRET_BBB") != http.StatusCreated || stored() != "SECRET_BBB" {
		t.Fatalf("new secret not persisted, stored=%q want SECRET_BBB", stored())
	}
	if post("") != http.StatusCreated || stored() != "SECRET_BBB" {
		t.Fatalf("empty save should keep existing, stored=%q want SECRET_BBB", stored())
	}
}
