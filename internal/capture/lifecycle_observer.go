// lifecycle_observer.go -- Re-exports lifecycle types for capture package backward compatibility.
// Why: Preserves capture package API while lifecycle logic lives in internal/lifecycle.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle"

// Type aliases for backward compatibility — all types now live in internal/lifecycle.
type (
	LifecycleEvent    = lifecycle.Event
	LifecycleListener = lifecycle.Listener
	LifecycleObserver = lifecycle.Observer
)

// Event constant aliases for backward compatibility.
const (
	EventUnknown                = lifecycle.EventUnknown
	EventCircuitOpened          = lifecycle.EventCircuitOpened
	EventCircuitClosed          = lifecycle.EventCircuitClosed
	EventExtensionConnected     = lifecycle.EventExtensionConnected
	EventExtensionDisconnected  = lifecycle.EventExtensionDisconnected
	EventBufferEviction         = lifecycle.EventBufferEviction
	EventRateLimitTriggered     = lifecycle.EventRateLimitTriggered
	EventCommandStateDesync     = lifecycle.EventCommandStateDesync
	EventSyncSnapshot           = lifecycle.EventSyncSnapshot
)

// NewLifecycleObserver re-exports lifecycle.NewObserver for backward compatibility.
var NewLifecycleObserver = lifecycle.NewObserver

// ParseLifecycleEvent re-exports lifecycle.ParseEvent for backward compatibility.
var ParseLifecycleEvent = lifecycle.ParseEvent
