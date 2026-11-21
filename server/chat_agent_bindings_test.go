package server

import (
	"encoding/json"
	"testing"

	"github.com/Autumn-27/artex/agent"
)

func agentPromptDefault(t *testing.T, key string) string {
	t.Helper()
	tmpl := agent.BuiltinPromptSeeds()[key]
	if tmpl == "" {
		t.Fatalf("no built-in prompt seed for %s", key)
	}
	return tmpl
}

// TestChatAgentToolBindingsMigration reproduces the legacy chat-agent bindings
// (report_finding / list_findings bound to pentest, auto bound to report_finding,
// no handoff/traffic readers) and asserts the one-shot migration converges them
// without touching task agents or user-customized tools.
func TestChatAgentToolBindingsMigration(t *testing.T) {
	s, _, _ := trafficEvidenceServer(t)
	pg := s.m.pg
	keys := []string{
		"report_finding", "list_findings", "add_task_hint", "list_tasks",
		"traffic_search", "traffic_get", "traffic_blob", "get_finding_traffic",
	}
	old := map[string]json.RawMessage{}
	for _, key := range keys {
		row, err := pg.GetTool(key)
		if err != nil || row == nil {
			t.Fatalf("missing tool %s: %v", key, err)
		}
		raw, _ := json.Marshal(row.Agents)
		old[key] = raw
	}
	flagBefore, _, _ := pg.GetSetting("chat_agent_tool_bindings_v1")
	t.Cleanup(func() {
		for _, key := range keys {
			row, err := pg.GetTool(key)
			if err != nil || row == nil {
				continue
			}
			_ = pg.UpdateTool(key, row.Description, row.Schema, old[key], row.Enabled)
		}
		_ = pg.SetSetting("chat_agent_tool_bindings_v1", flagBefore)
	})

	setAgents := func(key string, agents ...string) {
		t.Helper()
		row, err := pg.GetTool(key)
		if err != nil || row == nil {
			t.Fatalf("load %s: %v", key, err)
		}
		raw, _ := json.Marshal(agents)
		if err := pg.UpdateTool(key, row.Description, row.Schema, raw, row.Enabled); err != nil {
			t.Fatal(err)
		}
	}
	// Legacy shape: task-only tools leaked into the chat agents; a user tool on
	// report_finding must survive untouched.
	setAgents("report_finding", "mainagent", "planner", "worker", "auto", "pentest")
	setAgents("list_findings", "mainagent", "planner", "worker", "pentest")
	setAgents("add_task_hint", "auto")
	setAgents("list_tasks", "auto")
	setAgents("traffic_search", "worker", "reporter")
	setAgents("traffic_get", "worker", "reporter")
	setAgents("get_finding_traffic", "auto", "reporter", "worker", "planner", "mainagent")
	if err := pg.SetSetting("chat_agent_tool_bindings_v1", "false"); err != nil {
		t.Fatal(err)
	}

	s.seedChatAgentToolBindings()

	report, _ := pg.GetTool("report_finding")
	if contains(report.Agents, "pentest") || contains(report.Agents, "auto") {
		t.Fatalf("report_finding still bound to chat agents: %v", report.Agents)
	}
	for _, a := range []string{"mainagent", "planner", "worker"} {
		if !contains(report.Agents, a) {
			t.Fatalf("report_finding lost task agent %q: %v", a, report.Agents)
		}
	}
	list, _ := pg.GetTool("list_findings")
	if contains(list.Agents, "pentest") {
		t.Fatalf("list_findings still bound to pentest: %v", list.Agents)
	}
	for _, key := range []string{"add_task_hint", "list_tasks"} {
		row, _ := pg.GetTool(key)
		if !contains(row.Agents, "pentest") || !contains(row.Agents, "auto") {
			t.Fatalf("%s binding = %v, want auto+pentest", key, row.Agents)
		}
	}
	for _, key := range []string{"traffic_search", "traffic_get", "traffic_blob"} {
		row, _ := pg.GetTool(key)
		for _, a := range []string{"pentest", "worker", "reporter"} {
			if !contains(row.Agents, a) {
				t.Fatalf("%s binding = %v, want %s", key, row.Agents, a)
			}
		}
	}
	traffic, _ := pg.GetTool("get_finding_traffic")
	for _, a := range []string{"pentest", "retester"} {
		if !contains(traffic.Agents, a) {
			t.Fatalf("get_finding_traffic binding = %v, want %s", traffic.Agents, a)
		}
	}
	if v, _, _ := pg.GetSetting("chat_agent_tool_bindings_v1"); v != "true" {
		t.Fatalf("migration flag = %q after run, want true", v)
	}
}

// TestChatAgentPromptReseeds verifies the pentest/auto prompt migrations replace
// the legacy default text but never overwrite a user-edited prompt.
func TestChatAgentPromptReseeds(t *testing.T) {
	s, _, _ := trafficEvidenceServer(t)
	pg := s.m.pg

	cases := []struct {
		agentKey string
		flag     string
		legacy   string
		reseed   func()
	}{
		{"pentest", "pentest_prompt_chat_handoff_v1", pentestLegacyPrompt, s.reseedPentestChatPrompt},
		{"auto", "auto_prompt_tool_name_v1", autoLegacyPrompt, s.reseedAutoPromptToolName},
	}
	for _, c := range cases {
		a, err := pg.GetAgentByKey(c.agentKey)
		if err != nil || a == nil {
			t.Fatalf("agent %s missing: %v", c.agentKey, err)
		}
		current, err := pg.CurrentPrompt(a.ID)
		if err != nil {
			t.Fatal(err)
		}
		flagBefore, _, _ := pg.GetSetting(c.flag)
		t.Cleanup(func() {
			_, _ = pg.ResetPromptToDefault(a.ID, current)
			_ = pg.SetSetting(c.flag, flagBefore)
		})

		// Legacy default → replaced by the current code default.
		if _, err := pg.ResetPromptToDefault(a.ID, c.legacy); err != nil {
			t.Fatal(err)
		}
		_ = pg.SetSetting(c.flag, "false")
		c.reseed()
		got, _ := pg.CurrentPrompt(a.ID)
		if got != agentPromptDefault(t, c.agentKey) {
			t.Fatalf("%s: legacy prompt not migrated", c.agentKey)
		}

		// User edit → preserved, flag still settles.
		const custom = "USER CUSTOM PROMPT"
		if _, err := pg.ResetPromptToDefault(a.ID, custom); err != nil {
			t.Fatal(err)
		}
		_ = pg.SetSetting(c.flag, "false")
		c.reseed()
		if got, _ := pg.CurrentPrompt(a.ID); got != custom {
			t.Fatalf("%s: user prompt overwritten: %q", c.agentKey, got)
		}
		if v, _, _ := pg.GetSetting(c.flag); v != "true" {
			t.Fatalf("%s: flag = %q, want true", c.agentKey, v)
		}
	}
}

const (
	pentestLegacyPrompt = "确认漏洞 → report_finding（含可复现 PoC）。回看已报漏洞用 list_findings。"
	autoLegacyPrompt    = "2. **平台管理**：create_skill / update_skill 建改技能。"
)
