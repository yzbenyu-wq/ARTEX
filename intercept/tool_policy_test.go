package intercept

import (
	"encoding/json"
	"testing"
)

func TestToolPolicyWidensEnabledTools(t *testing.T) {
	ic := interceptorWith()
	ic.enabledTools = map[string]bool{"Bash": true}

	if ic.IsToolEnabled("artex_probe") {
		t.Fatal("tool not in the enabled set and no policy: must be disabled")
	}
	ic.SetToolPolicy(ToolPolicy{Enabled: func(name string) bool { return name == "artex_probe" }})
	if !ic.IsToolEnabled("artex_probe") {
		t.Fatal("policy-enabled tool must enter the intercept path")
	}
	if !ic.IsToolEnabled("Bash") {
		t.Fatal("saved enabled-tools set must still apply")
	}
	if ic.IsToolEnabled("other_tool") {
		t.Fatal("tool unknown to both set and policy must stay disabled")
	}
	ic.SetToolPolicy(ToolPolicy{})
	if ic.IsToolEnabled("artex_probe") {
		t.Fatal("clearing the policy must disable it again")
	}
}

func TestCommandSurfaceTrimsAndClears(t *testing.T) {
	ic := interceptorWith()
	if got := ic.CommandSurface("x", nil); got != "" {
		t.Fatalf("no policy: want empty surface, got %q", got)
	}
	ic.SetToolPolicy(ToolPolicy{Surface: func(string, json.RawMessage) string { return "  echo hi  " }})
	if got := ic.CommandSurface("x", nil); got != "echo hi" {
		t.Fatalf("surface not trimmed: %q", got)
	}
	ic.SetToolPolicy(ToolPolicy{})
	if got := ic.CommandSurface("x", nil); got != "" {
		t.Fatalf("cleared policy: want empty surface, got %q", got)
	}
}

func TestAugmentCommandInput(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		command string
	}{
		{"empty command keeps input", `{"args":"x"}`, ""},
		{"object gets command", `{"args":"x"}`, "echo hi"},
		{"existing command is replaced", `{"command":"old"}`, "new"},
		{"non-object input wrapped", `"raw"`, "echo hi"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := AugmentCommandInput(json.RawMessage(tc.input), tc.command)
			if len(out) == 0 {
				t.Fatalf("input was lost: %s", tc.input)
			}
			var got map[string]any
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatalf("result is not an object: %s", out)
			}
			if tc.command == "" {
				if _, has := got["command"]; has {
					t.Fatalf("empty surface must not add a command: %v", got)
				}
				return
			}
			if got["command"] != tc.command {
				t.Fatalf("command = %v, want %q", got["command"], tc.command)
			}
		})
	}
}

func TestMatchSeesEffectiveCommandForCustomTool(t *testing.T) {
	ic := interceptorWith(testRule("tool_input", "string", "rm -rf /var/www"))
	ic.enabledTools = map[string]bool{}
	ic.SetToolPolicy(ToolPolicy{
		Enabled: func(name string) bool { return name == "custom_probe" },
		Surface: func(name string, input json.RawMessage) string { return "cd /tmp && rm -rf /var/www" },
	})

	in := json.RawMessage(`{"path":"/var/www"}`)
	if _, ok := ic.Match("custom_probe", in); ok {
		t.Fatal("raw params alone must not match the command rule")
	}
	if !ic.IsToolEnabled("custom_probe") {
		t.Fatal("policy tool must be gated")
	}
	aug := AugmentCommandInput(in, ic.CommandSurface("custom_probe", in))
	dec, ok := ic.Match("custom_probe", aug)
	if !ok || dec.Action != "deny" {
		t.Fatalf("rendered command must reach rule matching: matched=%v dec=%+v", ok, dec)
	}
}
