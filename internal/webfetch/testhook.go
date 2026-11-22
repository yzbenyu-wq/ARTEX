package webfetch

import (
	"context"
	"net/url"
	"syscall"
)

// AllowLoopbackForTest disables the SSRF guard and returns a restore function.
// Only tests that must reach an httptest server on 127.0.0.1 call it; the
// production guard is on by default.
func AllowLoopbackForTest() func() {
	oldCheck, oldDial := checkTarget, dialGuard
	checkTarget = func(_ context.Context, _ *url.URL) error { return nil }
	dialGuard = func(_, _ string, _ syscall.RawConn) error { return nil }
	return func() { checkTarget, dialGuard = oldCheck, oldDial }
}
