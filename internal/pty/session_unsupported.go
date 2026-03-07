//go:build !darwin && !linux

// session_unsupported.go — PTY session stubs for unsupported platforms.
// Why: Keep non-Unix targets (for example, Windows) compiling without PTY ioctl support.
// Docs: docs/features/feature/terminal/index.md

package pty

import "errors"

// maxScrollback keeps API parity with Unix session implementation.
const maxScrollback = 256 * 1024

// Session is a stub PTY session on unsupported platforms.
type Session struct {
	ID string
}

// SpawnConfig configures a new PTY session.
type SpawnConfig struct {
	ID   string
	Cmd  string
	Args []string
	Dir  string
	Env  []string
	Cols uint16
	Rows uint16
}

// ErrSessionClosed is returned when operating on a closed session.
var ErrSessionClosed = errors.New("pty: session closed")

var errUnsupportedPTY = errors.New("pty: unsupported platform")

// Spawn returns unsupported on non-Unix platforms.
func Spawn(cfg SpawnConfig) (*Session, error) {
	return nil, errUnsupportedPTY
}

// Read is unsupported on non-Unix platforms.
func (s *Session) Read(buf []byte) (int, error) {
	return 0, errUnsupportedPTY
}

// Write is unsupported on non-Unix platforms.
func (s *Session) Write(data []byte) (int, error) {
	return 0, errUnsupportedPTY
}

// Resize is unsupported on non-Unix platforms.
func (s *Session) Resize(cols, rows uint16) error {
	return errUnsupportedPTY
}

// Close is a no-op for unsupported platform stubs.
func (s *Session) Close() error {
	return nil
}

// Wait is a no-op for unsupported platform stubs.
func (s *Session) Wait() error {
	return nil
}

// AppendScrollback is a no-op for unsupported platform stubs.
func (s *Session) AppendScrollback(data []byte) {}

// Scrollback returns an empty buffer on unsupported platforms.
func (s *Session) Scrollback() []byte {
	return nil
}

// IsAlive reports false on unsupported platforms.
func (s *Session) IsAlive() bool {
	return false
}

// Pid returns 0 on unsupported platforms.
func (s *Session) Pid() int {
	return 0
}

// ForceRedraw is a no-op on unsupported platforms.
func (s *Session) ForceRedraw() {}
