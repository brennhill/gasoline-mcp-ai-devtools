// Purpose: Defines checkpoint diff constants, state, and public response types.
// Why: Centralizing model types keeps diff computation and checkpoint lifecycle logic focused.
// Docs: docs/features/feature/push-alerts/index.md

package ai

import (
	"sync"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/server"
	gasTypes "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/types"
)

type NetworkBody = capture.NetworkBody
type WebSocketEvent = capture.WebSocketEvent
type EnhancedAction = capture.EnhancedAction

const (
	maxNamedCheckpoints   = 20
	maxCheckpointNameLen  = 50
	maxDiffEntriesPerCat  = 50
	maxMessageLen         = 200
	degradedLatencyFactor = 3
)

type Checkpoint struct {
	Name           string
	CreatedAt      time.Time
	LogTotal       int64
	NetworkTotal   int64
	WSTotal        int64
	ActionTotal    int64
	PageURL        string
	KnownEndpoints map[string]endpointState
	AlertDelivery  int64
}

type endpointState struct {
	Status   int
	Duration int
}

type GetChangesSinceParams struct {
	Checkpoint string
	Include    []string
	Severity   string
}

type DiffResponse struct {
	From              time.Time                   `json:"from"`
	To                time.Time                   `json:"to"`
	DurationMs        int64                       `json:"duration_ms"`
	Severity          string                      `json:"severity"`
	Summary           string                      `json:"summary"`
	TokenCount        int                         `json:"token_count"`
	Console           *ConsoleDiff                `json:"console,omitempty"`
	Network           *NetworkDiff                `json:"network,omitempty"`
	WebSocket         *WebSocketDiff              `json:"websocket,omitempty"`
	Actions           *ActionsDiff                `json:"actions,omitempty"`
	PerformanceAlerts []gasTypes.PerformanceAlert `json:"performance_alerts,omitempty"`
}

type ConsoleDiff struct {
	TotalNew int            `json:"total_new"`
	Errors   []ConsoleEntry `json:"errors,omitempty"`
	Warnings []ConsoleEntry `json:"warnings,omitempty"`
}

type ConsoleEntry struct {
	Message string `json:"message"`
	Source  string `json:"source,omitempty"`
	Count   int    `json:"count"`
}

type NetworkDiff struct {
	TotalNew     int               `json:"total_new"`
	Failures     []NetworkFailure  `json:"failures,omitempty"`
	NewEndpoints []string          `json:"new_endpoints,omitempty"`
	Degraded     []NetworkDegraded `json:"degraded,omitempty"`
}

type NetworkFailure struct {
	Path           string `json:"path"`
	Status         int    `json:"status"`
	PreviousStatus int    `json:"previous_status"`
}

type NetworkDegraded struct {
	Path     string `json:"path"`
	Duration int    `json:"duration_ms"`
	Baseline int    `json:"baseline_ms"`
}

type WebSocketDiff struct {
	TotalNew       int       `json:"total_new"`
	Disconnections []WSDisco `json:"disconnections,omitempty"`
	Connections    []WSConn  `json:"connections,omitempty"`
	Errors         []WSError `json:"errors,omitempty"`
}

type WSDisco struct {
	URL         string `json:"url"`
	CloseCode   int    `json:"close_code,omitempty"`
	CloseReason string `json:"close_reason,omitempty"`
}

type WSConn struct {
	URL string `json:"url"`
	ID  string `json:"id"`
}

type WSError struct {
	URL     string `json:"url"`
	Message string `json:"message"`
}

type ActionsDiff struct {
	TotalNew int           `json:"total_new"`
	Actions  []ActionEntry `json:"actions,omitempty"`
}

type ActionEntry struct {
	Type      string `json:"type"`
	URL       string `json:"url,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

type CheckpointManager struct {
	mu               sync.Mutex
	autoCheckpoint   *Checkpoint
	namedCheckpoints map[string]*Checkpoint
	namedOrder       []string

	server server.LogReader

	pendingAlerts []gasTypes.PerformanceAlert
	alertCounter  int64
	alertDelivery int64
	capture       *capture.Capture
}
