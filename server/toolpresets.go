package server

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Autumn-27/artex/db"
)

// The preset catalog is code-owned data embedded in the binary: recipes can't be
// altered by import/export round-trips, and installing one copies a frozen
// snapshot into the tools table (still an ordinary editable custom tool).
//
//go:embed toolpresets.json
var toolPresetsJSON []byte

type toolPreset struct {
	Key         string          `json:"key"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Binary      string          `json:"binary"`
	ProxyMode   string          `json:"proxy_mode"`
	VersionCmd  string          `json:"version_cmd"`
	Homepage    string          `json:"homepage"`
	Schema      json.RawMessage `json:"schema"`
	Exec        json.RawMessage `json:"exec"`
}

// loadToolPresets parses + validates the embedded catalog. A malformed preset is
// a build-time bug, so handlers surface it as a 500 instead of serving a partial
// set (an operator would otherwise install a silently broken tool).
func loadToolPresets() ([]toolPreset, error) {
	var list []toolPreset
	if err := json.Unmarshal(toolPresetsJSON, &list); err != nil {
		return nil, fmt.Errorf("解析预置目录失败: %w", err)
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("预置目录为空")
	}
	seen := make(map[string]bool, len(list))
	for i := range list {
		p := &list[i]
		p.Key = strings.TrimSpace(p.Key)
		if !reToolKey.MatchString(p.Key) {
			return nil, fmt.Errorf("预置工具 #%d 的 key 不合法: %q", i+1, p.Key)
		}
		if seen[p.Key] {
			return nil, fmt.Errorf("预置工具 key 重复: %s", p.Key)
		}
		seen[p.Key] = true
		mode, ok := normalizeProxyMode(p.ProxyMode)
		if !ok {
			return nil, fmt.Errorf("预置工具 %s 的 proxy_mode 不合法: %q", p.Key, p.ProxyMode)
		}
		p.ProxyMode = mode
		if err := validateToolExec("recipe", p.Schema, p.Exec, p.Binary); err != nil {
			return nil, fmt.Errorf("预置工具 %s: %w", p.Key, err)
		}
	}
	return list, nil
}

// pgListToolPresets returns the catalog plus an installed flag per preset.
// A preset counts as installed when any tool (built-in or custom) already owns
// the key, because installing would otherwise shadow or conflict with it.
func (s *Server) pgListToolPresets(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	presets, err := loadToolPresets()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	tools, err := pg.ListTools()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	existing := make(map[string]bool, len(tools))
	for _, t := range tools {
		existing[t.Key] = true
	}
	out := make([]map[string]any, 0, len(presets))
	for _, p := range presets {
		out = append(out, map[string]any{
			"key": p.Key, "description": p.Description, "category": p.Category,
			"binary": p.Binary, "proxy_mode": p.ProxyMode, "version_cmd": p.VersionCmd,
			"homepage": p.Homepage, "installed": existing[p.Key],
		})
	}
	writeJSON(w, 200, map[string]any{"presets": out})
}

const maxPresetInstall = 50

// pgInstallToolPresets copies selected presets into the tools table. Installed
// tools are disabled + deferred with no agent bindings: installing a binary
// recipe must never silently widen any agent's tool surface.
func (s *Server) pgInstallToolPresets(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	var req struct {
		Keys []string `json:"keys"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, 400, "无效的请求体")
		return
	}
	if len(req.Keys) == 0 {
		writeErr(w, 400, "未选择任何预置工具")
		return
	}
	if len(req.Keys) > maxPresetInstall {
		writeErr(w, 400, fmt.Sprintf("一次最多安装 %d 个预置工具", maxPresetInstall))
		return
	}
	presets, err := loadToolPresets()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	byKey := make(map[string]toolPreset, len(presets))
	for _, p := range presets {
		byKey[p.Key] = p
	}

	installed := make([]string, 0, len(req.Keys))
	skipped := make([]string, 0)
	errs := make([]string, 0)
	seen := make(map[string]bool, len(req.Keys))
	for _, raw := range req.Keys {
		key := strings.TrimSpace(raw)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		p, ok := byKey[key]
		if !ok {
			errs = append(errs, key+": 预置目录中没有该工具")
			skipped = append(skipped, key)
			continue
		}
		if exist, _ := pg.GetTool(key); exist != nil {
			skipped = append(skipped, key)
			continue
		}
		if err := pg.CreateCustomTool(&db.Tool{
			Key: key, Description: p.Description, Schema: p.Schema,
			Enabled: false, Kind: "recipe", Exec: p.Exec, Deferred: true,
			ToolMeta: db.ToolMeta{
				Category: p.Category, Binary: p.Binary, ProxyMode: p.ProxyMode,
				Source: "preset", VersionCmd: p.VersionCmd, Homepage: p.Homepage,
			},
		}); err != nil {
			errs = append(errs, key+": "+err.Error())
			skipped = append(skipped, key)
			continue
		}
		installed = append(installed, key)
	}
	if len(installed) > 0 {
		s.refreshToolPolicy()
	}
	writeJSON(w, 200, map[string]any{"installed": installed, "skipped": skipped, "errors": errs})
}
