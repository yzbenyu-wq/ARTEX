// Package netguard rejects outbound requests to loopback, link-local,
// unspecified and multicast addresses. RFC1918 ranges are deliberately allowed:
// ARTEX is a pentesting platform and internal ranges are legitimate targets.
package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"syscall"
)

// BlockedIP reports whether ip must not be dialed by a tool-driven fetch.
func BlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

// CheckHost rejects localhost names, blocked literal IPs, and hostnames whose
// resolution includes a blocked address. A DNS failure is not an error here:
// the actual fetch reports it, and a failed lookup is not an SSRF risk.
func CheckHost(ctx context.Context, host string) error {
	host = strings.TrimSuffix(strings.TrimSpace(host), ".")
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = host[1 : len(host)-1]
	}
	if zone := strings.IndexByte(host, '%'); zone >= 0 {
		host = host[:zone]
	}
	if host == "" {
		return errors.New("URL 缺少主机名")
	}
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return fmt.Errorf("拒绝访问本机地址 %q", host)
	}
	if ip := net.ParseIP(host); ip != nil {
		if BlockedIP(ip) {
			return fmt.Errorf("拒绝访问本机/链路本地地址 %s", ip)
		}
		return nil
	}
	// inet_aton-style forms (2130706433, 0x7f000001, 127.1) bypass net.ParseIP
	// but some dialers still accept them, so parse them ourselves.
	if looksNumericHost(host) {
		legacy, ok := parseLegacyIPv4(host)
		if !ok {
			return fmt.Errorf("拒绝无法解析的数字主机地址 %q", host)
		}
		if BlockedIP(legacy) {
			return fmt.Errorf("拒绝访问本机/链路本地地址 %s", legacy)
		}
		return nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil
	}
	for _, a := range addrs {
		if BlockedIP(a.IP) {
			return fmt.Errorf("拒绝访问本机/链路本地地址 %s（%s 解析结果）", a.IP, host)
		}
	}
	return nil
}

// CheckURL validates that u points at an http(s) host that is safe to fetch.
func CheckURL(ctx context.Context, u *url.URL) error {
	if u == nil {
		return errors.New("URL 为空")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("拒绝非 http(s) 协议 %q", u.Scheme)
	}
	return CheckHost(ctx, u.Hostname())
}

// DialControl is a net.Dialer Control hook that re-checks the resolved address
// at connect time, covering DNS answers that change between validation and dial.
func DialControl(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("无法解析目标地址 %q", host)
	}
	if BlockedIP(ip) {
		return fmt.Errorf("拒绝访问本机/链路本地地址 %s", ip)
	}
	return nil
}

// looksNumericHost reports whether host could be an inet_aton-style address.
func looksNumericHost(host string) bool {
	if host == "" || host[0] < '0' || host[0] > '9' {
		return false
	}
	for i := 0; i < len(host); i++ {
		c := host[i]
		switch {
		case c == '.':
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		case c == 'x' || c == 'X':
		default:
			return false
		}
	}
	return true
}

// parseLegacyIPv4 follows inet_aton's 1-, 2-, 3- and 4-part IPv4 notation.
func parseLegacyIPv4(host string) (net.IP, bool) {
	parts := strings.Split(host, ".")
	if len(parts) < 1 || len(parts) > 4 {
		return nil, false
	}
	values := make([]uint64, len(parts))
	for i, part := range parts {
		if part == "" {
			return nil, false
		}
		base := 10
		digits := part
		if len(digits) > 2 && digits[0] == '0' && (digits[1] == 'x' || digits[1] == 'X') {
			base, digits = 16, digits[2:]
		} else if len(digits) > 1 && digits[0] == '0' {
			base = 8
		}
		if digits == "" {
			return nil, false
		}
		value, err := strconv.ParseUint(digits, base, 32)
		if err != nil {
			return nil, false
		}
		values[i] = value
	}
	var value uint64
	switch len(values) {
	case 1:
		value = values[0]
	case 2:
		if values[0] > 0xff || values[1] > 0xffffff {
			return nil, false
		}
		value = values[0]<<24 | values[1]
	case 3:
		if values[0] > 0xff || values[1] > 0xff || values[2] > 0xffff {
			return nil, false
		}
		value = values[0]<<24 | values[1]<<16 | values[2]
	case 4:
		for _, part := range values {
			if part > 0xff {
				return nil, false
			}
		}
		value = values[0]<<24 | values[1]<<16 | values[2]<<8 | values[3]
	}
	if value > 0xffffffff {
		return nil, false
	}
	return net.IPv4(byte(value>>24), byte(value>>16), byte(value>>8), byte(value)), true
}
