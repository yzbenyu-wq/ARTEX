package childenv

import (
	"os"
	"strings"
	"testing"
)

func TestSanitizedStripsSecrets(t *testing.T) {
	t.Setenv("ARTEX_PG_DSN", "postgres://user:pw@db/artex")
	t.Setenv("POSTGRES_PASSWORD", "pg-secret")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-secret")
	t.Setenv("OPENAI_API_KEY", "sk-openai-secret")
	t.Setenv("ARTEX_HTTP_USER_AGENT", "Agent/1.0")
	t.Setenv("PATH", "/usr/bin:/bin")

	env := Sanitized()
	joined := strings.Join(env, "\n")
	for _, secret := range []string{"postgres://user:pw@db/artex", "pg-secret", "sk-ant-secret", "sk-openai-secret"} {
		if strings.Contains(joined, secret) {
			t.Errorf("secret %q leaked into child env", secret)
		}
	}
	if !strings.Contains(joined, "ARTEX_HTTP_USER_AGENT=Agent/1.0") {
		t.Error("non-secret variable was dropped")
	}
	if !strings.Contains(joined, "PATH=/usr/bin:/bin") {
		t.Error("PATH missing")
	}
}

func TestWithAppendsExtras(t *testing.T) {
	t.Setenv("PATH", "/usr/bin")
	env := With("HTTP_PROXY=http://127.0.0.1:8788", "NODE_USE_ENV_PROXY=1")
	joined := strings.Join(env, "\n")
	for _, want := range []string{"HTTP_PROXY=http://127.0.0.1:8788", "NODE_USE_ENV_PROXY=1", "PATH=/usr/bin"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in child env", want)
		}
	}
}

func TestSanitizedNeverEmpty(t *testing.T) {
	// Clear every variable: the fallback PATH keeps the slice non-empty so
	// consumers do not fall back to "inherit os.Environ" semantics.
	for _, kv := range os.Environ() {
		if eq := strings.IndexByte(kv, '='); eq > 0 {
			t.Setenv(kv[:eq], "")
		}
	}
	if got := Sanitized(); len(got) == 0 {
		t.Fatal("Sanitized() returned an empty slice")
	}
}
