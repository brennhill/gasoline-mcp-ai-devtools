// ws_relay.go — WebSocket relay loop, frame writer, and control message handling.
// Why: Isolates WebSocket relay concerns from HTTP handler logic for maintainability.
// Docs: docs/features/feature/terminal/index.md

package terminal

import (
	"bufio"
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
)

// wsLoop relays data between a WebSocket connection and a PTY session.
// Downstream (fan-out -> browser): binary WebSocket frames with raw terminal output.
// Upstream (browser -> PTY): binary frames as keystrokes via write buffer, text frames as JSON control.
// Server sends ping frames every PingInterval; if no frame (data or pong) arrives
// within PongTimeout the connection is considered dead and closed. The PTY session
// survives so the browser can reconnect with scrollback replay.
//
// Coordinated shutdown: all three goroutines share a connDone channel and a sync.Once-guarded
// closeConn function. Any goroutine that detects a terminal condition calls closeConn(),
// which closes connDone (unblocking the others) then closes the underlying TCP connection
// exactly once. This prevents double-close races and goroutine leaks.
func wsLoop(conn net.Conn, rw *bufio.ReadWriter, deps Deps, sess *pty.Session, relay *Relay, sub <-chan []byte) {
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

	// Multiple goroutines emit frames (downstream, keepalive ping, and upstream
	// control responses). Serialize writes to avoid interleaved/corrupted frames.
	writeFrame := NewFrameWriter(rw, deps)

	// Fan-out -> WebSocket (downstream): read from subscriber channel and send as binary frames.
	// Also tracks alt-screen state changes and notifies the frontend.
	downstreamDone := make(chan struct{})
	go func() { // lint:allow-bare-goroutine — bounded by connDone/channel close
		defer close(downstreamDone)
		prevAltScreen := sess.AltScreenActive()
		for {
			select {
			case data, ok := <-sub:
				if !ok {
					// Fanout closed (session ended) — send exit notification
					// so the browser can display the message and stop reconnecting.
					exitMsg, _ := json.Marshal(map[string]any{"type": "exited", "code": relay.exitCode})
					_ = writeFrame(0x1, exitMsg)
					_ = writeFrame(0x8, nil)
					closeConn()
					return
				}
				if err := writeFrame(0x2, data); err != nil {
					closeConn()
					return
				}
				// Notify frontend of alt-screen state changes.
				altScreen := sess.AltScreenActive()
				if altScreen != prevAltScreen {
					prevAltScreen = altScreen
					ctrl, _ := json.Marshal(map[string]any{"type": "alt_screen", "active": altScreen})
					_ = writeFrame(0x1, ctrl)
				}
			case <-connDone:
				return
			}
		}
	}()

	// Server-initiated ping keepalive — detects dead connections (browser crash,
	// laptop sleep) without ever timing out idle users.
	pingTicker := time.NewTicker(PingInterval)
	go func() { // lint:allow-bare-goroutine — bounded by connDone
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
	}()

	// WebSocket -> PTY (upstream): read frames and dispatch.
	// Uses relay.writeBuf for non-blocking writes with backpressure.
	// NOTE: Do NOT call sess.Close() on WebSocket disconnect — the session
	// must survive page refreshes so the browser can reconnect with scrollback replay.
	// Sessions are only killed explicitly via POST /terminal/stop (the Exit button).
	go func() { // lint:allow-bare-goroutine — bounded by connDone
		defer closeConn() // Close conn on exit so downstream detects it and browser auto-reconnects
		for {
			// Refresh read deadline on every iteration — any received frame
			// (data, pong, ping) proves the connection is alive.
			_ = conn.SetReadDeadline(time.Now().Add(PongTimeout))

			fin, opcode, payload, err := deps.WSReadFrame(rw)
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
			case 0x2: // Binary — raw keystrokes -> PTY stdin via write buffer
				_, _ = relay.writeBuf.Write(payload)
			case 0x1: // Text — JSON control message
				HandleControlMessage(payload, sess)
			}
		}
	}()

	<-downstreamDone
}

// NewFrameWriter returns a thread-safe frame writer for one WebSocket
// connection. All callers for that connection must share this writer.
func NewFrameWriter(rw *bufio.ReadWriter, deps Deps) func(opcode byte, payload []byte) error {
	var wsWriteMu sync.Mutex
	return func(opcode byte, payload []byte) error {
		wsWriteMu.Lock()
		defer wsWriteMu.Unlock()
		return deps.WSWriteFrame(rw, opcode, payload)
	}
}

// ControlMessage is a JSON control message from the browser terminal.
type ControlMessage struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// HandleControlMessage processes a JSON control message from the browser.
func HandleControlMessage(payload []byte, sess *pty.Session) {
	var msg ControlMessage
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
