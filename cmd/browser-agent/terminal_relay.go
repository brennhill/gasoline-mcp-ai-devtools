// terminal_relay.go — Per-session relay: fan-out PTY output, buffer writes, prompt detection.
// Why: Supports multiple WebSocket viewers per session and non-blocking input.

package main

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/pty"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
)

// terminalRelay manages per-session fan-out, buffered writes, and a PTY reader loop.
// The reader loop runs from relay creation until the session closes.
type terminalRelay struct {
	sess         *pty.Session
	fanout       *pty.Fanout
	writeBuf     *pty.WriteBuffer
	workspaceDir string
	done         chan struct{}
	exitCode     int // set by readLoop before closing fanout; read by downstream after channel close
}

// newTerminalRelay creates a relay and starts the PTY reader loop.
func newTerminalRelay(sess *pty.Session, workspaceDir string) *terminalRelay {
	r := &terminalRelay{
		sess:         sess,
		fanout:       pty.NewFanout(),
		writeBuf:     pty.NewWriteBuffer(sess),
		workspaceDir: workspaceDir,
		done:         make(chan struct{}),
	}
	util.SafeGo(r.readLoop)
	return r
}

// readLoop continuously reads PTY output, appends to scrollback, and broadcasts
// to all subscribers. Exits when the session closes or the process exits.
// Before closing the fanout, it reaps the child process to capture the exit code
// so downstream subscribers can notify the browser.
func (r *terminalRelay) readLoop() {
	defer close(r.done)
	defer r.fanout.Close()
	defer r.writeBuf.Close()
	buf := make([]byte, terminalReadBufSize)
	for {
		n, err := r.sess.Read(buf)
		if n > 0 {
			r.sess.AppendScrollback(buf[:n])
			r.fanout.Broadcast(buf[:n])
		}
		if err != nil {
			// Reap child process to capture exit code before fanout closes.
			// The write to exitCode happens-before fanout.Close() (in defers),
			// which closes subscriber channels, creating a happens-before edge
			// to the downstream goroutine's read of exitCode.
			r.reapExitCode()
			return
		}
	}
}

// reapExitCode waits for the child process exit code. Called after PTY read
// returns an error (typically EOF when the child exits), so the Session's
// reaper goroutine has usually already captured the exit code.
func (r *terminalRelay) reapExitCode() {
	r.sess.Wait() // blocks until child exits — usually instant since PTY EOF already received
	r.exitCode = r.sess.ExitCode()
}

// Close stops the write buffer. The readLoop exits when the session closes,
// which triggers fanout and writeBuf cleanup via defers.
func (r *terminalRelay) Close() {
	r.writeBuf.Close()
}

// terminalRelayMap manages per-session relays.
type terminalRelayMap struct {
	mu     sync.Mutex
	relays map[string]*terminalRelay
}

func newTerminalRelayMap() *terminalRelayMap {
	return &terminalRelayMap{relays: make(map[string]*terminalRelay)}
}

func (m *terminalRelayMap) get(id string) *terminalRelay {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.relays[id]
}

func (m *terminalRelayMap) getOrCreate(id string, sess *pty.Session, workspaceDir string) *terminalRelay {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r := m.relays[id]; r != nil {
		return r
	}
	r := newTerminalRelay(sess, workspaceDir)
	m.relays[id] = r
	return r
}

func (m *terminalRelayMap) remove(id string) {
	m.mu.Lock() // lint:manual-unlock — unlock before Close to avoid holding lock during I/O
	r := m.relays[id]
	delete(m.relays, id)
	m.mu.Unlock()
	if r != nil {
		r.Close()
	}
}

// writeToFirst writes data to the first active relay's PTY input.
// Assumes a single active terminal session (the typical case). If multiple
// sessions exist, the target is non-deterministic due to Go map iteration.
// Returns true if a relay was found and the write succeeded.
func (m *terminalRelayMap) writeToFirst(data []byte) bool {
	m.mu.Lock()
	var relay *terminalRelay
	for _, r := range m.relays {
		relay = r
		break
	}
	m.mu.Unlock()
	if relay == nil {
		return false
	}
	relay.writeBuf.Write(data)
	return true
}

func (m *terminalRelayMap) closeAll() {
	m.mu.Lock() // lint:manual-unlock — unlock before Close to avoid holding lock during I/O
	toClose := make([]*terminalRelay, 0, len(m.relays))
	for _, r := range m.relays {
		toClose = append(toClose, r)
	}
	m.relays = make(map[string]*terminalRelay)
	m.mu.Unlock()
	for _, r := range toClose {
		r.Close()
	}
}

// wsSubCounter generates unique subscriber IDs for WebSocket connections.
var wsSubCounter atomic.Uint64

func nextWSSubID() string {
	return fmt.Sprintf("ws-%d", wsSubCounter.Add(1))
}

// waitForPromptViaRelay subscribes to the relay's fan-out, watches for a
// shell prompt character, then writes the init command. Replaces the old
// direct-PTY-read approach so the relay's readLoop owns all PTY reads.
func waitForPromptViaRelay(relay *terminalRelay, initCmd string) {
	subID := "init-cmd"
	ch, err := relay.fanout.Subscribe(subID)
	if err != nil {
		_, _ = relay.writeBuf.Write([]byte(initCmd + "\n"))
		return
	}
	defer relay.fanout.Unsubscribe(subID)

	deadline := time.After(terminalInitTimeout)
	for {
		select {
		case <-deadline:
			_, _ = relay.writeBuf.Write([]byte(initCmd + "\n"))
			return
		case data, ok := <-ch:
			if !ok {
				return
			}
			for _, b := range data {
				if strings.ContainsRune(terminalPromptChars, rune(b)) {
					_, _ = relay.writeBuf.Write([]byte(initCmd + "\n"))
					return
				}
			}
		}
	}
}
