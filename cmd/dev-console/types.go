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
	Name            string    `json:"name"`              // Full URL
	URL             string    `json:"url"`               // Same as name
	InitiatorType   string    `json:"initiatorType"`     // "script", "stylesheet", "img", etc.
	Duration        float64   `json:"duration"`          // Total duration in ms
	StartTime       float64   `json:"startTime"`         // Relative to navigationStart
	FetchStart      float64   `json:"fetchStart"`        // When fetch began
	ResponseEnd     float64   `json:"responseEnd"`       // When response completed
	TransferSize    int       `json:"transferSize"`      // Bytes transferred (0 if cached)
	DecodedBodySize int       `json:"decodedBodySize"`   // Decompressed size
	EncodedBodySize int       `json:"encodedBodySize"`   // Compressed size
	PageURL         string    `json:"pageURL,omitempty"` // Page that loaded this resource
	Timestamp       time.Time `json:"timestamp,omitempty"` // Server-side timestamp
}

// NetworkWaterfallPayload is POSTed by the extension
type NetworkWaterfallPayload struct {
	Entries []NetworkWaterfallEntry `json:"entries"`
	PageURL string                  `json:"pageURL"`
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
	OpenedAt    string                  `json:"openedAt,omitempty"`
	Duration    string                  `json:"duration,omitempty"`
	MessageRate WebSocketMessageRate    `json:"messageRate"`
	LastMessage WebSocketLastMessage    `json:"lastMessage"`
	Schema      *WebSocketSchema        `json:"schema,omitempty"`
	Sampling    WebSocketSamplingStatus `json:"sampling"`
}

// WebSocketClosedConnection represents a closed WebSocket connection
type WebSocketClosedConnection struct {
	ID            string `json:"id"`
	URL           string `json:"url"`
	State         string `json:"state"`
	OpenedAt      string `json:"openedAt,omitempty"`
	ClosedAt      string `json:"closedAt,omitempty"`
	CloseCode     int    `json:"closeCode"`
	CloseReason   string `json:"closeReason"`
	TotalMessages struct {
		Incoming int `json:"incoming"`
		Outgoing int `json:"outgoing"`
	} `json:"totalMessages"`
}

// WebSocketMessageRate contains rate info for a direction
type WebSocketMessageRate struct {
	Incoming WebSocketDirectionStats `json:"incoming"`
	Outgoing WebSocketDirectionStats `json:"outgoing"`
}

// WebSocketDirectionStats contains stats for a message direction
type WebSocketDirectionStats struct {
	PerSecond float64 `json:"perSecond"`
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
	DetectedKeys []string `json:"detectedKeys,omitempty"`
	MessageCount int      `json:"messageCount"`
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
	RequestBody        string  `json:"requestBody,omitempty"`
	ResponseBody       string  `json:"responseBody,omitempty"`
	ContentType        string  `json:"contentType,omitempty"`
	Duration           int     `json:"duration,omitempty"`
	RequestTruncated   bool    `json:"requestTruncated,omitempty"`
	ResponseTruncated  bool    `json:"responseTruncated,omitempty"`
	ResponseHeaders    map[string]string `json:"responseHeaders,omitempty"`
	HasAuthHeader      bool              `json:"hasAuthHeader,omitempty"`
	BinaryFormat       string  `json:"binary_format,omitempty"`
	FormatConfidence   float64 `json:"format_confidence,omitempty"`
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
	LongTasks LongTaskMetrics   `json:"longTasks"`
	CLS       *float64          `json:"cumulativeLayoutShift,omitempty"`
	Resources []ResourceEntry   `json:"resources,omitempty"`
}

// PerformanceTiming holds navigation timing metrics
type PerformanceTiming struct {
	DomContentLoaded       float64  `json:"domContentLoaded"`
	Load                   float64  `json:"load"`
	FirstContentfulPaint   *float64 `json:"firstContentfulPaint"`
	LargestContentfulPaint *float64 `json:"largestContentfulPaint"`
	InteractionToNextPaint *float64 `json:"interactionToNextPaint,omitempty"`
	TimeToFirstByte        float64  `json:"timeToFirstByte"`
	DomInteractive         float64  `json:"domInteractive"`
}

// NetworkSummary holds aggregated network resource metrics
type NetworkSummary struct {
	RequestCount    int                    `json:"requestCount"`
	TransferSize    int64                  `json:"transferSize"`
	DecodedSize     int64                  `json:"decodedSize"`
	ByType          map[string]TypeSummary `json:"byType"`
	SlowestRequests []SlowRequest          `json:"slowestRequests"`
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
	TotalBlockingTime float64 `json:"totalBlockingTime"`
	Longest           float64 `json:"longest"`
}

// PerformanceBaseline holds averaged performance data for a URL path
type PerformanceBaseline struct {
	URL         string          `json:"url"`
	SampleCount int             `json:"sampleCount"`
	LastUpdated string          `json:"lastUpdated"`
	Timing      BaselineTiming  `json:"timing"`
	Network     BaselineNetwork `json:"network"`
	LongTasks   LongTaskMetrics `json:"longTasks"`
	CLS         *float64        `json:"cumulativeLayoutShift,omitempty"`
	Resources   []ResourceEntry `json:"resources,omitempty"`
}

// BaselineTiming holds averaged timing metrics
type BaselineTiming struct {
	DomContentLoaded       float64  `json:"domContentLoaded"`
	Load                   float64  `json:"load"`
	FirstContentfulPaint   *float64 `json:"firstContentfulPaint"`
	LargestContentfulPaint *float64 `json:"largestContentfulPaint"`
	TimeToFirstByte        float64  `json:"timeToFirstByte"`
	DomInteractive         float64  `json:"domInteractive"`
}

// BaselineNetwork holds averaged network metrics
type BaselineNetwork struct {
	RequestCount int   `json:"requestCount"`
	TransferSize int64 `json:"transferSize"`
}

// PerformanceRegression describes a detected performance regression
type PerformanceRegression struct {
	Metric         string  `json:"metric"`
	Current        float64 `json:"current"`
	Baseline       float64 `json:"baseline"`
	ChangePercent  float64 `json:"changePercent"`
	AbsoluteChange float64 `json:"absoluteChange"`
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

// Capture manages all buffered browser state: WebSocket events, network
// bodies, user actions, connections, queries, rate limiting, and performance.
type Capture struct {
	mu sync.RWMutex

	// TTL for read-time filtering (0 means unlimited)
	TTL time.Duration
	// WebSocket event ring buffer
	wsEvents     []WebSocketEvent
	wsAddedAt    []time.Time // parallel: when each event was added
	wsTotalAdded int64       // monotonic counter

	// Network bodies ring buffer
	networkBodies     []NetworkBody
	networkAddedAt    []time.Time // parallel: when each body was added
	networkTotalAdded int64       // monotonic counter

	// Enhanced actions ring buffer
	enhancedActions  []EnhancedAction
	actionAddedAt    []time.Time // parallel: when each action was added
	actionTotalAdded int64       // monotonic counter

	// Network waterfall ring buffer (PerformanceResourceTiming data)
	networkWaterfall         []NetworkWaterfallEntry // Complete network request timing data
	networkWaterfallCapacity int                     // Configurable capacity (default 1000)

	// Security flags ring buffer (detected threats from network waterfall)
	securityFlags []SecurityFlag // Ring buffer of detected security issues (max 1000)

	// Extension logs ring buffer (background/content script logs)
	extensionLogs []ExtensionLog // Ring buffer of extension internal logs (max 500)

	// Connection tracker
	connections map[string]*connectionState
	observeSem  chan struct{} // bounds concurrent observer goroutines
	closedConns []WebSocketClosedConnection
	connOrder   []string // Track insertion order for eviction

	// Pending queries
	pendingQueries []pendingQueryEntry
	queryResults   map[string]queryResultEntry
	queryCond      *sync.Cond
	queryIDCounter int

	// Rate limiting (sliding window)
	windowEventCount     int       // Events in current 1-second window
	rateWindowStart      time.Time // When current window started (monotonic)
	rateLimitStreak      int       // Consecutive seconds over threshold
	lastBelowThresholdAt time.Time // When rate first dropped below threshold
	circuitOpen          bool      // Circuit breaker state
	circuitOpenedAt      time.Time // When circuit was opened
	circuitReason        string    // Why circuit opened ("rate_exceeded" or "memory_exceeded")

	// Query timeout
	queryTimeout time.Duration

	// Async command tracking (correlation_id → result)
	completedResults map[string]*CommandResult // Completed async commands (60s TTL)
	failedCommands   []*CommandResult          // Failed/expired commands (circular buffer, max 100)
	resultsMu        sync.RWMutex              // Separate lock for async result operations

	// Extension communication tracking
	lastPollAt        time.Time // When extension last polled GET /pending-queries (command polling)
	extensionSession  string    // Current extension session ID (for reload detection)
	sessionChangedAt  time.Time // When session ID last changed (extension reload)
	pilotEnabled      bool      // AI Web Pilot toggle state from extension
	pilotUpdatedAt    time.Time // When pilotEnabled was last updated (from POST /settings)
	currentTestID     string    // Current test ID for CI test-boundary correlation

	// Tab tracking status (from extension status pings)
	trackingEnabled bool      // Whether tab tracking is active
	trackedTabID    int       // The tracked tab's ID (0 = none)
	trackedTabURL   string    // The tracked tab's URL
	trackingUpdated time.Time // When tracking status was last updated

	// Polling activity log (rotating buffer of 50 most recent GET /pending-queries and POST /settings calls)
	pollingLog      []PollingLogEntry // Circular buffer, size 50
	pollingLogIndex int               // Next write position (wraps at 50)

	// HTTP debug log (rotating buffer of 50 most recent HTTP requests/responses)
	httpDebugLog      []HTTPDebugEntry // Circular buffer, size 50
	httpDebugLogIndex int              // Next write position (wraps at 50)

	// Composed sub-structs
	a11y        A11yCache
	perf        PerformanceStore
	session     SessionTracker
	mem         MemoryState
	schemaStore *SchemaStore
	cspGen      *CSPGenerator

	// Multi-client support
	clientRegistry *ClientRegistry
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
	PerformanceDelta *PerformanceDelta `json:"performanceDelta,omitempty"`
	Errors           []SessionError    `json:"errors,omitempty"`
	Metadata         SessionMetadata   `json:"metadata"`
}

// PerformanceDelta represents the net change in performance metrics during a session
type PerformanceDelta struct {
	LoadTimeBefore   float64 `json:"loadTimeBefore"`
	LoadTimeAfter    float64 `json:"loadTimeAfter"`
	LoadTimeDelta    float64 `json:"loadTimeDelta"`
	FCPBefore        float64 `json:"fcpBefore,omitempty"`
	FCPAfter         float64 `json:"fcpAfter,omitempty"`
	FCPDelta         float64 `json:"fcpDelta,omitempty"`
	LCPBefore        float64 `json:"lcpBefore,omitempty"`
	LCPAfter         float64 `json:"lcpAfter,omitempty"`
	LCPDelta         float64 `json:"lcpDelta,omitempty"`
	CLSBefore        float64 `json:"clsBefore,omitempty"`
	CLSAfter         float64 `json:"clsAfter,omitempty"`
	CLSDelta         float64 `json:"clsDelta,omitempty"`
	BundleSizeBefore int64   `json:"bundleSizeBefore"`
	BundleSizeAfter  int64   `json:"bundleSizeAfter"`
	BundleSizeDelta  int64   `json:"bundleSizeDelta"`
}

// SessionError represents an error observed during a session
type SessionError struct {
	Message  string `json:"message"`
	Source   string `json:"source,omitempty"`
	Resolved bool   `json:"resolved"`
}

// SessionMetadata holds session-level aggregate stats
type SessionMetadata struct {
	DurationMs            int64 `json:"durationMs"`
	ReloadCount           int   `json:"reloadCount"`
	PerformanceCheckCount int   `json:"performanceCheckCount"`
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
	TransferSize   int64   `json:"transferSize"`
	Duration       float64 `json:"duration"`
	RenderBlocking bool    `json:"renderBlocking,omitempty"`
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
