// types.go — Core types, constants, and the Capture struct.
// All shared data structures live here: buffer types, MCP protocol types,
// performance baselines, and configuration constants.
// Design: Single file avoids scattered type definitions. Buffer sizes and
// limits are constants at the top for easy tuning.
package main

import (
	"encoding/json"
	"sync"
	"time"
)

// ============================================
// Types
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
}

// SamplingInfo describes the sampling state when a message was captured
type SamplingInfo struct {
	Rate   string `json:"rate"`
	Logged string `json:"logged"`
	Window string `json:"window"`
}

// ExtensionLog represents a log entry from the extension's background or content scripts
type ExtensionLog struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`    // "debug", "info", "warn", "error"
	Message   string                 `json:"message"`  // Log message
	Source    string                 `json:"source"`   // "background", "content", "inject"
	Category  string                 `json:"category,omitempty"` // DebugCategory (CONNECTION, CAPTURE, etc.)
	Data      map[string]interface{} `json:"data,omitempty"`     // Additional structured data
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

// WebSocketEventFilter defines filtering criteria for events
type WebSocketEventFilter struct {
	ConnectionID string
	URLFilter    string
	Direction    string
	Limit        int
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

// NetworkBody represents a captured network request/response
type NetworkBody struct {
	Timestamp          string  `json:"ts,omitempty"`
	Method             string  `json:"method"`
	URL                string  `json:"url"`
	Status             int     `json:"status"`
	RequestBody        string  `json:"request_body,omitempty"`
	ResponseBody       string  `json:"response_body,omitempty"`
	ContentType        string  `json:"content_type,omitempty"`
	Duration           int     `json:"duration,omitempty"`
	RequestTruncated   bool    `json:"request_truncated,omitempty"`
	ResponseTruncated  bool    `json:"response_truncated,omitempty"`
	ResponseHeaders    map[string]string `json:"response_headers,omitempty"`
	HasAuthHeader      bool              `json:"has_auth_header,omitempty"`
	BinaryFormat       string  `json:"binary_format,omitempty"`
	FormatConfidence   float64 `json:"format_confidence,omitempty"`
	TabId              int     `json:"tab_id,omitempty"` // Chrome tab ID that produced this request
}

// NetworkBodyFilter defines filtering criteria for network bodies
type NetworkBodyFilter struct {
	URLFilter string
	Method    string
	StatusMin int
	StatusMax int
	Limit     int
}

// PendingQuery represents a query waiting for extension response
type PendingQuery struct {
	Type          string          `json:"type"`
	Params        json.RawMessage `json:"params"`
	TabID         int             `json:"tab_id,omitempty"`         // Target tab ID (0 = active tab)
	CorrelationID string          `json:"correlation_id,omitempty"` // LLM-facing tracking ID for async commands
}

// PendingQueryResponse is the response format for pending queries
type PendingQueryResponse struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Params        json.RawMessage `json:"params"`
	TabID         int             `json:"tab_id,omitempty"`         // Target tab ID (0 = active tab)
	CorrelationID string          `json:"correlation_id,omitempty"` // LLM-facing tracking ID for async commands
}

// CommandResult represents the result of an async command execution
type CommandResult struct {
	CorrelationID string          `json:"correlation_id"`
	Status        string          `json:"status"` // "pending", "complete", "timeout", "expired"
	Result        json.RawMessage `json:"result,omitempty"`
	Error         string          `json:"error,omitempty"`
	CompletedAt   time.Time       `json:"completed_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// EnhancedAction represents a captured user action with multi-strategy selectors
type EnhancedAction struct {
	Type          string                 `json:"type"`
	Timestamp     int64                  `json:"timestamp"`
	URL           string                 `json:"url,omitempty"`
	Selectors     map[string]interface{} `json:"selectors,omitempty"`
	Value         string                 `json:"value,omitempty"`
	InputType     string                 `json:"inputType,omitempty"`
	Key           string                 `json:"key,omitempty"`
	FromURL       string                 `json:"fromUrl,omitempty"`
	ToURL         string                 `json:"toUrl,omitempty"`
	SelectedValue string                 `json:"selectedValue,omitempty"`
	SelectedText  string                 `json:"selectedText,omitempty"`
	ScrollY       int                    `json:"scrollY,omitempty"`
	TabId         int                    `json:"tab_id,omitempty"` // Chrome tab ID that produced this action
}

// EnhancedActionFilter defines filtering criteria for enhanced actions
type EnhancedActionFilter struct {
	LastN     int
	URLFilter string
}

// ============================================
// Performance Budget Types
// ============================================

// PerformanceSnapshot represents a captured performance snapshot from a page load
type PerformanceSnapshot struct {
	URL       string            `json:"url"`
	Timestamp string            `json:"timestamp"`
	Timing    PerformanceTiming `json:"timing"`
	Network   NetworkSummary    `json:"network"`
	LongTasks LongTaskMetrics   `json:"long_tasks"`
	CLS       *float64          `json:"cumulative_layout_shift,omitempty"` // snake_case (from browser LayoutShift)
	Resources []ResourceEntry   `json:"resources,omitempty"`
}

// PerformanceTiming holds navigation timing metrics
type PerformanceTiming struct {
	DomContentLoaded       float64  `json:"dom_content_loaded"`              // snake_case (from browser PerformanceTiming)
	Load                   float64  `json:"load"`                          // snake_case (from browser PerformanceTiming)
	FirstContentfulPaint   *float64 `json:"first_contentful_paint"`          // snake_case (from browser PerformancePaintTiming)
	LargestContentfulPaint *float64 `json:"largest_contentful_paint"`        // snake_case (from browser LargestContentfulPaint)
	InteractionToNextPaint *float64 `json:"interaction_to_next_paint,omitempty"` // snake_case (from browser EventTiming)
	TimeToFirstByte        float64  `json:"time_to_first_byte"`               // snake_case (from browser PerformanceTiming)
	DomInteractive         float64  `json:"dom_interactive"`                // snake_case (from browser PerformanceTiming)
}

// NetworkSummary holds aggregated network resource metrics
type NetworkSummary struct {
	RequestCount    int                    `json:"request_count"`   
	TransferSize    int64                  `json:"transfer_size"`   
	DecodedSize     int64                  `json:"decoded_size"`    
	ByType          map[string]TypeSummary `json:"by_type"`         
	SlowestRequests []SlowRequest          `json:"slowest_requests"`
}

// TypeSummary holds per-type resource metrics
type TypeSummary struct {
	Count int   `json:"count"`
	Size  int64 `json:"size"`
}

// SlowRequest represents one of the slowest network requests
type SlowRequest struct {
	URL      string  `json:"url"`
	Duration float64 `json:"duration"`
	Size     int64   `json:"size"`
}

// LongTaskMetrics holds accumulated long task data
type LongTaskMetrics struct {
	Count             int     `json:"count"`
	TotalBlockingTime float64 `json:"total_blocking_time"`
	Longest           float64 `json:"longest"`
}

// PerformanceBaseline holds averaged performance data for a URL path
type PerformanceBaseline struct {
	URL         string          `json:"url"`
	SampleCount int             `json:"sample_count"`
	LastUpdated string          `json:"last_updated"`
	Timing      BaselineTiming  `json:"timing"`
	Network     BaselineNetwork `json:"network"`
	LongTasks   LongTaskMetrics `json:"long_tasks"`
	CLS         *float64        `json:"cumulative_layout_shift,omitempty"` // snake_case (from browser LayoutShift)
	Resources   []ResourceEntry `json:"resources,omitempty"`
}

// BaselineTiming holds averaged timing metrics
type BaselineTiming struct {
	DomContentLoaded       float64  `json:"dom_content_loaded"`              // snake_case (from browser PerformanceTiming)
	Load                   float64  `json:"load"`                          // snake_case (from browser PerformanceTiming)
	FirstContentfulPaint   *float64 `json:"first_contentful_paint"`          // snake_case (from browser PerformancePaintTiming)
	LargestContentfulPaint *float64 `json:"largest_contentful_paint"`        // snake_case (from browser LargestContentfulPaint)
	TimeToFirstByte        float64  `json:"time_to_first_byte"`               // snake_case (from browser PerformanceTiming)
	DomInteractive         float64  `json:"dom_interactive"`                // snake_case (from browser PerformanceTiming)
}

// BaselineNetwork holds averaged network metrics
type BaselineNetwork struct {
	RequestCount int   `json:"request_count"`
	TransferSize int64 `json:"transfer_size"`
}

// PerformanceRegression describes a detected performance regression
type PerformanceRegression struct {
	Metric         string  `json:"metric"`
	Current        float64 `json:"current"`
	Baseline       float64 `json:"baseline"`
	ChangePercent  float64 `json:"change_percent"`
	AbsoluteChange float64 `json:"absolute_change"`
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

// pendingQueryEntry tracks a pending query with timeout
type pendingQueryEntry struct {
	query    PendingQueryResponse
	expires  time.Time
	clientID string // owning client for multi-client isolation
}

// queryResultEntry stores a query result with client ownership
type queryResultEntry struct {
	result    json.RawMessage
	clientID  string // owning client for multi-client isolation
	createdAt time.Time
}

// ============================================
// Constants
// ============================================

const (
	maxWSEvents             = 500
	maxNetworkBodies        = 100
	maxExtensionLogs        = 500
	maxEnhancedActions      = 50
	maxActiveConns          = 20
	maxClosedConns          = 10
	maxPendingQueries       = 5

	// Network waterfall capacity configuration
	DefaultNetworkWaterfallCapacity = 1000
	MinNetworkWaterfallCapacity     = 100
	MaxNetworkWaterfallCapacity     = 10000
	maxPerfSnapshots        = 20
	maxPerfBaselines        = 20
	defaultWSLimit          = 50
	defaultBodyLimit        = 20
	maxPostBodySize         = 5 << 20         // 5MB - max size for incoming POST request bodies
	maxRequestBodySize      = 8192            // 8KB - truncation limit for captured request bodies
	maxResponseBodySize     = 16384           // 16KB
	wsBufferMemoryLimit     = 4 * 1024 * 1024 // 4MB
	nbBufferMemoryLimit     = 8 * 1024 * 1024 // 8MB
	rateLimitThreshold      = 1000
	memoryHardLimit         = 50 * 1024 * 1024 // 50MB
	circuitOpenStreakCount  = 5                // consecutive seconds over threshold to open circuit
	circuitCloseSeconds     = 10               // seconds below threshold to close circuit
	circuitCloseMemoryLimit = 30 * 1024 * 1024 // 30MB - memory must be below this to close circuit
	defaultQueryTimeout     = 2 * time.Second // Extension polls every 1-2s, fast timeout prevents MCP hangs
	rateWindow              = 5 * time.Second // rolling window for msg/s calculation
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
	snapshots     map[string]PerformanceSnapshot
	snapshotOrder []string
	baselines     map[string]PerformanceBaseline
	baselineOrder []string
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

	networkBodies     []NetworkBody   // Ring buffer of HTTP request/response bodies (cap: maxNetworkBodies=100). Parallel with networkAddedAt.
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
	// Pending Queries (Extension ↔ Server RPC)
	// ============================================

	pendingQueries []pendingQueryEntry        // FIFO queue of pending queries awaiting extension response (max 5). Each has an expires timeout. Oldest dropped if full.
	queryResults   map[string]queryResultEntry // Completed query results keyed by query ID (not correlation_id). 60s TTL. Cleaned by startResultCleanup goroutine.
	queryCond      *sync.Cond                 // Condition var initialized with sync.NewCond(&c.mu). Broadcast when result arrives or cleanup happens.
	queryIDCounter int                        // Monotonic ID for next query (format: "q-<counter>"). Incremented in CreatePendingQueryWithClient.

	// ============================================
	// Rate Limiting & Circuit Breaker
	// ============================================

	windowEventCount     int       // Events in current 1-second window. Reset to 0 when window expires. Compared to rateLimitThreshold (1000 events/sec).
	rateWindowStart      time.Time // Monotonic time: when current window started. Used to detect expiration (now.Sub(rateWindowStart) > 1 second).
	rateLimitStreak      int       // Consecutive seconds window was over threshold. Incremented per second if over, reset to 0 if below. Circuit opens at 5 consecutive seconds.
	lastBelowThresholdAt time.Time // When rate first dropped below threshold. Initialized to time.Now() at startup (prevents false circuit-close on boot). Set to zero when over threshold. Used to measure "below threshold duration" for circuit close (10+ seconds required).
	circuitOpen          bool      // Circuit breaker state. true=reject all with 429. false=accept if within rate/memory limits. Opened when rateLimitStreak>=5 or memory>hard(50MB). Closed when rate below threshold for 10+ seconds AND memory<30MB.
	circuitOpenedAt      time.Time // Informational: when circuit was opened (display only, not used for enforcing minimum duration). Zero when circuit closed.
	circuitReason        string    // Reason circuit opened: "rate_exceeded" or "memory_exceeded". Reflects reason AT OPEN TIME (not necessarily current state). Cleared when closed.

	// ============================================
	// Query Timeout
	// ============================================

	queryTimeout time.Duration // Default: 2 seconds (defaultQueryTimeout=2*time.Second, types.go:427). Configurable. Applied to pending queries. Rationale: extension polls every 1-2s, fast timeout prevents MCP hangs.

	// ============================================
	// Async Command Results (Protected by resultsMu, NOT mu)
	// ============================================

	completedResults map[string]*CommandResult // Completed async results keyed by correlation_id (60s TTL). Protected by resultsMu. Cleaned by startResultCleanup goroutine every 10s. Expired entries moved to failedCommands.
	failedCommands   []*CommandResult          // Ring buffer of failed/expired commands for diagnostics (pre-allocated 100). Protected by resultsMu. Trimmed to max 100.
	resultsMu        sync.RWMutex              // SEPARATE lock protecting completedResults and failedCommands. Separate from mu to avoid blocking event ingest during async result operations. Observer goroutines use this lock.

	// ============================================
	// Extension Communication State
	// ============================================

	lastPollAt        time.Time // When extension last polled GET /pending-queries. Updated in HandlePendingQueries (line 373). Health endpoint uses 3s threshold to determine "connected" vs "stale".
	extensionSession  string    // Extension session ID from header (changes when extension reloads). Detects browser restart or extension update. Session change logged but does NOT auto-clear pending queries.
	sessionChangedAt  time.Time // When extensionSession last changed (used for display in health endpoint).
	pilotEnabled      bool      // AI Web Pilot toggle from POST /settings (or GET header fallback if settings >10s stale). Check before dispatching browser actions.
	pilotUpdatedAt    time.Time // When pilotEnabled was last updated from POST /settings. Staleness threshold: 10 seconds (queries.go:377-378). If >10s old, extension header takes priority.
	currentTestID     string    // CI test boundary correlation ID. Set via /test-boundary endpoint. Tags all events with test context. Cleared when test ends.

	// ============================================
	// Tab Tracking
	// ============================================

	trackingEnabled bool      // Single-tab mode active. true=track specific tab. false=observe all tabs (multi-tab).
	trackedTabID    int       // Browser tab ID when single-tab tracking (0=none). Invariant: if trackingEnabled then trackedTabID>0.
	trackedTabURL   string    // Tracked tab URL (informational, may be stale).
	trackingUpdated time.Time // When tracking status last refreshed from extension.

	// ============================================
	// Polling Activity Log (Circular Buffer, size 50)
	// ============================================

	pollingLog      []PollingLogEntry // Circular buffer of GET /pending-queries and POST /settings calls (50 entries). No TTL. For operator debugging.
	pollingLogIndex int               // Next write position (0-49, wraps to 0 after 49).

	// ============================================
	// HTTP Debug Log (Circular Buffer, size 50)
	// ============================================

	httpDebugLog      []HTTPDebugEntry // Circular buffer of HTTP requests/responses (50 entries). No TTL. For operator debugging.
	httpDebugLogIndex int              // Next write position (0-49, wraps to 0 after 49).

	// ============================================
	// Composed Sub-Structures
	// ============================================

	a11y        A11yCache        // Accessibility audit cache. Protected by parent mu (no separate lock). Accessed via getA11yCacheEntry/setA11yCacheEntry.
	perf        PerformanceStore // Performance snapshots and baselines. Protected by parent mu (no separate lock).
	session     SessionTracker   // Session-level performance aggregation. Protected by parent mu (no separate lock).
	mem         MemoryState      // Memory tracking and enforcement state. Protected by parent mu (no separate lock).
	schemaStore *SchemaStore     // API schema detection and tracking. HAS OWN LOCK (api_schema.go:199). Accessed by observer goroutines outside mu.
	cspGen      *CSPGenerator    // CSP policy generation. HAS OWN LOCK (csp.go:36). Accessed by observer goroutines outside mu.

	// ============================================
	// Multi-Client Support
	// ============================================

	clientRegistry *ClientRegistry // Registry of connected MCP clients. HAS OWN LOCK. Lock hierarchy: ClientRegistry.mu is position 1 (outermost), before Capture.mu.
}

// NewCapture creates a new Capture instance with initialized buffers
func NewCapture() *Capture {
	now := time.Now()
	c := &Capture{
		wsEvents:                 make([]WebSocketEvent, 0, maxWSEvents),
		networkBodies:            make([]NetworkBody, 0, maxNetworkBodies),
		extensionLogs:            make([]ExtensionLog, 0, maxExtensionLogs),
		enhancedActions:          make([]EnhancedAction, 0, maxEnhancedActions),
		networkWaterfall:         make([]NetworkWaterfallEntry, 0, DefaultNetworkWaterfallCapacity),
		networkWaterfallCapacity: DefaultNetworkWaterfallCapacity,
		connections:              make(map[string]*connectionState),
		closedConns:              make([]WebSocketClosedConnection, 0),
		connOrder:                make([]string, 0),
		pendingQueries:           make([]pendingQueryEntry, 0),
		queryResults:             make(map[string]queryResultEntry),
		rateWindowStart:          now,
		lastBelowThresholdAt:     now,
		queryTimeout:             defaultQueryTimeout,
		completedResults:         make(map[string]*CommandResult),
		failedCommands:           make([]*CommandResult, 0, 100), // Pre-allocate for 100 failed commands
		perf: PerformanceStore{
			snapshots:     make(map[string]PerformanceSnapshot),
			snapshotOrder: make([]string, 0),
			baselines:     make(map[string]PerformanceBaseline),
			baselineOrder: make([]string, 0),
		},
		session: SessionTracker{
			firstSnapshots: make(map[string]PerformanceSnapshot),
		},
		a11y: A11yCache{
			cache:      make(map[string]*a11yCacheEntry),
			cacheOrder: make([]string, 0),
			inflight:   make(map[string]*a11yInflightEntry),
		},
		pollingLog:   make([]PollingLogEntry, 50),  // Pre-allocate 50-entry circular buffer
		httpDebugLog: make([]HTTPDebugEntry, 50), // Pre-allocate 50-entry circular buffer for HTTP debug
	}
	c.observeSem = make(chan struct{}, 4)
	c.queryCond = sync.NewCond(&c.mu)
	c.schemaStore = NewSchemaStore()
	c.clientRegistry = NewClientRegistry()
	c.cspGen = NewCSPGenerator()
	return c
}

// ============================================
// Workflow Integration Types
// ============================================

// SessionSummary represents a compiled summary of a development session
type SessionSummary struct {
	Status           string            `json:"status"` // "ok", "no_performance_data", "insufficient_data"
	PerformanceDelta *PerformanceDelta `json:"performance_delta,omitempty"`
	Errors           []SessionError    `json:"errors,omitempty"`
	Metadata         SessionMetadata   `json:"metadata"`
}

// PerformanceDelta represents the net change in performance metrics during a session
type PerformanceDelta struct {
	LoadTimeBefore   float64 `json:"load_time_before"`
	LoadTimeAfter    float64 `json:"load_time_after"`
	LoadTimeDelta    float64 `json:"load_time_delta"`
	FCPBefore        float64 `json:"fcp_before,omitempty"`
	FCPAfter         float64 `json:"fcp_after,omitempty"`
	FCPDelta         float64 `json:"fcp_delta,omitempty"`
	LCPBefore        float64 `json:"lcp_before,omitempty"`
	LCPAfter         float64 `json:"lcp_after,omitempty"`
	LCPDelta         float64 `json:"lcp_delta,omitempty"`
	CLSBefore        float64 `json:"cls_before,omitempty"`
	CLSAfter         float64 `json:"cls_after,omitempty"`
	CLSDelta         float64 `json:"cls_delta,omitempty"`
	BundleSizeBefore int64   `json:"bundle_size_before"`
	BundleSizeAfter  int64   `json:"bundle_size_after"`
	BundleSizeDelta  int64   `json:"bundle_size_delta"`
}

// SessionError represents an error observed during a session
type SessionError struct {
	Message  string `json:"message"`
	Source   string `json:"source,omitempty"`
	Resolved bool   `json:"resolved"`
}

// SessionMetadata holds session-level aggregate stats
type SessionMetadata struct {
	DurationMs            int64 `json:"duration_ms"`
	ReloadCount           int   `json:"reload_count"`
	PerformanceCheckCount int   `json:"performance_check_count"`
}

// ============================================
// Push Regression Alert Types
// ============================================

// PerformanceAlert represents a pending regression alert to be delivered via get_changes_since
type PerformanceAlert struct {
	ID             int64                       `json:"id"`
	Type           string                      `json:"type"`
	URL            string                      `json:"url"`
	DetectedAt     string                      `json:"detected_at"`
	Summary        string                      `json:"summary"`
	Metrics        map[string]AlertMetricDelta `json:"metrics"`
	Recommendation string                      `json:"recommendation"`
	// Internal tracking (not serialized to JSON response)
	deliveredAt int64 // checkpoint counter at which this was delivered
}

// AlertMetricDelta describes the delta for a single regressed metric
type AlertMetricDelta struct {
	Baseline float64 `json:"baseline"`
	Current  float64 `json:"current"`
	DeltaMs  float64 `json:"delta_ms"`
	DeltaPct float64 `json:"delta_pct"`
}

// ============================================
// Causal Diffing Types
// ============================================

// ResourceEntry represents a single resource in a performance snapshot fingerprint
type ResourceEntry struct {
	URL            string  `json:"url"`
	Type           string  `json:"type"`
	TransferSize   int64   `json:"transfer_size"`              // snake_case (from browser PerformanceResourceTiming)
	Duration       float64 `json:"duration"`                  // snake_case (from browser PerformanceResourceTiming)
	RenderBlocking bool    `json:"renderBlocking,omitempty"`  // snake_case (from browser PerformanceResourceTiming)
}

// ResourceDiff holds the categorized differences between baseline and current resources
type ResourceDiff struct {
	Added   []AddedResource   `json:"added"`
	Removed []RemovedResource `json:"removed"`
	Resized []ResizedResource `json:"resized"`
	Retimed []RetimedResource `json:"retimed"`
}

// AddedResource is a resource present in current but not in baseline
type AddedResource struct {
	URL            string  `json:"url"`
	Type           string  `json:"type"`
	SizeBytes      int64   `json:"size_bytes"`
	DurationMs     float64 `json:"duration_ms"`
	RenderBlocking bool    `json:"render_blocking"`
}

// RemovedResource is a resource present in baseline but not in current
type RemovedResource struct {
	URL       string `json:"url"`
	Type      string `json:"type"`
	SizeBytes int64  `json:"size_bytes"`
}

// ResizedResource is a resource present in both with significant size change
type ResizedResource struct {
	URL           string `json:"url"`
	BaselineBytes int64  `json:"baseline_bytes"`
	CurrentBytes  int64  `json:"current_bytes"`
	DeltaBytes    int64  `json:"delta_bytes"`
}

// RetimedResource is a resource present in both with significant duration change
type RetimedResource struct {
	URL        string  `json:"url"`
	BaselineMs float64 `json:"baseline_ms"`
	CurrentMs  float64 `json:"current_ms"`
	DeltaMs    float64 `json:"delta_ms"`
}

// TimingDelta holds the timing differences between baseline and current
type TimingDelta struct {
	LoadMs float64 `json:"load_ms"`
	FCPMs  float64 `json:"fcp_ms"`
	LCPMs  float64 `json:"lcp_ms"`
}

// CausalDiffResult is the full response from the get_causal_diff tool
type CausalDiffResult struct {
	URL             string       `json:"url"`
	TimingDelta     TimingDelta  `json:"timing_delta"`
	ResourceChanges ResourceDiff `json:"resource_changes"`
	ProbableCause   string       `json:"probable_cause"`
	Recommendations []string     `json:"recommendations"`
}

// ============================================
// Session Tracking (for workflow integration)
// ============================================

// SessionTracker records the first and last performance snapshots for delta computation
type SessionTracker struct {
	firstSnapshots map[string]PerformanceSnapshot // first snapshot per URL
	snapshotCount  int                            // total snapshots added this session
}
