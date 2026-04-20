// nonce_test.go — Tests for the per-process upgrade nonce.

package upgrade

import (
	"regexp"
	"testing"
)

const testOrigin = "chrome-extension://abcdefghijklmnopabcdefghijklmnop"
const otherOrigin = "chrome-extension://zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"

func TestNonce_CurrentIs64HexChars(t *testing.T) {
	t.Parallel()
	n := NewNonce()
	got := n.Current()
	if len(got) != 64 {
		t.Fatalf("Current() length = %d, want 64", len(got))
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(got) {
		t.Fatalf("Current() = %q, want 64 lowercase hex chars", got)
	}
}

func TestNonce_CurrentStableAcrossCalls(t *testing.T) {
	t.Parallel()
	n := NewNonce()
	first := n.Current()
	second := n.Current()
	if first != second {
		t.Fatalf("Current() changed between calls: %q vs %q", first, second)
	}
}

func TestNonce_DifferentInstancesDiffer(t *testing.T) {
	t.Parallel()
	a := NewNonce()
	b := NewNonce()
	if a.Current() == b.Current() {
		t.Fatalf("two fresh nonces should differ, both = %q", a.Current())
	}
}

func TestNonce_PinRecordsFirstOrigin(t *testing.T) {
	t.Parallel()
	n := NewNonce()
	n.Pin(testOrigin)
	if got := n.PinnedOrigin(); got != testOrigin {
		t.Fatalf("PinnedOrigin() = %q, want %q", got, testOrigin)
	}
}

func TestNonce_PinIsIdempotent(t *testing.T) {
	t.Parallel()
	n := NewNonce()
	n.Pin(testOrigin)
	n.Pin(otherOrigin) // must be ignored
	if got := n.PinnedOrigin(); got != testOrigin {
		t.Fatalf("PinnedOrigin() after second Pin = %q, want %q (first origin must win)", got, testOrigin)
	}
}

func TestNonce_PinIgnoresEmptyOrigin(t *testing.T) {
	t.Parallel()
	n := NewNonce()
	n.Pin("")
	if got := n.PinnedOrigin(); got != "" {
		t.Fatalf("PinnedOrigin() = %q after Pin(\"\"), want empty", got)
	}
	n.Pin(testOrigin)
	if got := n.PinnedOrigin(); got != testOrigin {
		t.Fatalf("PinnedOrigin() = %q after real Pin, want %q", got, testOrigin)
	}
}

func TestNonce_VerifyAcceptsMatchingOrigin(t *testing.T) {
	t.Parallel()
	n := NewNonce()
	n.Pin(testOrigin)
	if !n.Verify(n.Current(), testOrigin) {
		t.Fatal("Verify with correct token + matching origin should be true")
	}
}

func TestNonce_VerifyRejectsMismatchedOrigin(t *testing.T) {
	t.Parallel()
	n := NewNonce()
	n.Pin(testOrigin)
	if n.Verify(n.Current(), otherOrigin) {
		t.Fatal("Verify with correct token but different origin must be false")
	}
}

func TestNonce_VerifyRejectsUnpinnedNonce(t *testing.T) {
	t.Parallel()
	n := NewNonce()
	// Never called Pin — even the "correct" origin must be rejected.
	if n.Verify(n.Current(), testOrigin) {
		t.Fatal("Verify on a never-pinned nonce must be false")
	}
}

func TestNonce_VerifyRejectsWrongToken(t *testing.T) {
	t.Parallel()
	n := NewNonce()
	n.Pin(testOrigin)
	wrong := "0000000000000000000000000000000000000000000000000000000000000000"
	if n.Verify(wrong, testOrigin) {
		t.Fatal("Verify(zeros) should be false")
	}
}

func TestNonce_VerifyRejectsEmptyToken(t *testing.T) {
	t.Parallel()
	n := NewNonce()
	n.Pin(testOrigin)
	if n.Verify("", testOrigin) {
		t.Fatal("Verify(\"\") should be false")
	}
}

func TestNonce_VerifyRejectsWrongLength(t *testing.T) {
	t.Parallel()
	n := NewNonce()
	n.Pin(testOrigin)
	if n.Verify("abc", testOrigin) {
		t.Fatal("Verify(\"abc\") should be false — wrong length should reject before constant-time compare")
	}
}

func TestNonce_VerifyRejectsEmptyOriginEvenWhenPinned(t *testing.T) {
	t.Parallel()
	n := NewNonce()
	n.Pin(testOrigin)
	if n.Verify(n.Current(), "") {
		t.Fatal("Verify with empty origin must be false even when a valid origin is pinned")
	}
}
