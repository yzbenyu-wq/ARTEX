package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The Bearer header and the HttpOnly session cookie are the only accepted
// credentials. A ?token= query param must never authenticate: tokens in URLs end
// up in proxy logs, browser history and Referer headers, so SSE reads the cookie
// set at login instead.
func TestAuthTokenFallback(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })
	preserveAuthSettings(t, m, authVerKey)

	s := &Server{m: m, jwtKey: []byte("test-only-signing-key-32-characters")}
	ver, err := s.authTokenVersion()
	if err != nil {
		t.Fatal(err)
	}
	valid, err := signJWT(s.jwtKey, ver)
	if err != nil {
		t.Fatal(err)
	}
	stale, err := signJWT([]byte("some-other-key-32-characters-long"), ver)
	if err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct {
		header, cookie, query string
		want                  bool
	}{
		"valid header":                             {header: valid, want: true},
		"invalid header is authoritative":          {header: stale, cookie: valid, query: valid, want: false},
		"valid cookie":                             {cookie: valid, want: true},
		"stale cookie":                             {cookie: stale, want: false},
		"query only is rejected":                   {query: valid, want: false},
		"stale cookie does not fall back to query": {cookie: stale, query: valid, want: false},
		"nothing":                                  {want: false},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", "Bearer "+tc.header)
			}
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: authCookieName, Value: tc.cookie})
			}
			if tc.query != "" {
				q := req.URL.Query()
				q.Set("token", tc.query)
				req.URL.RawQuery = q.Encode()
			}
			ok, err := s.authenticateRequest(req)
			if err != nil {
				t.Fatal(err)
			}
			if ok != tc.want {
				t.Fatalf("ok=%v want %v", ok, tc.want)
			}
		})
	}

	// Same matrix through requireAuth: the cookie-shaped SSE request must reach
	// the handler, while anonymous and query-token requests still get 401.
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: valid})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid cookie: status=%d body=%s", rec.Code, rec.Body.String())
	}
	for name, r := range map[string]*http.Request{
		"anonymous":   httptest.NewRequest(http.MethodGet, "/api/version", nil),
		"query token": httptest.NewRequest(http.MethodGet, "/api/version?token="+valid, nil),
	} {
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, r)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status=%d", name, rec.Code)
		}
	}
}
