// session.go — PTY session: spawns a CLI subprocess in a pseudo-terminal.
// Why: Isolates PTY lifecycle (open, I/O, resize, close) from session management and HTTP transport.
// Docs: docs/features/feature/terminal/index.md

package pty

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// maxScrollback is the maximum size of the terminal output scrollback buffer (256 KB).
const maxScrollback = 256 * 1024

// Session wraps a PTY master + child process for interactive terminal I/O.
type Session struct {
	ID         string
	ptmx       *os.File
	cmd        *exec.Cmd
	mu         sync.Mutex
	closed     bool
	scrollback []byte     // ring buffer of recent PTY output for reconnect replay
	scrollMu   sync.Mutex // protects scrollback
}

// winsize matches the C struct winsize for TIOCSWINSZ.
type winsize struct {
	Row uint16
	Col uint16
	X   uint16 // unused pixel width
	Y   uint16 // unused pixel height
}

// ErrSessionClosed is returned when operating on a closed session.
var ErrSessionClosed = errors.New("pty: session closed")

// ptsname returns the slave PTY path for a master fd.
// On macOS, TIOCPTYGNAME writes a null-terminated C string into a 128-byte buffer.
func ptsname(f *os.File) (string, error) {
	buf := make([]byte, 128)
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TIOCPTYGNAME, uintptr(unsafe.Pointer(&buf[0])))
	if errno != 0 {
		return "", fmt.Errorf("ptsname: %w", errno)
	}
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i]), nil
		}
	}
	return string(buf), nil
}

// openPTY opens a new PTY master/slave pair. Returns (master, slavePath, error).
func openPTY() (*os.File, string, error) {
	ptmx, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, "", fmt.Errorf("open /dev/ptmx: %w", err)
	}

	// Grant and unlock the slave.
	fd := ptmx.Fd()
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCPTYGRANT, 0); errno != 0 {
		ptmx.Close()
		return nil, "", fmt.Errorf("grantpt: %w", errno)
	}
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCPTYUNLK, 0); errno != 0 {
		ptmx.Close()
		return nil, "", fmt.Errorf("unlockpt: %w", errno)
	}

	slavePath, err := ptsname(ptmx)
	if err != nil {
		ptmx.Close()
		return nil, "", err
	}

	return ptmx, slavePath, nil
}

// SpawnConfig configures a new PTY session.
type SpawnConfig struct {
	ID   string   // Session identifier.
	Cmd  string   // CLI binary (e.g., "claude", "bash").
	Args []string // CLI arguments.
	Dir  string   // Working directory.
	Env  []string // Extra environment variables (appended to os.Environ).
	Cols uint16   // Initial terminal columns (default 80).
	Rows uint16   // Initial terminal rows (default 24).
}

// Spawn creates a new PTY session, starting the given command.
func Spawn(cfg SpawnConfig) (*Session, error) {
	if cfg.Cmd == "" {
		return nil, errors.New("pty: cmd is required")
	}
	if cfg.Cols == 0 {
		cfg.Cols = 80
	}
	if cfg.Rows == 0 {
		cfg.Rows = 24
	}
	if cfg.ID == "" {
		cfg.ID = "default"
	}

	ptmx, slavePath, err := openPTY()
	if err != nil {
		return nil, err
	}

	// Open slave.
	slave, err := os.OpenFile(slavePath, os.O_RDWR, 0)
	if err != nil {
		ptmx.Close()
		return nil, fmt.Errorf("open slave %s: %w", slavePath, err)
	}

	// Set initial terminal size on the slave fd (macOS requires slave, not master).
	ws := &winsize{Row: cfg.Rows, Col: cfg.Cols}
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, slave.Fd(), syscall.TIOCSWINSZ, uintptr(unsafe.Pointer(ws))); errno != 0 {
		slave.Close()
		ptmx.Close()
		return nil, fmt.Errorf("set winsize: %w", errno)
	}

	cmd := exec.Command(cfg.Cmd, cfg.Args...)
	cmd.Dir = cfg.Dir
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
	}

	// Build environment: inherit from parent, add TERM, append extras.
	env := os.Environ()
	env = append(env, "TERM=xterm-256color")
	env = append(env, cfg.Env...)
	cmd.Env = env

	if err := cmd.Start(); err != nil {
		slave.Close()
		ptmx.Close()
		return nil, fmt.Errorf("start %s: %w", cfg.Cmd, err)
	}

	// Slave fd is now owned by the child process; close our copy.
	slave.Close()

	return &Session{
		ID:   cfg.ID,
		ptmx: ptmx,
		cmd:  cmd,
	}, nil
}

// Read reads from the PTY master (child's stdout/stderr).
func (s *Session) Read(buf []byte) (int, error) {
	s.mu.Lock() // lint:manual-unlock — unlock before blocking I/O
	if s.closed {
		s.mu.Unlock()
		return 0, ErrSessionClosed
	}
	ptmx := s.ptmx
	s.mu.Unlock()
	return ptmx.Read(buf)
}

// Write writes to the PTY master (child's stdin).
func (s *Session) Write(data []byte) (int, error) {
	s.mu.Lock() // lint:manual-unlock — unlock before blocking I/O
	if s.closed {
		s.mu.Unlock()
		return 0, ErrSessionClosed
	}
	ptmx := s.ptmx
	s.mu.Unlock()
	return ptmx.Write(data)
}

// Resize changes the terminal dimensions.
func (s *Session) Resize(cols, rows uint16) error {
	s.mu.Lock() // lint:manual-unlock — unlock before ioctl syscall
	if s.closed {
		s.mu.Unlock()
		return ErrSessionClosed
	}
	ptmx := s.ptmx
	s.mu.Unlock()

	ws := &winsize{Row: rows, Col: cols}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, ptmx.Fd(), syscall.TIOCSWINSZ, uintptr(unsafe.Pointer(ws)))
	if errno != 0 {
		return fmt.Errorf("resize: %w", errno)
	}
	return nil
}

// Close terminates the child process and closes the PTY.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true

	// Signal the child to terminate.
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Signal(syscall.SIGTERM)
	}
	// Close PTY master — this also signals EOF to the child.
	err := s.ptmx.Close()

	// Reap the child process with a timeout. If SIGTERM + PTY close aren't
	// enough, escalate to SIGKILL. Without this timeout, cmd.Wait() blocks
	// indefinitely and deadlocks the Manager (which holds its write lock).
	done := make(chan struct{})
	go func() { // lint:allow-bare-goroutine — one-shot reaper, closes done channel
		_ = s.cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
		// Child exited cleanly.
	case <-time.After(2 * time.Second):
		// Escalate to SIGKILL.
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
		<-done // Wait for reap after SIGKILL.
	}
	return err
}

// Wait waits for the child process to exit and returns its error (nil on clean exit).
func (s *Session) Wait() error {
	return s.cmd.Wait()
}

// AppendScrollback appends data to the scrollback buffer, evicting oldest bytes
// if the buffer exceeds maxScrollback.
func (s *Session) AppendScrollback(data []byte) {
	s.scrollMu.Lock() // lint:manual-unlock — simple lock/unlock in same function
	s.scrollback = append(s.scrollback, data...)
	if len(s.scrollback) > maxScrollback {
		// Trim to keep only the most recent maxScrollback bytes.
		s.scrollback = s.scrollback[len(s.scrollback)-maxScrollback:]
	}
	s.scrollMu.Unlock()
}

// Scrollback returns a copy of the current scrollback buffer.
func (s *Session) Scrollback() []byte {
	s.scrollMu.Lock()
	defer s.scrollMu.Unlock()
	if len(s.scrollback) == 0 {
		return nil
	}
	out := make([]byte, len(s.scrollback))
	copy(out, s.scrollback)
	return out
}

// IsAlive returns true if the session is not closed and the child process is still running.
func (s *Session) IsAlive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.cmd.Process == nil {
		return false
	}
	// Signal 0 checks process existence without sending a signal.
	err := s.cmd.Process.Signal(syscall.Signal(0))
	return err == nil
}

// Pid returns the child process PID, or -1 if not started.
func (s *Session) Pid() int {
	if s.cmd.Process == nil {
		return -1
	}
	return s.cmd.Process.Pid
}

// ForceRedraw sends SIGWINCH to the child process, forcing TUI applications
// to redraw. Used on WebSocket reconnect where the terminal dimensions may
// not have changed (so TIOCSWINSZ alone wouldn't trigger SIGWINCH).
func (s *Session) ForceRedraw() {
	s.mu.Lock() // lint:manual-unlock — unlock before signal
	if s.closed || s.cmd.Process == nil {
		s.mu.Unlock()
		return
	}
	proc := s.cmd.Process
	s.mu.Unlock()
	_ = proc.Signal(syscall.SIGWINCH)
}
