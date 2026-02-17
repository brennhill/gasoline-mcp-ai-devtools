// Purpose: Owns capture-struct.go runtime behavior and integration logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

// capture-struct.go — Main Capture struct and factory function.
// Capture manages all buffered browser state: WebSocket events, network bodies,
// user actions, connections, queries, rate limiting, and performance.
package capture

import (
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/performance"
	"github.com/dev-console/dev-console/internal/redaction"
)

// Capture manages all buffered browser state: WebSocket events, network bodies,
// user actions, connections, queries, rate limiting, and performance.
//
// All fields are protected by mu (sync.RWMutex) unless noted otherwise.
// Lock hierarchy: Capture.mu is position 3 (after ClientRegistry, ClientState).
// Release locks before calling external callbacks. Use RLock() for read-only access.
// Sub-struct locks: a11y, perf, session, mem use parent mu.
//
// Ring buffers (wsEvents, networkBodies, enhancedActions) maintain three parallel invariants:
// 1. Parallel timestamp slices kept in perfect sync (wsAddedAt, networkAddedAt, actionAddedAt)
// 2. Monotonic counters that survive eviction (wsTotalAdded, networkTotalAdded, actionTotalAdded)
// 3. Memory totals that estimate buffer overhead (wsMemoryTotal, nbMemoryTotal)
//
// Rate limiting uses a sliding 1-second window with circuit breaker:
// windowEventCount resets per window. rateLimitStreak tracks consecutive seconds over threshold.
// Circuit opens after 5+ consecutive seconds or memory spike; closes after 10s below threshold + memory < 30MB.
// lastBelowThresholdAt tracks when rate dropped below threshold (initialized at startup to prevent false close).
type Capture struct {
	mu sync.RWMutex

	// TTL for read-time filtering (0 = unlimited, no filtering applied).
	// Applied during reads: events older than TTL are skipped.
	TTL time.Duration

	// ============================================
	// WebSocket Event Buffer (Ring Buffer)
	// ============================================

	wsEvents      []WebSocketEvent // Ring buffer of WS events (cap: MaxWSEvents). Kept in sync with wsAddedAt.
	wsAddedAt     []time.Time      // Parallel slice: insertion time for each wsEvents[i]. Used for TTL filtering and eviction order (oldest first).
	wsTotalAdded  int64            // Monotonic counter: total events ever added (never reset/decremented). Survives eviction. Used for cursor-based delta queries.
	wsMemoryTotal int64            // Approximate memory: sum of wsEventMemory(&wsEvents[i]). Estimate: len(Data)+200 bytes per event. Updated incrementally; recalc on critical eviction.

	// ============================================
	// Network Body Buffer (Ring Buffer)
	// ============================================

	networkBodies          []NetworkBody // Ring buffer of HTTP request/response bodies (cap: MaxNetworkBodies=100). Parallel with networkAddedAt.
	networkAddedAt         []time.Time   // Parallel slice: insertion time for each networkBodies[i]. Used for TTL filtering and LRU eviction.
	networkTotalAdded      int64         // Monotonic counter: total bodies ever added (never reset/decremented). Survives eviction. Used for cursor-based delta queries.
	networkErrorTotalAdded int64         // Monotonic counter: total HTTP error responses (status>=400) ever added. Survives eviction.
	nbMemoryTotal          int64         // Approximate memory: len(RequestBody)+len(ResponseBody)+300 bytes per entry. Updated incrementally on append/eviction.

	// ============================================
	// Enhanced Actions Buffer (Ring Buffer)
	// ============================================

	enhancedActions  []EnhancedAction // Ring buffer of browser actions. Parallel with actionAddedAt.
	actionAddedAt    []time.Time      // Parallel slice: insertion time for each enhancedActions[i].
	actionTotalAdded int64            // Monotonic counter: total actions ever added (never reset/decremented). Survives eviction.

	// ============================================
	// Timings and Performance Data
	// ============================================

	nw  NetworkWaterfallBuffer // Ring buffer of browser PerformanceResourceTiming data (configurable capacity, default 1000).
	elb ExtensionLogBuffer     // Ring buffer of extension internal logs (max 500). FIFO eviction. No TTL filtering.

	// ============================================
	// WebSocket Connection Tracking
	// ============================================

	ws WSConnectionTracker // Active + closed WS connections, LRU eviction order. Protected by parent mu (no separate lock).

	// ============================================
	// Query Dispatch (Own Locks)
	// ============================================

	qd *QueryDispatcher // Pending queries, results, async command tracking. Has own sync.Mutex + sync.RWMutex — independent of Capture.mu.

	// ============================================
	// Rate Limiting & Circuit Breaker (Own Lock)
	// ============================================

	circuit *CircuitBreaker // Rate limiting + circuit breaker state machine. Has own sync.RWMutex — independent of Capture.mu.

	// ============================================
	// Extension State (Protected by parent mu)
	// ============================================

	ext ExtensionState // Connection, pilot, tracking, test boundaries. Protected by parent mu (no separate lock).

	// ============================================
	// Debug Logging (Own Lock)
	// ============================================

	debug DebugLogger // Polling activity + HTTP debug circular buffers. Has own sync.Mutex — independent of Capture.mu.

	// Redaction engine for scrubbing sensitive values from extension debug logs.
	logRedactor *redaction.RedactionEngine

	// Recording Management — delegates to RecordingManager sub-struct.
	rec *RecordingManager // Recording lifecycle, playback, and log-diff. Has own sync.Mutex — independent of Capture.mu.

	// ============================================
	// Composed Sub-Structures
	// ============================================

	a11y    A11yCache        // Accessibility audit cache. Protected by parent mu (no separate lock). Accessed via getA11yCacheEntry/setA11yCacheEntry.
	perf    PerformanceStore // Performance snapshots and baselines. Protected by parent mu (no separate lock).
	session SessionTracker   // Session-level performance aggregation. Protected by parent mu (no separate lock).

	// ============================================
	// Multi-Client Support
	// ============================================

	clientRegistry ClientRegistry // Registry of connected MCP clients. HAS OWN LOCK. Lock hierarchy: ClientRegistry.mu is position 1 (outermost), before Capture.mu.

	// ============================================
	// Lifecycle Event Callbacks
	// ============================================

	lifecycleCallback func(event string, data map[string]any) // Optional callback for lifecycle events (circuit breaker, extension state, buffer overflow)

	// ============================================
	// Version Information
	// ============================================

	serverVersion string // Server version (e.g., "5.7.0"), set via SetServerVersion()
}

// NewCapture creates a new Capture instance with initialized buffers
func NewCapture() *Capture {
	c := &Capture{
		wsEvents:        make([]WebSocketEvent, 0, MaxWSEvents),
		networkBodies:   make([]NetworkBody, 0, MaxNetworkBodies),
		enhancedActions: make([]EnhancedAction, 0, MaxEnhancedActions),
		nw: NetworkWaterfallBuffer{
			entries:  make([]NetworkWaterfallEntry, 0, DefaultNetworkWaterfallCapacity),
			capacity: DefaultNetworkWaterfallCapacity,
		},
		elb: ExtensionLogBuffer{
			logs: make([]ExtensionLog, 0, MaxExtensionLogs),
		},
		ws: WSConnectionTracker{
			connections: make(map[string]*connectionState),
			closedConns: make([]WebSocketClosedConnection, 0),
			connOrder:   make([]string, 0),
		},
		ext: ExtensionState{
			activeTestIDs: make(map[string]bool),
		},
		perf: PerformanceStore{
			snapshots:       make(map[string]performance.PerformanceSnapshot),
			snapshotOrder:   make([]string, 0),
			baselines:       make(map[string]performance.PerformanceBaseline),
			baselineOrder:   make([]string, 0),
			beforeSnapshots: make(map[string]performance.PerformanceSnapshot),
		},
		session: SessionTracker{
			FirstSnapshots: make(map[string]performance.PerformanceSnapshot),
		},
		a11y: A11yCache{
			cache:      make(map[string]*a11yCacheEntry),
			cacheOrder: make([]string, 0),
			inflight:   make(map[string]*a11yInflightEntry),
		},
		debug: NewDebugLogger(),
		rec:   NewRecordingManager(),

		logRedactor: redaction.NewRedactionEngine(""),
	}
	c.qd = NewQueryDispatcher()
	c.circuit = NewCircuitBreaker(c.emitLifecycleEvent)

	// Note: clientRegistry is initialized by capture.New() in capture package
	// to avoid circular import (those packages import capture for NetworkBody, WebSocketEvent, etc.)
	return c
}

// Close stops background goroutines. Safe to call multiple times.
func (c *Capture) Close() {
	if c.qd != nil {
		c.qd.Close()
	}
}

// SetLifecycleCallback sets a callback function for lifecycle events.
// The callback receives an event name and data map with event-specific fields.
// Events: "circuit_opened", "circuit_closed", "extension_connected", "extension_disconnected",
// "buffer_eviction", "rate_limit_triggered"
func (c *Capture) SetLifecycleCallback(cb func(event string, data map[string]any)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lifecycleCallback = cb
}

// emitLifecycleEvent calls the lifecycle callback if set.
// Caller must NOT hold lock (callback may do I/O).
func (c *Capture) emitLifecycleEvent(event string, data map[string]any) {
	c.mu.RLock()
	cb := c.lifecycleCallback
	c.mu.RUnlock()
	if cb != nil {
		cb(event, data)
	}
}

// SetServerVersion sets server version for compatibility checking.
// Called once at startup with version from main.go.
func (c *Capture) SetServerVersion(v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.serverVersion = v
}

// GetServerVersion returns server version.
func (c *Capture) GetServerVersion() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverVersion
}
