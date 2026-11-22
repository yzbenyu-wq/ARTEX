package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// 内部簿记目录以点开头（任务删除暂存 .delete-staging、归档暂存 .staging），
// 它们不是用户数据，不能出现在工作区文件管理器里。
func TestWorkspaceListHidesDotEntries(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{".delete-staging", "archives"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{"report.md", filepath.Join("archives", ".staging")} {
		if err := os.WriteFile(filepath.Join(root, file), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	s := &Server{m: &Manager{dir: root}}

	list := func(path string) map[string]bool {
		t.Helper()
		w := httptest.NewRecorder()
		s.wsList(w, httptest.NewRequest("GET", "/api/workspace/list?path="+path, nil))
		if w.Code != 200 {
			t.Fatalf("list %q status=%d body=%s", path, w.Code, w.Body.String())
		}
		var payload struct {
			Entries []wsEntry `json:"entries"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("list %q decode: %v", path, err)
		}
		names := make(map[string]bool, len(payload.Entries))
		for _, e := range payload.Entries {
			names[e.Name] = true
		}
		return names
	}

	names := list("")
	if names[".delete-staging"] {
		t.Fatalf("根目录仍列出内部暂存目录: %v", names)
	}
	if !names["report.md"] || !names["archives"] {
		t.Fatalf("正常文件/目录被误过滤: %v", names)
	}

	nested := list("archives")
	if nested[".staging"] {
		t.Fatalf("子目录仍列出内部暂存目录: %v", nested)
	}
}

func TestWorkspaceDeniesSensitiveDirs(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"traffic/_ca", "keys", "notes"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	secret := filepath.Join(root, "traffic", "_ca", "mitmproxy-ca.pem")
	if err := os.WriteFile(secret, []byte("PRIVATE KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := &Server{m: &Manager{dir: root}}

	for _, rel := range []string{"traffic", "traffic/_ca", "traffic/_ca/mitmproxy-ca.pem", "TRAFFIC/_ca/mitmproxy-ca.pem", "keys", "keys/jwt.key"} {
		if abs, ok := s.wsResolve(rel); ok {
			t.Errorf("wsResolve(%q) allowed: %s", rel, abs)
		}
	}
	if _, ok := s.wsResolve("notes/../traffic/_ca/mitmproxy-ca.pem"); ok {
		t.Error("traversal into traffic allowed")
	}
	if abs, ok := s.wsResolve("notes/new-file.md"); !ok {
		t.Error("ordinary new path rejected")
	} else if abs != filepath.Join(root, "notes", "new-file.md") {
		t.Errorf("unexpected resolved path %q", abs)
	}
	if abs, ok := s.wsResolve("notes/deeper/new.md"); !ok {
		t.Errorf("new nested path rejected: %s", abs)
	}
}

func TestWorkspaceDeniesSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "jwt.key"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "leak")); err != nil {
		t.Fatal(err)
	}
	s := &Server{m: &Manager{dir: root}}

	if abs, ok := s.wsResolve("leak/jwt.key"); ok {
		t.Fatalf("symlink escape allowed: %s", abs)
	}
	if abs, ok := s.wsResolve("leak"); ok {
		t.Fatalf("symlink itself resolvable for file routes: %s", abs)
	}
	// The absolute-path branch used by preview must apply the same check.
	abs := filepath.Join(root, "leak", "jwt.key")
	if s.wsWithinRoot(abs) {
		t.Errorf("wsWithinRoot accepted escaping path %s", abs)
	}
}

func TestWorkspaceListHidesSensitiveDirs(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"traffic", "keys", "notes"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	s := &Server{m: &Manager{dir: root}}
	w := httptest.NewRecorder()
	s.wsList(w, httptest.NewRequest("GET", "/api/workspace/list?path=", nil))
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var payload struct {
		Entries []wsEntry `json:"entries"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	for _, e := range payload.Entries {
		if e.Name == "traffic" || e.Name == "keys" {
			t.Fatalf("sensitive dir listed: %s", e.Name)
		}
	}
	if len(payload.Entries) != 1 || payload.Entries[0].Name != "notes" {
		t.Fatalf("unexpected entries: %+v", payload.Entries)
	}
}
