// readiness_gate_test.go — Tests for cold-start readiness gate (WaitForExtensionConnected).
// Why: Validates that commands hold for up to ColdStartTimeout instead of failing instantly.
// Docs: docs/features/feature/cold-start-queuing/index.md

package capture

import (
	"context"
	"testing"
	"time"
)

func TestWaitForExtensionConnected_NeverConnects(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	// Extension never connects — should timeout

	timeout := 200 * time.Millisecond
	start := time.Now()
	ok := c.WaitForExtensionConnected(context.Background(), timeout)
	elapsed := time.Since(start)

	if ok {
		t.Fatal("expected WaitForExtensionConnected to return false when extension never connects")
	}
	if elapsed < timeout-20*time.Millisecond {
		t.Fatalf("expected to wait at least %v, only waited %v", timeout, elapsed)
	}
}

func TestWaitForExtensionConnected_ConnectsPartway(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Simulate connection after 150ms
	go func() {
		time.Sleep(150 * time.Millisecond)
		c.SimulateExtensionConnectForTest()
	}()

	start := time.Now()
	ok := c.WaitForExtensionConnected(context.Background(), 2*time.Second)
	elapsed := time.Since(start)

	if !ok {
		t.Fatal("expected WaitForExtensionConnected to return true after mid-wait connection")
	}
	// Should have detected connection between 150ms and 350ms (150ms + poll interval slack)
	if elapsed < 100*time.Millisecond {
		t.Fatalf("detected connection too fast (%v), connection fires at 150ms", elapsed)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("took too long to detect connection (%v), expected within 500ms", elapsed)
	}
}

func TestWaitForExtensionConnected_ZeroTimeout(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Zero timeout should behave like a single check
	ok := c.WaitForExtensionConnected(context.Background(), 0)
	if ok {
		t.Fatal("expected false with zero timeout and no connection")
	}
}

func TestWaitForExtensionConnected_ZeroTimeout_AlreadyConnected(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	c.SimulateExtensionConnectForTest()

	ok := c.WaitForExtensionConnected(context.Background(), 0)
	if !ok {
		t.Fatal("expected true with zero timeout when already connected")
	}
}
