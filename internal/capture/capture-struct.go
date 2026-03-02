// Purpose: Defines the core Capture state container and its concurrency-protected ring-buffer subsystem layout.
// Why: Centralizes all in-memory telemetry state so ingestion/query paths share one coherent source of truth.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"sync"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/performance"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/redaction"
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
// 3. Memory totals that estimate buffer overhead (wsMemoryTotal, networkBodyMemoryTotal)
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
	networkBodyMemoryTotal int64         // Approximate memory: len(RequestBody)+len(ResponseBody)+300 bytes per entry. Updated incrementally on append/eviction.

	// ============================================
	// Enhanced Actions Buffer (Ring Buffer)
	// ============================================

	enhancedActions  []EnhancedAction // Ring buffer of browser actions. Parallel with actionAddedAt.
	actionAddedAt    []time.Time      // Parallel slice: insertion time for each enhancedActions[i].
	actionTotalAdded int64            // Monotonic counter: total actions ever added (never reset/decremented). Survives eviction.

	// ============================================
	// Timings and Performance Data
	// ============================================

	networkWaterfall NetworkWaterfallBuffer // Ring buffer of browser PerformanceResourceTiming data (configurable capacity, default 1000).
	extensionLogs    ExtensionLogBuffer     // Ring buffer of extension internal logs (max 500). FIFO eviction. No TTL filtering.

	// ============================================
	// WebSocket Connection Tracking
	// ============================================

	wsConnections WSConnectionTracker // Active + closed WS connections, LRU eviction order. Protected by parent mu (no separate lock).

	// ============================================
	// Query Dispatch (Own Locks)
	// ============================================

	queryDispatcher *QueryDispatcher // Pending queries, results, async command tracking — delegates to QueryDispatcher sub-struct (aliased from internal/queries). Has own sync.Mutex + sync.RWMutex — independent of Capture.mu.

	// ============================================
	// Rate Limiting & Circuit Breaker (Own Lock)
	// ============================================

	circuit *CircuitBreaker // Rate limiting + circuit breaker state machine — delegates to internal/circuit. Has own sync.RWMutex — independent of Capture.mu.

	// ============================================
	// Extension State (Protected by parent mu)
	// ============================================

	extensionState ExtensionState // Connection, pilot, tracking, test boundaries. Protected by parent mu (no separate lock).

	// ============================================
	// Debug Logging (Own Lock)
	// ============================================

	debug DebugLogger // Polling activity + HTTP debug circular buffers. Has own sync.Mutex — independent of Capture.mu.

	// Redaction engine for scrubbing sensitive values from extension debug logs.
	logRedactor *redaction.Engine

	// Recording Management — delegates to RecordingManager sub-struct (aliased from internal/recording).
	recordingManager *RecordingManager // Recording lifecycle, playback, and log-diff. Has own sync.Mutex — independent of Capture.mu.

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

	lifecycleCallback  func(event string, data map[string]any) // Optional callback for lifecycle events (circuit breaker, extension state, buffer overflow)
	navigationCallback func()                                  // Optional callback fired after a navigation action is ingested (called outside lock)

	// ============================================
	// Version Information
	// ============================================

	serverVersion string // Server version (e.g., "5.7.0"), set via SetServerVersion()
}

// NewCapture creates a fully initialized Capture with all subcomponents wired.
//
// Invariants:
// - queryDispatcher/circuit/debug/recordingManager are non-nil in returned instance.
// - extensionState.activeTestIDs and extensionState.missingInProgressByCorr start as initialized maps.
func NewCapture() *Capture {
	c := &Capture{
		wsEvents:        make([]WebSocketEvent, 0, MaxWSEvents),
		networkBodies:   make([]NetworkBody, 0, MaxNetworkBodies),
		enhancedActions: make([]EnhancedAction, 0, MaxEnhancedActions),
		networkWaterfall: NetworkWaterfallBuffer{
			entries:  make([]NetworkWaterfallEntry, 0, DefaultNetworkWaterfallCapacity),
			capacity: DefaultNetworkWaterfallCapacity,
		},
		extensionLogs: ExtensionLogBuffer{
			logs: make([]ExtensionLog, 0, MaxExtensionLogs),
		},
		wsConnections: WSConnectionTracker{
			connections: make(map[string]*connectionState),
			closedConns: make([]WebSocketClosedConnection, 0),
			connOrder:   make([]string, 0),
		},
		extensionState: ExtensionState{
			activeTestIDs:           make(map[string]bool),
			missingInProgressByCorr: make(map[string]int),
			pilotSource:             PilotSourceAssumedStartup,
			securityMode:            SecurityModeNormal,
		},
		perf: PerformanceStore{
			snapshots:       make(map[string]performance.Snapshot),
			snapshotOrder:   make([]string, 0),
			baselines:       make(map[string]performance.Baseline),
			baselineOrder:   make([]string, 0),
			beforeSnapshots: make(map[string]performance.Snapshot),
		},
		session: SessionTracker{
			FirstSnapshots: make(map[string]performance.Snapshot),
		},
		a11y: A11yCache{
			cache:      make(map[string]*a11yCacheEntry),
			cacheOrder: make([]string, 0),
			inflight:   make(map[string]*a11yInflightEntry),
		},
		debug:            NewDebugLogger(),
		recordingManager: NewRecordingManager(),

		logRedactor: redaction.NewRedactionEngine(""),
	}
	c.queryDispatcher = NewQueryDispatcher()
	c.circuit = NewCircuitBreaker(c.emitLifecycleEvent)

	// Note: clientRegistry is initialized by capture.New() in capture package
	// to avoid circular import (those packages import capture for NetworkBody, WebSocketEvent, etc.)
	return c
}
