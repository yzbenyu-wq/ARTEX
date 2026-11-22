package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// A stale artex_token cookie must not veto a valid ?token= the browser just
// sent: SSE cannot set an Authorization header, so the query param is its only
// credential, and browsers holding a cookie from another deployment on the
// same host used to get 401 on every stream.
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
		"valid header":                     {header: valid, want: true},
		"invalid header is authoritative":  {header: stale, cookie: valid, query: valid, want: false},
		"stale cookie falls back to query": {cookie: stale, query: valid, want: true},
		"valid cookie wins over bad query": {cookie: valid, query: stale, want: true},
		"query only":                       {query: valid, want: true},
		"all stale":                        {cookie: stale, query: stale, want: false},
		"nothing":                          {want: false},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", "Bearer "+tc.header)
			}
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "artex_token", Value: tc.cookie})
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

	// Same matrix through requireAuth: the SSE-shaped request must reach the
	// handler, while an anonymous one still gets 401 "unauthorized".
	req := httptest.NewRequest(http.MethodGet, "/api/version?token="+valid, nil)
	req.AddCookie(&http.Cookie{Name: "artex_token", Value: stale})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("stale cookie + valid query: status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/version", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: status=%d", rec.Code)
	}
}
