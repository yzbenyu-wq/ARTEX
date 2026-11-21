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
