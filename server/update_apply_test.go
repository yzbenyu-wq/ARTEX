package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 容器里的一键更新会把官方二进制写进可写层：工具链丢了，重建容器还会退回。
// 接口必须直接拒绝，并告诉用户正确的升级方式是拉镜像。
func TestUpdateApplyRejectedInDocker(t *testing.T) {
	old := updateInDocker
	updateInDocker = func() bool { return true }
	t.Cleanup(func() { updateInDocker = old })

	s := &Server{}
	w := httptest.NewRecorder()
	s.updateApply(w, httptest.NewRequest(http.MethodPost, "/api/update/apply", nil))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "docker compose pull") {
		t.Fatalf("拒绝说明必须给出正确的升级方式: %s", w.Body.String())
	}
}
