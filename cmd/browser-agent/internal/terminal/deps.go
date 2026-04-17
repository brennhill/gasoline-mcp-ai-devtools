// deps.go -- Dependency interfaces for the terminal package.
// Why: Defines narrow interfaces so the terminal subsystem depends on abstractions, not the god Server type.

package terminal

import (
	"bufio"
	"io"
	"net/http"
)

// ServerDeps provides the subset of Server behavior needed by terminal handlers.
type ServerDeps interface {
	GetActiveCodebase() string
	SetActiveCodebase(path string)
}

// IntentDeps provides access to the intent store and PTY relay injection
// from the main Server. Used by intent handlers.
type IntentDeps interface {
	GetPtyRelays() RelayMap
	GetIntentStore() *IntentStore
}

// RelayMap is the interface for terminal relay map operations used by intent handlers.
type RelayMap interface {
	WriteToFirst(data []byte) bool
	CloseAll()
}

// Deps bundles all dependencies needed to register terminal routes.
type Deps struct {
	JSONResponse   func(w http.ResponseWriter, status int, data any)
	CORSMiddleware func(next http.HandlerFunc) http.HandlerFunc
	Stderrf        func(format string, args ...any)
	MaxPostBody    int64

	// WebSocket codec functions injected from the main package.
	WSReadFrame  func(r io.Reader) (fin bool, opcode byte, payload []byte, err error)
	WSWriteFrame func(w *bufio.ReadWriter, opcode byte, payload []byte) error
	WSAcceptKey  func(key string) string
}
