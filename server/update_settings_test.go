package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateSettingsHTTPPersistence(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	defer m.Close()
	old, existed, err := m.pg.GetSetting(updateSettingsKey)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if existed {
			m.pg.SetSetting(updateSettingsKey, old)
		} else {
			m.pg.Exec("DELETE FROM settings WHERE key=$1", updateSettingsKey)
		}
	}()
	m.pg.Exec("DELETE FROM settings WHERE key=$1", updateSettingsKey)
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := New(ctx, m, dir, dir, dir)
	token, err := signJWT(s.jwtKey, 0)
	if err != nil {
		t.Fatal(err)
	}
	handler := s.Handler()
	request := func(method, body string, authenticated bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/update/settings", bytes.NewBufferString(body))
		if authenticated {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res
	}
	for _, method := range []string{"GET", "PUT"} {
		if r := request(method, `{}`, false); r.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated %s: %d", method, r.Code)
		}
	}
	if cfg, err := s.loadUpdateSettings(); err != nil || cfg.Proxy != "" {
		t.Fatalf("default: %+v %v", cfg, err)
	}
	for _, body := range []string{`{"proxy":"127.0.0.1:7897"}`, `{"proxy":"http://proxy:65536"}`, `{`} {
		if r := request("PUT", body, true); r.Code != 400 {
			t.Fatalf("invalid setting accepted: %d %s", r.Code, r.Body.String())
		}
	}
	m.mu.Lock()
	m.globalProxy = "http://127.0.0.1:1"
	m.mu.Unlock()
	originalGlobal := m.GlobalProxy()
	for _, proxy := range []string{" http://127.0.0.1:7897 ", ""} {
		body, _ := json.Marshal(updateSettings{Proxy: proxy})
		if r := request("PUT", string(body), true); r.Code != 200 {
			t.Fatalf("save: %d %s", r.Code, r.Body.String())
		}
		// Read through a fresh Server to prove the setting comes from the DB.
		fresh := &Server{m: m}
		cfg, err := fresh.loadUpdateSettings()
		if err != nil || cfg.Proxy != strings.TrimSpace(proxy) {
			t.Fatalf("reload: %+v %v", cfg, err)
		}
		r := request("GET", "", true)
		if r.Code != 200 {
			t.Fatalf("GET settings: %d", r.Code)
		}
		want := cfg
		var got updateSettings
		if err := json.Unmarshal(r.Body.Bytes(), &got); err != nil || got != want {
			t.Fatalf("GET: %+v %v", got, err)
		}
		client, effective, err := fresh.updateClient()
		if err != nil {
			t.Fatal(err)
		}
		defer client.CloseIdleConnections()
		if proxy != "" && effective != "http://127.0.0.1:7897" {
			t.Fatalf("proxy not normalized: %q", effective)
		}
		if proxy == "" && client.Transport.(*http.Transport).Proxy != nil {
			t.Fatal("direct inherited proxy")
		}
		if m.GlobalProxy() != originalGlobal {
			t.Fatal("update setting changed global proxy")
		}
	}
}

func TestUpdateSettingsMasksProxyCredentials(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	defer m.Close()
	old, existed, err := m.pg.GetSetting(updateSettingsKey)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if existed {
			m.pg.SetSetting(updateSettingsKey, old)
		} else {
			m.pg.Exec("DELETE FROM settings WHERE key=$1", updateSettingsKey)
		}
	}()
	m.pg.Exec("DELETE FROM settings WHERE key=$1", updateSettingsKey)

	s := &Server{m: m}
	put := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.putUpdateSettings(w, httptest.NewRequest(http.MethodPut, "/api/update/settings", strings.NewReader(body)))
		return w
	}
	get := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.getUpdateSettings(w, httptest.NewRequest(http.MethodGet, "/api/update/settings", nil))
		return w
	}

	const saved = "http://user:secret@proxy.example:3128"
	if w := put(`{"proxy":"` + saved + `"}`); w.Code != http.StatusOK {
		t.Fatalf("save status=%d body=%s", w.Code, w.Body.String())
	}
	if cfg, err := s.loadUpdateSettings(); err != nil || cfg.Proxy != saved {
		t.Fatalf("stored proxy=%q err=%v", cfg.Proxy, err)
	}
	w := get()
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("GET leaked credentials: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"proxy":"http://***@proxy.example:3128"`) {
		t.Fatalf("GET proxy not masked: %s", w.Body.String())
	}

	// Editing the host from the masked form keeps the saved credentials.
	if w := put(`{"proxy":"http://***@new.example:9090"}`); w.Code != http.StatusOK {
		t.Fatalf("masked save status=%d body=%s", w.Code, w.Body.String())
	}
	if cfg, err := s.loadUpdateSettings(); err != nil || cfg.Proxy != "http://user:secret@new.example:9090" {
		t.Fatalf("merged proxy=%q err=%v", cfg.Proxy, err)
	}

	// A mask with nothing saved cannot be spliced; reject instead of storing it.
	if w := put(`{"proxy":""}`); w.Code != http.StatusOK {
		t.Fatalf("clear status=%d", w.Code)
	}
	if w := put(`{"proxy":"http://***@x.example:1"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("mask without saved auth status=%d body=%s", w.Code, w.Body.String())
	}
	if cfg, _ := s.loadUpdateSettings(); cfg.Proxy != "" {
		t.Fatalf("rejected save changed proxy to %q", cfg.Proxy)
	}
}
