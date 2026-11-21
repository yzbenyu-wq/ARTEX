package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Autumn-27/artex/db"
)

func TestLoadToolPresets(t *testing.T) {
	presets, err := loadToolPresets()
	if err != nil {
		t.Fatalf("loadToolPresets: %v", err)
	}
	if len(presets) != 12 {
		t.Fatalf("preset count = %d, want 12", len(presets))
	}
	want := map[string]bool{
		"httpx": true, "subfinder": true, "dnsx": true, "tlsx": true, "naabu": true,
		"katana": true, "nuclei": true, "gau": true, "waybackurls": true,
		"ffuf": true, "feroxbuster": true, "nmap": true,
	}
	for _, p := range presets {
		if !want[p.Key] {
			t.Fatalf("unexpected preset %q", p.Key)
		}
		delete(want, p.Key)
		if p.Binary == "" || p.Description == "" || p.Category == "" {
			t.Fatalf("%s: missing binary/description/category: %+v", p.Key, p)
		}
		if !hasSchemaProps(p.Schema) {
			t.Fatalf("%s: empty schema", p.Key)
		}
		if _, err := parseRecipeExec(p.Exec); err != nil {
			t.Fatalf("%s: bad exec: %v", p.Key, err)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing presets: %v", want)
	}
}

func TestPresetHttpxRendersMinimalArgv(t *testing.T) {
	presets, err := loadToolPresets()
	if err != nil {
		t.Fatal(err)
	}
	var httpx toolPreset
	for _, p := range presets {
		if p.Key == "httpx" {
			httpx = p
		}
	}
	spec, err := parseRecipeExec(httpx.Exec)
	if err != nil {
		t.Fatal(err)
	}
	got, err := buildRecipeArgv(spec, map[string]any{"target": "https://example.com"}, "http://127.0.0.1:8788")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-silent", "-no-color", "-u", "https://example.com", "-json", "-proxy", "http://127.0.0.1:8788"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("argv = %q want %q", got, want)
	}
}

func TestPresetHandlers(t *testing.T) {
	pg := openToolboxDB(t)
	for _, k := range []string{"httpx", "subfinder"} {
		_ = pg.DeleteCustomTool(k)
	}
	t.Cleanup(func() {
		for _, k := range []string{"httpx", "subfinder"} {
			_ = pg.DeleteCustomTool(k)
		}
	})
	s := &Server{m: &Manager{pg: pg}}

	rec := httptest.NewRecorder()
	s.pgInstallToolPresets(rec, httptest.NewRequest(http.MethodPost, "/api/tools/presets/install",
		strings.NewReader(`{"keys":["httpx","not_a_preset","httpx"]}`)))
	if rec.Code != 200 {
		t.Fatalf("install status %d: %s", rec.Code, rec.Body.String())
	}
	out := decodeBody[struct {
		Installed []string `json:"installed"`
		Skipped   []string `json:"skipped"`
		Errors    []string `json:"errors"`
	}](t, rec)
	if len(out.Installed) != 1 || out.Installed[0] != "httpx" || len(out.Errors) != 1 {
		t.Fatalf("install result = %+v", out)
	}

	row, err := pg.GetTool("httpx")
	if err != nil || row == nil {
		t.Fatalf("installed tool missing: %v", err)
	}
	if row.System || row.Enabled || !row.Deferred || row.Kind != "recipe" ||
		row.Source != "preset" || row.ProxyMode != "http" || row.Binary != "httpx" ||
		row.Category != "recon" || len(row.Agents) != 0 || len(row.Exec) == 0 {
		t.Fatalf("installed defaults wrong: %+v", row)
	}

	rec = httptest.NewRecorder()
	s.pgInstallToolPresets(rec, httptest.NewRequest(http.MethodPost, "/api/tools/presets/install",
		strings.NewReader(`{"keys":["httpx"]}`)))
	again := decodeBody[struct {
		Installed []string `json:"installed"`
		Skipped   []string `json:"skipped"`
	}](t, rec)
	if len(again.Installed) != 0 || len(again.Skipped) != 1 {
		t.Fatalf("reinstall should skip: %+v", again)
	}

	rec = httptest.NewRecorder()
	s.pgListToolPresets(rec, httptest.NewRequest(http.MethodGet, "/api/tools/presets", nil))
	if rec.Code != 200 {
		t.Fatalf("list status %d: %s", rec.Code, rec.Body.String())
	}
	list := decodeBody[struct {
		Presets []struct {
			Key       string `json:"key"`
			Installed bool   `json:"installed"`
		} `json:"presets"`
	}](t, rec)
	flags := map[string]bool{}
	for _, p := range list.Presets {
		flags[p.Key] = p.Installed
	}
	if !flags["httpx"] || flags["subfinder"] {
		t.Fatalf("installed flags wrong: %+v", flags)
	}

	rec = httptest.NewRecorder()
	s.pgInstallToolPresets(rec, httptest.NewRequest(http.MethodPost, "/api/tools/presets/install",
		strings.NewReader(`{"keys":[]}`)))
	if rec.Code != 400 {
		t.Fatalf("empty keys should be 400, got %d", rec.Code)
	}
}

func TestCreateRecipeToolValidation(t *testing.T) {
	pg := openToolboxDB(t)
	keys := []string{"tbx_t_recipe_ok", "tbx_t_recipe_noschema", "tbx_t_recipe_badargv"}
	for _, k := range keys {
		_ = pg.DeleteCustomTool(k)
	}
	t.Cleanup(func() {
		for _, k := range keys {
			_ = pg.DeleteCustomTool(k)
		}
	})
	s := &Server{m: &Manager{pg: pg}}
	post := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		s.pgCreateCustomTool(rec, httptest.NewRequest(http.MethodPost, "/api/tools/custom", strings.NewReader(body)))
		return rec
	}

	if rec := post(`{"key":"tbx_t_recipe_noschema","kind":"recipe","proxy_mode":"http",
		"exec":{"argv":[["-silent"]]}}`); rec.Code != 400 {
		t.Fatalf("recipe without schema should be 400, got %d (%s)", rec.Code, rec.Body.String())
	}
	if rec := post(`{"key":"tbx_t_recipe_badargv","kind":"recipe","proxy_mode":"http",
		"schema":{"type":"object","properties":{"target":{"type":"string"}}},"exec":{"argv":[]}}`); rec.Code != 400 {
		t.Fatalf("recipe with empty argv should be 400, got %d (%s)", rec.Code, rec.Body.String())
	}
	rec := post(`{"key":"tbx_t_recipe_ok","kind":"recipe","proxy_mode":"env",
		"schema":{"type":"object","properties":{"target":{"type":"string"}}},
		"exec":{"argv":[["-silent"],["-u","{target}"]]}}`)
	if rec.Code != 200 {
		t.Fatalf("valid recipe should be 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	row, err := pg.GetTool("tbx_t_recipe_ok")
	if err != nil || row == nil || row.Kind != "recipe" || row.ProxyMode != "env" {
		t.Fatalf("created recipe wrong: %+v (%v)", row, err)
	}

	var raw map[string]any
	if err := json.Unmarshal(row.Schema, &raw); err != nil || raw["properties"] == nil {
		t.Fatalf("schema not persisted: %s", row.Schema)
	}
}

func TestUpdateRecipeToolKeepsSource(t *testing.T) {
	pg := openToolboxDB(t)
	key := "tbx_t_recipe_upd"
	_ = pg.DeleteCustomTool(key)
	t.Cleanup(func() { _ = pg.DeleteCustomTool(key) })
	if err := pg.CreateCustomTool(&db.Tool{
		Key: key, Kind: "recipe", Enabled: false, Deferred: true,
		Schema:   json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"}}}`),
		Exec:     json.RawMessage(`{"argv":[["-u","{target}"]]}`),
		ToolMeta: db.ToolMeta{Binary: "sh", ProxyMode: "inherit", Source: "preset"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	s := &Server{m: &Manager{pg: pg}}
	rec := httptest.NewRecorder()
	body := `{"kind":"recipe","proxy_mode":"none","enabled":true,"deferred":false,
		"schema":{"type":"object","properties":{"target":{"type":"string"}}},
		"exec":{"argv":[["-u","{target}"]],"timeout_ms":1000}}`
	req := httptest.NewRequest(http.MethodPut, "/api/tools/custom/"+key, strings.NewReader(body))
	req.SetPathValue("key", key)
	s.pgUpdateCustomTool(rec, req)
	if rec.Code != 200 {
		t.Fatalf("update status %d: %s", rec.Code, rec.Body.String())
	}
	row, _ := pg.GetTool(key)
	if row == nil || row.Kind != "recipe" || row.Source != "preset" || row.ProxyMode != "none" || !row.Enabled || row.Deferred {
		t.Fatalf("updated row wrong: %+v", row)
	}
}
