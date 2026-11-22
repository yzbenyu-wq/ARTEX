package server

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	// Five failures inside one minute lock the source out for five minutes.
	loginMaxFailures       = 5
	loginFailureWindow     = time.Minute
	loginLockout           = 5 * time.Minute
	loginLimiterMaxEntries = 10000
)

// loginLimiter bounds password guessing per client address. It is a fixed
// window: failures inside loginFailureWindow are counted, reaching
// loginMaxFailures locks the address for loginLockout, and a successful login
// clears the record.
type loginLimiter struct {
	mu      sync.Mutex
	entries map[string]loginAttempt
	now     func() time.Time
}

type loginAttempt struct {
	failures    int
	windowStart time.Time
	lockedUntil time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{entries: map[string]loginAttempt{}, now: time.Now}
}

// lockedFor reports how much longer the address is blocked. Zero means it may try.
func (l *loginLimiter) lockedFor(key string) time.Duration {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[key]
	if !ok {
		return 0
	}
	remaining := e.lockedUntil.Sub(l.now())
	if remaining <= 0 {
		return 0
	}
	return remaining
}

// fail records one rejected credential attempt.
func (l *loginLimiter) fail(key string) {
	if l == nil {
		return
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[key]
	if !ok {
		if len(l.entries) >= loginLimiterMaxEntries {
			l.sweepLocked(now)
			if len(l.entries) >= loginLimiterMaxEntries {
				// The table is full of live records: stop tracking new sources
				// rather than grow without bound. Existing lockouts stay in force.
				return
			}
		}
		e = loginAttempt{windowStart: now}
	} else if now.Sub(e.windowStart) >= loginFailureWindow {
		e = loginAttempt{windowStart: now}
	}
	e.failures++
	if e.failures >= loginMaxFailures {
		e.lockedUntil = now.Add(loginLockout)
	}
	l.entries[key] = e
}

// reset clears the record after a successful login.
func (l *loginLimiter) reset(key string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

// sweepLocked drops records whose window has passed and whose lockout has expired.
func (l *loginLimiter) sweepLocked(now time.Time) {
	for key, e := range l.entries {
		if now.Sub(e.windowStart) >= loginFailureWindow && !now.Before(e.lockedUntil) {
			delete(l.entries, key)
		}
	}
}

// loginClientKey resolves the rate-limit key with the same address logic the MCP
// endpoint uses, so behind a trusted reverse proxy each client is limited
// individually instead of the proxy as a whole. A configuration that cannot be
// read falls back to the TCP peer.
func loginClientKey(r *http.Request, store interface {
	GetSetting(string) (string, bool, error)
}) string {
	var access mcpAccess
	if _, parsed, err := loadMCPAccess(store); err == nil {
		access = parsed
	}
	if client, ok := access.clientAddr(r); ok {
		return client.String()
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
