package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestLoginLimiterLocksAfterFiveFailures(t *testing.T) {
	now := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	l := newLoginLimiter()
	l.now = func() time.Time { return now }

	for i := 1; i <= loginMaxFailures-1; i++ {
		l.fail("10.0.0.1")
		if wait := l.lockedFor("10.0.0.1"); wait != 0 {
			t.Fatalf("failure %d locked for %v, want free", i, wait)
		}
	}
	l.fail("10.0.0.1")
	if wait := l.lockedFor("10.0.0.1"); wait != loginLockout {
		t.Fatalf("locked for %v, want %v", wait, loginLockout)
	}
	if wait := l.lockedFor("10.0.0.2"); wait != 0 {
		t.Fatalf("other address locked for %v", wait)
	}

	now = now.Add(loginLockout - time.Second)
	if wait := l.lockedFor("10.0.0.1"); wait != time.Second {
		t.Fatalf("wait after lock=%v, want 1s", wait)
	}
	now = now.Add(time.Second)
	if wait := l.lockedFor("10.0.0.1"); wait != 0 {
		t.Fatalf("wait after lock expiry=%v, want 0", wait)
	}
	// The counter restarts after the window passed.
	l.fail("10.0.0.1")
	if wait := l.lockedFor("10.0.0.1"); wait != 0 {
		t.Fatalf("one failure after expiry locked for %v", wait)
	}
}

func TestLoginLimiterResetsOnSuccess(t *testing.T) {
	now := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	l := newLoginLimiter()
	l.now = func() time.Time { return now }

	for i := 0; i < loginMaxFailures-1; i++ {
		l.fail("10.0.0.1")
	}
	l.reset("10.0.0.1")
	l.fail("10.0.0.1")
	if wait := l.lockedFor("10.0.0.1"); wait != 0 {
		t.Fatalf("locked for %v after reset, want free", wait)
	}
}

func TestLoginLimiterWindowExpiresFailures(t *testing.T) {
	now := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	l := newLoginLimiter()
	l.now = func() time.Time { return now }

	for i := 0; i < loginMaxFailures-1; i++ {
		l.fail("10.0.0.1")
	}
	now = now.Add(loginFailureWindow + time.Second)
	l.fail("10.0.0.1")
	if wait := l.lockedFor("10.0.0.1"); wait != 0 {
		t.Fatalf("stale window counted: locked for %v", wait)
	}
}

func TestLoginLimiterCapsTableWithoutDroppingLockouts(t *testing.T) {
	now := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	l := newLoginLimiter()
	l.now = func() time.Time { return now }

	for i := 0; i < loginMaxFailures; i++ {
		l.fail("victim")
	}
	if wait := l.lockedFor("victim"); wait == 0 {
		t.Fatal("victim not locked")
	}
	for i := 0; i < loginLimiterMaxEntries; i++ {
		l.fail("filler-" + time.Duration(i).String())
	}
	l.fail("late")
	if wait := l.lockedFor("late"); wait != 0 {
		t.Fatalf("new source tracked past the cap: %v", wait)
	}
	if wait := l.lockedFor("victim"); wait == 0 {
		t.Fatal("cap handling dropped an existing lockout")
	}
}

func TestAuthLoginRateLimited(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil {
		t.Skipf("postgres unavailable (%v)", err)
	}
	defer m.Close()
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-horse"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.pg.SetSetting(authPassKey, string(hash)); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	s := New(ctx, m, dir, dir, dir)
	defer func() {
		cancel()
		s.Close()
	}()
	now := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	s.loginLimiter.now = func() time.Time { return now }

	login := func(remote, user, pass string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"username": user, "password": pass})
		r := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
		r.RemoteAddr = remote
		w := httptest.NewRecorder()
		s.authLogin(w, r)
		return w
	}

	for i := 0; i < loginMaxFailures; i++ {
		w := login("203.0.113.7:5000", "ARTEX", "wrong")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status=%d body=%s", i+1, w.Code, w.Body.String())
		}
	}
	w := login("203.0.113.7:5001", "ARTEX", "correct-horse")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("locked login status=%d body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Retry-After"); got != "300" {
		t.Fatalf("Retry-After=%q, want 300", got)
	}
	if w2 := login("203.0.113.8:5002", "ARTEX", "correct-horse"); w2.Code != http.StatusOK {
		t.Fatalf("other address status=%d body=%s", w2.Code, w2.Body.String())
	}

	now = now.Add(loginLockout + time.Second)
	if w := login("203.0.113.7:5003", "ARTEX", "correct-horse"); w.Code != http.StatusOK {
		t.Fatalf("after lock expiry status=%d body=%s", w.Code, w.Body.String())
	}

	// A success clears earlier failures in the same window.
	for i := 0; i < loginMaxFailures-1; i++ {
		if w := login("203.0.113.7:5004", "ARTEX", "wrong"); w.Code != http.StatusUnauthorized {
			t.Fatalf("pre-reset attempt %d status=%d", i+1, w.Code)
		}
	}
	if w := login("203.0.113.7:5005", "ARTEX", "correct-horse"); w.Code != http.StatusOK {
		t.Fatalf("reset login status=%d body=%s", w.Code, w.Body.String())
	}
	if w := login("203.0.113.7:5006", "ARTEX", "wrong"); w.Code != http.StatusUnauthorized {
		t.Fatalf("post-reset failure status=%d, want 401 (counter was cleared)", w.Code)
	}
}
