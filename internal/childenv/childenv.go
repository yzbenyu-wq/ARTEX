// Package childenv builds the environment handed to agent-spawned processes.
// The server itself keeps its secrets in os.Environ(), but shells and recipe
// tools must not inherit them: an agent can print or exfiltrate anything it
// sees, and that output lands in transcripts.
package childenv

import (
	"os"
	"strings"
)

// secretKeys are never passed to child processes. ARTEX_PG_DSN carries the
// database password; the LLM keys are fallbacks for provider configuration.
var secretKeys = map[string]bool{
	"ARTEX_PG_DSN":     true,
	"POSTGRES_PASSWORD": true,
	"ANTHROPIC_API_KEY": true,
	"OPENAI_API_KEY":    true,
}

// Sanitized returns os.Environ() with secret entries removed. The result is
// always non-empty: SDK tools interpret an empty env as "inherit everything".
func Sanitized() []string {
	src := os.Environ()
	out := make([]string, 0, len(src))
	for _, kv := range src {
		if eq := strings.IndexByte(kv, '='); eq > 0 && secretKeys[kv[:eq]] {
			continue
		}
		out = append(out, kv)
	}
	if len(out) == 0 {
		out = append(out, "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
	}
	return out
}

// With returns Sanitized() plus extra entries (session proxy variables and the
// like), which are appended so they win over inherited duplicates.
func With(extra ...string) []string {
	out := Sanitized()
	return append(out, extra...)
}
