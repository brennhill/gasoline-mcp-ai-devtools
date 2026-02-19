// types.go — Types, constants, and constructors for the streaming package.
package streaming

import (
	"io"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/types"
)

// ============================================
// Stream Constants
// ============================================

const (
	DefaultThrottleSeconds    = 5
	DefaultSeverityMin        = "warning"
	MaxNotificationsPerMinute = 12
	DedupWindow               = 30 * time.Second
	MaxPendingBatch           = 100
)

// ============================================
// Alert Constants
// ============================================

const (
	AlertBufferCap       = 50
	CIResultsCap         = 10
	CorrelationWindow    = 5 * time.Second
	AnomalyWindowSeconds = 60
	AnomalyBucketSeconds = 10
)

// ============================================
// Stream Types
// ============================================

// StreamConfig holds the user-configured streaming settings.
type StreamConfig struct {
	Enabled         bool     `json:"enabled"`
	Events          []string `json:"events"`
	ThrottleSeconds int      `json:"throttle_seconds"`
	URLFilter       string   `json:"url"`
	SeverityMin     string   `json:"severity_min"`
}

// StreamState manages active context streaming.
type StreamState struct {
	Config       StreamConfig
	LastNotified time.Time
	SeenMessages map[string]time.Time // dedupKey → last sent
	NotifyCount  int                  // count in current minute
	MinuteStart  time.Time
	PendingBatch []types.Alert
	Mu           sync.Mutex
	Writer       io.Writer // defaults to nil (no output)
}

// MCPNotification is the MCP notification format for streaming alerts.
type MCPNotification struct {
	JSONRPC string             `json:"jsonrpc"`
	Method  string             `json:"method"`
	Params  NotificationParams `json:"params"`
}

// NotificationParams holds the notification payload.
type NotificationParams struct {
	Level  string `json:"level"`
	Logger string `json:"logger"`
	Data   any    `json:"data"`
}

// ============================================
// AlertBuffer
// ============================================

// AlertBuffer owns the alert, CI, and anomaly state.
// Fields are exported for test access (internal package only).
type AlertBuffer struct {
	Mu         sync.Mutex
	Alerts     []types.Alert
	CIResults  []types.CIResult
	ErrorTimes []time.Time
	Stream     *StreamState
}

// NewAlertBuffer creates an AlertBuffer with a default StreamState.
func NewAlertBuffer() *AlertBuffer {
	return &AlertBuffer{
		Stream: NewStreamState(),
	}
}
