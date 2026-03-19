// navigate_test.go — Tests for auto-navigate after dev server detection.

package scaffold

import (
	"context"
	"fmt"
	"testing"
)

// ============================================
// Auto-Navigate
// ============================================

func TestAutoNavigate_CallsNavigateWithCorrectURL(t *testing.T) {
	var navigatedURL string
	navigateFn := func(ctx context.Context, url string) error {
		navigatedURL = url
		return nil
	}

	err := AutoNavigate(context.Background(), 5173, navigateFn)
	if err != nil {
		t.Fatalf("AutoNavigate: %v", err)
	}

	expected := "http://localhost:5173"
	if navigatedURL != expected {
		t.Errorf("navigated to %q, want %q", navigatedURL, expected)
	}
}

func TestAutoNavigate_PropagatesError(t *testing.T) {
	navigateFn := func(ctx context.Context, url string) error {
		return fmt.Errorf("navigation failed")
	}

	err := AutoNavigate(context.Background(), 5173, navigateFn)
	if err == nil {
		t.Error("AutoNavigate should propagate navigate error")
	}
}

func TestAutoNavigate_RespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	navigateFn := func(ctx context.Context, url string) error {
		return ctx.Err()
	}

	err := AutoNavigate(ctx, 5173, navigateFn)
	if err == nil {
		t.Error("AutoNavigate should fail with cancelled context")
	}
}

func TestAutoNavigate_CustomPort(t *testing.T) {
	var navigatedURL string
	navigateFn := func(ctx context.Context, url string) error {
		navigatedURL = url
		return nil
	}

	err := AutoNavigate(context.Background(), 3000, navigateFn)
	if err != nil {
		t.Fatalf("AutoNavigate: %v", err)
	}

	expected := "http://localhost:3000"
	if navigatedURL != expected {
		t.Errorf("navigated to %q, want %q", navigatedURL, expected)
	}
}
