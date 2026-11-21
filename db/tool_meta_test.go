package db

import (
	"encoding/json"
	"testing"
)

// TestToolMetadataRoundTrip verifies toolbox metadata survives create/read/update
// for custom tools and that empty values normalize to sane defaults.
func TestToolMetadataRoundTrip(t *testing.T) {
	d, err := Open(testDSN(t))
	if err != nil {
		t.Skipf("postgres unavailable (%v) — skipping", err)
	}
	defer d.Close()

	key := "ct_meta_test_tool"
	_ = d.DeleteCustomTool(key)
	t.Cleanup(func() { _ = d.DeleteCustomTool(key) })

	if err := d.CreateCustomTool(&Tool{
		Key: key, Description: "meta", Schema: json.RawMessage(`{}`),
		Enabled: true, Kind: "command", Exec: json.RawMessage(`{"command":"echo hi"}`),
		ToolMeta: ToolMeta{
			Category: "recon", Binary: "httpx", ProxyMode: "socks5",
			Source: "imported", VersionCmd: "httpx -version", Homepage: "https://example.test",
		},
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := d.GetTool(key)
	if err != nil || got == nil {
		t.Fatalf("get: %v (nil=%v)", err, got == nil)
	}
	if got.Category != "recon" || got.Binary != "httpx" || got.ProxyMode != "socks5" ||
		got.Source != "imported" || got.VersionCmd != "httpx -version" || got.Homepage != "https://example.test" {
		t.Fatalf("metadata not persisted: %+v", got.ToolMeta)
	}

	// UpdateCustomTool also writes metadata.
	upd := *got
	upd.ToolMeta = ToolMeta{Category: "web", Binary: "nuclei", ProxyMode: "env", Source: "custom"}
	if err := d.UpdateCustomTool(&upd); err != nil {
		t.Fatalf("update custom: %v", err)
	}
	got2, _ := d.GetTool(key)
	if got2.Category != "web" || got2.Binary != "nuclei" || got2.ProxyMode != "env" || got2.Source != "custom" {
		t.Fatalf("metadata not updated: %+v", got2.ToolMeta)
	}

	// UpdateToolWithMeta (system-tool path) writes the same columns.
	if err := d.UpdateToolWithMeta(key, "meta2", json.RawMessage(`{}`), json.RawMessage(`[]`), false,
		ToolMeta{Category: "dns", Binary: "dnsx", ProxyMode: "raw", Source: "builtin"}); err != nil {
		t.Fatalf("update with meta: %v", err)
	}
	got3, _ := d.GetTool(key)
	if got3.Category != "dns" || got3.Binary != "dnsx" || got3.ProxyMode != "raw" ||
		got3.Source != "builtin" || got3.Enabled || got3.Description != "meta2" {
		t.Fatalf("UpdateToolWithMeta not applied: %+v", got3)
	}

	// Empty metadata normalizes: proxy_mode→inherit, source→custom.
	keyDefault := "ct_meta_default_test_tool"
	_ = d.DeleteCustomTool(keyDefault)
	t.Cleanup(func() { _ = d.DeleteCustomTool(keyDefault) })
	if err := d.CreateCustomTool(&Tool{
		Key: keyDefault, Kind: "command", Exec: json.RawMessage(`{"command":"echo hi"}`),
	}); err != nil {
		t.Fatalf("create default: %v", err)
	}
	def, _ := d.GetTool(keyDefault)
	if def.ProxyMode != "inherit" || def.Source != "custom" {
		t.Fatalf("default metadata = %+v, want inherit/custom", def.ToolMeta)
	}
}

func TestNormalizeToolMetaTrims(t *testing.T) {
	got := normalizeToolMeta(ToolMeta{
		Category: " recon ", Binary: " httpx ", ProxyMode: " ", Source: " ",
		VersionCmd: " httpx -version ", Homepage: " https://example.test ",
	})
	if got.Category != "recon" || got.Binary != "httpx" || got.ProxyMode != "inherit" ||
		got.Source != "custom" || got.VersionCmd != "httpx -version" || got.Homepage != "https://example.test" {
		t.Fatalf("normalize = %+v", got)
	}
}
