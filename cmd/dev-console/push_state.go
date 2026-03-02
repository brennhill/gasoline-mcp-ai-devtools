// push_state.go — Shared bridge↔daemon state for push client capabilities.
package main

import (
	"encoding/json"
	"sync"

	"github.com/dev-console/dev-console/internal/bridge"
	"github.com/dev-console/dev-console/internal/push"
)

// pushState holds client capabilities and framing mode detected during MCP initialize.
// Bridge and daemon share the same Go binary, so package-level state is fine.
var pushState struct {
	mu       sync.RWMutex
	caps     push.ClientCapabilities
	framing  bridge.StdioFraming
	onChange func(push.ClientCapabilities)
}

// setPushClientCapabilities stores capabilities extracted from MCP initialize.
func setPushClientCapabilities(caps push.ClientCapabilities) {
	cb := func() func(push.ClientCapabilities) {
		pushState.mu.Lock()
		defer pushState.mu.Unlock()
		pushState.caps = caps
		return pushState.onChange
	}()

	if cb != nil {
		cb(caps)
	}
}

// getPushClientCapabilities returns the current client capabilities.
func getPushClientCapabilities() push.ClientCapabilities {
	pushState.mu.RLock()
	defer pushState.mu.RUnlock()
	return pushState.caps
}

// onPushCapabilitiesChange registers a callback for capability updates.
func onPushCapabilitiesChange(fn func(push.ClientCapabilities)) {
	pushState.mu.Lock()
	defer pushState.mu.Unlock()
	pushState.onChange = fn
}

// storeBridgeFraming saves the framing mode detected during MCP initialize.
func storeBridgeFraming(f bridge.StdioFraming) {
	pushState.mu.Lock()
	defer pushState.mu.Unlock()
	pushState.framing = f
}

// getBridgeFraming returns the stored framing mode.
func getBridgeFraming() bridge.StdioFraming {
	pushState.mu.RLock()
	defer pushState.mu.RUnlock()
	return pushState.framing
}

// extractClientCapabilities parses client capabilities from MCP initialize params.
func extractClientCapabilities(rawParams json.RawMessage) push.ClientCapabilities {
	if len(rawParams) == 0 {
		return push.ClientCapabilities{}
	}

	var params struct {
		Capabilities struct {
			Sampling json.RawMessage `json:"sampling"`
		} `json:"capabilities"`
		ClientInfo struct {
			Name string `json:"name"`
		} `json:"clientInfo"`
	}

	if err := json.Unmarshal(rawParams, &params); err != nil {
		return push.ClientCapabilities{}
	}

	caps := push.ClientCapabilities{
		ClientName: params.ClientInfo.Name,
	}

	// Sampling is supported if the field exists and is non-null
	if len(params.Capabilities.Sampling) > 0 && string(params.Capabilities.Sampling) != "null" {
		caps.SupportsSampling = true
	}

	// Notifications are generally supported by all MCP clients
	if caps.ClientName != "" {
		caps.SupportsNotifications = true
	}

	return caps
}
