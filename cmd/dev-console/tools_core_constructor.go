// Purpose: ToolHandler constructor and startup-time defaults.
// Why: Isolates initialization policy from dispatch/type definitions.

package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/analysis"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/audit"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/noise"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/persistence"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/redaction"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/security"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/session"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/streaming"
)

// defaultColdStartTimeout is how long requireExtension waits for the extension
// to connect during a cold start before returning an error.
// This eliminates "no_data" failures when the LLM sends a command before the
// extension's first /sync heartbeat arrives.
// Note: MaybeWaitForCommand does only an instant IsExtensionConnected() check;
// the blocking wait is exclusively in requireExtension (P1-2: no double wait).
const defaultColdStartTimeout = 5 * time.Second

// testExtensionReadinessTimeout keeps extension-gate failures fast in unit tests.
// Production remains at capture.ExtensionReadinessTimeout (5s).
const testExtensionReadinessTimeout = 1 * time.Millisecond

func defaultExtensionReadinessTimeout() time.Duration {
	if strings.HasSuffix(os.Args[0], ".test") {
		return testExtensionReadinessTimeout
	}
	return capture.ExtensionReadinessTimeout
}

// NewToolHandler creates an MCP handler with composite tool capabilities.
func NewToolHandler(server *Server, capture *capture.Store) *MCPHandler {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	handler := &ToolHandler{
		MCPHandler:                NewMCPHandler(server, version),
		capture:                   capture,
		shutdownCtx:               shutdownCtx,
		shutdownCancel:            shutdownCancel,
		coldStartTimeout:          defaultColdStartTimeout,
		extensionReadinessTimeout: defaultExtensionReadinessTimeout(),
		playbackSessions:          newPlaybackSessionsMap(),
		evidenceByCommand:         make(map[string]*commandEvidenceState),
		retryByCommand:            make(map[string]*commandRetryState),
		elementIndexRegistry:      newElementIndexRegistry(),
		networkRecording:          &networkRecordingState{},
	}

	// Initialize health metrics.
	handler.healthMetrics = NewHealthMetrics()
	handler.toolCallLimiter = NewToolCallLimiter(500, time.Minute)
	handler.alertBuffer = streaming.NewAlertBuffer()

	// Initialize session store (use current working directory as project path).
	cwd, err := os.Getwd()
	if err == nil {
		if store, err := persistence.NewSessionStore(cwd); err == nil {
			handler.sessionStoreImpl = store
		}
	}

	// Initialize noise filtering with persistence support.
	if handler.sessionStoreImpl != nil {
		handler.noiseConfig = noise.NewNoiseConfigWithStore(handler.sessionStoreImpl)
	} else {
		handler.noiseConfig = noise.NewNoiseConfig()
	}
	handler.redactionEngine = redaction.NewRedactionEngine("")

	// Use server-scoped annotation store for draw mode.
	handler.annotationStore = server.getAnnotationStore()

	// Wire async annotation waiter → CommandTracker completion.
	if handler.capture != nil {
		handler.annotationStore.SetCommandCompleter(func(correlationID string, result json.RawMessage) {
			handler.capture.CompleteCommand(correlationID, result, "")
		})
	}

	// Wire automatic noise detection hooks.
	wireNoiseAutoDetect(handler)
	wireNoiseFirstConnect(handler)

	// Initialize security and audit tools.
	handler.securityScannerImpl = security.NewSecurityScanner()
	handler.thirdPartyAuditorImpl = analysis.NewThirdPartyAuditor()
	handler.apiContractValidator = analysis.NewAPIContractValidator()
	handler.sessionManager = session.NewSessionManager(10, newToolCaptureStateReader(handler))
	handler.auditTrail = audit.NewAuditTrail(audit.Config{
		MaxEntries:   10000,
		Enabled:      true,
		RedactParams: true,
	})
	handler.auditSessionMap = make(map[string]string)

	// Initialize upload security config from package-level var set by CLI.
	handler.uploadSecurity = uploadSecurityConfig
	handler.recordingInteractHandler = newRecordingInteractHandler(handler)
	handler.uploadInteractHandler = newUploadInteractHandler(handler)
	handler.testGenHandler = newTestGenHandler(handler)

	// Initialize dispatch modules and tool schemas once at startup.
	handler.ensureToolModules()
	handler.ensureToolSchemas()

	// Return as MCPHandler but with overridden methods via the wrapper.
	return &MCPHandler{
		server:      server,
		toolHandler: handler,
	}
}
