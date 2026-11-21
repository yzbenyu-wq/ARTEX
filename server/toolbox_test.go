package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Autumn-27/artex/db"
)

func openToolboxDB(t *testing.T) *db.DB {
	t.Helper()
	dsn, _, err := db.DSN()
	if err != nil {
		t.Skipf("no database config (%v) — skipping", err)
	}
	pg, err := db.Open(dsn)
	if err != nil {
		t.Skipf("postgres unavailable (%v) — skipping", err)
	}
	t.Cleanup(func() { pg.Close() })
	return pg
}

func writeScript(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "ver.sh")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(bytes.TrimSpace(rec.Body.Bytes()), &out); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return out
}

func TestProbeToolVersion(t *testing.T) {
	multi := writeScript(t, "echo 1.2.3\necho noise\n")
	if got := probeToolVersion(context.Background(), multi); got != "1.2.3" {
		t.Fatalf("first line not kept: %q", got)
	}
	if got := probeToolVersion(context.Background(), "echo http://user:pass@host"); got != "http://***@host" {
		t.Fatalf("credentials not masked: %q", got)
	}
	if got := probeToolVersion(context.Background(), "/definitely/not/a/binary --version"); got != "" {
		t.Fatalf("missing binary should yield empty, got %q", got)
	}
	if got := probeToolVersion(context.Background(), "   "); got != "" {
		t.Fatalf("empty cmd should yield empty, got %q", got)
	}
	long := writeScript(t, "head -c 5000 /dev/zero | tr '\\0' x\n")
	if got := probeToolVersion(context.Background(), long); len(got) > maxToolVersion {
		t.Fatalf("output not capped: %d > %d", len(got), maxToolVersion)
	}
}

func TestToolStatusHandler(t *testing.T) {
	pg := openToolboxDB(t)
	for _, k := range []string{"tbx_t_status_ok", "tbx_t_status_missing", "tbx_t_status_nobin"} {
		_ = pg.DeleteCustomTool(k)
	}
	t.Cleanup(func() {
		for _, k := range []string{"tbx_t_status_ok", "tbx_t_status_missing", "tbx_t_status_nobin"} {
			_ = pg.DeleteCustomTool(k)
		}
	})
	ver := writeScript(t, "echo 9.9.9\n")
	rows := []*db.Tool{
		{Key: "tbx_t_status_ok", Kind: "command", Exec: json.RawMessage(`{"command":"echo hi"}`),
			ToolMeta: db.ToolMeta{Binary: "sh", VersionCmd: ver}},
		{Key: "tbx_t_status_missing", Kind: "command", Exec: json.RawMessage(`{"command":"echo hi"}`),
			ToolMeta: db.ToolMeta{Binary: "artex-no-such-binary-xyz"}},
		{Key: "tbx_t_status_nobin", Kind: "command", Exec: json.RawMessage(`{"command":"echo hi"}`)},
	}
	for _, r := range rows {
		if err := pg.CreateCustomTool(r); err != nil {
			t.Fatalf("seed %s: %v", r.Key, err)
		}
	}
	s := &Server{m: &Manager{pg: pg}}

	run := func(query string) map[string]toolStatus {
		rec := httptest.NewRecorder()
		s.pgToolStatus(rec, httptest.NewRequest(http.MethodGet, "/api/tools/status"+query, nil))
		if rec.Code != 200 {
			t.Fatalf("status code %d: %s", rec.Code, rec.Body.String())
		}
		out := decodeBody[struct {
			Status []toolStatus `json:"status"`
		}](t, rec)
		m := map[string]toolStatus{}
		for _, st := range out.Status {
			m[st.Key] = st
		}
		return m
	}

	plain := run("")
	if plain["tbx_t_status_ok"].Installed != true || plain["tbx_t_status_ok"].Path == "" {
		t.Fatalf("installed binary not detected: %+v", plain["tbx_t_status_ok"])
	}
	if plain["tbx_t_status_ok"].Version != "" {
		t.Fatalf("version should be empty without ?version=1: %+v", plain["tbx_t_status_ok"])
	}
	if plain["tbx_t_status_missing"].Installed || plain["tbx_t_status_missing"].Error != "未安装" {
		t.Fatalf("missing binary status: %+v", plain["tbx_t_status_missing"])
	}
	if plain["tbx_t_status_nobin"].Error != "未配置二进制" {
		t.Fatalf("unset binary status: %+v", plain["tbx_t_status_nobin"])
	}

	withVer := run("?version=1")
	if withVer["tbx_t_status_ok"].Version != "9.9.9" {
		t.Fatalf("version not probed: %+v", withVer["tbx_t_status_ok"])
	}
}

func TestExportCustomToolsOnly(t *testing.T) {
	pg := openToolboxDB(t)
	key := "tbx_t_export"
	_ = pg.DeleteCustomTool(key)
	t.Cleanup(func() { _ = pg.DeleteCustomTool(key) })
	if err := pg.CreateCustomTool(&db.Tool{
		Key: key, Description: "export me", Kind: "command",
		Exec:     json.RawMessage(`{"command":"echo hi"}`),
		ToolMeta: db.ToolMeta{Category: "recon", Binary: "sh", Source: "custom"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := httptest.NewRecorder()
	(&Server{m: &Manager{pg: pg}}).pgExportTools(rec, httptest.NewRequest(http.MethodGet, "/api/tools/export", nil))
	if rec.Code != 200 {
		t.Fatalf("status code %d: %s", rec.Code, rec.Body.String())
	}
	out := decodeBody[struct {
		Version int       `json:"version"`
		Tools   []db.Tool `json:"tools"`
	}](t, rec)
	if out.Version != 1 {
		t.Fatalf("version = %d", out.Version)
	}
	found := false
	for _, tool := range out.Tools {
		if tool.System {
			t.Fatalf("export leaked built-in tool %q", tool.Key)
		}
		if tool.Key == key {
			found = true
			if tool.Category != "recon" || tool.Binary != "sh" {
				t.Fatalf("export dropped metadata: %+v", tool.ToolMeta)
			}
		}
	}
	if !found {
		t.Fatal("custom tool missing from export")
	}
}

func TestImportToolsValidationAndDefaults(t *testing.T) {
	pg := openToolboxDB(t)
	keys := []string{"tbx_t_imp_ok", "tbx_t_imp_dup", "tbx_t_imp_badkind", "tbx_t_imp_badproxy",
		"tbx_t_imp_http", "tbx_t_imp_on"}
	clean := func() {
		for _, k := range keys {
			_ = pg.DeleteCustomTool(k)
		}
	}
	clean()
	t.Cleanup(clean)

	body := `{"tools":[
		{"key":"tbx_t_imp_ok","description":"d","kind":"command","exec":{"command":"echo {args}"},
		 "agents":["worker"],"category":"recon","binary":"sh","proxy_mode":"socks5",
		 "version_cmd":"sh --version","homepage":"https://example.test"},
		{"key":"tbx_t_imp_dup","kind":"command","exec":{"command":"echo hi"}},
		{"key":"tbx_t_imp_dup","kind":"command","exec":{"command":"echo hi"}},
		{"key":"Bad Key","kind":"command"},
		{"key":"tbx_t_imp_badkind","kind":"nope"},
		{"key":"tbx_t_imp_badproxy","kind":"command","proxy_mode":"ftp"},
		{"key":"tbx_t_imp_http","kind":"http","exec":{"url":"https://example.test"}},
		{"key":"tbx_t_imp_on","kind":"command","exec":{"command":"echo hi"},"enabled":true,"deferred":false}
	]}`
	s := &Server{m: &Manager{pg: pg}}
	rec := httptest.NewRecorder()
	s.pgImportTools(rec, httptest.NewRequest(http.MethodPost, "/api/tools/import", strings.NewReader(body)))
	if rec.Code != 200 {
		t.Fatalf("status code %d: %s", rec.Code, rec.Body.String())
	}
	out := decodeBody[struct {
		Imported int      `json:"imported"`
		Skipped  int      `json:"skipped"`
		Errors   []string `json:"errors"`
	}](t, rec)
	if out.Imported != 3 || out.Skipped != 5 || len(out.Errors) != 5 {
		t.Fatalf("import result = %+v", out)
	}

	ok, err := pg.GetTool("tbx_t_imp_ok")
	if err != nil || ok == nil {
		t.Fatalf("imported tool missing: %v", err)
	}
	if ok.Enabled || !ok.Deferred || ok.Source != "imported" || ok.ProxyMode != "socks5" ||
		ok.Category != "recon" || ok.Binary != "sh" || ok.VersionCmd != "sh --version" ||
		ok.Homepage != "https://example.test" || len(ok.Agents) != 1 {
		t.Fatalf("imported defaults/metadata wrong: %+v", ok)
	}
	on, _ := pg.GetTool("tbx_t_imp_on")
	if on == nil || !on.Enabled || on.Deferred {
		t.Fatalf("explicit enabled/deferred not honored: %+v", on)
	}

	for name, payload := range map[string]string{
		"empty":     `{"tools":[]}`,
		"malformed": `not json`,
	} {
		rec := httptest.NewRecorder()
		s.pgImportTools(rec, httptest.NewRequest(http.MethodPost, "/api/tools/import", strings.NewReader(payload)))
		if rec.Code != 400 {
			t.Fatalf("%s: want 400, got %d (%s)", name, rec.Code, rec.Body.String())
		}
	}
}
