// SSE transport infrastructure for MCP
// Manages Server-Sent Events connections, session state, and streaming notifications.
// Supports MCP 2024-11-05 spec with bidirectional communication via SSE + HTTP POST.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// SSEConnection represents an active SSE client connection
type SSEConnection struct {
	SessionID    string
	ClientID     string
	Writer       http.ResponseWriter
	Flusher      http.Flusher
	Context      context.Context
	ConnectedAt  time.Time
	LastActivity time.Time
	mu           sync.Mutex
}

// SSERegistry manages active SSE connections
type SSERegistry struct {
	connections map[string]*SSEConnection
	mu          sync.RWMutex
}

// generateSessionID creates a new random session ID
func generateSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to time-based ID if random fails
		return fmt.Sprintf("session-%d", time.Now().UnixNano())
	}
	return "session-" + hex.EncodeToString(b)
}

// NewSSERegistry creates a new SSE connection registry
func NewSSERegistry() *SSERegistry {
	registry := &SSERegistry{
		connections: make(map[string]*SSEConnection),
	}
	// Start cleanup goroutine
	go registry.cleanupStaleConnections()
	return registry
}

// Register adds a new SSE connection to the registry
func (r *SSERegistry) Register(sessionID, clientID string, w http.ResponseWriter, req *http.Request) (*SSEConnection, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("SSE not supported: ResponseWriter does not implement http.Flusher")
	}

	conn := &SSEConnection{
		SessionID:    sessionID,
		ClientID:     clientID,
		Writer:       w,
		Flusher:      flusher,
		Context:      req.Context(),
		ConnectedAt:  time.Now(),
		LastActivity: time.Now(),
	}

	r.mu.Lock()
	r.connections[sessionID] = conn
	r.mu.Unlock()

	return conn, nil
}

// Unregister removes an SSE connection from the registry
func (r *SSERegistry) Unregister(sessionID string) {
	r.mu.Lock()
	delete(r.connections, sessionID)
	r.mu.Unlock()
}

// Get retrieves an SSE connection by session ID
func (r *SSERegistry) Get(sessionID string) (*SSEConnection, bool) {
	r.mu.RLock()
	conn, exists := r.connections[sessionID]
	r.mu.RUnlock()
	return conn, exists
}

// SendMessage writes an SSE message event to a specific session
func (r *SSERegistry) SendMessage(sessionID string, data any) error {
	conn, exists := r.Get(sessionID)
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return conn.WriteEvent("message", string(jsonData))
}

// BroadcastNotification sends an MCP notification to all connected clients
func (r *SSERegistry) BroadcastNotification(notification any) {
	jsonData, err := json.Marshal(notification)
	if err != nil {
		// Log error but don't fail broadcast
		fmt.Fprintf(io.Discard, "failed to marshal notification: %v\n", err)
		return
	}

	r.mu.RLock()
	conns := make([]*SSEConnection, 0, len(r.connections))
	for _, conn := range r.connections {
		conns = append(conns, conn)
	}
	r.mu.RUnlock()

	// Broadcast to all connections outside the read lock
	for _, conn := range conns {
		_ = conn.WriteEvent("message", string(jsonData))
	}
}

// WriteEvent writes an SSE event to the connection
func (c *SSEConnection) WriteEvent(event, data string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if context is cancelled (client disconnected)
	select {
	case <-c.Context.Done():
		return fmt.Errorf("connection closed")
	default:
	}

	formatted := formatSSEEvent(event, data)
	_, err := c.Writer.Write([]byte(formatted))
	if err != nil {
		return err
	}

	c.Flusher.Flush()
	c.LastActivity = time.Now()
	return nil
}

// formatSSEEvent formats data as an SSE event
func formatSSEEvent(event, data string) string {
	var b strings.Builder
	b.WriteString("event: ")
	b.WriteString(event)
	b.WriteString("\n")

	// Handle multi-line data
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		b.WriteString("data: ")
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	return b.String()
}

// cleanupStaleConnections removes connections that have been idle too long
func (r *SSERegistry) cleanupStaleConnections() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		r.mu.Lock()
		now := time.Now()
		for sessionID, conn := range r.connections {
			// Remove connections idle for > 1 hour
			if now.Sub(conn.LastActivity) > 1*time.Hour {
				delete(r.connections, sessionID)
			}
		}
		r.mu.Unlock()
	}
}

// handleMCPSSE handles GET /mcp/sse - establishes SSE connection
func handleMCPSSE(registry *SSERegistry, server *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		// Generate unique session ID (reuses existing function from audit_trail.go)
		sessionID := generateSessionID()

		// Extract client ID from header (optional)
		clientID := r.Header.Get("X-Gasoline-Client")
		if clientID == "" {
			clientID = "default"
		}

		// Register connection
		conn, err := registry.Register(sessionID, clientID, w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		defer registry.Unregister(sessionID)

		// Log connection
		_ = server.appendToFile([]LogEntry{{
			"type":       "lifecycle",
			"event":      "mcp_sse_connect",
			"session_id": sessionID,
			"client_id":  clientID,
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		}})

		// Send endpoint event with POST URI
		endpointData := map[string]string{
			"uri": "/mcp/messages/" + sessionID,
		}
		endpointJSON, _ := json.Marshal(endpointData)
		if err := conn.WriteEvent("endpoint", string(endpointJSON)); err != nil {
			return
		}

		// Block until client disconnects
		<-r.Context().Done()

		// Log disconnection
		_ = server.appendToFile([]LogEntry{{
			"type":       "lifecycle",
			"event":      "mcp_sse_disconnect",
			"session_id": sessionID,
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		}})
	}
}

// handleMCPMessages handles POST /mcp/messages/{session-id} - client requests
func handleMCPMessages(registry *SSERegistry, mcp *MCPHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract session ID from URL path
		path := strings.TrimPrefix(r.URL.Path, "/mcp/messages/")
		sessionID := strings.Split(path, "/")[0]

		// Log all requests to this endpoint
		fmt.Fprintf(os.Stderr, "[gasoline] POST /mcp/messages/%s - Method: %s\n", sessionID, r.Method)

		if sessionID == "" {
			http.Error(w, "Missing session ID", http.StatusBadRequest)
			return
		}

		// Verify session exists
		if _, exists := registry.Get(sessionID); !exists {
			fmt.Fprintf(os.Stderr, "[gasoline] ERROR: Session not found: %s\n", sessionID)
			http.Error(w, "Invalid session ID", http.StatusNotFound)
			return
		}

		fmt.Fprintf(os.Stderr, "[gasoline] Processing MCP request for session: %s\n", sessionID)

		// Read JSON-RPC request
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			// Send JSON-RPC parse error via SSE
			errResp := JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error: &JSONRPCError{
					Code:    -32700,
					Message: "Parse error: " + err.Error(),
				},
			}
			_ = registry.SendMessage(sessionID, errResp)
			w.WriteHeader(http.StatusAccepted)
			return
		}

		// Extract client ID from header
		clientID := r.Header.Get("X-Gasoline-Client")
		if clientID != "" {
			req.ClientID = clientID
		}

		// Process request
		resp := mcp.HandleRequest(req)

		// Send response via SSE
		if err := registry.SendMessage(sessionID, resp); err != nil {
			http.Error(w, "Failed to send response", http.StatusInternalServerError)
			return
		}

		// Acknowledge receipt
		w.WriteHeader(http.StatusAccepted)
	}
}
