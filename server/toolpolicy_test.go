package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Autumn-27/artex/db"
	"github.com/Autumn-27/artex/guard"
	"github.com/Autumn-27/artex/intercept"
	actool "github.com/Autumn-27/norma/tool"
)

func policySuffix() string { return fmt.Sprintf("%d", time.Now().UnixNano()) }

func TestCustomToolPolicySnapshotAndRefresh(t *testing.T) {
	pg := openToolboxDB(t)
	sfx := policySuffix()
	cmdKey := "zz_pol_cmd_" + sfx
	httpKey := "zz_pol_http_" + sfx
	recipeKey := "zz_pol_recipe_" + sfx
	offKey := "zz_pol_off_" + sfx
	shellKey := "zz_pol_shell_" + sfx
	keys := []string{cmdKey, httpKey, recipeKey, offKey, shellKey}
	cleanup := func() {
		for _, k := range keys {
			_ = pg.DeleteCustomTool(k)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	rows := []*db.Tool{
		{Key: cmdKey, Kind: "command", Enabled: true,
			Exec: json.RawMessage(`{"command":"nmap -p {ports} {target}"}`)},
		{Key: httpKey, Kind: "http", Enabled: true,
			Exec: json.RawMessage(`{"method":"post","url":"https://{target}/api"}`)},
		{Key: recipeKey, Kind: "recipe", Enabled: true,
			Exec:     json.RawMessage(`{"argv":[["-u","{target}"],["--proxy","{proxy}"]]}`),
			ToolMeta: db.ToolMeta{Binary: "httpx"}},
		{Key: offKey, Kind: "command", Enabled: false,
			Exec: json.RawMessage(`{"command":"echo {args}"}`)},
		{Key: shellKey, Kind: "shell", Enabled: true},
	}
	for _, r := range rows {
		if err := pg.CreateCustomTool(r); err != nil {
			t.Fatalf("seed %s: %v", r.Key, err)
		}
	}

	p := newCustomToolPolicy(pg)
	p.refresh()

	for _, k := range []string{cmdKey, httpKey, recipeKey} {
		if !p.enabled(k) {
			t.Errorf("%s should be gated", k)
		}
	}
	if p.enabled(offKey) {
		t.Error("disabled tool must not be gated")
	}
	if p.enabled(shellKey) {
		t.Error("shell hint rows are not callable and must not be gated")
	}
	if p.enabled("zz_pol_missing_" + sfx) {
		t.Error("unknown tool must not be gated")
	}

	cmdSurface := p.surface(cmdKey, json.RawMessage(`{"target":"10.0.0.5","ports":"80,443"}`))
	if want := "nmap -p '80,443' '10.0.0.5'"; cmdSurface != want {
		t.Errorf("command surface = %q, want %q", cmdSurface, want)
	}
	httpSurface := p.surface(httpKey, json.RawMessage(`{"target":"10.0.0.5"}`))
	if want := "POST https://10.0.0.5/api"; httpSurface != want {
		t.Errorf("http surface = %q, want %q", httpSurface, want)
	}
	recipeSurface := p.surface(recipeKey, json.RawMessage(`{"target":"10.0.0.5"}`))
	if want := "httpx -u 10.0.0.5"; recipeSurface != want {
		t.Errorf("recipe surface = %q, want %q", recipeSurface, want)
	}
	if strings.Contains(strings.ToLower(recipeSurface), "proxy") {
		t.Errorf("surface must not expose proxy config: %q", recipeSurface)
	}
	if got := p.surface(offKey, nil); got != "" {
		t.Errorf("disabled tool must have no surface, got %q", got)
	}

	if err := pg.DeleteCustomTool(cmdKey); err != nil {
		t.Fatal(err)
	}
	p.refresh()
	if p.enabled(cmdKey) || p.surface(cmdKey, nil) != "" {
		t.Error("refresh must drop deleted tools from the policy")
	}
}

func TestCommandToolEnforcesBashSafetyFloor(t *testing.T) {
	s := &Server{m: &Manager{}}
	tc := &actool.ToolContext{WorkingDir: t.TempDir()}
	// Matches norma's catastrophicBash mkfs rule; the device path does not exist,
	// so even a broken floor cannot touch a real device.
	execRaw := json.RawMessage(`{"command":"mkfs.ext4 /dev/artex-floor-probe"}`)
	res, err := s.runCommandTool(context.Background(), execRaw, map[string]any{}, tc)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("catastrophic command must be rejected, got %q", res.Flatten())
	}
	if !strings.Contains(res.Flatten(), "安全底线") {
		t.Fatalf("rejection must come from the safety floor, got %q", res.Flatten())
	}
}

// TestGuardCoversCustomToolsEndToEnd drives the real guard hook: a custom tool
// that is not in the static enabled-tools set must still reach rule matching,
// with its rendered command used for the match and the audit log.
func TestGuardCoversCustomToolsEndToEnd(t *testing.T) {
	pg := openToolboxDB(t)
	sfx := policySuffix()
	key := "zz_pol_guard_" + sfx
	token := "ARTEX_POLICY_PROBE_" + sfx
	_ = pg.DeleteCustomTool(key)
	t.Cleanup(func() { _ = pg.DeleteCustomTool(key) })

	if err := pg.CreateCustomTool(&db.Tool{
		Key: key, Kind: "command", Enabled: true,
		Exec: json.RawMessage(`{"command":"echo ` + token + ` {note}"}`),
	}); err != nil {
		t.Fatalf("seed tool: %v", err)
	}

	m := &Manager{pg: pg, interceptor: intercept.New(pg)}
	s := &Server{m: m}
	s.wireToolInterceptPolicy()

	rule, err := pg.CreateInterceptRule("zz_pol_rule_"+sfx, "tool_input", "string", token,
		"deny", "测试：命令面命中", 500, true, false, 0, "deny")
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}
	t.Cleanup(func() { _ = pg.DeleteInterceptRule(rule.ID) })

	g := guard.NewWithInterceptor(m.interceptor)
	// The token exists only in the rendered command (DB template), never in the
	// call's input parameters.
	blocked, msg, _ := g.Hooks().PreToolUse(context.Background(), key, json.RawMessage(`{"note":"x"}`))
	if !blocked {
		t.Fatalf("custom tool call must be blocked by the rule, msg=%q", msg)
	}
	if !strings.Contains(msg, "平台管控") {
		t.Fatalf("block message must be the platform-governance framing: %q", msg)
	}

	var audited string
	for _, e := range g.Audit() {
		if e.Tool == key {
			audited = e.Command
		}
	}
	if !strings.Contains(audited, token) {
		t.Fatalf("audit log must record the rendered command, got %q", audited)
	}

	// Once the tool is deleted, the same call must pass: the gate follows the
	// live snapshot, not a static list.
	if err := pg.DeleteCustomTool(key); err != nil {
		t.Fatal(err)
	}
	s.refreshToolPolicy()
	blocked, _, _ = g.Hooks().PreToolUse(context.Background(), key, json.RawMessage(`{"note":"x"}`))
	if blocked {
		t.Fatal("deleted tool must no longer enter intercept evaluation")
	}
}
