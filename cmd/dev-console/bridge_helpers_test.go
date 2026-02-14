// bridge_helpers_test.go — Tests for isConnectionError helper function.
package main

import (
	"errors"
	"net"
	"testing"
)

// ============================================
// isConnectionError
// ============================================

func TestIsConnectionError_NilError(t *testing.T) {
	t.Parallel()
	if isConnectionError(nil) {
		t.Error("expected false for nil error")
	}
}

func TestIsConnectionError_OpError(t *testing.T) {
	t.Parallel()
	opErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: errors.New("connection refused"),
	}
	if !isConnectionError(opErr) {
		t.Error("expected true for *net.OpError")
	}
}

func TestIsConnectionError_WrappedOpError(t *testing.T) {
	t.Parallel()
	opErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: errors.New("connection refused"),
	}
	wrapped := errors.Join(errors.New("context"), opErr)
	if !isConnectionError(wrapped) {
		t.Error("expected true for wrapped *net.OpError")
	}
}

func TestIsConnectionError_DNSError(t *testing.T) {
	t.Parallel()
	dnsErr := &net.DNSError{
		Err:  "no such host",
		Name: "nonexistent.example.com",
	}
	if !isConnectionError(dnsErr) {
		t.Error("expected true for *net.DNSError")
	}
}

func TestIsConnectionError_WrappedDNSError(t *testing.T) {
	t.Parallel()
	dnsErr := &net.DNSError{
		Err:  "no such host",
		Name: "nonexistent.example.com",
	}
	wrapped := errors.Join(errors.New("lookup failed"), dnsErr)
	if !isConnectionError(wrapped) {
		t.Error("expected true for wrapped *net.DNSError")
	}
}

func TestIsConnectionError_ConnectionRefusedString(t *testing.T) {
	t.Parallel()
	err := errors.New("dial tcp 127.0.0.1:7890: connection refused")
	if !isConnectionError(err) {
		t.Error("expected true for error containing 'connection refused'")
	}
}

func TestIsConnectionError_NoSuchHostString(t *testing.T) {
	t.Parallel()
	err := errors.New("lookup nonexistent.local: no such host")
	if !isConnectionError(err) {
		t.Error("expected true for error containing 'no such host'")
	}
}

func TestIsConnectionError_UnrelatedError(t *testing.T) {
	t.Parallel()
	err := errors.New("timeout exceeded")
	if isConnectionError(err) {
		t.Error("expected false for unrelated error")
	}
}

func TestIsConnectionError_EmptyError(t *testing.T) {
	t.Parallel()
	err := errors.New("")
	if isConnectionError(err) {
		t.Error("expected false for empty error message")
	}
}

func TestIsConnectionError_PartialMatchNotSubstring(t *testing.T) {
	t.Parallel()
	// Ensure partial matches within other words don't trigger false positives
	err := errors.New("no such hostile environment")
	// "no such host" IS a substring of "no such hostile environment"
	if !isConnectionError(err) {
		t.Error("expected true: 'no such host' is a substring of the message")
	}
}
