package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// preserveAuthSettings restores auth-token/password settings after a test so
// other tests in the package (which sign tokens with version 0) stay valid.
func preserveAuthSettings(t *testing.T, m *Manager, keys ...string) {
	t.Helper()
	old := map[string]string{}
	exists := map[string]bool{}
	for _, k := range keys {
		v, ok, err := m.pg.GetSetting(k)
		if err != nil {
			t.Fatalf("read %s: %v", k, err)
		}
		old[k], exists[k] = v, ok
	}
	t.Cleanup(func() {
		for _, k := range keys {
			if exists[k] {
				_ = m.pg.SetSetting(k, old[k])
			} else {
				_, _ = m.pg.Exec("DELETE FROM settings WHERE key=$1", k)
			}
		}
	})
}

func TestTokenVersionRevokesSessions(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })
	preserveAuthSettings(t, m, authVerKey)

	s := &Server{m: m, jwtKey: []byte("test-only-signing-key-32-characters")}
	cur, err := s.authTokenVersion()
	if err != nil {
		t.Fatal(err)
	}
	tok, err := signJWT(s.jwtKey, cur)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := s.verifyAdminJWT(tok); err != nil || !ok {
		t.Fatalf("current token must pass: ok=%v err=%v", ok, err)
	}

	next, err := s.bumpAuthTokenVersion()
	if err != nil {
		t.Fatal(err)
	}
	if next != cur+1 {
		t.Fatalf("version = %d, want %d", next, cur+1)
	}
	if ok, err := s.verifyAdminJWT(tok); err != nil || ok {
		t.Fatalf("old token must be revoked: ok=%v err=%v", ok, err)
	}
	fresh, err := signJWT(s.jwtKey, next)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := s.verifyAdminJWT(fresh); err != nil || !ok {
		t.Fatalf("fresh token must pass: ok=%v err=%v", ok, err)
	}

	// Tokens minted before versioning existed carry no "ver" claim → version 0.
	legacy, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject: "ARTEX",
	}).SignedString(s.jwtKey)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := s.verifyAdminJWT(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if want := next == 0; ok != want {
		t.Fatalf("legacy token validity = %v, want %v (version %d)", ok, want, next)
	}
}

func TestRevokeEndpointInvalidatesTokens(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })
	preserveAuthSettings(t, m, authVerKey)

	s := &Server{m: m, jwtKey: []byte("test-only-signing-key-32-characters")}
	cur, err := s.authTokenVersion()
	if err != nil {
		t.Fatal(err)
	}
	tok, err := signJWT(s.jwtKey, cur)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/revoke", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	s.authRevokeSessions(rec, req)
	if rec.Code != 200 {
		t.Fatalf("revoke: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/version", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token must be rejected by requireAuth, got %d %s", rec.Code, rec.Body.String())
	}

	// Unauthenticated revoke is rejected.
	rec = httptest.NewRecorder()
	s.authRevokeSessions(rec, httptest.NewRequest(http.MethodPost, "/api/auth/revoke", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous revoke = %d, want 401", rec.Code)
	}
}

func TestChangePasswordRevokesOtherSessions(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })
	preserveAuthSettings(t, m, authVerKey, authPassKey)

	hash, err := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.pg.SetSetting(authPassKey, string(hash)); err != nil {
		t.Fatal(err)
	}

	s := &Server{m: m, jwtKey: []byte("test-only-signing-key-32-characters")}
	cur, err := s.authTokenVersion()
	if err != nil {
		t.Fatal(err)
	}
	oldTok, err := signJWT(s.jwtKey, cur)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/change-password",
		strings.NewReader(`{"old_password":"old-password","new_password":"new-password-1"}`))
	req.Header.Set("Authorization", "Bearer "+oldTok)
	rec := httptest.NewRecorder()
	s.authChangePassword(rec, req)
	if rec.Code != 200 {
		t.Fatalf("change password: %d %s", rec.Code, rec.Body.String())
	}

	if ok, err := s.verifyAdminJWT(oldTok); err != nil || ok {
		t.Fatalf("token issued before the password change must be revoked: ok=%v err=%v", ok, err)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Token == "" {
		t.Fatal("password change must return a replacement token")
	}
	if ok, err := s.verifyAdminJWT(out.Token); err != nil || !ok {
		t.Fatalf("replacement token must pass: ok=%v err=%v", ok, err)
	}
}
