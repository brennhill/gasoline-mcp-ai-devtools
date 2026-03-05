// terminal_handlers.go — HTTP handlers for the in-browser terminal.
// Why: Isolates terminal WebSocket upgrade, session lifecycle, and static asset serving
// from the main route wiring for maintainability and test focus.
// Docs: docs/features/feature/terminal/index.md

package main

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/pty"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
)

// terminalPingInterval is how often the server sends WebSocket ping frames.
// Browser WebSocket API auto-replies with pong — no client code needed.
const terminalPingInterval = 30 * time.Second

// terminalPongTimeout is the max time allowed without receiving any frame (data or pong).
// If exceeded, the connection is considered dead and closed. The PTY session survives
// so the browser can reconnect with scrollback replay.
const terminalPongTimeout = 60 * time.Second

// terminalReadBufSize is the buffer size for PTY reads relayed to the browser.
const terminalReadBufSize = 4096

// terminalInitTimeout is the max time to wait for a shell prompt before
// writing init_command. Replaces the old hardcoded 500ms sleep with an
// adaptive readiness check that looks for prompt characters.
const terminalInitTimeout = 2 * time.Second

// terminalPromptChars contains characters that indicate a shell prompt is ready.
const terminalPromptChars = "$#>%"

// registerTerminalRoutes adds terminal-related routes to the mux.
// NOT MCP — These are daemon-served endpoints for the in-browser terminal.
func registerTerminalRoutes(mux *http.ServeMux, server *Server, mgr *pty.Manager, cap *capture.Store) {
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
		handleTerminalStart(w, r, server, mgr, cap)
	}))
	mux.HandleFunc("/terminal/stop", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handleTerminalStop(w, r, mgr)
	}))

	// Session validation — checks a specific token against a live session.
	mux.HandleFunc("/terminal/validate", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handleTerminalValidate(w, r, mgr)
	}))

	// Session configuration.
	mux.HandleFunc("/terminal/config", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handleTerminalConfig(w, r, mgr)
	}))

	// NOTE: /config/active-codebase is registered in registerCoreRoutes (not terminal-specific).
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
	// NOTE: After a successful handshake, conn.Close() is handled by closeConn
	// inside terminalWSLoop via sync.Once. We only close here on handshake failure.

	// Send 101 handshake.
	accept := wsAcceptKey(key)
	handshake := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	if _, err := bufrw.WriteString(handshake); err != nil {
		_ = conn.Close()
		return
	}
	if err := bufrw.Flush(); err != nil {
		_ = conn.Close()
		return
	}

	// Replay scrollback so the reconnecting terminal sees prior output.
	if history := sess.Scrollback(); len(history) > 0 {
		// Send in chunks to avoid oversized frames.
		for off := 0; off < len(history); off += terminalReadBufSize {
			end := off + terminalReadBufSize
			if end > len(history) {
				end = len(history)
			}
			if err := wsWriteFrame(bufrw, 0x2, history[off:end]); err != nil {
				_ = conn.Close()
				return
			}
		}
	}

	terminalWSLoop(conn, bufrw, sess)
}

// terminalWSLoop relays data between a WebSocket connection and a PTY session.
// Downstream (PTY -> browser): binary WebSocket frames with raw terminal output.
// Upstream (browser -> PTY): binary frames as keystrokes, text frames as JSON control messages.
// Server sends ping frames every terminalPingInterval; if no frame (data or pong) arrives
// within terminalPongTimeout the connection is considered dead and closed. The PTY session
// survives so the browser can reconnect with scrollback replay.
//
// Coordinated shutdown: all three goroutines share a connDone channel and a sync.Once-guarded
// closeConn function. Any goroutine that detects a terminal condition calls closeConn(),
// which closes connDone (unblocking the others) then closes the underlying TCP connection
// exactly once. This prevents double-close races and goroutine leaks.
func terminalWSLoop(conn net.Conn, rw *bufio.ReadWriter, sess *pty.Session) {
	// Coordinated shutdown: connDone signals all goroutines to exit,
	// closeConn ensures conn.Close() is called exactly once.
	connDone := make(chan struct{})
	var connDoneOnce sync.Once
	closeConn := func() {
		connDoneOnce.Do(func() {
			close(connDone)
			_ = conn.Close()
		})
	}

	// Multiple goroutines emit frames (PTY downstream, keepalive ping, and upstream
	// control responses). Serialize writes to avoid interleaved/corrupted frames.
	writeFrame := newTerminalFrameWriter(rw)

	// PTY -> WebSocket (downstream): read PTY output and send as binary frames.
	// Selects on connDone so it exits promptly when the WebSocket dies.
	ptyDone := make(chan struct{})
	util.SafeGo(func() {
		defer close(ptyDone)
		buf := make([]byte, terminalReadBufSize)
		for {
			n, err := sess.Read(buf)
			if err != nil {
				// PTY closed or process exited — send close frame and shut down.
				_ = writeFrame(0x8, nil)
				closeConn()
				return
			}
			if n > 0 {
				sess.AppendScrollback(buf[:n])
				if err := writeFrame(0x2, buf[:n]); err != nil {
					closeConn()
					return
				}
			}
			// Check if connection was closed by another goroutine.
			select {
			case <-connDone:
				return
			default:
			}
		}
	})

	// Server-initiated ping keepalive — detects dead connections (browser crash,
	// laptop sleep) without ever timing out idle users.
	pingTicker := time.NewTicker(terminalPingInterval)
	util.SafeGo(func() {
		defer pingTicker.Stop()
		for {
			select {
			case <-connDone:
				return
			case <-pingTicker.C:
				if err := writeFrame(0x9, nil); err != nil {
					closeConn()
					return
				}
			}
		}
	})

	// WebSocket -> PTY (upstream): read frames and dispatch.
	// NOTE: Do NOT call sess.Close() on WebSocket disconnect — the session
	// must survive page refreshes so the browser can reconnect with scrollback replay.
	// Sessions are only killed explicitly via POST /terminal/stop (the Exit button).
	util.SafeGo(func() {
		defer closeConn() // Close conn on exit so downstream detects it and browser auto-reconnects
		for {
			// Refresh read deadline on every iteration — any received frame
			// (data, pong, ping) proves the connection is alive.
			_ = conn.SetReadDeadline(time.Now().Add(terminalPongTimeout))

			fin, opcode, payload, err := wsReadFrame(rw)
			if err != nil {
				// Read deadline expired or connection error — close silently.
				// PTY stays alive for reconnection.
				return
			}

			// Reject fragmented frames (FIN=0). Terminal messages are always
			// single-frame; accepting fragments would require reassembly state
			// and risks incomplete data being written to the PTY.
			if !fin {
				_ = writeFrame(0x8, nil) // Send close frame per RFC 6455.
				return
			}

			switch opcode {
			case 0x8: // Close
				_ = writeFrame(0x8, nil)
				return // WebSocket closed — stop relaying but keep PTY alive
			case 0x9: // Ping -> Pong
				_ = writeFrame(0xA, payload)
			case 0xA: // Pong — no-op, deadline already refreshed above
			case 0x2: // Binary — raw keystrokes -> PTY stdin
				if _, err := sess.Write(payload); err != nil {
					return
				}
			case 0x1: // Text — JSON control message
				handleTerminalControlMessage(payload, sess)
			}
		}
	})

	<-ptyDone
}

// newTerminalFrameWriter returns a thread-safe frame writer for one WebSocket
// connection. All callers for that connection must share this writer.
func newTerminalFrameWriter(rw *bufio.ReadWriter) func(opcode byte, payload []byte) error {
	var wsWriteMu sync.Mutex
	return func(opcode byte, payload []byte) error {
		wsWriteMu.Lock()
		defer wsWriteMu.Unlock()
		return wsWriteFrame(rw, opcode, payload)
	}
}

// waitForShellPrompt reads PTY output until a prompt character appears or
// terminalInitTimeout expires. This is more robust than a fixed sleep because
// shell startup time varies (e.g., .zshrc loading, slow NFS home dirs).
// Output is appended to scrollback so reconnecting clients see it.
func waitForShellPrompt(sess *pty.Session) {
	deadline := time.NewTimer(terminalInitTimeout)
	defer deadline.Stop()
	buf := make([]byte, 256)
	for {
		select {
		case <-deadline.C:
			return // Timeout — write init_command anyway as a best-effort fallback.
		default:
		}
		n, err := sess.Read(buf)
		if err != nil {
			return
		}
		if n > 0 {
			sess.AppendScrollback(buf[:n])
			for _, b := range buf[:n] {
				if strings.ContainsRune(terminalPromptChars, rune(b)) {
					return // Prompt detected — shell is ready.
				}
			}
		}
	}
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
			// Always force SIGWINCH so TUI apps redraw — TIOCSWINSZ only
			// sends SIGWINCH when dimensions actually change, but on reconnect
			// the dimensions may match while the display is stale.
			sess.ForceRedraw()
		}
	}
}

// handleTerminalStart creates a new terminal session.
func handleTerminalStart(w http.ResponseWriter, r *http.Request, server *Server, mgr *pty.Manager, cap *capture.Store) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)

	var req struct {
		ID          string   `json:"id"`
		Cmd         string   `json:"cmd"`
		Args        []string `json:"args"`
		Dir         string   `json:"dir"`
		Cols        int      `json:"cols"`
		Rows        int      `json:"rows"`
		InitCommand string   `json:"init_command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	// Default to shell if no command specified.
	if req.Cmd == "" {
		req.Cmd = "/bin/zsh"
	}

	// CWD priority: request dir > active_codebase (set via MCP/extension) > auto-detect
	if req.Dir == "" && server != nil {
		req.Dir = server.GetActiveCodebase()
	}
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
	// Write init_command to PTY stdin after shell prompt appears.
	// Instead of a fixed 500ms sleep, read PTY output until a prompt character
	// ('$', '#', '>', '%') appears or a 2s timeout expires.
	if err == nil && req.InitCommand != "" {
		go func(sessionID, initCmd string) { // lint:allow-bare-goroutine — one-shot init, bounded by timeout
			sess, getErr := mgr.Get(sessionID)
			if getErr != nil {
				return
			}
			waitForShellPrompt(sess)
			// Write the command followed by newline so the shell executes it.
			cmd := initCmd + "\n"
			_, _ = sess.Write([]byte(cmd))
		}(result.SessionID, req.InitCommand)
	}
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
		// Return existing session's token so the client can reconnect instead of killing it.
		sessionID := req.ID
		if sessionID == "" {
			sessionID = "default"
		}
		existingToken := mgr.GetTokenForSession(sessionID)
		jsonResponse(w, http.StatusConflict, map[string]any{
			"error":      err.Error(),
			"session_id": sessionID,
			"token":      existingToken,
		})
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

	// List() returns any — extract CWD from the first client.
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

// handleTerminalValidate checks whether a specific token maps to a live PTY session.
// Returns {"valid": true} if the token resolves to a running session, false otherwise.
func handleTerminalValidate(w http.ResponseWriter, r *http.Request, mgr *pty.Manager) {
	if r.Method != "GET" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		jsonResponse(w, http.StatusOK, map[string]bool{"valid": false})
		return
	}
	sess, err := mgr.GetByToken(token)
	if err != nil {
		jsonResponse(w, http.StatusOK, map[string]bool{"valid": false})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]bool{"valid": sess.IsAlive()})
}

// handleActiveCodebase gets or sets the active codebase path used as terminal CWD.
func handleActiveCodebase(w http.ResponseWriter, r *http.Request, server *Server) {
	switch r.Method {
	case "GET":
		jsonResponse(w, http.StatusOK, map[string]string{
			"active_codebase": server.GetActiveCodebase(),
		})
	case "PUT", "POST":
		r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
		var body struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		server.SetActiveCodebase(strings.TrimSpace(body.Path))
		jsonResponse(w, http.StatusOK, map[string]string{
			"status":          "ok",
			"active_codebase": server.GetActiveCodebase(),
		})
	default:
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
	}
}
