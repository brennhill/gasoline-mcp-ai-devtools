package main

import (
	"encoding/json"
	"sync"
	"time"
)

// ============================================
// v4 Types
// ============================================

// WebSocketEvent represents a captured WebSocket event
type WebSocketEvent struct {
	Timestamp   string        `json:"ts,omitempty"`
	Type        string        `json:"type,omitempty"`
	Event       string        `json:"event"`
	ID          string        `json:"id"`
	URL         string        `json:"url,omitempty"`
	Direction   string        `json:"direction,omitempty"`
	Data        string        `json:"data,omitempty"`
	Size        int           `json:"size,omitempty"`
	CloseCode   int           `json:"code,omitempty"`
	CloseReason string        `json:"reason,omitempty"`
	Sampled     *SamplingInfo `json:"sampled,omitempty"`
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
	Timestamp         string `json:"ts,omitempty"`
	Method            string `json:"method"`
	URL               string `json:"url"`
	Status            int    `json:"status"`
	RequestBody       string `json:"requestBody,omitempty"`
	ResponseBody      string `json:"responseBody,omitempty"`
	ContentType       string `json:"contentType,omitempty"`
	Duration          int    `json:"duration,omitempty"`
	RequestTruncated  bool   `json:"requestTruncated,omitempty"`
	ResponseTruncated bool   `json:"responseTruncated,omitempty"`
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
	Type   string          `json:"type"`
	Params json.RawMessage `json:"params"`
}

// PendingQueryResponse is the response format for pending queries
type PendingQueryResponse struct {
	ID     string          `json:"id"`
	Type   string          `json:"type"`
	Params json.RawMessage `json:"params"`
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
}

// PerformanceTiming holds navigation timing metrics
type PerformanceTiming struct {
	DomContentLoaded       float64  `json:"domContentLoaded"`
	Load                   float64  `json:"load"`
	FirstContentfulPaint   *float64 `json:"firstContentfulPaint"`
	LargestContentfulPaint *float64 `json:"largestContentfulPaint"`
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
	query   PendingQueryResponse
	expires time.Time
}

// ============================================
// Constants
// ============================================

const (
	maxWSEvents         = 500
	maxNetworkBodies    = 100
	maxEnhancedActions  = 50
	maxActiveConns      = 20
	maxClosedConns      = 10
	maxPendingQueries   = 5
	maxPerfSnapshots    = 20
	maxPerfBaselines    = 20
	defaultWSLimit      = 50
	defaultBodyLimit    = 20
	maxRequestBodySize  = 8192            // 8KB
	maxResponseBodySize = 16384           // 16KB
	wsBufferMemoryLimit = 4 * 1024 * 1024 // 4MB
	nbBufferMemoryLimit = 8 * 1024 * 1024 // 8MB
	rateLimitThreshold  = 1000
	memoryHardLimit     = 50 * 1024 * 1024 // 50MB
	defaultQueryTimeout = 10 * time.Second
	rateWindow          = 5 * time.Second // rolling window for msg/s calculation
)

// ============================================
// Capture
// ============================================

// Capture handles v4-specific state and operations
type Capture struct {
	mu sync.RWMutex

	// WebSocket event ring buffer
	wsEvents     []WebSocketEvent
	wsAddedAt    []time.Time // parallel: when each event was added
	wsTotalAdded int64       // monotonic counter

	// Network bodies ring buffer
	networkBodies     []NetworkBody
	networkAddedAt    []time.Time // parallel: when each body was added
	networkTotalAdded int64       // monotonic counter

	// Enhanced actions ring buffer (v5)
	enhancedActions  []EnhancedAction
	actionAddedAt    []time.Time // parallel: when each action was added
	actionTotalAdded int64       // monotonic counter

	// Connection tracker
	connections map[string]*connectionState
	closedConns []WebSocketClosedConnection
	connOrder   []string // Track insertion order for eviction

	// Pending queries
	pendingQueries []pendingQueryEntry
	queryResults   map[string]json.RawMessage
	queryCond      *sync.Cond
	queryIDCounter int

	// Rate limiting
	eventCount    int
	rateResetTime time.Time

	// Memory simulation (for testing)
	simulatedMemory int64

	// Performance snapshots
	perfSnapshots     map[string]PerformanceSnapshot
	perfSnapshotOrder []string
	perfBaselines     map[string]PerformanceBaseline
	perfBaselineOrder []string

	// Query timeout
	queryTimeout time.Duration

	// Memory enforcement
	minimalMode     bool
	lastEvictionTime time.Time
	totalEvictions  int
	evictedEntries  int

	// A11y audit cache
	a11yCache      map[string]*a11yCacheEntry
	a11yCacheOrder []string // Track insertion order for eviction
	lastKnownURL   string
	a11yInflight   map[string]*a11yInflightEntry
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

// NewCapture creates a new v4 server instance
func NewCapture() *Capture {
	c := &Capture{
		wsEvents:          make([]WebSocketEvent, 0, maxWSEvents),
		networkBodies:     make([]NetworkBody, 0, maxNetworkBodies),
		enhancedActions:   make([]EnhancedAction, 0, maxEnhancedActions),
		connections:       make(map[string]*connectionState),
		closedConns:       make([]WebSocketClosedConnection, 0),
		connOrder:         make([]string, 0),
		pendingQueries:    make([]pendingQueryEntry, 0),
		queryResults:      make(map[string]json.RawMessage),
		rateResetTime:     time.Now(),
		queryTimeout:      defaultQueryTimeout,
		perfSnapshots:     make(map[string]PerformanceSnapshot),
		perfSnapshotOrder: make([]string, 0),
		perfBaselines:     make(map[string]PerformanceBaseline),
		perfBaselineOrder: make([]string, 0),
		a11yCache:         make(map[string]*a11yCacheEntry),
		a11yCacheOrder:    make([]string, 0),
		a11yInflight:      make(map[string]*a11yInflightEntry),
	}
	c.queryCond = sync.NewCond(&c.mu)
	return c
}
