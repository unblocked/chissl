package auth

import "testing"

func TestAuth0DisabledRejectsTokens(t *testing.T) {
	m, err := NewAuth0Middleware(&Auth0Config{Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.ValidateToken("Bearer anything"); err == nil {
		t.Fatal("disabled middleware must reject tokens")
	}
}

func TestAuth0RejectsGarbageWithoutNetwork(t *testing.T) {
	m, err := NewAuth0Middleware(&Auth0Config{Enabled: true, Domain: "example.invalid", Audience: "chissl"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tok := range []string{"", "x", "Bearer x", "Bearer eyJhbGciOiJub25lIn0.e30."} {
		if info, err := m.ValidateToken(tok); err == nil {
			t.Fatalf("token %q accepted as %+v", tok, info)
		}
	}
}
