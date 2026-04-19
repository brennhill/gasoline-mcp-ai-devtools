// nonce_test.go — Tests for the per-process upgrade nonce.

package upgrade

import (
	"regexp"
	"testing"
)

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

func TestNonce_VerifyAcceptsCurrent(t *testing.T) {
	t.Parallel()
	n := NewNonce()
	if !n.Verify(n.Current()) {
		t.Fatal("Verify(Current()) should be true")
	}
}

func TestNonce_VerifyRejectsWrong(t *testing.T) {
	t.Parallel()
	n := NewNonce()
	wrong := "0000000000000000000000000000000000000000000000000000000000000000"
	if n.Verify(wrong) {
		t.Fatal("Verify(zeros) should be false")
	}
}

func TestNonce_VerifyRejectsEmpty(t *testing.T) {
	t.Parallel()
	n := NewNonce()
	if n.Verify("") {
		t.Fatal("Verify(\"\") should be false")
	}
}

func TestNonce_VerifyRejectsWrongLength(t *testing.T) {
	t.Parallel()
	n := NewNonce()
	if n.Verify("abc") {
		t.Fatal("Verify(\"abc\") should be false — wrong length should reject before constant-time compare")
	}
}
