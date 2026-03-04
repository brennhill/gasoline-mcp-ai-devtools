// manager.go — PTY session manager: create, get, destroy sessions with auth tokens.
// Why: Centralizes session lifecycle and token-based access control for the terminal WebSocket endpoint.

package pty

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
)

// Manager manages PTY sessions with token-based authentication.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session // keyed by session ID
	tokens   map[string]string   // token → session ID
}

// NewManager creates a new session manager.
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		tokens:   make(map[string]string),
	}
}

// StartConfig is the configuration for starting a new terminal session.
type StartConfig struct {
	ID   string   // Session ID (default: "default").
	Cmd  string   // CLI binary.
	Args []string // CLI arguments.
	Dir  string   // Working directory.
	Env  []string // Extra environment variables.
	Cols uint16   // Terminal columns.
	Rows uint16   // Terminal rows.
}

// StartResult is returned after successfully starting a session.
type StartResult struct {
	SessionID string
	Token     string
	Pid       int
}

// ErrSessionExists is returned when a session ID is already in use.
var ErrSessionExists = errors.New("pty: session already exists")

// ErrSessionNotFound is returned when a session ID is not found.
var ErrSessionNotFound = errors.New("pty: session not found")

// ErrInvalidToken is returned when an authentication token is invalid.
var ErrInvalidToken = errors.New("pty: invalid token")

// Start creates and starts a new PTY session. Returns the session token for WebSocket auth.
func (m *Manager) Start(cfg StartConfig) (*StartResult, error) {
	if cfg.ID == "" {
		cfg.ID = "default"
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[cfg.ID]; exists {
		return nil, fmt.Errorf("%w: %s", ErrSessionExists, cfg.ID)
	}

	sess, err := Spawn(SpawnConfig{
		ID:   cfg.ID,
		Cmd:  cfg.Cmd,
		Args: cfg.Args,
		Dir:  cfg.Dir,
		Env:  cfg.Env,
		Cols: cfg.Cols,
		Rows: cfg.Rows,
	})
	if err != nil {
		return nil, err
	}

	token, err := generateToken()
	if err != nil {
		sess.Close()
		return nil, fmt.Errorf("generate token: %w", err)
	}

	m.sessions[cfg.ID] = sess
	m.tokens[token] = cfg.ID

	return &StartResult{
		SessionID: cfg.ID,
		Token:     token,
		Pid:       sess.Pid(),
	}, nil
}

// GetByToken returns the session associated with the given auth token.
func (m *Manager) GetByToken(token string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessionID, ok := m.tokens[token]
	if !ok {
		return nil, ErrInvalidToken
	}
	sess, ok := m.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	return sess, nil
}

// Get returns the session by ID.
func (m *Manager) Get(id string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sess, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, id)
	}
	return sess, nil
}

// Stop destroys a session by ID, cleaning up PTY and child process.
func (m *Manager) Stop(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("%w: %s", ErrSessionNotFound, id)
	}

	// Remove token mapping.
	for token, sid := range m.tokens {
		if sid == id {
			delete(m.tokens, token)
			break
		}
	}
	delete(m.sessions, id)

	return sess.Close()
}

// StopAll destroys all active sessions. Called during daemon shutdown.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, sess := range m.sessions {
		_ = sess.Close()
		delete(m.sessions, id)
	}
	for token := range m.tokens {
		delete(m.tokens, token)
	}
}

// List returns the IDs of all active sessions.
func (m *Manager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	return ids
}

// Count returns the number of active sessions.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// generateToken creates a cryptographically random 32-byte hex token.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
