package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Autumn-27/artex/db"
	actool "github.com/Autumn-27/norma/tool"
)

// recipe-kind tools: a declarative argv recipe, no shell. Each argv entry is a
// group; a group is included only when every {param} in it has a value, so
// optional flags (["-p","{ports}"]) vanish when the caller leaves them out.
// A group item that is exactly "{name}" expands an array value into repeated
// argv items (["-u","{targets}"] with targets=["a","b"] → -u a -u b).
//
// proxy_mode gates proxy handling (values seeded by the toolbox UI):
//   inherit/http/env — pass the caller's proxy env through (agents already get
//     ProxyEnv from the harness); {proxy} = effective proxy URL.
//   socks5 — same env, plus ALL_PROXY forced to the effective URL for tools
//     that only read ALL_PROXY.
//   none — strip every proxy/CA env var and blank {proxy} (raw-socket tools).
//   raw — leave env untouched; {proxy} = effective URL (recipe decides itself).
//
// Placeholder values are never shell-quoted: exec.Command receives argv
// directly, so "10.0.0.1; rm -rf /" stays one inert argument.

var reRecipeSpan = regexp.MustCompile(`\{([A-Za-z0-9_]+)\}`)
var reRecipeExact = regexp.MustCompile(`^\{([A-Za-z0-9_]+)\}$`)

const (
	recipeHeadBytes = 4 << 20
	recipeTailBytes = 256 << 10
	recipeWaitDelay = 5 * time.Second
)

// cappedOutput keeps the head and tail of a child's combined output so a runaway
// scanner cannot exhaust memory; the middle is replaced with a marker.
type cappedOutput struct {
	mu      sync.Mutex
	head    []byte
	tail    []byte
	headMax int
	tailMax int
	dropped int64
}

func newCappedOutput(headMax, tailMax int) *cappedOutput {
	return &cappedOutput{headMax: headMax, tailMax: tailMax}
}

func (c *cappedOutput) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := len(p)
	if room := c.headMax - len(c.head); room > 0 {
		take := room
		if take > len(p) {
			take = len(p)
		}
		c.head = append(c.head, p[:take]...)
		p = p[take:]
	}
	if len(p) == 0 || c.tailMax <= 0 {
		c.dropped += int64(len(p))
		return n, nil
	}
	c.tail = append(c.tail, p...)
	if over := len(c.tail) - c.tailMax; over > 0 {
		c.tail = c.tail[over:]
		c.dropped += int64(over)
	}
	return n, nil
}

func (c *cappedOutput) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.dropped == 0 {
		return string(c.head) + string(c.tail)
	}
	return string(c.head) + fmt.Sprintf("\n... [输出过大，中间省略 %d 字节] ...\n", c.dropped) + string(c.tail)
}

// staticFirstArg reports whether the first non-empty argv group starts with a
// literal executable name. When ToolMeta.Binary is empty the executor falls back
// to argv[0], so a placeholder there would let the caller choose the binary.
func staticFirstArg(groups []recipeGroup) bool {
	for _, g := range groups {
		if len(g) == 0 {
			continue
		}
		return !reRecipeSpan.MatchString(g[0])
	}
	return false
}

// recipeProxyEnvKeys are removed from the child env for proxy_mode=none, so a
// raw-socket tool never accidentally tunnels through HTTP_PROXY inherited from
// the container or the BashEnv.
var recipeProxyEnvKeys = map[string]bool{
	"HTTP_PROXY": true, "HTTPS_PROXY": true, "ALL_PROXY": true, "NO_PROXY": true,
	"http_proxy": true, "https_proxy": true, "all_proxy": true, "no_proxy": true,
	"NODE_USE_ENV_PROXY": true,
	"SSL_CERT_FILE":      true, "CURL_CA_BUNDLE": true, "REQUESTS_CA_BUNDLE": true,
	"GIT_SSL_CAINFO": true, "NODE_EXTRA_CA_CERTS": true,
}

type recipeGroup []string

func (g *recipeGroup) UnmarshalJSON(b []byte) error {
	var single string
	if err := json.Unmarshal(b, &single); err == nil {
		*g = []string{single}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return errors.New("argv 每一项需为字符串或字符串数组")
	}
	*g = many
	return nil
}

type recipeExec struct {
	Argv             []recipeGroup `json:"argv"`
	Stdin            string        `json:"stdin"`
	TimeoutMs        int           `json:"timeout_ms"`
	AllowedExitCodes []int         `json:"allowed_exit_codes"`
}

func parseRecipeExec(raw json.RawMessage) (recipeExec, error) {
	var spec recipeExec
	if len(raw) == 0 {
		return spec, errors.New("recipe exec 为空")
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		return spec, fmt.Errorf("recipe exec 无效: %v", err)
	}
	if len(spec.Argv) == 0 {
		return spec, errors.New("recipe argv 不能为空")
	}
	return spec, nil
}

// buildRecipeArgv renders argv groups, dropping any group that references a
// missing/empty parameter. The returned slice excludes the binary when the
// caller resolves it from ToolMeta.Binary separately.
func buildRecipeArgv(spec recipeExec, params map[string]any, proxy string) ([]string, error) {
	var argv []string
	for _, group := range spec.Argv {
		args, ok := renderRecipeGroup(group, params, proxy)
		if !ok {
			continue
		}
		argv = append(argv, args...)
	}
	if len(argv) == 0 {
		return nil, errors.New("recipe 未生成任何参数(检查必填参数)")
	}
	return argv, nil
}

func renderRecipeGroup(group recipeGroup, params map[string]any, proxy string) ([]string, bool) {
	// An exact placeholder holding an array repeats the whole group per element:
	// ["-u","{targets}"] + targets=[a,b] → -u a -u b. A blank element drops its
	// copy (a flag must not be emitted without its value).
	if name, arr, ok := findArrayParam(group, params); ok {
		if len(arr) == 0 {
			return nil, false
		}
		var out []string
		for _, e := range arr {
			sub := make(map[string]any, len(params))
			for k, v := range params {
				sub[k] = v
			}
			sub[name] = e
			rendered, ok := renderRecipeGroup(group, sub, proxy)
			if ok {
				out = append(out, rendered...)
			}
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	}
	out := make([]string, 0, len(group))
	for _, item := range group {
		// Exact bool placeholder acts as a switch: true keeps the group's static
		// flag (the placeholder itself adds nothing), false drops the group.
		if m := reRecipeExact.FindStringSubmatch(item); m != nil {
			if b, ok := params[m[1]].(bool); ok {
				if !b {
					return nil, false
				}
				continue
			}
		}
		rendered, ok := renderRecipeItem(item, params, proxy)
		if !ok {
			return nil, false
		}
		out = append(out, rendered)
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// findArrayParam returns the first exact {name} whose value is an array.
func findArrayParam(group recipeGroup, params map[string]any) (string, []any, bool) {
	for _, item := range group {
		m := reRecipeExact.FindStringSubmatch(item)
		if m == nil {
			continue
		}
		if arr, ok := params[m[1]].([]any); ok {
			return m[1], arr, true
		}
	}
	return "", nil, false
}

// renderRecipeItem replaces every {name}; a referenced parameter that is
// absent, blank, or false drops the whole group (bool flags behave as switches).
func renderRecipeItem(item string, params map[string]any, proxy string) (string, bool) {
	names := reRecipeSpan.FindAllStringSubmatch(item, -1)
	if len(names) == 0 {
		return item, true
	}
	out := item
	for _, m := range names {
		name := m[1]
		if name == "proxy" {
			if proxy == "" {
				return "", false
			}
			out = strings.ReplaceAll(out, m[0], proxy)
			continue
		}
		v, ok := params[name]
		if !ok {
			return "", false
		}
		switch x := v.(type) {
		case nil:
			return "", false
		case string:
			if strings.TrimSpace(x) == "" {
				return "", false
			}
		case bool:
			if !x {
				return "", false
			}
		}
		if sv, ok := scalarStr(v); ok {
			out = strings.ReplaceAll(out, m[0], sv)
			continue
		}
		out = strings.ReplaceAll(out, m[0], valToStr(v))
	}
	return out, true
}

// envValue returns the last assignment of key (session env is appended after
// os.Environ, so the last one is what the child would have seen).
func envValue(env []string, key string) string {
	prefix := key + "="
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(env[i], prefix) {
			return strings.TrimPrefix(env[i], prefix)
		}
	}
	return ""
}

func detectProxy(env []string) string {
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		if v := envValue(env, key); v != "" {
			return v
		}
	}
	return ""
}

// recipeEnv builds the child environment: os.Environ + the caller's BashEnv,
// then applies proxy_mode. For none every proxy/CA var is removed from both.
func recipeEnv(sessionEnv []string, mode, proxy string) []string {
	env := append(os.Environ(), sessionEnv...)
	switch mode {
	case "none":
		filtered := env[:0]
		for _, kv := range env {
			if eq := strings.IndexByte(kv, '='); eq > 0 && recipeProxyEnvKeys[kv[:eq]] {
				continue
			}
			filtered = append(filtered, kv)
		}
		return filtered
	case "socks5":
		if proxy != "" {
			env = append(env, "ALL_PROXY="+proxy, "all_proxy="+proxy)
		}
	}
	return env
}

func (s *Server) runRecipeTool(ctx context.Context, execRaw json.RawMessage, params map[string]any, tc *actool.ToolContext, meta db.ToolMeta) (actool.Result, error) {
	spec, err := parseRecipeExec(execRaw)
	if err != nil {
		return actool.Errorf(err.Error()), nil
	}
	if params == nil {
		params = map[string]any{}
	}
	workDir := s.m.dir
	var sessionEnv []string
	if tc != nil {
		if tc.WorkingDir != "" {
			workDir = tc.WorkingDir
		}
		sessionEnv = tc.Env
	}
	mode, ok := normalizeProxyMode(meta.ProxyMode)
	if !ok {
		mode = "inherit"
	}
	proxy := detectProxy(append(os.Environ(), sessionEnv...))
	if mode == "none" {
		proxy = ""
	}
	argv, err := buildRecipeArgv(spec, params, proxy)
	if err != nil {
		return actool.Errorf(err.Error()), nil
	}
	bin := strings.TrimSpace(meta.Binary)
	if bin == "" && !staticFirstArg(spec.Argv) {
		return actool.Errorf("recipe 未配置 binary：argv 首项必须是固定可执行名，不能含 {param}"), nil
	}
	if bin == "" {
		bin = argv[0]
		argv = argv[1:]
	}
	binPath, err := exec.LookPath(bin)
	if err != nil {
		return actool.Errorf("未安装 " + bin + "（先安装二进制，再用工具箱「检测版本」确认）"), nil
	}
	timeout := timeoutOr(spec.TimeoutMs, 120000)
	if timeout > time.Hour {
		timeout = time.Hour
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, binPath, argv...)
	cmd.Dir = workDir
	cmd.Env = recipeEnv(sessionEnv, mode, proxy)
	cmd.WaitDelay = recipeWaitDelay
	if strings.TrimSpace(spec.Stdin) != "" {
		if stdin, ok := renderRecipeItem(strings.TrimSpace(spec.Stdin), params, proxy); ok && stdin != "" {
			cmd.Stdin = strings.NewReader(stdin)
		}
	}
	out := newCappedOutput(recipeHeadBytes, recipeTailBytes)
	cmd.Stdout, cmd.Stderr = out, out
	runErr := cmd.Run()
	body := out.String()
	if runCtx.Err() == context.DeadlineExceeded {
		body += "\n... [超时终止] ..."
		return actool.Errorf(actool.Capture(tc, body)), nil
	}
	if runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			return actool.Errorf(maskSecrets(runErr.Error())), nil
		}
		code := exitErr.ExitCode()
		allowed := false
		for _, c := range spec.AllowedExitCodes {
			if c == code {
				allowed = true
				break
			}
		}
		if !allowed {
			body += fmt.Sprintf("\n[exit: %d]", code)
			return actool.Errorf(actool.Capture(tc, body)), nil
		}
		body += fmt.Sprintf("\n[exit: %d (allowed)]", code)
	}
	if strings.TrimSpace(body) == "" {
		body = "(无输出)"
	}
	return actool.Text(actool.Capture(tc, body)), nil
}
