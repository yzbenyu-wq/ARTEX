package agent

import (
	"strings"
	"testing"
)

func seedAgents(t *testing.T, key string) []string {
	t.Helper()
	for _, s := range BuiltinToolSeeds() {
		if s.Key == key {
			return s.Agents
		}
	}
	t.Fatalf("seed %q not found", key)
	return nil
}

func hasAgent(agents []string, want string) bool {
	for _, a := range agents {
		if a == want {
			return true
		}
	}
	return false
}

// report_finding / list_findings need the task exploration graph, which a chat
// turn never has. Chat-page agents must not be seeded with them; task agents must.
func TestChatAgentSeedsExcludeTaskOnlyTools(t *testing.T) {
	for _, key := range []string{"report_finding", "list_findings"} {
		agents := seedAgents(t, key)
		for _, chatAgent := range []string{"pentest", "auto"} {
			if hasAgent(agents, chatAgent) {
				t.Errorf("%s seeded to chat agent %q — it can only fail without a task graph", key, chatAgent)
			}
		}
	}

	rf := seedAgents(t, "report_finding")
	for _, taskAgent := range []string{"mainagent", "planner", "worker"} {
		if !hasAgent(rf, taskAgent) {
			t.Errorf("report_finding lost task agent %q: %v", taskAgent, rf)
		}
	}
}

// Chat-page agents keep their asset tools and the pentest prompt's handoff path
// lives on list_tasks / add_task_hint, bound by the orchestration seeds.
func TestChatAgentSeedsKeepAssetTools(t *testing.T) {
	for _, agentKey := range []string{"pentest", "auto"} {
		for _, tool := range []string{"list_assets", "insert_assets", "list_companies"} {
			if !hasAgent(seedAgents(t, tool), agentKey) {
				t.Errorf("%s lost %q", agentKey, tool)
			}
		}
	}
}

// The pentest prompt must route findings through add_task_hint and must not tell
// the model to call the task-only tools; the auto prompt must name the real skill
// editing tool.
func TestChatAgentPromptSeeds(t *testing.T) {
	seeds := BuiltinPromptSeeds()

	pentest := seeds["pentest"]
	for _, want := range []string{"add_task_hint", "list_tasks", "traffic_search", "traffic_get", "get_finding_traffic"} {
		if !strings.Contains(pentest, want) {
			t.Errorf("pentest prompt lacks %q", want)
		}
	}
	if strings.Contains(pentest, "回看已报漏洞用 list_findings") || strings.Contains(pentest, "→ report_finding") {
		t.Error("pentest prompt still instructs calling task-only tools")
	}
	if n := strings.Count(pentest, "report_finding"); n != 1 {
		t.Errorf("pentest prompt mentions report_finding %d times, want only the unavailable note", n)
	}

	auto := seeds["auto"]
	if !strings.Contains(auto, "update_skill_file") {
		t.Error("auto prompt should name update_skill_file")
	}
	if strings.Contains(auto, "create_skill / update_skill 建改技能") {
		t.Error("auto prompt still names nonexistent update_skill")
	}
}
