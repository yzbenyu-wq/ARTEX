package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
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

// TestPutSettingsValidatesPythonInterp pins the save-time check: a bad path is a
// client error at save, not a surprise at the first script-tool run.
func TestPutSettingsValidatesPythonInterp(t *testing.T) {
	s := guardTestServer(t)
	if err := s.m.pg.SetSetting(settingPythonInterp, "/usr/bin/python3"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = s.m.pg.Exec("DELETE FROM settings WHERE key=$1", settingPythonInterp) })

	for _, body := range []string{
		`{"python_interpreter":"python3"}`,
		`{"python_interpreter":"/nonexistent/py-artex-test"}`,
	} {
		w := putSettingsForTest(s, body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body=%s status=%d, want 400; body=%s", body, w.Code, w.Body.String())
		}
	}
	got, _, err := s.m.pg.GetSetting(settingPythonInterp)
	if err != nil {
		t.Fatal(err)
	}
	if got != "/usr/bin/python3" {
		t.Fatalf("stored interpreter=%q, a refused save must not rewrite it", got)
	}

	executable := filepath.Join(t.TempDir(), "python3")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	w := putSettingsForTest(s, fmt.Sprintf(`{"python_interpreter":%q}`, executable))
	if w.Code != http.StatusOK {
		t.Fatalf("valid path status=%d body=%s", w.Code, w.Body.String())
	}
	got, _, err = s.m.pg.GetSetting(settingPythonInterp)
	if err != nil {
		t.Fatal(err)
	}
	if got != executable {
		t.Fatalf("stored interpreter=%q, want %q", got, executable)
	}
}

// TestSetWebSearchTrimsKeys pins the trim: whitespace around a pasted key would
// otherwise reach the provider as part of the header value and fail at runtime.
func TestSetWebSearchTrimsKeys(t *testing.T) {
	s := guardTestServer(t)
	t.Cleanup(func() {
		_, _ = s.m.pg.Exec("DELETE FROM settings WHERE key=$1", settingBraveKey)
		_, _ = s.m.pg.Exec("DELETE FROM settings WHERE key=$1", settingTavilyKey)
	})
	brave, tavily := "  brave-key  ", "\ttavily-key\n"
	if err := s.m.SetWebSearch(false, "ddgs", &brave, &tavily, nil); err != nil {
		t.Fatal(err)
	}
	_, _, gotBrave, gotTavily, _ := s.m.WebSearch()
	if gotBrave != "brave-key" || gotTavily != "tavily-key" {
		t.Fatalf("keys=%q/%q, want trimmed brave-key/tavily-key", gotBrave, gotTavily)
	}
}

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

// TestPutMCPServerSettingsPartialUpdate tables the merge semantics: a field
// the request omits keeps its stored list, an explicit empty list clears it,
// and an invalid entry leaves the stored row untouched.
func TestPutMCPServerSettingsPartialUpdate(t *testing.T) {
	s := guardTestServer(t)
	const initial = `{"allowed_ips":["203.0.113.1"],"trusted_proxies":["10.0.0.0/8"]}`
	t.Cleanup(func() { _, _ = s.m.pg.Exec("DELETE FROM settings WHERE key=$1", db.SettingMCPServerAccess) })

	put := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.putMCPServerSettings(w, httptest.NewRequest(http.MethodPut, "/api/settings/mcp-server", strings.NewReader(body)))
		return w
	}
	tests := []struct {
		name        string
		body        string
		wantStatus  int
		wantAllowed []string
		wantTrusted []string
	}{
		{
			name: "trusted only keeps allowed", body: `{"trusted_proxies":["10.1.0.0/16"]}`,
			wantStatus: http.StatusOK, wantAllowed: []string{"203.0.113.1"}, wantTrusted: []string{"10.1.0.0/16"},
		},
		{
			name: "allowed only keeps trusted", body: `{"allowed_ips":["203.0.113.0/24"]}`,
			wantStatus: http.StatusOK, wantAllowed: []string{"203.0.113.0/24"}, wantTrusted: []string{"10.0.0.0/8"},
		},
		{
			name: "both lists replace both", body: `{"allowed_ips":["198.51.100.7"],"trusted_proxies":["10.2.0.0/16"]}`,
			wantStatus: http.StatusOK, wantAllowed: []string{"198.51.100.7"}, wantTrusted: []string{"10.2.0.0/16"},
		},
		{
			name: "explicit empty list clears it", body: `{"allowed_ips":[]}`,
			wantStatus: http.StatusOK, wantAllowed: []string{}, wantTrusted: []string{"10.0.0.0/8"},
		},
		{
			name: "invalid entry is refused", body: `{"allowed_ips":["not-an-ip"]}`,
			wantStatus: http.StatusBadRequest, wantAllowed: []string{"203.0.113.1"}, wantTrusted: []string{"10.0.0.0/8"},
		},
		{
			name: "empty body is refused", body: `{}`,
			wantStatus: http.StatusBadRequest, wantAllowed: []string{"203.0.113.1"}, wantTrusted: []string{"10.0.0.0/8"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := s.m.pg.SetSetting(db.SettingMCPServerAccess, initial); err != nil {
				t.Fatal(err)
			}
			if w := put(tc.body); w.Code != tc.wantStatus {
				t.Fatalf("status=%d, want %d; body=%s", w.Code, tc.wantStatus, w.Body.String())
			}
			raw, _, err := s.m.pg.GetSetting(db.SettingMCPServerAccess)
			if err != nil {
				t.Fatal(err)
			}
			var saved mcpAccessConfig
			if err := json.Unmarshal([]byte(raw), &saved); err != nil {
				t.Fatalf("stored row is not JSON: %v (%q)", err, raw)
			}
			if !reflect.DeepEqual(saved.AllowedIPs, tc.wantAllowed) {
				t.Fatalf("allowed_ips=%v, want %v", saved.AllowedIPs, tc.wantAllowed)
			}
			if !reflect.DeepEqual(saved.TrustedProxies, tc.wantTrusted) {
				t.Fatalf("trusted_proxies=%v, want %v", saved.TrustedProxies, tc.wantTrusted)
			}
		})
	}
}
