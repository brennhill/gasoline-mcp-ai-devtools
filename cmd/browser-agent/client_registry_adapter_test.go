// client_registry_adapter_test.go — Guards against typed-nil leaks through
// the any-returning ClientRegistry adapter methods.

package main

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session"
)

// TestSessionClientRegistryAdapter_GetReturnsUntypedNilForMissingClient pins
// the fix for the bug where GET /clients/{id} on an unknown ID returned
// `200 null` instead of `404 {"error":"Client not found"}`. Cause: the adapter
// returned `a.reg.Get(id)` directly — that's a `*session.ClientState`, which
// when nil gets wrapped by the `any` interface and becomes a typed-nil pointer.
// Callers that check `if cs == nil` missed it because `interface{typedNil}`
// != untyped nil. Keep this test green or the handler silently regresses.
func TestSessionClientRegistryAdapter_GetReturnsUntypedNilForMissingClient(t *testing.T) {
	t.Parallel()
	reg := session.NewClientRegistry()
	adapter := newSessionClientRegistryAdapter(reg)

	got := adapter.Get("does-not-exist")
	if got != nil {
		t.Fatalf("expected untyped nil for missing client, got %#v", got)
	}
}

func TestSessionClientRegistryAdapter_GetReturnsRealValueForExistingClient(t *testing.T) {
	t.Parallel()
	reg := session.NewClientRegistry()
	registered := reg.Register("/tmp/project")
	if registered == nil {
		t.Fatal("Register returned nil")
	}

	adapter := newSessionClientRegistryAdapter(reg)
	got := adapter.Get(registered.ID)
	if got == nil {
		t.Fatalf("expected non-nil ClientState for existing client, got nil")
	}
}
