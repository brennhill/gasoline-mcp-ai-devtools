// terminal_handlers.go — HTTP handlers for the in-browser terminal.
// Why: Isolates terminal WebSocket upgrade, session lifecycle, and static asset serving
// from the main route wiring for maintainability and test focus.

package main

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/pty"
)

// terminalWSIdleTimeout is the idle timeout for terminal WebSocket connections.
// Longer than the test harness since terminal sessions are long-lived.
const terminalWSIdleTimeout = 5 * time.Minute

// terminalReadBufSize is the buffer size for PTY reads relayed to the browser.
const terminalReadBufSize = 4096

// registerTerminalRoutes adds terminal-related routes to the mux.
// NOT MCP — These are daemon-served endpoints for the in-browser terminal.
func registerTerminalRoutes(mux *http.ServeMux, mgr *pty.Manager, cap *capture.Store) {
	// Serve terminal HTML page.
	mux.HandleFunc("/terminal", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handleTerminalPage(w, r)
	}))

	// Serve xterm.js and other static assets.
	staticFS, err := fs.Sub(terminalAssetsFS, "terminal_assets")
	if err != nil {
		stderrf("[gasoline] failed to create terminal static FS: %v\n", err)
		return
	}
	mux.Handle("/terminal/static/", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/terminal/static")
		http.FileServer(http.FS(staticFS)).ServeHTTP(w, r)
	}))

	// WebSocket upgrade for PTY I/O.
	mux.HandleFunc("/terminal/ws", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handleTerminalWS(w, r, mgr)
	}))

	// Session lifecycle.
	mux.HandleFunc("/terminal/start", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handleTerminalStart(w, r, mgr, cap)
	}))
	mux.HandleFunc("/terminal/stop", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handleTerminalStop(w, r, mgr)
	}))

	// Session configuration.
	mux.HandleFunc("/terminal/config", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handleTerminalConfig(w, r, mgr)
	}))
}

// handleTerminalPage serves the terminal HTML page.
func handleTerminalPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}
	data, err := terminalAssetsFS.ReadFile("terminal_assets/terminal.html")
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "failed to read terminal page"})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// handleTerminalWS upgrades a GET /terminal/ws request to a WebSocket connection
// that relays raw PTY I/O to/from the browser's xterm.js terminal emulator.
func handleTerminalWS(w http.ResponseWriter, r *http.Request, mgr *pty.Manager) {
	token := r.URL.Query().Get("token")
	if token == "" {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "missing token"})
		return
	}

	sess, err := mgr.GetByToken(token)
	if err != nil {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" || strings.ToLower(r.Header.Get("Upgrade")) != "websocket" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "websocket upgrade required"})
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "server does not support hijacking"})
		return
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer conn.Close()

	// Send 101 handshake.
	accept := wsAcceptKey(key)
	handshake := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	if _, err := bufrw.WriteString(handshake); err != nil {
		return
	}
	if err := bufrw.Flush(); err != nil {
		return
	}

	terminalWSLoop(conn, bufrw, sess)
}

// terminalWSLoop relays data between a WebSocket connection and a PTY session.
// Downstream (PTY → browser): binary WebSocket frames with raw terminal output.
// Upstream (browser → PTY): binary frames as keystrokes, text frames as JSON control messages.
func terminalWSLoop(conn net.Conn, rw *bufio.ReadWriter, sess *pty.Session) {
	// PTY → WebSocket (downstream): read PTY output and send as binary frames.
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, terminalReadBufSize)
		for {
			n, err := sess.Read(buf)
			if err != nil {
				// PTY closed or process exited — send close frame.
				_ = wsWriteFrame(rw, 0x8, nil)
				return
			}
			if n > 0 {
				if err := wsWriteFrame(rw, 0x2, buf[:n]); err != nil {
					return
				}
			}
		}
	}()

	// WebSocket → PTY (upstream): read frames and dispatch.
	go func() {
		for {
			_ = conn.SetReadDeadline(time.Now().Add(terminalWSIdleTimeout))

			fin, opcode, payload, err := wsReadFrame(rw)
			if err != nil {
				sess.Close()
				return
			}
			_ = fin // Terminal messages are not fragmented in practice.

			switch {
			case opcode == 0x8: // Close
				_ = wsWriteFrame(rw, 0x8, nil)
				sess.Close()
				return
			case opcode == 0x9: // Ping → Pong
				_ = wsWriteFrame(rw, 0xA, payload)
			case opcode == 0x2: // Binary — raw keystrokes → PTY stdin
				if _, err := sess.Write(payload); err != nil {
					return
				}
			case opcode == 0x1: // Text — JSON control message
				handleTerminalControlMessage(payload, sess)
			}
		}
	}()

	<-done
}

// terminalControlMessage is a JSON control message from the browser terminal.
type terminalControlMessage struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// handleTerminalControlMessage processes a JSON control message from the browser.
func handleTerminalControlMessage(payload []byte, sess *pty.Session) {
	var msg terminalControlMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return
	}
	switch msg.Type {
	case "resize":
		if msg.Cols > 0 && msg.Rows > 0 {
			_ = sess.Resize(uint16(msg.Cols), uint16(msg.Rows))
		}
	}
}

// handleTerminalStart creates a new terminal session.
func handleTerminalStart(w http.ResponseWriter, r *http.Request, mgr *pty.Manager, cap *capture.Store) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)

	var req struct {
		ID   string   `json:"id"`
		Cmd  string   `json:"cmd"`
		Args []string `json:"args"`
		Dir  string   `json:"dir"`
		Cols int      `json:"cols"`
		Rows int      `json:"rows"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	// Default to shell if no command specified.
	if req.Cmd == "" {
		req.Cmd = "/bin/zsh"
	}

	// Auto-detect CWD from client registry if not specified.
	if req.Dir == "" && cap != nil {
		req.Dir = autoDetectCWD(cap)
	}

	result, err := mgr.Start(pty.StartConfig{
		ID:   req.ID,
		Cmd:  req.Cmd,
		Args: req.Args,
		Dir:  req.Dir,
		Cols: uint16(req.Cols),
		Rows: uint16(req.Rows),
	})
	if err != nil {
		// Detect macOS sandbox restriction (MCP stdio-spawned daemon can't fork).
		if isSandboxError(err) {
			jsonResponse(w, http.StatusServiceUnavailable, map[string]any{
				"error":       "sandbox_restricted",
				"message":     "The daemon was started by an MCP client and cannot spawn terminal processes due to macOS sandbox restrictions.",
				"instruction": "Run this command in a separate terminal to restart the daemon with full permissions:",
				"command":     "gasoline-mcp --stop && gasoline-mcp --daemon",
			})
			return
		}
		jsonResponse(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"session_id": result.SessionID,
		"token":      result.Token,
		"pid":        result.Pid,
	})
}

// autoDetectCWD gets the CWD from the first registered MCP client.
func autoDetectCWD(cap *capture.Store) string {
	reg := cap.GetClientRegistry()
	if reg == nil {
		return ""
	}
	clients := reg.List()
	if clients == nil {
		return ""
	}

	// The List() return type is any — extract CWD from the first client.
	// ClientRegistry.List() returns []*ClientState which has a CWD field.
	type cwdExtractor interface {
		GetCWD() string
	}

	switch v := clients.(type) {
	case []any:
		for _, c := range v {
			if m, ok := c.(map[string]any); ok {
				if cwd, ok := m["cwd"].(string); ok && cwd != "" {
					return cwd
				}
			}
		}
	default:
		// Try JSON roundtrip as fallback.
		data, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		var entries []map[string]any
		if err := json.Unmarshal(data, &entries); err != nil {
			return ""
		}
		for _, e := range entries {
			if cwd, ok := e["cwd"].(string); ok && cwd != "" {
				return cwd
			}
		}
	}
	return ""
}

// handleTerminalStop destroys a terminal session.
func handleTerminalStop(w http.ResponseWriter, r *http.Request, mgr *pty.Manager) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.ID == "" {
		req.ID = "default"
	}

	if err := mgr.Stop(req.ID); err != nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// isSandboxError returns true if err looks like a macOS sandbox/fork restriction.
// MCP stdio-spawned daemons inherit a restricted environment that blocks posix_spawn/fork.
func isSandboxError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "operation not permitted") ||
		strings.Contains(msg, "Operation not permitted") ||
		strings.Contains(msg, "not permitted")
}

// handleTerminalConfig returns or updates terminal configuration.
func handleTerminalConfig(w http.ResponseWriter, r *http.Request, mgr *pty.Manager) {
	switch r.Method {
	case "GET":
		sessions := mgr.List()
		jsonResponse(w, http.StatusOK, map[string]any{
			"sessions": sessions,
			"count":    mgr.Count(),
		})
	default:
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
	}
}
