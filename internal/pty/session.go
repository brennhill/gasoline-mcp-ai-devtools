// session.go — PTY session: spawns a CLI subprocess in a pseudo-terminal.
// Why: Isolates PTY lifecycle (open, I/O, resize, close) from session management and HTTP transport.

package pty

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
)

// Session wraps a PTY master + child process for interactive terminal I/O.
type Session struct {
	ID     string
	ptmx   *os.File
	cmd    *exec.Cmd
	mu     sync.Mutex
	closed bool
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
	s.mu.Lock()
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
	s.mu.Lock()
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
	s.mu.Lock()
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

	// Reap the child process (non-blocking wait).
	_ = s.cmd.Wait()
	return err
}

// Wait waits for the child process to exit and returns its error (nil on clean exit).
func (s *Session) Wait() error {
	return s.cmd.Wait()
}

// Pid returns the child process PID, or -1 if not started.
func (s *Session) Pid() int {
	if s.cmd.Process == nil {
		return -1
	}
	return s.cmd.Process.Pid
}
