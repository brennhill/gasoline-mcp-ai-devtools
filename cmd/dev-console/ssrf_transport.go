// ssrf_transport.go — shared SSRF-safe dialer/transport helpers.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

const ssrfLookupTimeout = 5 * time.Second

// resolvePublicIP resolves host and returns the first non-private IP.
func resolvePublicIP(ctx context.Context, host string) (net.IP, error) {
	normalized := strings.TrimSpace(host)
	if normalized == "" {
		return nil, fmt.Errorf("empty hostname")
	}

	// Strip optional IPv6 zone suffix to keep ParseIP deterministic.
	if idx := strings.IndexByte(normalized, '%'); idx != -1 {
		normalized = normalized[:idx]
	}

	if ip := net.ParseIP(normalized); ip != nil {
		if isPrivateIP(ip) {
			return nil, fmt.Errorf("host %q is private IP %s", host, ip.String())
		}
		return ip, nil
	}

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, normalized)
	if err != nil {
		return nil, fmt.Errorf("DNS lookup failed for %q: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("DNS lookup returned no addresses for %q", host)
	}

	for _, ipAddr := range ips {
		if ipAddr.IP == nil {
			continue
		}
		if !isPrivateIP(ipAddr.IP) {
			return ipAddr.IP, nil
		}
	}

	return nil, fmt.Errorf("hostname %q resolves only to private IP addresses", host)
}

// ssrfSafeDialContext validates destination address and dials a pinned public IP.
func ssrfSafeDialContext(ctx context.Context, network, addr string, allowPrivate bool) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("ssrf_blocked: invalid address %s", addr)
	}

	// Check --ssrf-allow-host flag (test use: allows localhost test servers)
	if isSSRFAllowedHost(addr) || isSSRFAllowedHost(host) {
		allowPrivate = true
	}

	// Test hook for httptest servers on loopback.
	if allowPrivate {
		var d net.Dialer
		return d.DialContext(ctx, network, net.JoinHostPort(host, port))
	}

	lookupCtx, cancel := context.WithTimeout(ctx, ssrfLookupTimeout)
	defer cancel()

	ip, err := resolvePublicIP(lookupCtx, host)
	if err != nil {
		return nil, fmt.Errorf("ssrf_blocked: %w", err)
	}

	var d net.Dialer
	return d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
}

// newSSRFSafeTransport returns an HTTP transport that blocks private/internal targets
// and pins DNS resolution to the dialed IP per connection.
func newSSRFSafeTransport(allowPrivateFn func() bool) *http.Transport {
	transport := (&http.Transport{}).Clone()
	if base, ok := http.DefaultTransport.(*http.Transport); ok && base != nil {
		transport = base.Clone()
	}
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		allowPrivate := false
		if allowPrivateFn != nil {
			allowPrivate = allowPrivateFn()
		}
		return ssrfSafeDialContext(ctx, network, addr, allowPrivate)
	}
	return transport
}
