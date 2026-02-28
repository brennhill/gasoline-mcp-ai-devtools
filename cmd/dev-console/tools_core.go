// tools_core.go — ToolHandler struct and shared infrastructure for MCP tool dispatch.

package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/ai"
	"github.com/dev-console/dev-console/internal/analysis"
	"github.com/dev-console/dev-console/internal/audit"
	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/mcp"
	"github.com/dev-console/dev-console/internal/redaction"
	"github.com/dev-console/dev-console/internal/security"
	"github.com/dev-console/dev-console/internal/session"
	"github.com/dev-console/dev-console/internal/streaming"
)

// defaultColdStartTimeout is how long requireExtension waits for the extension
// to connect during a cold start before returning an error.
// This eliminates "no_data" failures when the LLM sends a command before the
// extension's first /sync heartbeat arrives.
// Note: MaybeWaitForCommand does only an instant IsExtensionConnected() check;
// the blocking wait is exclusively in requireExtension (P1-2: no double wait).
const defaultColdStartTimeout = 5 * time.Second

// ============================================
// Shared Utilities
// ============================================

// randomInt63 generates a random int64 for correlation IDs using crypto/rand.
func randomInt63() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback to time-based if rand fails (should never happen)
		return time.Now().UnixNano()
	}
	return int64(binary.BigEndian.Uint64(b[:]) & 0x7FFFFFFFFFFFFFFF)
}

// ============================================
// MCP Typed Response Structs (aliases to internal/mcp)
// ============================================

type MCPContentBlock = mcp.MCPContentBlock
type MCPToolResult = mcp.MCPToolResult
type MCPInitializeResult = mcp.MCPInitializeResult
type MCPServerInfo = mcp.MCPServerInfo
type MCPCapabilities = mcp.MCPCapabilities
type MCPToolsCapability = mcp.MCPToolsCapability
type MCPResourcesCapability = mcp.MCPResourcesCapability
type MCPResource = mcp.MCPResource
type MCPResourcesListResult = mcp.MCPResourcesListResult
type MCPResourceContent = mcp.MCPResourceContent
type MCPResourcesReadResult = mcp.MCPResourcesReadResult
type MCPToolsListResult = mcp.MCPToolsListResult
type MCPResourceTemplatesListResult = mcp.MCPResourceTemplatesListResult

// ============================================
// Tool Call Rate Limiter
// ============================================

// ToolCallLimiter implements a sliding window rate limiter for MCP tool calls.
// Thread-safe: uses its own mutex independent of other locks.
type ToolCallLimiter struct {
	mu         sync.Mutex
	timestamps []time.Time
	maxCalls   int
	window     time.Duration
}

// NewToolCallLimiter creates a rate limiter allowing maxCalls within the given window.
func NewToolCallLimiter(maxCalls int, window time.Duration) *ToolCallLimiter {
	return &ToolCallLimiter{
		timestamps: make([]time.Time, 0, maxCalls),
		maxCalls:   maxCalls,
		window:     window,
	}
}

// Allow checks if a new call is permitted. If allowed, records it and returns true.
func (l *ToolCallLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	// Compact: remove expired timestamps
	valid := 0
	for _, ts := range l.timestamps {
		if ts.After(cutoff) {
			l.timestamps[valid] = ts
			valid++
		}
	}
	l.timestamps = l.timestamps[:valid]

	if len(l.timestamps) >= l.maxCalls {
		return false
	}

	l.timestamps = append(l.timestamps, now)
	return true
}

// Note: Response helpers, error codes, and validation functions have been moved to:
// - tools_response.go — Response formatting helpers
// - tools_errors.go — Error codes and structured error handling
// - tools_validation.go — Parameter validation utilities

// ============================================
// ToolHandler Definition
// ============================================

// ToolHandler extends MCPHandler with composite tool dispatch
type ToolHandler struct {
	*MCPHandler
	capture *capture.Capture

	// shutdownCtx is cancelled when the ToolHandler is closed. Gates like
	// requireExtension pass this context to blocking waits so they abort
	// promptly on server shutdown instead of leaking goroutines.
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc

	// Health metrics for MCP get_health tool
	healthMetrics *HealthMetrics

	// Redaction engine for scrubbing sensitive data from tool responses
	redactionEngine RedactionEngine

	// Rate limiter for MCP tool calls (sliding window)
	toolCallLimiter *ToolCallLimiter

	// Alert system + context streaming (delegates to internal/streaming)
	alertBuffer *streaming.AlertBuffer

	// Concrete implementations (interface signatures differ from types package)
	// These are used directly by tool handlers rather than through the interface fields above.
	noiseConfig           *ai.NoiseConfig
	sessionStoreImpl      *ai.SessionStore
	securityScannerImpl   *security.SecurityScanner
	thirdPartyAuditorImpl *analysis.ThirdPartyAuditor
	sessionManager        *session.SessionManager
	auditTrail            *audit.AuditTrail

	// Per-client audit session mapping (client_id -> session_id).
	auditMu         sync.Mutex
	auditSessionMap map[string]string

	// Draw mode annotation store (in-memory, TTL-based)
	annotationStore *AnnotationStore

	// API contract validation state (incremental over captured network bodies).
	apiContractMu        sync.Mutex
	apiContractValidator *analysis.APIContractValidator
	apiContractOffset    int

	// Upload security config (folder-scoped permissions + denylist)
	uploadSecurity *UploadSecurity

	// Cold-start readiness gate timeout: how long requireExtension waits
	// for the extension to connect before failing. MaybeWaitForCommand only
	// does an instant check (P1-2: no double wait).
	// Default: 5s. Set to 0 in tests to restore instant-fail behavior.
	coldStartTimeout time.Duration

	// Cached interact dispatch map (initialized once via sync.Once)
	interactOnce     sync.Once
	interactHandlers map[string]interactHandler

	// Scoped element index registry used by list_interactive/index follow-up actions.
	elementIndexRegistry *elementIndexRegistry

	// Active test boundaries: test_id → start time.
	// Used to detect out-of-order test_boundary_end calls.
	activeBoundariesMu sync.Mutex
	activeBoundaries   map[string]time.Time

	// Playback results store: recording_id → session after playback completes.
	playbackMu       sync.RWMutex
	playbackSessions map[string]*capture.PlaybackSession

	// Interact recording state gate (record_start/record_stop sequencing).
	recordInteractMu sync.Mutex
	recordInteract   interactRecordingState

	// Optional evidence capture state keyed by correlation_id.
	// Tracks before/after screenshots for interact actions when evidence mode is enabled.
	evidenceMu        sync.Mutex
	evidenceByCommand map[string]*commandEvidenceState

	// Deterministic retry contract metadata keyed by correlation_id.
	retryContractMu sync.Mutex
	retryByCommand  map[string]*commandRetryState

	// Passive network traffic recording state (start/stop capture).
	networkRecording *networkRecordingState

	// Action jitter: randomized micro-delays before interact actions.
	jitterMu          sync.RWMutex
	actionJitterMaxMs int // max jitter before each interact action (0 = disabled)

	// Module registry for plugin-style tool dispatch (incremental migration).
	toolModulesOnce sync.Once
	toolModules     *toolModuleRegistry

	// Tool schema cache for parameter-warning validation.
	toolSchemasOnce sync.Once
	toolSchemas     map[string]map[string]any

	// Session-level summary preference cache.
	summaryPrefMu    sync.RWMutex
	summaryPrefValue bool
	summaryPrefReady bool

	// extensionReadinessTimeout overrides the cold-start wait duration for requireExtension.
	// Zero uses capture.ExtensionReadinessTimeout (5s). Tests set this to 100ms.
	extensionReadinessTimeout time.Duration

	// noiseFirstConnectFn overrides the noise auto-detect function for first-connection.
	// When nil, runNoiseAutoDetect() is used. Set in tests to inject counting stubs.
	noiseFirstConnectFn func()
}

// Close cancels the shutdown context, unblocking any in-flight readiness gates.
func (h *ToolHandler) Close() {
	if h.shutdownCancel != nil {
		h.shutdownCancel()
	}
}

// GetCapture returns the capture instance
func (h *ToolHandler) GetCapture() *capture.Capture {
	return h.capture
}

// GetLogEntries returns a snapshot of the server's log entries and their timestamps.
// The returned slices are copies — safe to use without holding the server lock.
func (h *ToolHandler) GetLogEntries() ([]LogEntry, []time.Time) {
	h.server.mu.RLock()
	defer h.server.mu.RUnlock()
	entries := make([]LogEntry, len(h.server.entries))
	copy(entries, h.server.entries)
	addedAt := make([]time.Time, len(h.server.logAddedAt))
	copy(addedAt, h.server.logAddedAt)
	return entries, addedAt
}

// GetLogTotalAdded returns the monotonic counter of total log entries ever added.
func (h *ToolHandler) GetLogTotalAdded() int64 {
	h.server.mu.RLock()
	defer h.server.mu.RUnlock()
	return h.server.logTotalAdded
}

// GetAnnotationStore returns the annotation store for draw mode data.
func (h *ToolHandler) GetAnnotationStore() *AnnotationStore {
	return h.annotationStore
}

// GetToolCallLimiter returns the tool call limiter
func (h *ToolHandler) GetToolCallLimiter() RateLimiter {
	return h.toolCallLimiter
}

// GetRedactionEngine returns the redaction engine
func (h *ToolHandler) GetRedactionEngine() RedactionEngine {
	return h.redactionEngine
}

// newPlaybackSessionsMap returns an initialized playback sessions map.
// Separated to avoid the parameter name "capture" shadowing the package import.
func newPlaybackSessionsMap() map[string]*capture.PlaybackSession {
	return make(map[string]*capture.PlaybackSession)
}

// NewToolHandler creates an MCP handler with composite tool capabilities
func NewToolHandler(server *Server, capture *capture.Capture) *MCPHandler {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	handler := &ToolHandler{
		MCPHandler:           NewMCPHandler(server, version),
		capture:              capture,
		shutdownCtx:          shutdownCtx,
		shutdownCancel:       shutdownCancel,
		coldStartTimeout:     defaultColdStartTimeout,
		playbackSessions:     newPlaybackSessionsMap(),
		evidenceByCommand:    make(map[string]*commandEvidenceState),
		retryByCommand:       make(map[string]*commandRetryState),
		elementIndexRegistry: newElementIndexRegistry(),
		networkRecording:     &networkRecordingState{},
	}

	// Initialize health metrics
	handler.healthMetrics = NewHealthMetrics()
	handler.toolCallLimiter = NewToolCallLimiter(500, time.Minute)
	handler.alertBuffer = streaming.NewAlertBuffer()

	// Initialize session store (use current working directory as project path)
	cwd, err := os.Getwd()
	if err == nil {
		if store, err := ai.NewSessionStore(cwd); err == nil {
			handler.sessionStoreImpl = store
		}
	}

	// Initialize noise filtering with persistence support
	if handler.sessionStoreImpl != nil {
		handler.noiseConfig = ai.NewNoiseConfigWithStore(handler.sessionStoreImpl)
	} else {
		handler.noiseConfig = ai.NewNoiseConfig()
	}
	handler.redactionEngine = redaction.NewRedactionEngine("")

	// Use server-scoped annotation store for draw mode.
	handler.annotationStore = server.getAnnotationStore()

	// Wire async annotation waiter → CommandTracker completion
	if handler.capture != nil {
		handler.annotationStore.SetCommandCompleter(func(correlationID string, result json.RawMessage) {
			handler.capture.CompleteCommand(correlationID, result, "")
		})
	}

	// Wire automatic noise detection after page navigations
	wireNoiseAutoDetect(handler)

	// Wire automatic noise detection on first extension connection (#264)
	wireNoiseFirstConnect(handler)

	// Initialize security tools (concrete types - interface signatures differ)
	handler.securityScannerImpl = security.NewSecurityScanner()
	handler.thirdPartyAuditorImpl = analysis.NewThirdPartyAuditor()
	handler.apiContractValidator = analysis.NewAPIContractValidator()
	handler.sessionManager = session.NewSessionManager(10, newToolCaptureStateReader(handler))
	handler.auditTrail = audit.NewAuditTrail(audit.AuditConfig{
		MaxEntries:   10000,
		Enabled:      true,
		RedactParams: true,
	})
	handler.auditSessionMap = make(map[string]string)

	// Initialize upload security config from package-level var set by CLI
	handler.uploadSecurity = uploadSecurityConfig

	// Initialize dispatch modules and tool schemas once at startup.
	handler.ensureToolModules()
	handler.ensureToolSchemas()

	// Wire error clustering: feed error-level log entries into the cluster manager.
	// Use SetOnEntries for thread-safe assignment (avoids racing with addEntries).
	// Error clustering disabled for now (not initialized)

	// Return as MCPHandler but with overridden methods via the wrapper
	return &MCPHandler{
		server:      server,
		toolHandler: handler,
	}
}

// maybeWaitForCommand, formatCommandResult, and related async infrastructure
// moved to tools_async.go

// handleToolCall dispatches composite tool calls by mode parameter.
func (h *ToolHandler) HandleToolCall(req JSONRPCRequest, name string, args json.RawMessage) (JSONRPCResponse, bool) {
	start := time.Now()

	h.ensureToolModules()
	h.ensureToolSchemas()
	resp, handled := h.dispatchViaModules(req, name, args)
	if !handled {
		return JSONRPCResponse{}, false
	}

	parsedResult, parsedOK := parseToolResultForPostProcessing(resp.Result)
	resultIsError := false
	if parsedOK {
		resultIsError = parsedResult.IsError
	} else {
		resultIsError = isToolResultError(resp.Result)
	}

	// Validate params against tool schema and append warnings for unknown fields.
	// Skip validation for error responses (already failed, warnings would be noise).
	if !resultIsError {
		if schema := h.getToolSchema(name); schema != nil {
			if warnings := validateParamsAgainstSchema(args, schema); len(warnings) > 0 {
				if parsedOK && mcp.AppendWarningsToToolResult(parsedResult, warnings) {
					resp.Result = safeMarshal(parsedResult, string(resp.Result))
				} else {
					resp = appendWarningsToResponse(resp, warnings)
				}
			}
		}
	}

	if h.healthMetrics != nil {
		h.healthMetrics.IncrementRequest(name)
		if resp.Error != nil || resultIsError {
			h.healthMetrics.IncrementError(name)
		}
	}

	h.recordAuditToolCall(req, name, args, resp, start)

	return resp, true
}

// getToolSchema returns the InputSchema for a tool by name (cached).
func (h *ToolHandler) getToolSchema(name string) map[string]any {
	h.ensureToolSchemas()
	return h.toolSchemas[name]
}

func (h *ToolHandler) ensureToolModules() {
	h.toolModulesOnce.Do(func() {
		h.toolModules = h.buildToolModuleRegistry()
	})
}

func (h *ToolHandler) ensureToolSchemas() {
	h.toolSchemasOnce.Do(func() {
		h.toolSchemas = make(map[string]map[string]any)
		for _, tool := range h.ToolsList() {
			h.toolSchemas[tool.Name] = tool.InputSchema
		}
	})
}

func parseToolResultForPostProcessing(raw json.RawMessage) (*MCPToolResult, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var result MCPToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, false
	}
	return &result, true
}

func isToolResultError(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var result MCPToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return false
	}
	return result.IsError
}
