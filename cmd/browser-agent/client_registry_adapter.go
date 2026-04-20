// Purpose: Adapts session.ClientRegistry to the capture.ClientRegistry interface without importing session into capture.
// Why: Preserves package boundaries while allowing daemon bootstrap to inject a concrete registry implementation.
// Docs: docs/features/feature/request-session-correlation/index.md

package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session"
)

// sessionClientRegistryAdapter bridges session.ClientRegistry to capture.ClientRegistry.
// Return values are widened to any by interface contract; concrete values remain
// *session.ClientState and []session.ClientInfo.
type sessionClientRegistryAdapter struct {
	reg *session.ClientRegistry
}

func newSessionClientRegistryAdapter(reg *session.ClientRegistry) capture.ClientRegistry {
	if reg == nil {
		return nil
	}
	return &sessionClientRegistryAdapter{reg: reg}
}

func (a *sessionClientRegistryAdapter) Count() int {
	return a.reg.Count()
}

func (a *sessionClientRegistryAdapter) List() any {
	return a.reg.List()
}

func (a *sessionClientRegistryAdapter) Register(cwd string) any {
	return a.reg.Register(cwd)
}

func (a *sessionClientRegistryAdapter) Get(id string) any {
	// Return untyped nil (not a typed-nil *ClientState wrapped in an interface)
	// so callers can use `v == nil` to detect "not found" instead of reflecting
	// into the interface.
	cs := a.reg.Get(id)
	if cs == nil {
		return nil
	}
	return cs
}

func (a *sessionClientRegistryAdapter) Unregister(id string) bool {
	return a.reg.Unregister(id)
}
