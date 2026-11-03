package chserver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NextChapterSoftware/chissl/share/database"
)

// Security review: the client IP fed into rate limiting, login backoff,
// session binding and security events must not be forgeable through
// X-Forwarded-For unless the TCP peer is a configured trusted proxy.
func TestClientIPTrustsForwardedForOnlyFromTrustedProxies(t *testing.T) {
	untrusted, _ := newAuthTestServer(t, nil)
	trusted, _ := newAuthTestServer(t, &Config{Security: SecurityConfig{
		TrustedProxies: []string{"10.0.0.0/8", "2001:db8::1"},
	}})

	cases := []struct {
		name   string
		srv    *Server
		remote string
		xff    []string
		want   string
	}{
		{"no proxies configured: header ignored", untrusted, "64.118.149.80:5555", []string{"203.0.113.8"}, "64.118.149.80"},
		{"no header", trusted, "10.1.2.3:4444", nil, "10.1.2.3"},
		{"untrusted peer: header ignored", trusted, "64.118.149.80:5555", []string{"203.0.113.8"}, "64.118.149.80"},
		{"trusted peer: rightmost untrusted hop wins", trusted, "10.1.2.3:4444", []string{"203.0.113.8, 198.51.100.7"}, "198.51.100.7"},
		{"trusted peer: trusted hops are skipped", trusted, "10.1.2.3:4444", []string{"198.51.100.7, 10.9.9.9"}, "198.51.100.7"},
		{"trusted peer: multiple headers", trusted, "10.1.2.3:4444", []string{"203.0.113.8", "198.51.100.7"}, "198.51.100.7"},
		{"trusted peer: all hops trusted falls back to peer", trusted, "10.1.2.3:4444", []string{"10.9.9.9"}, "10.1.2.3"},
		{"trusted peer: malformed hop falls back to peer", trusted, "10.1.2.3:4444", []string{"203.0.113.8, not-an-ip"}, "10.1.2.3"},
		{"trusted IPv6 peer", trusted, "[2001:db8::1]:443", []string{"198.51.100.7"}, "198.51.100.7"},
		{"peer without port", trusted, "10.1.2.3", []string{"198.51.100.7"}, "198.51.100.7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remote
			for _, v := range tc.xff {
				req.Header.Add("X-Forwarded-For", v)
			}
			if got := tc.srv.clientIP(req); got != tc.want {
				t.Fatalf("clientIP = %q, want %q", got, tc.want)
			}
		})
	}

	if _, err := NewServer(&Config{Security: SecurityConfig{TrustedProxies: []string{"not-a-cidr"}}}); err == nil {
		t.Fatal("invalid --trusted-proxy value accepted")
	}
}

// Security review: an admin-saved AI provider endpoint must not be usable
// to reach internal hosts, because preview/refine/chat are user-level
// endpoints. Private endpoints are refused unless explicitly enabled.
func TestAIProviderPrivateEndpointRefusedByDefault(t *testing.T) {
	post := func(srv *Server, endpoint string) *httptest.ResponseRecorder {
		h := srv.HTTPHandler()
		c := login(t, h, "admin", "adminpass")
		body := `{"name":"p","provider_type":"openai","api_key":"sk-test","model":"gpt-4","api_endpoint":"` + endpoint + `","enabled":true}`
		req := httptest.NewRequest(http.MethodPost, "/api/ai-providers", strings.NewReader(body))
		req.AddCookie(c)
		req.Header.Set("Origin", "http://"+req.Host)
		req.Header.Set("Content-Type", "application/json")
		return do(h, req)
	}

	def, _ := newAuthTestServer(t, nil)
	for _, ep := range []string{"http://127.0.0.1:1", "http://169.254.169.254/computeMetadata/v1", "http://10.1.151.85:80", "http://localhost:11434/v1"} {
		if rr := post(def, ep); rr.Code != http.StatusBadRequest {
			t.Fatalf("private endpoint %s accepted by default: %d %s", ep, rr.Code, rr.Body.String())
		}
	}
	if rr := post(def, "https://api.openai.com/v1"); rr.Code != http.StatusCreated {
		t.Fatalf("public endpoint rejected: %d %s", rr.Code, rr.Body.String())
	}
	// Even if a private endpoint is already stored (legacy row), the dialer
	// refuses to connect to it.
	if _, err := def.aiProviderHTTPClient(2 * time.Second).Get("http://127.0.0.1:1/"); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("dialer reached private address, err=%v", err)
	}

	optIn, _ := newAuthTestServer(t, &Config{Security: SecurityConfig{AllowPrivateAIEndpoints: true}})
	if rr := post(optIn, "http://10.1.151.85:11434/v1"); rr.Code != http.StatusCreated {
		t.Fatalf("private endpoint rejected despite --ai-allow-private-endpoints: %d %s", rr.Code, rr.Body.String())
	}
}

// Security review: the preview endpoint used to echo the raw transport
// error (SSH banners, "connection refused", metadata responses) to the user.
func TestAIPreviewDoesNotLeakUpstreamErrors(t *testing.T) {
	srv, db := newAuthTestServer(t, nil)
	key, err := database.EncryptAPIKey("sk-test")
	if err != nil {
		t.Fatal(err)
	}
	p := &database.AIProvider{Name: "legacy", ProviderType: "openai", APIKey: key, APIEndpoint: "http://127.0.0.1:1", Model: "gpt-4", Enabled: true}
	if err := db.CreateAIProvider(p); err != nil {
		t.Fatal(err)
	}
	h := srv.HTTPHandler()
	c := login(t, h, "alice", "alicepass")
	body := `{"ai_provider_id":"` + p.ID + `","openapi_spec":"{\"openapi\":\"3.0.0\",\"info\":{\"title\":\"qa\",\"version\":\"1\"},\"paths\":{}}"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai-listeners/preview", strings.NewReader(body))
	req.AddCookie(c)
	req.Header.Set("Origin", "http://"+req.Host)
	req.Header.Set("Content-Type", "application/json")
	rr := do(h, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("preview against private endpoint: %d %s", rr.Code, rr.Body.String())
	}
	for _, leak := range []string{"127.0.0.1", "dial", "connection refused", "not allowed"} {
		if strings.Contains(rr.Body.String(), leak) {
			t.Fatalf("preview error leaks %q: %s", leak, rr.Body.String())
		}
	}
	if !strings.Contains(rr.Body.String(), aiGenerationFailedMessage) {
		t.Fatalf("preview error missing generic message: %s", rr.Body.String())
	}
}

// Security review: the verbose request log must record the TCP peer, not a
// client-supplied X-Forwarded-For / X-Real-IP, so access-log entries cannot
// be forged.
func TestRequestLogRecordsPeerNotForwardedHeaders(t *testing.T) {
	var buf bytes.Buffer
	h := requestLogHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), &buf)
	req := httptest.NewRequest(http.MethodPost, "/dashboard", nil)
	req.RemoteAddr = "64.118.149.80:5555"
	req.Header.Set("X-Forwarded-For", "203.0.113.8")
	req.Header.Set("X-Real-IP", "203.0.113.9")
	h.ServeHTTP(httptest.NewRecorder(), req)

	line := buf.String()
	if !strings.Contains(line, "(64.118.149.80)") {
		t.Fatalf("access log should record the peer address, got %q", line)
	}
	if strings.Contains(line, "203.0.113.") {
		t.Fatalf("access log recorded a forwarded header address: %q", line)
	}
}
