package agent

import (
	"strings"
	"testing"
)

func TestProxyEnvEmptyIsSanitized(t *testing.T) {
	// Without a proxy the child still gets an explicit environment so the SDK
	// does not inherit os.Environ() (which holds the DSN and LLM keys).
	env := proxyEnv("", "")
	if len(env) == 0 {
		t.Fatal("proxyEnv(\"\", \"\") is empty; SDK would inherit os.Environ()")
	}
	var hasPath bool
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			hasPath = true
		}
		if strings.HasPrefix(e, "ARTEX_PG_DSN=") || strings.HasPrefix(e, "ANTHROPIC_API_KEY=") || strings.HasPrefix(e, "OPENAI_API_KEY=") || strings.HasPrefix(e, "POSTGRES_PASSWORD=") {
			t.Fatalf("secret inherited by child env: %s", e)
		}
	}
	if !hasPath {
		t.Errorf("sanitized env missing PATH: %v", env)
	}
}

func TestProxyEnvSetsAllProxyForSocks5(t *testing.T) {
	// Capture-off egress path: a socks5 proxy, no MITM CA. ALL_PROXY must be set
	// (curl reads socks5 only from there), and no CA vars should appear.
	env := proxyEnv("socks5://10.0.0.1:1080", "")
	has := func(prefix string) bool {
		for _, e := range env {
			if strings.HasPrefix(e, prefix) {
				return true
			}
		}
		return false
	}
	for _, want := range []string{"HTTP_PROXY=", "HTTPS_PROXY=", "ALL_PROXY=", "all_proxy="} {
		if !has(want) {
			t.Errorf("proxyEnv missing %s: %v", want, env)
		}
	}
	if has("SSL_CERT_FILE=") || has("CURL_CA_BUNDLE=") {
		t.Errorf("proxyEnv without CA must not inject CA vars: %v", env)
	}
}

func TestProxyEnvInjectsCAWhenRecording(t *testing.T) {
	env := proxyEnv("http://127.0.0.1:8788", "/data/ca.pem")
	has := func(prefix string) bool {
		for _, e := range env {
			if strings.HasPrefix(e, prefix) {
				return true
			}
		}
		return false
	}
	for _, want := range []string{"SSL_CERT_FILE=", "CURL_CA_BUNDLE=", "REQUESTS_CA_BUNDLE=", "NODE_EXTRA_CA_CERTS="} {
		if !has(want) {
			t.Errorf("proxyEnv with CA missing %s: %v", want, env)
		}
	}
}
