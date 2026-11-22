package server

import (
	"encoding/json"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Autumn-27/artex/db"
	"github.com/Autumn-27/artex/intercept"
)

// customToolPolicy exposes user-defined tools to the intercept layer. Those tools
// are created at runtime and are not in the static intercept-enabled set, so
// without this bridge the guard's rule match / LLM judge never saw them, and the
// audit log only had a tool name. The policy is an in-memory snapshot refreshed
// on every custom-tool CRUD/import/preset install; lookups never touch the DB.
type customToolPolicy struct {
	pg      *db.DB
	snap    atomic.Pointer[map[string]*db.Tool]
	lastTry atomic.Int64 // unix nanos of the last lazy load attempt
}

// toolPolicyRetryInterval bounds lazy reload attempts when the initial snapshot
// failed (e.g. DB not ready yet): one attempt per interval, not one per call.
const toolPolicyRetryInterval = 30 * time.Second

// surfaceCap bounds the command string handed to the audit log, rules and judge.
const surfaceCap = 4096

func newCustomToolPolicy(pg *db.DB) *customToolPolicy {
	return &customToolPolicy{pg: pg}
}

// refresh reloads the snapshot from the tools table.
func (p *customToolPolicy) refresh() {
	if p == nil || p.pg == nil {
		return
	}
	rows, err := p.pg.ListCustomTools()
	if err != nil {
		log.Printf("[intercept] custom tool policy reload: %v", err)
		return
	}
	m := make(map[string]*db.Tool, len(rows))
	for _, t := range rows {
		if t != nil {
			m[t.Key] = t
		}
	}
	p.snap.Store(&m)
}

// ensure retries a failed initial load at most once per toolPolicyRetryInterval.
func (p *customToolPolicy) ensure() {
	if p == nil || p.pg == nil || p.snap.Load() != nil {
		return
	}
	now := time.Now().UnixNano()
	last := p.lastTry.Load()
	if now-last < int64(toolPolicyRetryInterval) {
		return
	}
	if !p.lastTry.CompareAndSwap(last, now) {
		return
	}
	p.refresh()
}

func (p *customToolPolicy) lookup(name string) *db.Tool {
	if p == nil || p.pg == nil {
		return nil
	}
	p.ensure()
	m := p.snap.Load()
	if m == nil {
		return nil
	}
	return (*m)[name]
}

// enabled reports whether name is a callable custom tool. shell-kind rows are
// Bash description hints, not callable tools, so they are not gated here.
func (p *customToolPolicy) enabled(name string) bool {
	t := p.lookup(name)
	if t == nil || !t.Enabled {
		return false
	}
	switch t.Kind {
	case "command", "script", "http", "recipe":
		return true
	default:
		return false
	}
}

// surface renders the command/argv/request line a custom tool call will execute,
// so rules written against command content also cover custom tools and the audit
// log records what actually ran. Proxy credentials are never included: recipes
// render with an empty proxy (proxy-only flags drop out) and http tools expose
// only method + URL, never the configured proxy.
func (p *customToolPolicy) surface(name string, input json.RawMessage) string {
	t := p.lookup(name)
	if t == nil || !t.Enabled {
		return ""
	}
	var params map[string]any
	if len(input) > 0 {
		_ = json.Unmarshal(input, &params)
	}
	if params == nil {
		params = map[string]any{}
	}
	var out string
	switch t.Kind {
	case "command":
		var spec commandExec
		if json.Unmarshal(t.Exec, &spec) == nil {
			out = renderTemplate(spec.Command, params, shellQuote)
		}
	case "http":
		var spec httpExec
		if json.Unmarshal(t.Exec, &spec) == nil {
			method := strings.ToUpper(strings.TrimSpace(spec.Method))
			if method == "" {
				method = "GET"
			}
			out = method + " " + renderTemplate(spec.URL, params, identity)
		}
	case "recipe":
		spec, err := parseRecipeExec(t.Exec)
		if err != nil {
			break
		}
		argv, err := buildRecipeArgv(spec, params, "")
		if err != nil {
			break
		}
		if bin := strings.TrimSpace(t.Binary); bin != "" {
			argv = append([]string{bin}, argv...)
		}
		out = strings.Join(argv, " ")
	}
	out = strings.TrimSpace(out)
	if len(out) > surfaceCap {
		out = out[:surfaceCap] + "…"
	}
	return out
}

// wireToolInterceptPolicy installs the custom-tool policy into the interceptor.
func (s *Server) wireToolInterceptPolicy() {
	if s == nil || s.m == nil || s.m.interceptor == nil || s.m.PG() == nil {
		return
	}
	s.toolPolicy = newCustomToolPolicy(s.m.PG())
	s.toolPolicy.refresh()
	s.m.interceptor.SetToolPolicy(intercept.ToolPolicy{
		Enabled: s.toolPolicy.enabled,
		Surface: s.toolPolicy.surface,
	})
}

// refreshToolPolicy re-snapshots custom tools after create/update/delete/import.
func (s *Server) refreshToolPolicy() {
	if s.toolPolicy != nil {
		s.toolPolicy.refresh()
	}
}
