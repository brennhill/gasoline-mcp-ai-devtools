package ai

import (
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/checkpoint"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/noise"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/persistence"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/server"
)

// Checkpoint aliases.
type (
	NetworkBody         = checkpoint.NetworkBody
	WebSocketEvent      = checkpoint.WebSocketEvent
	EnhancedAction      = checkpoint.EnhancedAction
	Checkpoint          = checkpoint.Checkpoint
	GetChangesSinceParams = checkpoint.GetChangesSinceParams
	DiffResponse        = checkpoint.DiffResponse
	ConsoleDiff         = checkpoint.ConsoleDiff
	ConsoleEntry        = checkpoint.ConsoleEntry
	NetworkDiff         = checkpoint.NetworkDiff
	NetworkFailure      = checkpoint.NetworkFailure
	NetworkDegraded     = checkpoint.NetworkDegraded
	WebSocketDiff       = checkpoint.WebSocketDiff
	WSDisconnection     = checkpoint.WSDisconnection
	WSConn              = checkpoint.WSConn
	WSError             = checkpoint.WSError
	ActionsDiff         = checkpoint.ActionsDiff
	ActionEntry         = checkpoint.ActionEntry
	CheckpointManager   = checkpoint.CheckpointManager
)

func NewCheckpointManager(serverReader server.LogReader, captureStore *capture.Store) *CheckpointManager {
	return checkpoint.NewCheckpointManager(serverReader, captureStore)
}

func FingerprintMessage(msg string) string { return checkpoint.FingerprintMessage(msg) }

// Noise aliases.
type (
	LogEntry           = noise.LogEntry
	NoiseMatchSpec     = noise.NoiseMatchSpec
	NoiseRule          = noise.NoiseRule
	NoiseProposal      = noise.NoiseProposal
	NoiseStatistics    = noise.NoiseStatistics
	PersistedNoiseData = noise.PersistedNoiseData
	NoiseConfig        = noise.NoiseConfig
)

func NewNoiseConfig() *NoiseConfig { return noise.NewNoiseConfig() }

func NewNoiseConfigWithStore(store *SessionStore) *NoiseConfig {
	return noise.NewNoiseConfigWithStore(store)
}

// Session persistence aliases.
type (
	SessionStore      = persistence.SessionStore
	ProjectMeta       = persistence.ProjectMeta
	SessionContext    = persistence.SessionContext
	ErrorHistoryEntry = persistence.ErrorHistoryEntry
	StoreStats        = persistence.StoreStats
	SessionStoreArgs  = persistence.SessionStoreArgs
)

func NewSessionStore(projectPath string) (*SessionStore, error) {
	return persistence.NewSessionStore(projectPath)
}

func NewSessionStoreWithInterval(projectPath string, flushInterval time.Duration) (*SessionStore, error) {
	return persistence.NewSessionStoreWithInterval(projectPath, flushInterval)
}
