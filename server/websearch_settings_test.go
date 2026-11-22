package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func preserveWebSearchSettings(t *testing.T, m *Manager) {
	t.Helper()
	keys := []string{settingWebSearchOn, settingWebSearchBackend}
	old := map[string]struct {
		value   string
		existed bool
	}{}
	for _, key := range keys {
		v, ok, err := m.pg.GetSetting(key)
		if err != nil {
			t.Fatal(err)
		}
		old[key] = struct {
			value   string
			existed bool
		}{v, ok}
	}
	t.Cleanup(func() {
		for _, key := range keys {
			if old[key].existed {
				if err := m.pg.SetSetting(key, old[key].value); err != nil {
					t.Error(err)
				}
			} else if _, err := m.pg.Exec("DELETE FROM settings WHERE key=$1", key); err != nil {
				t.Error(err)
			}
		}
	})
}

func TestSetWebSearchBackendWhitelist(t *testing.T) {
	m := proxyTestManager(t)
	preserveWebSearchSettings(t, m)

	for _, tc := range []struct{ input, want string }{
		{"", "ddgs"},
		{"ddgs", "ddgs"},
		{"brave-free", "brave-free"},
		{"deepseek", "deepseek"},
		{"  tavily  ", "tavily"},
	} {
		if err := m.SetWebSearch(false, tc.input, nil, nil, nil); err != nil {
			t.Fatalf("backend %q: %v", tc.input, err)
		}
		if _, got, _, _, _ := m.WebSearch(); got != tc.want {
			t.Fatalf("backend %q stored as %q, want %q", tc.input, got, tc.want)
		}
	}

	for _, bad := range []string{"bogus", "brave"} {
		if err := m.SetWebSearch(false, bad, nil, nil, nil); err == nil {
			t.Fatalf("unknown backend %q accepted", bad)
		}
		if _, got, _, _, _ := m.WebSearch(); got != "tavily" {
			t.Fatalf("rejected backend %q changed stored value to %q", bad, got)
		}
	}
}

func TestWebSearchBackendRejectedByHandlers(t *testing.T) {
	m := proxyTestManager(t)
	preserveWebSearchSettings(t, m)
	s := &Server{m: m}

	put := httptest.NewRecorder()
	s.putSettings(put, httptest.NewRequest(http.MethodPut, "/api/settings",
		strings.NewReader(`{"web_search_backend":"bogus"}`)))
	if put.Code != http.StatusBadRequest {
		t.Fatalf("putSettings status=%d body=%s", put.Code, put.Body.String())
	}

	probe := httptest.NewRecorder()
	s.testWebSearch(probe, httptest.NewRequest(http.MethodPost, "/api/settings/websearch/test",
		strings.NewReader(`{"web_search_backend":"bogus"}`)))
	if probe.Code != http.StatusBadRequest {
		t.Fatalf("testWebSearch status=%d body=%s", probe.Code, probe.Body.String())
	}
}
