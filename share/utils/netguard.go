package utils

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// cgnatBlock is RFC 6598 shared address space (100.64.0.0/10).
var cgnatBlock = mustCIDR("100.64.0.0/10")

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

// IsPublicIP reports whether ip is a globally routable unicast address.
// It returns false for loopback, RFC1918 private, link-local (including
// 169.254.169.254 cloud metadata), IPv6 unique-local (fc00::/7), unspecified,
// multicast, CGNAT (100.64.0.0/10) and the IPv4-mapped IPv6 equivalents.
func IsPublicIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	// Normalise IPv4-mapped IPv6 (::ffff:a.b.c.d) to 4-byte form so the
	// IPv4 checks below apply to it as well.
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if ip.IsUnspecified() || ip.IsLoopback() || ip.IsMulticast() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsPrivate() {
		return false
	}
	if ip.To4() != nil {
		if cgnatBlock.Contains(ip) {
			return false
		}
		// 0.0.0.0/8 ("this" network) and 255.255.255.255 broadcast.
		if ip[0] == 0 || ip.Equal(net.IPv4bcast) {
			return false
		}
	}
	return true
}

// ValidateOutboundURL parses raw and checks that it is safe to be fetched by
// the server on behalf of a user. The scheme must be http or https, the host
// must be non-empty and the URL must not carry userinfo. Unless allowPrivate
// is set, IP-literal hosts and "localhost" that are not public are rejected.
// Hostnames are NOT resolved here; SafeDialContext performs that check at
// dial time so that the vetted IP is the one actually connected to.
func ValidateOutboundURL(raw string, allowPrivate bool) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, errors.New("URL scheme must be http or https")
	}
	if u.User != nil {
		return nil, errors.New("URL must not contain credentials")
	}
	host := u.Hostname()
	if host == "" {
		return nil, errors.New("URL must include a host")
	}
	if allowPrivate {
		return u, nil
	}
	lower := strings.ToLower(strings.TrimSuffix(host, "."))
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return nil, errors.New("URL host is not allowed: localhost")
	}
	// Strip IPv6 zone (fe80::1%eth0) before parsing.
	if i := strings.IndexByte(host, '%'); i >= 0 {
		host = host[:i]
	}
	if ip := net.ParseIP(host); ip != nil && !IsPublicIP(ip) {
		return nil, fmt.Errorf("URL host is not allowed: %s", host)
	}
	return u, nil
}

// SafeDialContext returns a DialContext function suitable for http.Transport.
// It resolves the host, rejects any resolved address that is not public
// (unless allowPrivate) and dials only the vetted IP addresses, so a DNS
// answer cannot change between validation and connect (DNS rebinding).
func SafeDialContext(allowPrivate bool) func(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if !allowPrivate {
			lower := strings.ToLower(strings.TrimSuffix(host, "."))
			if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
				return nil, fmt.Errorf("dial to %s is not allowed", host)
			}
		}
		ipAddrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		if len(ipAddrs) == 0 {
			return nil, fmt.Errorf("no addresses found for %s", host)
		}
		if !allowPrivate {
			for _, ia := range ipAddrs {
				if !IsPublicIP(ia.IP) {
					return nil, fmt.Errorf("dial to %s (%s) is not allowed: non-public address", host, ia.IP)
				}
			}
		}
		var lastErr error
		for _, ia := range ipAddrs {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ia.IP.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
			if ctx.Err() != nil {
				break
			}
		}
		return nil, lastErr
	}
}
