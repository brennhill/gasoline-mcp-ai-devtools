// ssrf_bypass_test.go — Tests for SSRF bypass resistance.

package main

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func TestSSRFBypassDecimalIP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 2130706433 is 127.0.0.1 in decimal
	// Go's net.ParseIP does NOT parse decimal IPs
	// DNS lookup should fail (or if it succeeds, should be blocked as private)
	_, err := resolvePublicIP(ctx, "2130706433")
	if err == nil {
		t.Error("resolvePublicIP() expected error for decimal IP representation, got nil")
	}
}

func TestSSRFBypassHexIP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 0x7f000001 is 127.0.0.1 in hex
	// Go's net.ParseIP does NOT parse hex IPs
	_, err := resolvePublicIP(ctx, "0x7f000001")
	if err == nil {
		t.Error("resolvePublicIP() expected error for hex IP representation, got nil")
	}
}

func TestSSRFBypassOctalIP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 0177.0.0.1 is 127.0.0.1 in octal.
	// Go's net.ParseIP does NOT parse octal IPs, so it falls to DNS.
	// Behaviour is system-dependent: some C resolvers interpret octal (→ 127.0.0.1,
	// which isPrivateIP catches), others fail DNS lookup entirely, and some may
	// resolve to an unrelated address. Accept any error; only fail if it silently
	// returns a private IP without error.
	ip, err := resolvePublicIP(ctx, "0177.0.0.1")
	if err == nil && ip != nil && isPrivateIP(ip) {
		t.Errorf("resolvePublicIP() returned private IP %s without error for octal representation", ip)
	}
}

func TestSSRFBypassIPv6MappedIPv4(t *testing.T) {
	tests := []struct {
		name string
		host string
	}{
		{
			name: "IPv6-mapped loopback",
			host: "::ffff:127.0.0.1",
		},
		{
			name: "IPv6-mapped private 10.x",
			host: "::ffff:10.0.0.1",
		},
		{
			name: "IPv6-mapped private 192.168.x",
			host: "::ffff:192.168.1.1",
		},
		{
			name: "IPv6-mapped private 172.16.x",
			host: "::ffff:172.16.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			// net.ParseIP DOES parse IPv6-mapped IPv4
			// Should be detected as private
			_, err := resolvePublicIP(ctx, tt.host)
			if err == nil {
				t.Errorf("resolvePublicIP() expected error for IPv6-mapped private IP %q, got nil", tt.host)
			}

			if err != nil && !strings.Contains(err.Error(), "private") {
				t.Errorf("resolvePublicIP() error should mention 'private', got: %v", err)
			}
		})
	}
}

func TestSSRFBypassCredentialsInURL(t *testing.T) {
	// Save and restore skipSSRFCheck
	originalSkip := skipSSRFCheck
	skipSSRFCheck = false
	defer func() { skipSSRFCheck = originalSkip }()

	tests := []struct {
		name string
		url  string
	}{
		{
			name: "credentials with loopback",
			url:  "http://admin:password@127.0.0.1:80/",
		},
		{
			name: "credentials with private IP",
			url:  "http://user:pass@192.168.1.1/",
		},
		{
			name: "credentials with localhost",
			url:  "http://admin:password@localhost:8080/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFormActionURL(tt.url)
			if err == nil {
				t.Errorf("validateFormActionURL() expected error for URL with credentials and private IP %q, got nil", tt.url)
			}

			// Should be blocked due to private IP, not credentials
			if err != nil && !strings.Contains(err.Error(), "private") && !strings.Contains(err.Error(), "localhost") {
				t.Logf("validateFormActionURL() error for %q: %v", tt.url, err)
			}
		})
	}
}

func TestSSRFBypassDNSRebinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DNS-dependent test in short mode")
	}

	tests := []struct {
		name string
		host string
	}{
		{
			name: "nip.io loopback",
			host: "127.0.0.1.nip.io",
		},
		{
			name: "nip.io private 10.x",
			host: "10.0.0.1.nip.io",
		},
		{
			name: "nip.io private 192.168.x",
			host: "192.168.1.1.nip.io",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			// DNS may fail (network issue) or succeed
			// If it succeeds and resolves to private IP, should be blocked
			_, err := resolvePublicIP(ctx, tt.host)

			// We expect an error either way:
			// - DNS failure: acceptable
			// - DNS success → private IP detection: required
			if err == nil {
				t.Errorf("resolvePublicIP() expected error for DNS rebinding host %q, got nil", tt.host)
			}

			// If error mentions "private", it's working correctly
			// If error is DNS-related, that's also fine
			if err != nil {
				errStr := err.Error()
				if !strings.Contains(errStr, "private") && !strings.Contains(errStr, "no such host") &&
					!strings.Contains(errStr, "lookup") && !strings.Contains(errStr, "timeout") {
					t.Logf("resolvePublicIP() for %q returned: %v", tt.host, err)
				}
			}
		})
	}
}

func TestSSRFBypassIPv4MappedShortForms(t *testing.T) {
	tests := []struct {
		name string
		host string
	}{
		{
			name: "IPv6-mapped short form loopback",
			host: "::ffff:7f00:1",
		},
		{
			name: "IPv6-mapped short form private",
			host: "::ffff:c0a8:101", // 192.168.1.1
		},
		{
			name: "IPv6-mapped short form 10.x",
			host: "::ffff:a00:1", // 10.0.0.1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			_, err := resolvePublicIP(ctx, tt.host)
			if err == nil {
				t.Errorf("resolvePublicIP() expected error for IPv6-mapped short form %q, got nil", tt.host)
			}

			// Check if the IP is correctly identified as private
			// Parse the IP first to verify Go handles it
			if ip := net.ParseIP(tt.host); ip != nil {
				if !isPrivateIP(ip) {
					t.Errorf("isPrivateIP() failed to identify %q as private", tt.host)
				}
			}
		})
	}
}

func TestIsPrivateIPComprehensive(t *testing.T) {
	tests := []struct {
		name      string
		ip        string
		wantPriv  bool
	}{
		// IPv4 private ranges
		{name: "loopback 127.0.0.1", ip: "127.0.0.1", wantPriv: true},
		{name: "loopback 127.255.255.255", ip: "127.255.255.255", wantPriv: true},
		{name: "private 10.0.0.0", ip: "10.0.0.0", wantPriv: true},
		{name: "private 10.255.255.255", ip: "10.255.255.255", wantPriv: true},
		{name: "private 172.16.0.0", ip: "172.16.0.0", wantPriv: true},
		{name: "private 172.31.255.255", ip: "172.31.255.255", wantPriv: true},
		{name: "private 192.168.0.0", ip: "192.168.0.0", wantPriv: true},
		{name: "private 192.168.255.255", ip: "192.168.255.255", wantPriv: true},
		{name: "link-local 169.254.0.0", ip: "169.254.0.0", wantPriv: true},
		{name: "link-local 169.254.255.255", ip: "169.254.255.255", wantPriv: true},

		// IPv4 public (should be false)
		{name: "public 8.8.8.8", ip: "8.8.8.8", wantPriv: false},
		{name: "public 1.1.1.1", ip: "1.1.1.1", wantPriv: false},

		// IPv6 loopback
		{name: "IPv6 loopback", ip: "::1", wantPriv: true},

		// IPv6 private (ULA)
		{name: "IPv6 ULA fc00::", ip: "fc00::", wantPriv: true},
		{name: "IPv6 ULA fdff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", ip: "fdff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", wantPriv: true},

		// IPv6 link-local
		{name: "IPv6 link-local", ip: "fe80::1", wantPriv: true},

		// IPv6-mapped IPv4
		{name: "IPv6-mapped 127.0.0.1", ip: "::ffff:127.0.0.1", wantPriv: true},
		{name: "IPv6-mapped 10.0.0.1", ip: "::ffff:10.0.0.1", wantPriv: true},
		{name: "IPv6-mapped 192.168.1.1", ip: "::ffff:192.168.1.1", wantPriv: true},
		{name: "IPv6-mapped public 8.8.8.8", ip: "::ffff:8.8.8.8", wantPriv: false},

		// IPv6 public (should be false)
		{name: "IPv6 public 2001:4860:4860::8888", ip: "2001:4860:4860::8888", wantPriv: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("net.ParseIP(%q) failed", tt.ip)
			}

			got := isPrivateIP(ip)
			if got != tt.wantPriv {
				t.Errorf("isPrivateIP(%q) = %v, want %v", tt.ip, got, tt.wantPriv)
			}
		})
	}
}

func TestResolvePublicIPTimeout(t *testing.T) {
	// Test that context timeout is respected
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Use a host that will timeout
	_, err := resolvePublicIP(ctx, "this-should-timeout.invalid")
	if err == nil {
		t.Error("resolvePublicIP() expected timeout error, got nil")
	}

	// Error should be context-related
	if err != nil && !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "timeout") {
		t.Logf("resolvePublicIP() timeout error: %v", err)
	}
}
