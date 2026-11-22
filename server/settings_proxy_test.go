package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestPutSettingsMergesMaskedProxyCredentials locks the contract between the
// settings form (which echoes "http://***@host") and the stored proxy: editing
// the host must keep the saved credentials instead of reverting the whole value.
func TestPutSettingsMergesMaskedProxyCredentials(t *testing.T) {
	m := proxyTestManager(t)
	// Start from "no proxy" regardless of what an earlier test left behind.
	if err := m.setProxyPoolEntries(nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := m.SetGlobalProxy(""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = m.setProxyPoolEntries(nil, "")
		_ = m.SetGlobalProxy("")
	})
	m.mu.Lock()
	m.globalProxy = "http://old:secret@old.example:8080"
	m.webSearchProxy = "http://search:secret@search.example:3128"
	m.mu.Unlock()
	t.Cleanup(func() {
		m.mu.Lock()
		m.webSearchProxy = ""
		m.mu.Unlock()
		_, _ = m.pg.Exec("DELETE FROM settings WHERE key=$1", settingWebSearchProxy)
	})

	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	s := New(ctx, m, dir, dir, dir)
	defer func() {
		cancel()
		s.Close()
	}()

	put := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
		w := httptest.NewRecorder()
		s.putSettings(w, req)
		return w
	}

	w := put(`{"global_proxy":"http://***@new.example:9090"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("global proxy status=%d body=%s", w.Code, w.Body.String())
	}
	if got := m.GlobalProxy(); got != "http://old:secret@new.example:9090" {
		t.Fatalf("global proxy=%q, want new host with saved credentials", got)
	}
	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if got, _ := payload["global_proxy"].(string); got != "http://***@new.example:9090" {
		t.Fatalf("payload global_proxy=%v, want masked echo", payload["global_proxy"])
	}

	w = put(`{"web_search_proxy":"http://***@new-search.example:3128"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("web search proxy status=%d body=%s", w.Code, w.Body.String())
	}
	if _, _, _, _, got := m.WebSearch(); got != "http://search:secret@new-search.example:3128" {
		t.Fatalf("web search proxy=%q", got)
	}
}

func TestPutSettingsRejectsOversizedBody(t *testing.T) {
	m := proxyTestManager(t)
	s := &Server{m: m}
	body := `{"python_interpreter":"` + strings.Repeat("a", 70<<10) + `"}`
	w := httptest.NewRecorder()
	s.putSettings(w, httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("oversized body status=%d body=%.200s", w.Code, w.Body.String())
	}
}

// TestPutSettingsValidatesBeforeWriting pins the ordering guarantee: the form
// saves many fields at once, so an invalid value anywhere in the payload must
// reject the whole request before any field reaches the database.
func TestPutSettingsValidatesBeforeWriting(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		check func(t *testing.T, m *Manager)
	}{
		{
			name: "invalid digest leaves global proxy untouched",
			body: `{"global_proxy":"http://***@new.example:9090","notify_digest_interval_min":0}`,
			check: func(t *testing.T, m *Manager) {
				if got := m.GlobalProxy(); got != "http://old:secret@old.example:8080" {
					t.Fatalf("global proxy=%q, must be untouched", got)
				}
			},
		},
		{
			name: "invalid global proxy leaves notify switch untouched",
			body: `{"notify_enabled":true,"global_proxy":"ftp://bad"}`,
			check: func(t *testing.T, m *Manager) {
				if on := m.pg.GetBool(settingNotifyEnabled, false); on {
					t.Fatal("notify_enabled persisted before proxy validation failed")
				}
			},
		},
		{
			name: "out-of-range proxy port leaves notify switch untouched",
			body: `{"notify_enabled":true,"global_proxy":"http://proxy.example:65536"}`,
			check: func(t *testing.T, m *Manager) {
				if on := m.pg.GetBool(settingNotifyEnabled, false); on {
					t.Fatal("notify_enabled persisted before port validation failed")
				}
			},
		},
		{
			name: "invalid pool settings leave notify base untouched",
			body: `{"notify_public_base_url":"https://hook.example","proxy_rotate_strategy":"bogus"}`,
			check: func(t *testing.T, m *Manager) {
				got, ok, err := m.pg.GetSetting(settingNotifyPublicBaseURL)
				if err != nil {
					t.Fatal(err)
				}
				if ok && got != "" {
					t.Fatalf("notify base=%q, must be untouched", got)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := proxyTestManager(t)
			if err := m.SetGlobalProxy(""); err != nil {
				t.Fatal(err)
			}
			if err := m.pg.SetBool(settingNotifyEnabled, false); err != nil {
				t.Fatal(err)
			}
			if err := m.pg.SetSetting(settingNotifyPublicBaseURL, ""); err != nil {
				t.Fatal(err)
			}
			m.mu.Lock()
			m.globalProxy = "http://old:secret@old.example:8080"
			m.mu.Unlock()
			t.Cleanup(func() { _ = m.SetGlobalProxy("") })

			dir := t.TempDir()
			ctx, cancel := context.WithCancel(context.Background())
			s := New(ctx, m, dir, dir, dir)
			defer func() {
				cancel()
				s.Close()
			}()

			w := httptest.NewRecorder()
			s.putSettings(w, httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(tc.body)))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			tc.check(t, m)
		})
	}
}
