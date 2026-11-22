package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// restoreSetting returns key to its prior value when the test ends, or deletes
// it if it was absent.
func restoreSetting(t *testing.T, m *Manager, key string) {
	t.Helper()
	old, existed, err := m.pg.GetSetting(key)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if existed {
			_ = m.pg.SetSetting(key, old)
		} else {
			_, _ = m.pg.Exec("DELETE FROM settings WHERE key=$1", key)
		}
	})
}

func putSettingsBody(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	s.putSettings(w, httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body)))
	return w
}

func wantSetting(t *testing.T, m *Manager, key, want string) {
	t.Helper()
	got, ok, err := m.pg.GetSetting(key)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got != want {
		t.Fatalf("%s=%q (present=%v), want %q", key, got, ok, want)
	}
}

// TestPutSettingsWritesAllFieldsInOneTransaction checks the happy path: every
// field of a multi-field save is persisted and echoed back.
func TestPutSettingsWritesAllFieldsInOneTransaction(t *testing.T) {
	m := proxyTestManager(t)
	for _, key := range []string{
		settingNotifyEnabled, settingNotifyDigestMinutes, settingPythonInterp,
		settingConstraintsInjectPlanner, settingEgressProbeURL,
	} {
		restoreSetting(t, m, key)
	}
	s := &Server{m: m}

	w := putSettingsBody(t, s, `{"notify_enabled":false,"notify_digest_interval_min":17,`+
		`"python_interpreter":"  /usr/bin/python3  ","constraints_inject_planner":false,`+
		`"egress_probe_url":"https://example.com/check"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	wantSetting(t, m, settingNotifyEnabled, "false")
	wantSetting(t, m, settingNotifyDigestMinutes, "17")
	wantSetting(t, m, settingPythonInterp, "/usr/bin/python3")
	wantSetting(t, m, settingConstraintsInjectPlanner, "false")
	wantSetting(t, m, settingEgressProbeURL, "https://example.com/check")
	if !strings.Contains(w.Body.String(), `"python_interpreter":"/usr/bin/python3"`) {
		t.Fatalf("payload did not echo the stored value: %s", w.Body.String())
	}
}

// TestPutSettingsRollsBackOnWriteFailure proves the whole save is atomic: a
// failure while writing one field must leave every other field untouched.
func TestPutSettingsRollsBackOnWriteFailure(t *testing.T) {
	m := proxyTestManager(t)
	for _, key := range []string{settingNotifyEnabled, settingNotifyDigestMinutes} {
		restoreSetting(t, m, key)
	}
	if err := m.pg.SetSetting(settingNotifyEnabled, "true"); err != nil {
		t.Fatal(err)
	}
	if err := m.pg.SetSetting(settingNotifyDigestMinutes, "5"); err != nil {
		t.Fatal(err)
	}

	// Inject a failure on the second key so a non-transactional save would leave
	// notify_enabled flipped.
	if _, err := m.pg.Exec(`CREATE OR REPLACE FUNCTION artex_test_block_digest() RETURNS trigger AS $$
BEGIN
	IF NEW.key = 'notify_digest_interval_min' THEN
		RAISE EXCEPTION 'injected settings failure';
	END IF;
	RETURN NEW;
END
$$ LANGUAGE plpgsql`); err != nil {
		t.Fatal(err)
	}
	if _, err := m.pg.Exec(`CREATE TRIGGER artex_test_block_digest BEFORE INSERT OR UPDATE ON settings
FOR EACH ROW EXECUTE FUNCTION artex_test_block_digest()`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = m.pg.Exec(`DROP TRIGGER IF EXISTS artex_test_block_digest ON settings`)
		_, _ = m.pg.Exec(`DROP FUNCTION IF EXISTS artex_test_block_digest()`)
	})

	s := &Server{m: m}
	w := putSettingsBody(t, s, `{"notify_enabled":false,"notify_digest_interval_min":30}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "injected settings failure") {
		t.Fatalf("unexpected error body: %s", w.Body.String())
	}
	wantSetting(t, m, settingNotifyEnabled, "true")
	wantSetting(t, m, settingNotifyDigestMinutes, "5")
}

// browserMCPArgs returns the persisted raw args of the seeded browser MCP.
func browserMCPArgs(t *testing.T, m *Manager) string {
	t.Helper()
	servers, err := m.pg.ListMCP()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range servers {
		if s.Name == browserMCPName {
			return string(s.Args)
		}
	}
	t.Fatalf("%s MCP not seeded", browserMCPName)
	return ""
}

// TestPutSettingsRollsBackMCPProjectionOnSettingsFailure proves the derived
// browser-MCP row is written in the same transaction: when a settings write
// fails after the projection succeeded, the projection rolls back too.
func TestPutSettingsRollsBackMCPProjectionOnSettingsFailure(t *testing.T) {
	m := proxyTestManager(t)
	restoreSetting(t, m, settingTrafficCapture)
	ctx, cancel := context.WithCancel(context.Background())
	s := New(ctx, m, t.TempDir(), t.TempDir(), t.TempDir())
	// Cancel before Close: proxyPoolWG only drains once the server context ends.
	defer func() { cancel(); s.Close() }()

	if _, err := m.pg.Exec(`CREATE OR REPLACE FUNCTION artex_test_block_traffic_capture() RETURNS trigger AS $$
BEGIN
	IF NEW.key = 'traffic_capture' THEN
		RAISE EXCEPTION 'injected settings failure';
	END IF;
	RETURN NEW;
END
$$ LANGUAGE plpgsql`); err != nil {
		t.Fatal(err)
	}
	if _, err := m.pg.Exec(`CREATE TRIGGER artex_test_block_traffic_capture BEFORE INSERT OR UPDATE ON settings
FOR EACH ROW EXECUTE FUNCTION artex_test_block_traffic_capture()`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = m.pg.Exec(`DROP TRIGGER IF EXISTS artex_test_block_traffic_capture ON settings`)
		_, _ = m.pg.Exec(`DROP FUNCTION IF EXISTS artex_test_block_traffic_capture()`)
	})

	before := browserMCPArgs(t, m)
	w := putSettingsBody(t, s, `{"traffic_capture":true}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if after := browserMCPArgs(t, m); after != before {
		t.Fatalf("browser MCP args leaked past the rollback: before=%s after=%s", before, after)
	}
}

// TestPutSettingsRollsBackSettingsOnMCPFailure proves the reverse direction: a
// failed projection write must abort the whole save before any setting lands.
func TestPutSettingsRollsBackSettingsOnMCPFailure(t *testing.T) {
	m := proxyTestManager(t)
	for _, key := range []string{settingTrafficCapture, settingNotifyEnabled} {
		restoreSetting(t, m, key)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := New(ctx, m, t.TempDir(), t.TempDir(), t.TempDir())
	// Cancel before Close: proxyPoolWG only drains once the server context ends.
	defer func() { cancel(); s.Close() }()

	if _, err := m.pg.Exec(`CREATE OR REPLACE FUNCTION artex_test_block_browser_mcp() RETURNS trigger AS $$
BEGIN
	IF NEW.name = 'browser' THEN
		RAISE EXCEPTION 'injected mcp failure';
	END IF;
	RETURN NEW;
END
$$ LANGUAGE plpgsql`); err != nil {
		t.Fatal(err)
	}
	if _, err := m.pg.Exec(`CREATE TRIGGER artex_test_block_browser_mcp BEFORE UPDATE ON mcp_servers
FOR EACH ROW EXECUTE FUNCTION artex_test_block_browser_mcp()`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = m.pg.Exec(`DROP TRIGGER IF EXISTS artex_test_block_browser_mcp ON mcp_servers`)
		_, _ = m.pg.Exec(`DROP FUNCTION IF EXISTS artex_test_block_browser_mcp()`)
	})

	beforeArgs := browserMCPArgs(t, m)
	w := putSettingsBody(t, s, `{"traffic_capture":true,"notify_enabled":false}`)
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "injected mcp failure") {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if after := browserMCPArgs(t, m); after != beforeArgs {
		t.Fatalf("browser MCP row changed despite the failure: before=%s after=%s", beforeArgs, after)
	}
	// The settings write happens after the projection, so it never ran.
	if got, ok, _ := m.pg.GetSetting(settingNotifyEnabled); ok && got == "false" {
		t.Fatalf("%s persisted despite the aborted save", settingNotifyEnabled)
	}
}
