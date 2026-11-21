package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/Autumn-27/artex/db"
)

// toolStatus is the read-only binary probe surfaced by the toolbox. It never
// installs anything: the UI shows "installed / missing" plus a version string so
// the operator can fix the image or PATH, not trigger a download from a web page.
type toolStatus struct {
	Key       string `json:"key"`
	Binary    string `json:"binary,omitempty"`
	Installed bool   `json:"installed"`
	Path      string `json:"path,omitempty"`
	Version   string `json:"version,omitempty"`
	Error     string `json:"error,omitempty"`
}

const (
	toolProbeTimeout = 3 * time.Second
	maxToolVersion   = 4096
	maxImportTools   = 200
	maxImportBytes   = 4 << 20
)

func (s *Server) pgToolStatus(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	tools, err := pg.ListTools()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	withVersions := r.URL.Query().Get("version") == "1"
	out := make([]toolStatus, 0, len(tools))
	for _, t := range tools {
		st := toolStatus{Key: t.Key, Binary: t.Binary}
		if strings.TrimSpace(t.Binary) == "" {
			st.Error = "未配置二进制"
			out = append(out, st)
			continue
		}
		path, err := exec.LookPath(t.Binary)
		if err != nil {
			st.Error = "未安装"
			out = append(out, st)
			continue
		}
		st.Installed = true
		st.Path = path
		if withVersions && strings.TrimSpace(t.VersionCmd) != "" {
			st.Version = probeToolVersion(r.Context(), t.VersionCmd)
		}
		out = append(out, st)
	}
	writeJSON(w, 200, map[string]any{"status": out})
}

// probeToolVersion runs an operator-configured version command directly (argv
// split, no shell), with a short timeout and a hard output cap.
func probeToolVersion(ctx context.Context, cmd string) string {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return ""
	}
	cctx, cancel := context.WithTimeout(ctx, toolProbeTimeout)
	defer cancel()
	out, _ := exec.CommandContext(cctx, fields[0], fields[1:]...).CombinedOutput()
	text := strings.TrimSpace(string(out))
	if len(text) > maxToolVersion {
		text = text[:maxToolVersion]
	}
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = strings.TrimSpace(text[:i])
	}
	return maskSecrets(text)
}

// pgExportTools exports user-defined tools only. Built-in tools are code-owned and
// must not be round-tripped into another instance as if they were data.
func (s *Server) pgExportTools(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	rows, err := pg.ListCustomTools()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if rows == nil {
		rows = []*db.Tool{}
	}
	writeJSON(w, 200, map[string]any{"version": 1, "tools": rows})
}

type importTool struct {
	Key         string          `json:"key"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
	Agents      []string        `json:"agents"`
	Enabled     *bool           `json:"enabled"`
	Kind        string          `json:"kind"`
	Exec        json.RawMessage `json:"exec"`
	Deferred    *bool           `json:"deferred"`
	Category    string          `json:"category"`
	Binary      string          `json:"binary"`
	ProxyMode   string          `json:"proxy_mode"`
	VersionCmd  string          `json:"version_cmd"`
	Homepage    string          `json:"homepage"`
}

// pgImportTools imports custom tool definitions without overwriting existing keys.
// Imported definitions are disabled unless the file explicitly says enabled=true;
// deferred defaults to true so a newly imported tool does not immediately expand
// the model's schema list. The import never executes anything.
func (s *Server) pgImportTools(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBytes)
	var req struct {
		Tools []importTool `json:"tools"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, 400, "无效的导入文件: "+err.Error())
		return
	}
	if len(req.Tools) == 0 {
		writeErr(w, 400, "导入文件里没有 tools")
		return
	}
	if len(req.Tools) > maxImportTools {
		writeErr(w, 400, fmt.Sprintf("一次最多导入 %d 个工具", maxImportTools))
		return
	}

	imported, skipped := 0, 0
	errs := make([]string, 0)
	for i, t := range req.Tools {
		key := strings.TrimSpace(t.Key)
		label := key
		if label == "" {
			label = fmt.Sprintf("#%d", i+1)
		}
		if !reToolKey.MatchString(key) {
			errs = append(errs, label+": key 不合法")
			skipped++
			continue
		}
		if !validCustomKind(t.Kind) {
			errs = append(errs, label+": kind 需为 "+customKindList)
			skipped++
			continue
		}
		proxyMode, ok := normalizeProxyMode(t.ProxyMode)
		if !ok {
			errs = append(errs, label+": proxy_mode 不合法")
			skipped++
			continue
		}
		if err := validateToolExec(t.Kind, t.Schema, t.Exec); err != nil {
			errs = append(errs, label+": "+err.Error())
			skipped++
			continue
		}
		if exist, _ := pg.GetTool(key); exist != nil {
			errs = append(errs, label+": 已存在，跳过")
			skipped++
			continue
		}
		enabled := false
		if t.Enabled != nil {
			enabled = *t.Enabled
		}
		deferred := true
		if t.Deferred != nil {
			deferred = *t.Deferred
		}
		err := pg.CreateCustomTool(&db.Tool{
			Key: key, Description: t.Description, Schema: t.Schema, Agents: t.Agents,
			Enabled: enabled, Kind: t.Kind, Exec: t.Exec, Deferred: deferred,
			ToolMeta: db.ToolMeta{
				Category: t.Category, Binary: t.Binary, ProxyMode: proxyMode,
				Source: "imported", VersionCmd: t.VersionCmd, Homepage: t.Homepage,
			},
		})
		if err != nil {
			errs = append(errs, label+": "+err.Error())
			skipped++
			continue
		}
		imported++
	}
	writeJSON(w, 200, map[string]any{"imported": imported, "skipped": skipped, "errors": errs})
}
