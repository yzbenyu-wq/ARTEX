package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/Autumn-27/artex/db"
	actool "github.com/Autumn-27/norma/tool"
)

func recipeSpec(t *testing.T, raw string) recipeExec {
	t.Helper()
	spec, err := parseRecipeExec(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parseRecipeExec(%s): %v", raw, err)
	}
	return spec
}

func TestParseRecipeExec(t *testing.T) {
	for name, raw := range map[string]string{
		"empty":       "",
		"malformed":   `not json`,
		"no argv":     `{"timeout_ms":1}`,
		"empty argv":  `{"argv":[]}`,
		"argv number": `{"argv":[1]}`,
	} {
		if _, err := parseRecipeExec(json.RawMessage(raw)); err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
	spec := recipeSpec(t, `{"argv":["-silent",["-u","{targets}"]],"timeout_ms":5000,"allowed_exit_codes":[1]}`)
	if len(spec.Argv) != 2 || spec.Argv[0][0] != "-silent" || spec.Argv[1][0] != "-u" {
		t.Fatalf("groups not parsed: %+v", spec.Argv)
	}
	if spec.TimeoutMs != 5000 || len(spec.AllowedExitCodes) != 1 {
		t.Fatalf("fields not parsed: %+v", spec)
	}
}

func TestBuildRecipeArgv(t *testing.T) {
	spec := recipeSpec(t, `{"argv":[
		["-silent"],
		["-u","{target}"],
		["-p","{ports}"],
		["--tag={tag}"],
		["-v","{verbose}"],
		["-u","{targets}"],
		["-proxy","{proxy}"]
	]}`)
	cases := []struct {
		name   string
		params map[string]any
		proxy  string
		want   []string
		wantOK bool
	}{
		{
			name:   "required only drops optionals",
			params: map[string]any{"target": "https://example.com"},
			want:   []string{"-silent", "-u", "https://example.com"},
			wantOK: true,
		},
		{
			name:   "optionals rendered",
			params: map[string]any{"target": "t", "ports": "80,443", "tag": "web", "proxy": "http://127.0.0.1:8080"},
			proxy:  "http://127.0.0.1:8080",
			want:   []string{"-silent", "-u", "t", "-p", "80,443", "--tag=web", "-proxy", "http://127.0.0.1:8080"},
			wantOK: true,
		},
		{
			name:   "array repeats the group",
			params: map[string]any{"target": "t", "targets": []any{"a", "b"}},
			want:   []string{"-silent", "-u", "t", "-u", "a", "-u", "b"},
			wantOK: true,
		},
		{
			name:   "empty array drops its group",
			params: map[string]any{"target": "t", "targets": []any{}},
			want:   []string{"-silent", "-u", "t"},
			wantOK: true,
		},
		{
			name:   "blank values drop groups",
			params: map[string]any{"target": "t", "ports": "   "},
			want:   []string{"-silent", "-u", "t"},
			wantOK: true,
		},
		{
			name:   "bool true switches on",
			params: map[string]any{"target": "t", "verbose": true},
			want:   []string{"-silent", "-u", "t", "-v"},
			wantOK: true,
		},
		{
			name:   "bool false switches off",
			params: map[string]any{"target": "t", "verbose": false},
			want:   []string{"-silent", "-u", "t"},
			wantOK: true,
		},
	}
	for _, tc := range cases {
		got, err := buildRecipeArgv(spec, tc.params, tc.proxy)
		if !tc.wantOK {
			if err == nil {
				t.Fatalf("%s: expected error, got %v", tc.name, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}

	onlyOptional := recipeSpec(t, `{"argv":[["-u","{target}"]]}`)
	if _, err := buildRecipeArgv(onlyOptional, map[string]any{"ports": "80"}, ""); err == nil {
		t.Fatal("all groups dropped should be an error")
	}
}

func TestRenderRecipeItemProxyAndInjection(t *testing.T) {
	spec := recipeSpec(t, `{"argv":[["--url={url}"]]}`)
	injected := "https://example.com/a?x=1&y=$(id); rm -rf /"
	got, err := buildRecipeArgv(spec, map[string]any{"url": injected}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "--url="+injected {
		t.Fatalf("value split or rewritten: %q", got)
	}
	if _, err := buildRecipeArgv(spec, map[string]any{"url": ""}, ""); err == nil {
		t.Fatal("blank url should drop the group")
	}
	proxySpec := recipeSpec(t, `{"argv":[["-proxy","{proxy}"],["ok"]]}`)
	if got, _ := buildRecipeArgv(proxySpec, nil, ""); !reflect.DeepEqual(got, []string{"ok"}) {
		t.Fatalf("blank proxy should drop its group: %q", got)
	}
	if got, _ := buildRecipeArgv(proxySpec, nil, "socks5://p:1"); !reflect.DeepEqual(got, []string{"-proxy", "socks5://p:1", "ok"}) {
		t.Fatalf("proxy not rendered: %q", got)
	}
}

func TestRecipeEnvModes(t *testing.T) {
	for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "all_proxy", "http_proxy", "https_proxy", "SSL_CERT_FILE"} {
		t.Setenv(k, "")
	}
	t.Setenv("HTTP_PROXY", "http://user:pass@127.0.0.1:9999")
	t.Setenv("SSL_CERT_FILE", "/tmp/ca.pem")

	none := recipeEnv([]string{"ALL_PROXY=http://session:1"}, "none", "")
	for _, k := range []string{"HTTP_PROXY", "ALL_PROXY", "SSL_CERT_FILE"} {
		if v := envValue(none, k); v != "" {
			t.Fatalf("mode none leaked %s=%q", k, v)
		}
	}

	inherit := recipeEnv(nil, "inherit", "")
	if got := envValue(inherit, "HTTP_PROXY"); got != "http://user:pass@127.0.0.1:9999" {
		t.Fatalf("inherit dropped proxy: %q", got)
	}

	socks := recipeEnv(nil, "socks5", "socks5://127.0.0.1:1080")
	if got := envValue(socks, "ALL_PROXY"); got != "socks5://127.0.0.1:1080" {
		t.Fatalf("socks5 ALL_PROXY = %q", got)
	}

	if got := detectProxy([]string{"HTTPS_PROXY=http://a:1", "HTTP_PROXY=http://b:2"}); got != "http://a:1" {
		t.Fatalf("detectProxy = %q", got)
	}
	if got := detectProxy([]string{"HTTP_PROXY=http://a:1", "HTTP_PROXY=http://b:2"}); got != "http://b:2" {
		t.Fatalf("last assignment should win, got %q", got)
	}
}

func newRecipeServer(t *testing.T) *Server {
	t.Helper()
	return &Server{m: &Manager{dir: t.TempDir()}}
}

func runRecipe(t *testing.T, raw string, params map[string]any, meta db.ToolMeta) actool.Result {
	t.Helper()
	res, err := newRecipeServer(t).runRecipeTool(context.Background(), json.RawMessage(raw), params, &actool.ToolContext{}, meta)
	if err != nil {
		t.Fatalf("runRecipeTool error: %v", err)
	}
	return res
}

func TestRunRecipeToolExitCodes(t *testing.T) {
	res := runRecipe(t, `{"argv":[["-c","exit 3"]]}`, nil, db.ToolMeta{Binary: "sh"})
	if !res.IsError || !strings.Contains(res.Flatten(), "[exit: 3]") {
		t.Fatalf("undeclared exit code not reported: %q (error=%v)", res.Flatten(), res.IsError)
	}

	res = runRecipe(t, `{"argv":[["-c","exit 3"]],"allowed_exit_codes":[3]}`, nil, db.ToolMeta{Binary: "sh"})
	if res.IsError || !strings.Contains(res.Flatten(), "[exit: 3 (allowed)]") {
		t.Fatalf("allowed exit code flagged as error: %q (error=%v)", res.Flatten(), res.IsError)
	}

	res = runRecipe(t, `{"argv":[["-c","printf ok"]]}`, nil, db.ToolMeta{Binary: "sh"})
	if res.IsError || res.Flatten() != "ok" {
		t.Fatalf("success output: %q (error=%v)", res.Flatten(), res.IsError)
	}
}

func TestRunRecipeToolMissingBinary(t *testing.T) {
	res := runRecipe(t, `{"argv":[["--version"]]}`, nil, db.ToolMeta{Binary: "artex-no-such-binary-xyz"})
	if !res.IsError || !strings.Contains(res.Flatten(), "未安装") {
		t.Fatalf("missing binary: %q (error=%v)", res.Flatten(), res.IsError)
	}
}

func TestRunRecipeToolTimeout(t *testing.T) {
	res := runRecipe(t, `{"argv":[["-c","sleep 5"]],"timeout_ms":100}`, nil, db.ToolMeta{Binary: "sh"})
	if !res.IsError || !strings.Contains(res.Flatten(), "超时终止") {
		t.Fatalf("timeout not reported: %q (error=%v)", res.Flatten(), res.IsError)
	}
}

func TestRunRecipeToolStdinAndBinaryFromArgv(t *testing.T) {
	res := runRecipe(t, `{"argv":[["cat"]],"stdin":"{text}"}`, map[string]any{"text": "piped-in 数据"}, db.ToolMeta{})
	if res.IsError || res.Flatten() != "piped-in 数据" {
		t.Fatalf("stdin not fed / argv[0] not used as binary: %q (error=%v)", res.Flatten(), res.IsError)
	}
}

func TestRunRecipeToolNoShellExpansion(t *testing.T) {
	dir := t.TempDir()
	s := &Server{m: &Manager{dir: dir}}
	got, err := s.runRecipeTool(context.Background(),
		json.RawMessage(`{"argv":[["{value}"]]}`),
		map[string]any{"value": "x; touch pwned"}, &actool.ToolContext{}, db.ToolMeta{Binary: "echo"})
	if err != nil || got.IsError {
		t.Fatalf("echo failed: %v %q", err, got.Flatten())
	}
	if strings.TrimSpace(got.Flatten()) != "x; touch pwned" {
		t.Fatalf("value split/reinterpreted: %q", got.Flatten())
	}
	if _, statErr := os.Stat(dir + "/pwned"); statErr == nil {
		t.Fatal("value was executed by a shell")
	}
}

func TestRunRecipeToolProxyModeNone(t *testing.T) {
	for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "all_proxy", "http_proxy", "https_proxy"} {
		t.Setenv(k, "")
	}
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:9999")
	res := runRecipe(t, `{"argv":[["-c","printf '%s' \"$HTTP_PROXY\""]]}`, nil, db.ToolMeta{Binary: "sh", ProxyMode: "none"})
	if res.IsError || res.Flatten() != "(无输出)" {
		t.Fatalf("proxy_mode=none kept HTTP_PROXY: %q (error=%v)", res.Flatten(), res.IsError)
	}

	res = runRecipe(t, `{"argv":[["-c","printf '%s' \"$HTTP_PROXY\""]]}`, nil, db.ToolMeta{Binary: "sh", ProxyMode: "inherit"})
	if res.IsError || res.Flatten() != "http://127.0.0.1:9999" {
		t.Fatalf("inherit should pass HTTP_PROXY: %q (error=%v)", res.Flatten(), res.IsError)
	}
}

func TestRunRecipeToolRejectsParameterizedExecutable(t *testing.T) {
	res := runRecipe(t, `{"argv":[["{target}"],["-c","id"]]}`, map[string]any{"target": "sh"}, db.ToolMeta{})
	if !res.IsError || !strings.Contains(res.Flatten(), "argv 首项") {
		t.Fatalf("parameterized executable not rejected: %q (error=%v)", res.Flatten(), res.IsError)
	}
	// With an explicit binary the same shape stays an ordinary argument list.
	res = runRecipe(t, `{"argv":[["%s","{target}"]]}`, map[string]any{"target": "ok"}, db.ToolMeta{Binary: "printf"})
	if res.IsError || strings.TrimSpace(res.Flatten()) != "ok" {
		t.Fatalf("explicit binary should still work: %q (error=%v)", res.Flatten(), res.IsError)
	}
}

func TestValidateRecipeExecutableBinding(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"target":{"type":"string"}}}`)
	bad := json.RawMessage(`{"argv":[["{target}"]]}`)
	if err := validateToolExec("recipe", schema, bad, ""); err == nil {
		t.Fatal("expected rejection when binary is empty and argv[0] is a placeholder")
	}
	if err := validateToolExec("recipe", schema, bad, "httpx"); err != nil {
		t.Fatalf("explicit binary should pass: %v", err)
	}
	static := json.RawMessage(`{"argv":[["cat"]]}`)
	if err := validateToolExec("recipe", schema, static, ""); err != nil {
		t.Fatalf("static argv[0] should pass: %v", err)
	}
}

func TestCappedOutputBounds(t *testing.T) {
	c := newCappedOutput(1024, 256)
	chunk := bytes.Repeat([]byte("a"), 64<<10)
	for i := 0; i < 8; i++ {
		if _, err := c.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	got := c.String()
	if len(got) > 1024+256+128 {
		t.Fatalf("output not bounded: %d bytes", len(got))
	}
	if !strings.Contains(got, "中间省略") {
		t.Fatalf("missing truncation marker")
	}
}

// The editor dry-run must honor the form's binary field; without it the executor
// falls back to argv[0] and a flag like "-sT" is mistaken for the binary.
func TestPgTestCustomToolRecipeUsesBinary(t *testing.T) {
	pg := openToolboxDB(t)
	s := &Server{m: &Manager{pg: pg, dir: t.TempDir()}}
	body := `{"kind":"recipe","binary":"sh","proxy_mode":"none","params":{},
		"exec":{"argv":[["-c","printf recipe-ok"]]}}`
	rec := httptest.NewRecorder()
	s.pgTestCustomTool(rec, httptest.NewRequest(http.MethodPost, "/api/tools/custom/test", strings.NewReader(body)))
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	out := decodeBody[struct {
		Output  string `json:"output"`
		IsError bool   `json:"is_error"`
	}](t, rec)
	if out.IsError || !strings.Contains(out.Output, "recipe-ok") {
		t.Fatalf("test run ignored binary meta: %+v", out)
	}
}
