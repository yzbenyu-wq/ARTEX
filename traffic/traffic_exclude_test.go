package traffic

import "testing"

func TestParseExcludeHosts(t *testing.T) {
	got, err := ParseExcludeHosts(" Deb.Debian.org\n*.example.com, *.example.com\r\n\n10.0.0.* ")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"deb.debian.org", "*.example.com", "10.0.0.*"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}

	for _, bad := range []string{"two hosts", "http://deb.debian.org", ".example.com", "example.com.", "a/b"} {
		if _, err := ParseExcludeHosts(bad); err == nil {
			t.Errorf("ParseExcludeHosts(%q) should fail", bad)
		}
	}
	if got, err := ParseExcludeHosts(" \n "); err != nil || len(got) != 0 {
		t.Fatalf("blank input should yield empty list, got %v, %v", got, err)
	}
}

func TestHostExcluded(t *testing.T) {
	tr := &Traffic{}
	tr.SetExcludeHosts([]string{"deb.debian.org", "*.example.com", "10.0.0.*"})

	for host, want := range map[string]bool{
		"deb.debian.org":       true,
		"DEB.Debian.ORG":       true,
		"deb.debian.org.evil":  false, // exact pattern must not become a suffix match
		"api.example.com":      true,
		"deep.api.example.com": true,
		"example.com":          false, // *.example.com needs a leading label
		"10.0.0.5":             true,
		"10.0.1.5":             false,
		"host.docker.internal": false,
	} {
		if got := tr.hostExcluded(host); got != want {
			t.Errorf("hostExcluded(%q) = %v, want %v", host, got, want)
		}
	}

	tr.SetExcludeHosts(nil)
	if tr.hostExcluded("deb.debian.org") {
		t.Fatal("clearing the list must disable exclusion")
	}
}

func TestRecordSkipsExcludedHost(t *testing.T) {
	tr, _ := openTraffic(t)

	tr.SetExcludeHosts([]string{"noise.example"})
	tr.record(newFlow("noise.example", "GET", "/health", nil, nil))
	if n, err := tr.Count(); err != nil || n != 0 {
		t.Fatalf("excluded host must not be recorded, count=%d err=%v", n, err)
	}

	tr.record(newFlow("target.example", "GET", "/health", nil, nil))
	if n, err := tr.Count(); err != nil || n != 1 {
		t.Fatalf("non-excluded host must be recorded, count=%d err=%v", n, err)
	}
}
