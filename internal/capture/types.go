// types.go — Core capture types and the Capture struct.
// WebSocket events, network bodies, user actions, and the main Capture buffer.
// Design: Capture-specific types remain here; domain types moved to their packages.
package capture

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/performance"
	"github.com/dev-console/dev-console/internal/queries"
	"github.com/dev-console/dev-console/internal/recording"
	"github.com/dev-console/dev-console/internal/types"
)

// ============================================
// Abstracted Component Interfaces
// ============================================

// SchemaStore defines the interface for API schema detection and tracking.
// Implemented by *analysis.SchemaStore. Methods called by HTTP handlers and observers.
// Has its own lock; safe to call outside Capture.mu.
type SchemaStore interface {
	// EndpointCount returns the number of unique endpoints observed
	EndpointCount() int
}

// CSPGenerator defines the interface for Content-Security-Policy generation.
// Implemented by *security.CSPGenerator. Called by HTTP handlers.
// Has its own lock; safe to call outside Capture.mu.
type CSPGenerator interface {
	// GenerateCSP produces a CSP policy from observed origins (stub or full).
	// Signature matches security.CSPGenerator.GenerateCSP(params any) any
	// For type safety in capture, callers will use type assertions.
}

// ClientRegistry defines the interface for managing connected MCP clients.
// Implemented by *session.ClientRegistry. Called by HTTP handlers.
// Lock hierarchy: ClientRegistry.mu is position 1 (outermost), before Capture.mu.
type ClientRegistry interface {
	// Count returns the number of registered clients
	Count() int
	// List returns all registered clients (returns []session.ClientInfo)
	List() any
	// Register creates a new client registration (returns *session.ClientState)
	Register(cwd string) any
	// Get returns a specific client by ID (returns *session.ClientState)
	Get(id string) any
}

// Type aliases for imported packages to avoid qualifying every use.
// These are real type aliases (= syntax), not any forward declarations.
type (
	PerformanceSnapshot   = performance.PerformanceSnapshot   // Alias for convenience (avoid qualifying as performance.PerformanceSnapshot everywhere)
	PerformanceBaseline   = performance.PerformanceBaseline   // Alias for convenience
	PerformanceRegression = performance.PerformanceRegression // Alias for convenience
	ResourceEntry         = performance.ResourceEntry         // Alias for convenience
	ResourceDiff          = performance.ResourceDiff          // Alias for convenience
	CausalDiffResult      = performance.CausalDiffResult      // Alias for convenience
	Recording             = recording.Recording               // Alias for convenience (avoid qualifying as recording.Recording everywhere)
	RecordingAction       = recording.RecordingAction         // Alias for convenience
	PendingQueryResponse  = queries.PendingQueryResponse      // Alias for convenience (avoid qualifying as queries.PendingQueryResponse everywhere)
	PendingQuery          = queries.PendingQuery              // Alias for convenience
	CommandResult         = queries.CommandResult             // Alias for convenience (avoid qualifying as queries.CommandResult everywhere)
)

// ============================================
// Session Tracking Types
// ============================================

// SessionTracker records the first and last performance snapshots for delta computation.
// Local to capture package; tracks per-URL first snapshot for session-level regression detection.
type SessionTracker struct {
	FirstSnapshots map[string]performance.PerformanceSnapshot // first snapshot per URL
	SnapshotCount  int                                        // total snapshots added this session
}

// ============================================
// Security Threat Flagging
// ============================================

// SecurityFlag represents a detected security issue detected from network waterfall analysis.
type SecurityFlag struct {
	Type      string    `json:"type"`      // "suspicious_tld", "non_standard_port", etc.
	Severity  string    `json:"severity"`  // "low", "medium", "high", "critical"
	Origin    string    `json:"origin"`    // The flagged origin
	Message   string    `json:"message"`   // Human-readable explanation
	Resource  string    `json:"resource"`  // Specific resource URL (optional)
	PageURL   string    `json:"page_url"`  // Page that loaded this resource
	Timestamp time.Time `json:"timestamp"` // When flagged
}

// ============================================
// Network Waterfall Types
// ============================================

// NetworkWaterfallEntry represents a single network resource timing entry
// from the browser's PerformanceResourceTiming API
type NetworkWaterfallEntry struct {
	Name            string    `json:"name"`                         // Full URL
	URL             string    `json:"url"`                          // Same as name
	InitiatorType   string    `json:"initiator_type"`                // snake_case (from browser PerformanceResourceTiming)
	Duration        float64   `json:"duration"`                     // snake_case (from browser PerformanceResourceTiming)
	StartTime       float64   `json:"start_time"`                    // snake_case (from browser PerformanceResourceTiming)
	FetchStart      float64   `json:"fetch_start"`                   // snake_case (from browser PerformanceResourceTiming)
	ResponseEnd     float64   `json:"response_end"`                  // snake_case (from browser PerformanceResourceTiming)
	TransferSize    int       `json:"transfer_size"`                 // snake_case (from browser PerformanceResourceTiming)
	DecodedBodySize int       `json:"decoded_body_size"`              // snake_case (from browser PerformanceResourceTiming)
	EncodedBodySize int       `json:"encoded_body_size"`              // snake_case (from browser PerformanceResourceTiming)
	PageURL         string    `json:"page_url,omitempty"`
	Timestamp       time.Time `json:"timestamp,omitempty"`          // Server-side timestamp
}

// NetworkWaterfallPayload is POSTed by the extension
type NetworkWaterfallPayload struct {
	Entries []NetworkWaterfallEntry `json:"entries"`
	PageURL string                  `json:"page_url"`
}

// ============================================
// WebSocket Types
// ============================================

// WebSocketEvent represents a captured WebSocket event
type WebSocketEvent struct {
	Timestamp        string        `json:"ts,omitempty"`
	Type             string        `json:"type,omitempty"`
	Event            string        `json:"event"`
	ID               string        `json:"id"`
	URL              string        `json:"url,omitempty"`
	Direction        string        `json:"direction,omitempty"`
	Data             string        `json:"data,omitempty"`
	Size             int           `json:"size,omitempty"`
	CloseCode        int           `json:"code,omitempty"`
	CloseReason      string        `json:"reason,omitempty"`
	Sampled          *SamplingInfo `json:"sampled,omitempty"`
	BinaryFormat     string        `json:"binary_format,omitempty"`
	FormatConfidence float64       `json:"format_confidence,omitempty"`
	TabId            int           `json:"tab_id,omitempty"` // Chrome tab ID that produced this event
	TestIDs          []string      `json:"test_ids,omitempty"` // Test IDs this event belongs to (for test boundary correlation)
}

// SamplingInfo describes the sampling state when a message was captured
type SamplingInfo struct {
	Rate   string `json:"rate"`
	Logged string `json:"logged"`
	Window string `json:"window"`
}

// WebSocketEventFilter defines filtering criteria for events
type WebSocketEventFilter struct {
	ConnectionID string
	URLFilter    string
	Direction    string
	Limit        int
	TestID       string // If set, filter events where TestID is in event's TestIDs array
}

// WebSocketStatusFilter defines filtering criteria for status
type WebSocketStatusFilter struct {
	URLFilter    string
	ConnectionID string
}

// WebSocketStatusResponse is the response from get_websocket_status
type WebSocketStatusResponse struct {
	Connections []WebSocketConnection       `json:"connections"`
	Closed      []WebSocketClosedConnection `json:"closed"`
}

// WebSocketConnection represents an active WebSocket connection
type WebSocketConnection struct {
	ID          string                  `json:"id"`
	URL         string                  `json:"url"`
	State       string                  `json:"state"`
	OpenedAt    string                  `json:"opened_at,omitempty"`
	Duration    string                  `json:"duration,omitempty"`
	MessageRate WebSocketMessageRate    `json:"message_rate"`
	LastMessage WebSocketLastMessage    `json:"last_message"`
	Schema      *WebSocketSchema        `json:"schema,omitempty"`
	Sampling    WebSocketSamplingStatus `json:"sampling"`
}

// WebSocketClosedConnection represents a closed WebSocket connection
type WebSocketClosedConnection struct {
	ID            string `json:"id"`
	URL           string `json:"url"`
	State         string `json:"state"`
	OpenedAt      string `json:"opened_at,omitempty"`
	ClosedAt      string `json:"closed_at,omitempty"`
	CloseCode     int    `json:"close_code"`
	CloseReason   string `json:"close_reason"`
	TotalMessages struct {
		Incoming int `json:"incoming"`
		Outgoing int `json:"outgoing"`
	} `json:"total_messages"`
}

// WebSocketMessageRate contains rate info for a direction
type WebSocketMessageRate struct {
	Incoming WebSocketDirectionStats `json:"incoming"`
	Outgoing WebSocketDirectionStats `json:"outgoing"`
}

// WebSocketDirectionStats contains stats for a message direction
type WebSocketDirectionStats struct {
	PerSecond float64 `json:"per_second"`
	Total     int     `json:"total"`
	Bytes     int     `json:"bytes"`
}

// WebSocketLastMessage contains last message info
type WebSocketLastMessage struct {
	Incoming *WebSocketMessagePreview `json:"incoming,omitempty"`
	Outgoing *WebSocketMessagePreview `json:"outgoing,omitempty"`
}

// WebSocketMessagePreview contains a preview of the last message
type WebSocketMessagePreview struct {
	At      string `json:"at"`
	Age     string `json:"age"`
	Preview string `json:"preview"`
}

// WebSocketSchema describes detected message schema
type WebSocketSchema struct {
	DetectedKeys []string `json:"detected_keys,omitempty"`
	MessageCount int      `json:"message_count"`
	Consistent   bool     `json:"consistent"`
	Variants     []string `json:"variants,omitempty"`
}

// WebSocketSamplingStatus describes sampling state
type WebSocketSamplingStatus struct {
	Active bool   `json:"active"`
	Rate   string `json:"rate,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// ============================================
// Network Body Types
// ============================================

// NetworkBody is an alias to the canonical definition in internal/types/network.go
type NetworkBody = types.NetworkBody

// NetworkBodyFilter is an alias to the canonical definition in internal/types/network.go
type NetworkBodyFilter = types.NetworkBodyFilter

// ============================================
// Extension Logging Types
// ============================================

// ExtensionLog represents a log entry from the extension's background or content scripts
type ExtensionLog struct {
	Timestamp time.Time       `json:"timestamp"`
	Level     string          `json:"level"`              // "debug", "info", "warn", "error"
	Message   string          `json:"message"`            // Log message
	Source    string          `json:"source"`             // "background", "content", "inject"
	Category  string          `json:"category,omitempty"` // DebugCategory (CONNECTION, CAPTURE, etc.)
	Data      json.RawMessage `json:"data,omitempty"`     // Additional structured data (any JSON)
}

// PollingLogEntry tracks a single polling request (GET /pending-queries or POST /settings)
type PollingLogEntry struct {
	Timestamp    time.Time `json:"timestamp"`
	Endpoint     string    `json:"endpoint"` // "pending-queries" or "settings"
	Method       string    `json:"method"`   // "GET" or "POST"
	SessionID    string    `json:"session_id,omitempty"`
	PilotEnabled *bool     `json:"pilot_enabled,omitempty"` // Only for POST /settings
	PilotHeader  string    `json:"pilot_header,omitempty"`  // Only for GET with X-Gasoline-Pilot header
	QueryCount   int       `json:"query_count,omitempty"`   // Number of pending queries returned
}

// HTTPDebugEntry tracks detailed request/response data for debugging
type HTTPDebugEntry struct {
	Timestamp       time.Time         `json:"timestamp"`
	Endpoint        string            `json:"endpoint"`        // URL path
	Method          string            `json:"method"`          // HTTP method
	SessionID       string            `json:"session_id,omitempty"`
	ClientID        string            `json:"client_id,omitempty"`
	Headers         map[string]string `json:"headers,omitempty"`         // Request headers (redacted auth)
	RequestBody     string            `json:"request_body,omitempty"`    // First 1KB of request body
	ResponseStatus  int               `json:"response_status,omitempty"` // HTTP status code
	ResponseBody    string            `json:"response_body,omitempty"`   // First 1KB of response body
	DurationMs      int64             `json:"duration_ms"`               // Request processing duration
	Error           string            `json:"error,omitempty"`           // Error message if any
}

// ============================================
// Enhanced Actions Types
// ============================================

// EnhancedAction represents a captured user action with multi-strategy selectors
type EnhancedAction struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	URL       string `json:"url,omitempty"`
	// any: Selectors map contains multiple selector strategies (css, xpath, text, testId, etc.)
	// with string values, but some strategies have nested objects (e.g., aria-label with role)
	Selectors     map[string]any `json:"selectors,omitempty"`
	Value         string         `json:"value,omitempty"`
	InputType     string         `json:"inputType,omitempty"`
	Key           string         `json:"key,omitempty"`
	FromURL       string         `json:"fromUrl,omitempty"`
	ToURL         string         `json:"toUrl,omitempty"`
	SelectedValue string         `json:"selectedValue,omitempty"`
	SelectedText  string         `json:"selectedText,omitempty"`
	ScrollY       int            `json:"scrollY,omitempty"`
	TabId         int            `json:"tab_id,omitempty"`    // Chrome tab ID that produced this action
	TestIDs       []string       `json:"test_ids,omitempty"` // Test IDs this action belongs to (for test boundary correlation)
	Source        string         `json:"source,omitempty"`   // "human" for user actions, "ai" for AI-driven actions via interact tool
}

// EnhancedActionFilter defines filtering criteria for enhanced actions
type EnhancedActionFilter struct {
	LastN     int
	URLFilter string
	TestID    string // If set, filter actions where TestID is in action's TestIDs array
}

// ============================================
// Internal types
// ============================================

// connectionState tracks state for an active connection
type connectionState struct {
	id         string
	url        string
	state      string
	openedAt   string
	incoming   directionStats
	outgoing   directionStats
	sampling   bool
	lastSample *SamplingInfo
}

type directionStats struct {
	total       int
	bytes       int
	lastAt      string
	lastData    string
	recentTimes []time.Time // timestamps within rate window for rate calculation
}


// ============================================
// Constants
// ============================================

const (
	// Buffer capacity constants (exported for health metrics)
	MaxWSEvents        = 500
	MaxNetworkBodies   = 100
	MaxExtensionLogs   = 500
	MaxEnhancedActions = 50
	RateLimitThreshold = 1000
	MemoryHardLimit    = 50 * 1024 * 1024 // 50MB

	maxActiveConns    = 20
	maxClosedConns    = 10
	maxPendingQueries = 5

	// Network waterfall capacity configuration
	DefaultNetworkWaterfallCapacity = 1000
	MinNetworkWaterfallCapacity     = 100
	MaxNetworkWaterfallCapacity     = 10000

	defaultWSLimit          = 50
	defaultBodyLimit        = 20
	maxExtensionPostBody    = 5 << 20         // 5MB - max size for incoming extension POST bodies
	maxRequestBodySize      = 8192            // 8KB - truncation limit for captured request bodies
	maxResponseBodySize     = 16384           // 16KB
	wsBufferMemoryLimit     = 4 * 1024 * 1024 // 4MB
	nbBufferMemoryLimit     = 8 * 1024 * 1024 // 8MB
	circuitOpenStreakCount  = 5                // consecutive seconds over threshold to open circuit
	circuitCloseSeconds     = 10               // seconds below threshold to close circuit
	circuitCloseMemoryLimit = 30 * 1024 * 1024 // 30MB - memory must be below this to close circuit
	rateWindow              = 5 * time.Second  // rolling window for msg/s calculation
)

// ============================================
// Sub-structs for Capture composition
// ============================================

// A11yCache manages the accessibility audit result cache with LRU eviction
// and concurrent deduplication of inflight requests.
type A11yCache struct {
	cache      map[string]*a11yCacheEntry
	cacheOrder []string // Track insertion order for eviction
	lastURL    string
	inflight   map[string]*a11yInflightEntry
}

const maxA11yCacheEntries = 10
const a11yCacheTTL = 30 * time.Second

type a11yCacheEntry struct {
	result    json.RawMessage
	createdAt time.Time
	url       string
}

type a11yInflightEntry struct {
	done   chan struct{}
	result json.RawMessage
	err    error
}

// PerformanceStore manages performance snapshots and baselines with LRU eviction.
type PerformanceStore struct {
	snapshots       map[string]performance.PerformanceSnapshot
	snapshotOrder   []string
	baselines       map[string]performance.PerformanceBaseline
	baselineOrder   []string
	beforeSnapshots map[string]performance.PerformanceSnapshot // keyed by correlation_id, for perf_diff
}

// MemoryState tracks memory enforcement state including eviction counters
// and minimal mode flag.
type MemoryState struct {
	minimalMode      bool
	lastEvictionTime time.Time
	totalEvictions   int
	evictedEntries   int
	simulatedMemory  int64
}

// ============================================
// Capture
// ============================================

// Capture manages all buffered browser state: WebSocket events, network bodies,
// user actions, connections, queries, rate limiting, and performance.
//
// All fields are protected by mu (sync.RWMutex) unless noted otherwise.
// Lock hierarchy: Capture.mu is position 3 (after ClientRegistry, ClientState).
// Release locks before calling external callbacks. Use RLock() for read-only access.
// Sub-struct locks: a11y, perf, session, mem use parent mu. Only schemaStore and cspGen have own mutexes.
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

	wsEvents      []WebSocketEvent // Ring buffer of WS events (cap: effectiveWSCapacity). Kept in sync with wsAddedAt.
	wsAddedAt     []time.Time      // Parallel slice: insertion time for each wsEvents[i]. Used for TTL filtering and eviction order (oldest first).
	wsTotalAdded  int64            // Monotonic counter: total events ever added (never reset/decremented). Survives eviction. Used for cursor-based delta queries.
	wsMemoryTotal int64            // Approximate memory: sum of wsEventMemory(&wsEvents[i]). Estimate: len(Data)+200 bytes per event. Updated incrementally; recalc on critical eviction.

	// ============================================
	// Network Body Buffer (Ring Buffer)
	// ============================================

	networkBodies     []NetworkBody   // Ring buffer of HTTP request/response bodies (cap: MaxNetworkBodies=100). Parallel with networkAddedAt.
	networkAddedAt    []time.Time     // Parallel slice: insertion time for each networkBodies[i]. Used for TTL filtering and LRU eviction.
	networkTotalAdded int64           // Monotonic counter: total bodies ever added (never reset/decremented). Survives eviction. Used for cursor-based delta queries.
	nbMemoryTotal     int64           // Approximate memory: len(RequestBody)+len(ResponseBody)+300 bytes per entry. Updated incrementally on append/eviction.

	// ============================================
	// Enhanced Actions Buffer (Ring Buffer)
	// ============================================

	enhancedActions  []EnhancedAction // Ring buffer of browser actions. Parallel with actionAddedAt.
	actionAddedAt    []time.Time      // Parallel slice: insertion time for each enhancedActions[i].
	actionTotalAdded int64            // Monotonic counter: total actions ever added (never reset/decremented). Survives eviction.

	// ============================================
	// Timings and Performance Data
	// ============================================

	networkWaterfall         []NetworkWaterfallEntry // Ring buffer of browser PerformanceResourceTiming data (cap: networkWaterfallCapacity, default 1000, reconfigurable).
	networkWaterfallCapacity int                     // Configurable capacity for network waterfall (default DefaultNetworkWaterfallCapacity=1000).
	securityFlags            []SecurityFlag          // Ring buffer of security threat flags detected from network waterfall (max 1000). FIFO eviction.
	extensionLogs            []ExtensionLog          // Ring buffer of extension internal logs (max 500). FIFO eviction. No TTL filtering.

	// ============================================
	// WebSocket Connection Tracking
	// ============================================

	connections map[string]*connectionState  // Active WS connections by ID (max 20 total). LRU eviction via connOrder.
	observeSem  chan struct{}                // Semaphore limiting concurrent observer goroutines to 4. Prevents goroutine explosion.
	closedConns []WebSocketClosedConnection  // Ring buffer of closed connections (max 10, maxClosedConns). Preserves history for a while.
	connOrder   []string                     // Insertion order for LRU eviction of active connections.

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

	// Recording Management — delegates to RecordingManager sub-struct.
	rec *RecordingManager // Recording lifecycle, playback, and log-diff. Has own sync.Mutex — independent of Capture.mu.

	// ============================================
	// Composed Sub-Structures
	// ============================================

	a11y        A11yCache        // Accessibility audit cache. Protected by parent mu (no separate lock). Accessed via getA11yCacheEntry/setA11yCacheEntry.
	perf        PerformanceStore // Performance snapshots and baselines. Protected by parent mu (no separate lock).
	session     SessionTracker   // Session-level performance aggregation. Protected by parent mu (no separate lock).
	mem         MemoryState      // Memory tracking and enforcement state. Protected by parent mu (no separate lock).
	schemaStore SchemaStore      // API schema detection and tracking. HAS OWN LOCK (api_schema.go:199). Accessed by observer goroutines outside mu.
	cspGen      CSPGenerator     // CSP policy generation. HAS OWN LOCK (csp.go:36). Accessed by observer goroutines outside mu.

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
		wsEvents:                 make([]WebSocketEvent, 0, MaxWSEvents),
		networkBodies:            make([]NetworkBody, 0, MaxNetworkBodies),
		extensionLogs:            make([]ExtensionLog, 0, MaxExtensionLogs),
		enhancedActions:          make([]EnhancedAction, 0, MaxEnhancedActions),
		networkWaterfall:         make([]NetworkWaterfallEntry, 0, DefaultNetworkWaterfallCapacity),
		networkWaterfallCapacity: DefaultNetworkWaterfallCapacity,
		connections:              make(map[string]*connectionState),
		closedConns:              make([]WebSocketClosedConnection, 0),
		connOrder:                make([]string, 0),
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
	}
	c.observeSem = make(chan struct{}, 4)
	c.qd = NewQueryDispatcher()
	c.circuit = NewCircuitBreaker(
		func() int64 { return c.getMemoryForCircuit() },
		c.emitLifecycleEvent,
	)

	// Note: schemaStore, clientRegistry, cspGen are initialized by capture.New() in capture package
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
// Caller must NOT hold the lock (callback may do I/O).
func (c *Capture) emitLifecycleEvent(event string, data map[string]any) {
	c.mu.RLock()
	cb := c.lifecycleCallback
	c.mu.RUnlock()
	if cb != nil {
		cb(event, data)
	}
}

// SetServerVersion sets the server version for compatibility checking.
// Called once at startup with the version from main.go.
func (c *Capture) SetServerVersion(v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.serverVersion = v
}

// GetServerVersion returns the server version.
func (c *Capture) GetServerVersion() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverVersion
}

