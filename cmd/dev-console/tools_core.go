// tools_core.go — Core MCP tool types, constants, and response helpers.
// This file contains the foundational pieces used by all tool handlers:
// - MCP typed response structs
// - Tool call rate limiter
// - Response helpers (mcpTextResponse, mcpJSONResponse, mcpStructuredError)
// - Error codes and StructuredError type
// - Unknown parameter warning helpers
// - ToolHandler struct definition and constructor
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
// Deviations from snake_case MUST be tagged with // SPEC:<spec-name> at the field level.
// SPEC:MCP — Fields in this file use camelCase where required by the MCP protocol spec.
package main

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/ai"
	"github.com/dev-console/dev-console/internal/analysis"
	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/security"
)

// ============================================
// MCP Typed Response Structs
// ============================================

// MCPContentBlock represents a single content block in an MCP tool result.
type MCPContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MCPToolResult represents the result of an MCP tool call.
type MCPToolResult struct {
	Content []MCPContentBlock `json:"content"`
	IsError bool              `json:"isError"` // SPEC:MCP
}

// MCPInitializeResult represents the result of an MCP initialize request.
type MCPInitializeResult struct {
	ProtocolVersion string          `json:"protocolVersion"` // SPEC:MCP
	ServerInfo      MCPServerInfo   `json:"serverInfo"`      // SPEC:MCP
	Capabilities    MCPCapabilities `json:"capabilities"`
	Instructions    string          `json:"instructions,omitempty"`
}

// MCPServerInfo identifies the MCP server.
type MCPServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPCapabilities declares the server's MCP capabilities.
type MCPCapabilities struct {
	Tools     MCPToolsCapability     `json:"tools"`
	Resources MCPResourcesCapability `json:"resources"`
}

// MCPToolsCapability declares tool support.
type MCPToolsCapability struct{}

// MCPResourcesCapability declares resource support.
type MCPResourcesCapability struct{}

// MCPResource describes an available resource.
type MCPResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"` // SPEC:MCP
}

// MCPResourcesListResult represents the result of a resources/list request.
type MCPResourcesListResult struct {
	Resources []MCPResource `json:"resources"`
}

// MCPResourceContent represents the content of a resource.
type MCPResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"` // SPEC:MCP
	Text     string `json:"text,omitempty"`
}

// MCPResourcesReadResult represents the result of a resources/read request.
type MCPResourcesReadResult struct {
	Contents []MCPResourceContent `json:"contents"`
}

// MCPToolsListResult represents the result of a tools/list request.
type MCPToolsListResult struct {
	Tools []MCPTool `json:"tools"`
}

// MCPResourceTemplatesListResult represents the result of a resources/templates/list request.
type MCPResourceTemplatesListResult struct {
	ResourceTemplates []any `json:"resourceTemplates"` // SPEC:MCP
}

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

	// Health metrics for MCP get_health tool
	healthMetrics *HealthMetrics

	// Redaction engine for scrubbing sensitive data from tool responses
	redactionEngine *RedactionEngine

	// Rate limiter for MCP tool calls (sliding window)
	toolCallLimiter *ToolCallLimiter

	// Context streaming: active push notifications via MCP
	streamState *StreamState

	// Alert buffer state (local management)
	alertMu   sync.Mutex
	alerts    []Alert
	ciResults []CIResult
	// Anomaly detection: sliding window error counter
	errorTimes []time.Time

	// Concrete implementations (interface signatures differ from types package)
	// These are used directly by tool handlers rather than through the interface fields above.
	noiseConfig           *ai.NoiseConfig
	sessionStoreImpl      *ai.SessionStore
	securityScannerImpl   *security.SecurityScanner
	thirdPartyAuditorImpl *analysis.ThirdPartyAuditor

	// Draw mode annotation store (in-memory, TTL-based)
	annotationStore *AnnotationStore

	// Upload automation security flags (disabled by default)
	uploadAutomationEnabled bool            // --enable-upload-automation
	uploadSecurity          *UploadSecurity // folder-scoped permissions + denylist

	// Cached interact dispatch map (initialized once via sync.Once)
	interactOnce     sync.Once
	interactHandlers map[string]interactHandler
}

// GetCapture returns the capture instance
func (h *ToolHandler) GetCapture() *capture.Capture {
	return h.capture
}

// GetToolCallLimiter returns the tool call limiter
func (h *ToolHandler) GetToolCallLimiter() RateLimiter {
	return h.toolCallLimiter
}

// GetRedactionEngine returns the redaction engine
func (h *ToolHandler) GetRedactionEngine() RedactionEngine {
	if h.redactionEngine != nil {
		return *h.redactionEngine
	}
	return nil
}

// NewToolHandler creates an MCP handler with composite tool capabilities
func NewToolHandler(server *Server, capture *capture.Capture) *MCPHandler {
	handler := &ToolHandler{
		MCPHandler: NewMCPHandler(server, version),
		capture:    capture,
	}

	// Initialize health metrics
	handler.healthMetrics = NewHealthMetrics()
	handler.toolCallLimiter = NewToolCallLimiter(500, time.Minute)
	handler.streamState = NewStreamState()

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

	// Use global annotation store for draw mode
	handler.annotationStore = globalAnnotationStore

	// Wire async annotation waiter → CommandTracker completion
	if handler.capture != nil {
		handler.annotationStore.SetCommandCompleter(func(correlationID string, result json.RawMessage) {
			handler.capture.CompleteCommand(correlationID, result, "")
		})
	}

	// Initialize security tools (concrete types - interface signatures differ)
	handler.securityScannerImpl = security.NewSecurityScanner()
	handler.thirdPartyAuditorImpl = analysis.NewThirdPartyAuditor()

	// Initialize upload automation flags from package-level vars set by CLI
	handler.uploadAutomationEnabled = uploadAutomationFlag
	handler.uploadSecurity = uploadSecurityConfig

	// Wire error clustering: feed error-level log entries into the cluster manager.
	// Use SetOnEntries for thread-safe assignment (avoids racing with addEntries).
	// Error clustering disabled for now (not initialized)

	// Return as MCPHandler but with overridden methods via the wrapper
	return &MCPHandler{
		server:      server,
		toolHandler: handler,
	}
}

// handleToolCall dispatches composite tool calls by mode parameter.
func (h *ToolHandler) HandleToolCall(req JSONRPCRequest, name string, args json.RawMessage) (JSONRPCResponse, bool) {
	switch name {
	case "observe":
		return h.toolObserve(req, args), true
	case "analyze":
		return h.toolAnalyze(req, args), true
	case "generate":
		return h.toolGenerate(req, args), true
	case "configure":
		return h.toolConfigure(req, args), true
	case "interact":
		return h.toolInteract(req, args), true
	}
	return JSONRPCResponse{}, false
}
