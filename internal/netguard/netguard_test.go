package netguard

import (
	"context"
	"net"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestBlockedIP(t *testing.T) {
	tests := []struct {
		ip      string
		blocked bool
	}{
		{"127.0.0.1", true},
		{"127.1.2.3", true},
		{"::1", true},
		{"0.0.0.0", true},
		{"::", true},
		{"169.254.169.254", true},
		{"fe80::1", true},
		{"224.0.0.1", true},
		{"ff02::1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"10.0.0.5", false},
		{"172.16.0.9", false},
		{"192.168.1.1", false},
	}
	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		if ip == nil {
			t.Fatalf("bad test ip %q", tt.ip)
		}
		if got := BlockedIP(ip); got != tt.blocked {
			t.Errorf("BlockedIP(%s) = %v, want %v", tt.ip, got, tt.blocked)
		}
	}
	if !BlockedIP(nil) {
		t.Error("BlockedIP(nil) = false, want true")
	}
}

func TestCheckHost(t *testing.T) {
	tests := []struct {
		host string
		ok   bool
	}{
		{"127.0.0.1", false},
		{"127.0.0.1.", false},
		{"[::1]", false},
		{"::1", false},
		{"localhost", false},
		{"LOCALHOST.", false},
		{"foo.localhost", false},
		{"169.254.169.254", false},
		{"fe80::1%eth0", false},
		{"0.0.0.0", false},
		{"", false},
		{"10.1.2.3", true},
		{"192.168.0.10", true},
		{"8.8.8.8", true},
		{"2130706433", false},
		{"0x7f000001", false},
		{"127.1", false},
		{"017700000001", false},
		{"0x08080808", true},
	}
	for _, tt := range tests {
		err := CheckHost(context.Background(), tt.host)
		if tt.ok && err != nil {
			t.Errorf("CheckHost(%q) = %v, want nil", tt.host, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("CheckHost(%q) = nil, want error", tt.host)
		}
	}
}

func TestCheckURL(t *testing.T) {
	tests := []struct {
		raw string
		ok  bool
	}{
		{"http://127.0.0.1:8787/api/health", false},
		{"https://localhost/admin", false},
		{"http://169.254.169.254/latest/meta-data", false},
		{"ftp://example.com/file", false},
		{"http://10.0.0.10:8080/", true},
		{"https://example.com/page", true},
	}
	for _, tt := range tests {
		u, err := url.Parse(tt.raw)
		if err != nil {
			t.Fatalf("parse %q: %v", tt.raw, err)
		}
		err = CheckURL(context.Background(), u)
		if tt.ok && err != nil {
			t.Errorf("CheckURL(%q) = %v, want nil", tt.raw, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("CheckURL(%q) = nil, want error", tt.raw)
		}
	}
}

func TestDialControl(t *testing.T) {
	tests := []struct {
		address string
		ok      bool
	}{
		{"127.0.0.1:8787", false},
		{"[::1]:5432", false},
		{"169.254.169.254:80", false},
		{"10.0.0.5:443", true},
		{"8.8.8.8:443", true},
	}
	for _, tt := range tests {
		err := DialControl("tcp", tt.address, nil)
		if tt.ok && err != nil {
			t.Errorf("DialControl(%q) = %v, want nil", tt.address, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("DialControl(%q) = nil, want error", tt.address)
		}
	}
	if err := DialControl("tcp", "bad-address", nil); err == nil {
		t.Error("DialControl(bad-address) = nil, want error")
	}
}

func TestCheckHostBlocksResolvedLoopback(t *testing.T) {
	// DNS resolution is only exercised when the host is not a literal; use a
	// short deadline so an offline resolver does not stall the suite.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := CheckHost(ctx, "localhost")
	if err == nil || !strings.Contains(err.Error(), "本机") {
		t.Fatalf("CheckHost(localhost) = %v, want 本机 rejection", err)
	}
}
