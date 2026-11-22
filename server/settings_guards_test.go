package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Autumn-27/artex/db"
)

func guardTestServer(t *testing.T) *Server {
	t.Helper()
	m := proxyTestManager(t)
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	s := New(ctx, m, dir, dir, dir)
	t.Cleanup(func() {
		cancel()
		s.Close()
	})
	return s
}

func putSettingsForTest(s *Server, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	s.putSettings(w, httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body)))
	return w
}

// TestPutSettingsRejectsGlobalProxyWithManagedPool pins the guard: once a pool
// holds entries the pool API owns egress, so the legacy field must be refused
// rather than silently ignored with a 200.
func TestPutSettingsRejectsGlobalProxyWithManagedPool(t *testing.T) {
	s := guardTestServer(t)
	entries, active := legacyProxyPool("http://127.0.0.1:3128")
	if err := s.m.setProxyPoolEntries(entries, active); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.m.setProxyPoolEntries(nil, "") }()

	before := s.m.GlobalProxy()
	for _, body := range []string{
		`{"global_proxy":"http://new.example:9090"}`,
		`{"global_proxy":""}`,
	} {
		w := putSettingsForTest(s, body)
		if w.Code != http.StatusConflict {
			t.Fatalf("body=%s status=%d, want 409; body=%s", body, w.Code, w.Body.String())
		}
	}
	if got := s.m.GlobalProxy(); got != before {
		t.Fatalf("global proxy=%q, want unchanged %q", got, before)
	}
	if len(s.m.proxyPoolEntries()) != 1 {
		t.Fatalf("pool entries=%d, want 1 (a refused save must not clear the pool)", len(s.m.proxyPoolEntries()))
	}
}

// TestPutSettingsRejectsInvalidConcurrencyLimit pins the 400: a non-positive
// cap is a client error, not something to silently replace with the default.
func TestPutSettingsRejectsInvalidConcurrencyLimit(t *testing.T) {
	s := guardTestServer(t)
	if err := s.m.pg.SetSetting(settingConcurrencyLimit, "7"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = s.m.pg.Exec("DELETE FROM settings WHERE key=$1", settingConcurrencyLimit) })

	for _, body := range []string{
		`{"task_concurrency_limit":0}`,
		`{"task_concurrency_limit":-3}`,
	} {
		w := putSettingsForTest(s, body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body=%s status=%d, want 400; body=%s", body, w.Code, w.Body.String())
		}
	}
	got, ok, err := s.m.pg.GetSetting(settingConcurrencyLimit)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got != "7" {
		t.Fatalf("stored limit=%q ok=%v, want untouched 7", got, ok)
	}
}

// TestPutMCPServerSettingsCorruptAccess pins both halves of the corrupt-row
// rule: a partial update is refused (it would silently clear the other list,
// and an empty allowlist means "allow any"), while submitting both lists
// repairs the row.
func TestPutMCPServerSettingsCorruptAccess(t *testing.T) {
	s := guardTestServer(t)
	if err := s.m.pg.SetSetting(db.SettingMCPServerAccess, "not-json"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = s.m.pg.Exec("DELETE FROM settings WHERE key=$1", db.SettingMCPServerAccess) })

	put := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.putMCPServerSettings(w, httptest.NewRequest(http.MethodPut, "/api/settings/mcp-server", strings.NewReader(body)))
		return w
	}
	for _, body := range []string{
		`{"trusted_proxies":["10.0.0.0/8"]}`,
		`{"allowed_ips":["203.0.113.0/24"]}`,
	} {
		if w := put(body); w.Code != http.StatusConflict {
			t.Fatalf("body=%s status=%d, want 409; body=%s", body, w.Code, w.Body.String())
		}
	}
	if raw, _, _ := s.m.pg.GetSetting(db.SettingMCPServerAccess); raw != "not-json" {
		t.Fatalf("corrupt row=%q, a refused save must not rewrite it", raw)
	}

	w := put(`{"allowed_ips":["203.0.113.0/24"],"trusted_proxies":["10.0.0.0/8"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("repair status=%d body=%s", w.Code, w.Body.String())
	}
	raw, _, err := s.m.pg.GetSetting(db.SettingMCPServerAccess)
	if err != nil {
		t.Fatal(err)
	}
	var saved mcpAccessConfig
	if err := json.Unmarshal([]byte(raw), &saved); err != nil {
		t.Fatalf("repaired row is not JSON: %v (%q)", err, raw)
	}
	if len(saved.AllowedIPs) != 1 || saved.AllowedIPs[0] != "203.0.113.0/24" {
		t.Fatalf("allowed_ips=%v, want the repaired list", saved.AllowedIPs)
	}
	if len(saved.TrustedProxies) != 1 || saved.TrustedProxies[0] != "10.0.0.0/8" {
		t.Fatalf("trusted_proxies=%v, want the repaired list", saved.TrustedProxies)
	}
}

// TestPutMCPServerSettingsPartialUpdateKeepsOtherList is the control case: a
// valid stored row still accepts partial updates and keeps the unset list.
func TestPutMCPServerSettingsPartialUpdateKeepsOtherList(t *testing.T) {
	s := guardTestServer(t)
	if err := s.m.pg.SetSetting(db.SettingMCPServerAccess, `{"allowed_ips":["203.0.113.1"],"trusted_proxies":["10.0.0.0/8"]}`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = s.m.pg.Exec("DELETE FROM settings WHERE key=$1", db.SettingMCPServerAccess) })

	w := httptest.NewRecorder()
	s.putMCPServerSettings(w, httptest.NewRequest(http.MethodPut, "/api/settings/mcp-server",
		strings.NewReader(`{"trusted_proxies":["10.1.0.0/16"]}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	raw, _, err := s.m.pg.GetSetting(db.SettingMCPServerAccess)
	if err != nil {
		t.Fatal(err)
	}
	var saved mcpAccessConfig
	if err := json.Unmarshal([]byte(raw), &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.AllowedIPs) != 1 || saved.AllowedIPs[0] != "203.0.113.1" {
		t.Fatalf("allowed_ips=%v, a partial update must keep the unset list", saved.AllowedIPs)
	}
	if len(saved.TrustedProxies) != 1 || saved.TrustedProxies[0] != "10.1.0.0/16" {
		t.Fatalf("trusted_proxies=%v, want the submitted list", saved.TrustedProxies)
	}
}
