package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Autumn-27/artex/db"
	"github.com/Autumn-27/artex/notify"
)

func TestMaskMCPServerEnv(t *testing.T) {
	m := &db.MCPServer{Env: json.RawMessage(`{"API_KEY":"sk-super-secret-value","NODE_ENV":"production"}`)}
	maskMCPServerEnv(m)
	var env map[string]string
	if err := json.Unmarshal(m.Env, &env); err != nil {
		t.Fatal(err)
	}
	for k, v := range env {
		if !notify.IsMasked(v) {
			t.Errorf("%s = %q, want masked", k, v)
		}
	}
	if strings.Contains(string(m.Env), "sk-super-secret-value") {
		t.Fatal("secret leaked through mask")
	}
	if !strings.Contains(env["API_KEY"], "value") {
		t.Errorf("mask should keep a short tail hint: %q", env["API_KEY"])
	}
}

func TestSpliceMaskedEnv(t *testing.T) {
	stored := json.RawMessage(`{"API_KEY":"real-key-123456","OLD":"old-secret-987"}`)
	tests := []struct {
		name     string
		incoming string
		want     map[string]string
	}{
		{
			name:     "mask restores stored",
			incoming: `{"API_KEY":"__masked__:…123456"}`,
			want:     map[string]string{"API_KEY": "real-key-123456"},
		},
		{
			name:     "plain value replaces",
			incoming: `{"API_KEY":"new-key-abcdef"}`,
			want:     map[string]string{"API_KEY": "new-key-abcdef"},
		},
		{
			name:     "masked unknown key dropped",
			incoming: `{"NEW":"__masked__"}`,
			want:     map[string]string{},
		},
		{
			name:     "mixed",
			incoming: `{"API_KEY":"__masked__","ADDED":"plain","OLD":"__masked__:…cret-987"}`,
			want:     map[string]string{"API_KEY": "real-key-123456", "ADDED": "plain", "OLD": "old-secret-987"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRaw := spliceMaskedEnv(json.RawMessage(tt.incoming), stored)
			var got map[string]string
			if err := json.Unmarshal(gotRaw, &got); err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("%s = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}
