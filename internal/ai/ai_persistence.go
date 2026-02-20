// ai_persistence.go — File-based persistent key-value store for session data.
// Provides namespace-scoped storage that survives server restarts, enabling AI
// assistants to save and retrieve structured data across sessions.
// Design: Each namespace maps to a JSON file on disk. Individual values are capped
// at 1MB, total storage per project at 10MB. Operations are mutex-guarded for
// concurrent safety. Supports save, load, list, delete, and stats actions.
package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/state"
)

const (
	maxFileSize          = 1 * 1024 * 1024  // 1MB per file
	maxProjectSize       = 10 * 1024 * 1024 // 10MB per project
	maxErrorHistory      = 500
	staleErrorThreshold  = 30 * 24 * time.Hour // 30 days
	defaultFlushInterval = 30 * time.Second
	dirPermissions       = 0o755
	filePermissions      = 0o644
)

// SessionStore provides persistent cross-session memory backed by disk.
// Data is stored under ~/.gasoline/projects/{project-path}/.
//
// Lock ordering: mu before dirtyMu. Never hold dirtyMu while acquiring mu.
type SessionStore struct {
	mu          sync.RWMutex
	projectPath string // project root directory (CWD)
	projectDir  string // projectPath/.gasoline
	meta        *ProjectMeta

	// Dirty data buffer: namespace/key -> data
	dirty   map[string][]byte
	dirtyMu sync.Mutex // acquired independently; never nest with mu

	// Background flush
	flushInterval time.Duration
	stopCh        chan struct{}
	stopped       bool
}

// ProjectMeta is stored in meta.json
type ProjectMeta struct {
	ProjectID    string    `json:"project_id"`
	ProjectPath  string    `json:"project_path"`
	FirstCreated time.Time `json:"first_created"`
	LastSession  time.Time `json:"last_session"`
	SessionCount int       `json:"session_count"`
}

// SessionContext is returned by LoadSessionContext
type SessionContext struct {
	ProjectID    string              `json:"project_id"`
	SessionCount int                 `json:"session_count"`
	Baselines    []string            `json:"baselines"`
	NoiseConfig  map[string]any      `json:"noise_config,omitempty"`
	ErrorHistory []ErrorHistoryEntry `json:"error_history"`
	APISchema    map[string]any      `json:"api_schema,omitempty"`
	Performance  map[string]any      `json:"performance,omitempty"`
}

// ErrorHistoryEntry tracks an error across sessions
type ErrorHistoryEntry struct {
	Fingerprint string    `json:"fingerprint"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Count       int       `json:"count"`
	Resolved    bool      `json:"resolved"`
	ResolvedAt  time.Time `json:"resolved_at,omitempty"`
}

// StoreStats holds storage statistics
type StoreStats struct {
	TotalBytes   int64          `json:"total_bytes"`
	SessionCount int            `json:"session_count"`
	Namespaces   map[string]int `json:"namespaces"`
}

// SessionStoreArgs represents arguments for the session_store MCP tool
type SessionStoreArgs struct {
	Action    string          `json:"action"`
	Namespace string          `json:"namespace,omitempty"`
	Key       string          `json:"key,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// NewSessionStore creates a new SessionStore with default flush interval.
// projectPath is the project root directory; data is stored under ~/.gasoline/projects/
func NewSessionStore(projectPath string) (*SessionStore, error) {
	return NewSessionStoreWithInterval(projectPath, defaultFlushInterval)
}

// NewSessionStoreWithInterval creates a new SessionStore with a custom flush interval.
func NewSessionStoreWithInterval(projectPath string, flushInterval time.Duration) (*SessionStore, error) {
	absPath, projectDir, err := resolveProjectDir(projectPath)
	if err != nil {
		return nil, err
	}
	return newSessionStoreInDir(absPath, projectDir, flushInterval)
}

// resolveProjectDir validates a project path and resolves its persistence directory.
func resolveProjectDir(projectPath string) (absPath, projectDir string, err error) {
	absPath, err = filepath.Abs(projectPath)
	if err != nil {
		return "", "", fmt.Errorf("invalid project path: %w", err)
	}
	if strings.Contains(absPath, "..") {
		return "", "", fmt.Errorf("project path contains '..': %s", absPath)
	}
	projectDir, err = state.ProjectDir(absPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve project directory: %w", err)
	}
	return absPath, projectDir, nil
}

// newSessionStoreInDir creates a SessionStore with an explicit project directory.
// Used by tests to avoid env-var dependencies in parallel tests.
func newSessionStoreInDir(projectPath, projectDir string, flushInterval time.Duration) (*SessionStore, error) {
	s := &SessionStore{
		projectPath:   projectPath,
		projectDir:    projectDir,
		dirty:         make(map[string][]byte),
		flushInterval: flushInterval,
		stopCh:        make(chan struct{}),
	}

	if err := os.MkdirAll(projectDir, dirPermissions); err != nil {
		return nil, fmt.Errorf("failed to create project directory: %w", err)
	}

	if err := s.loadOrCreateMeta(); err != nil {
		return nil, fmt.Errorf("failed to load meta: %w", err)
	}

	go s.backgroundFlush()

	return s, nil
}

func (s *SessionStore) loadOrCreateMeta() error {
	metaPath := filepath.Join(s.projectDir, "meta.json")
	data, err := os.ReadFile(metaPath) // #nosec G304 -- path is constructed from internal projectDir field // nosemgrep: go_filesystem_rule-fileread -- local persistence store I/O
	if err != nil || len(data) == 0 {
		// Fresh store
		now := time.Now()
		s.meta = &ProjectMeta{
			ProjectID:    s.projectPath,
			ProjectPath:  s.projectPath,
			FirstCreated: now,
			LastSession:  now,
			SessionCount: 1,
		}
		return s.saveMeta()
	}

	var meta ProjectMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		// Corrupted JSON: start fresh
		now := time.Now()
		s.meta = &ProjectMeta{
			ProjectID:    s.projectPath,
			ProjectPath:  s.projectPath,
			FirstCreated: now,
			LastSession:  now,
			SessionCount: 1,
		}
		return s.saveMeta()
	}

	// Increment session
	meta.SessionCount++
	meta.LastSession = time.Now()
	s.meta = &meta
	return s.saveMeta()
}

func (s *SessionStore) saveMeta() error {
	metaPath := filepath.Join(s.projectDir, "meta.json")
	data, err := json.Marshal(s.meta)
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath, data, filePermissions)
}

// GetMeta returns a copy of the project metadata.
func (s *SessionStore) GetMeta() ProjectMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.meta == nil {
		return ProjectMeta{}
	}
	return *s.meta
}

// validateStoreInput checks that a namespace or key value is safe for use as a
// filesystem path component. Rejects path traversal sequences and separators.
func validateStoreInput(value, label string) error {
	if value == "" {
		return nil // empty values are handled by callers
	}
	if strings.Contains(value, "..") {
		return fmt.Errorf("%s contains path traversal sequence", label)
	}
	if strings.ContainsRune(value, filepath.Separator) || strings.Contains(value, "/") {
		return fmt.Errorf("%s contains path separator", label)
	}
	return nil
}

// validatePathInDir ensures the target path is within the base directory.
// Defense-in-depth check applied after filepath.Join construction.
func validatePathInDir(base, target string) error {
	cleanBase := filepath.Clean(base) + string(os.PathSeparator)
	cleanTarget := filepath.Clean(target)
	if !strings.HasPrefix(cleanTarget, cleanBase) {
		return fmt.Errorf("path escapes project directory")
	}
	return nil
}

func (s *SessionStore) validatedNsDir(namespace string) (string, error) {
	if err := validateStoreInput(namespace, "namespace"); err != nil {
		return "", err
	}
	nsDir := filepath.Join(s.projectDir, namespace)
	if err := validatePathInDir(s.projectDir, nsDir); err != nil {
		return "", err
	}
	return nsDir, nil
}

func (s *SessionStore) validatedPath(namespace, key string) (nsDir, filePath string, err error) {
	nsDir, err = s.validatedNsDir(namespace)
	if err != nil {
		return "", "", err
	}
	if err := validateStoreInput(key, "key"); err != nil {
		return "", "", err
	}
	filePath = filepath.Join(nsDir, key+".json")
	if err := validatePathInDir(s.projectDir, filePath); err != nil {
		return "", "", err
	}
	return nsDir, filePath, nil
}

// jsonKeysFromDir reads a directory and returns the names of .json files
// (without extension). Returns an empty slice if the directory does not exist.
func jsonKeysFromDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	var keys []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasSuffix(name, ".json") {
			keys = append(keys, strings.TrimSuffix(name, ".json"))
		}
	}
	if keys == nil {
		keys = []string{}
	}
	return keys, nil
}

// Save writes data as JSON to <namespace>/<key>.json.
func (s *SessionStore) Save(namespace, key string, data []byte) error {
	nsDir, filePath, err := s.validatedPath(namespace, key)
	if err != nil {
		return err
	}
	if len(data) > maxFileSize {
		return fmt.Errorf("data exceeds maximum file size (1MB): %d bytes", len(data))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	currentSize, sizeErr := s.projectSize()
	if sizeErr == nil && currentSize+int64(len(data)) > maxProjectSize {
		return fmt.Errorf("project size limit exceeded (10MB): current=%d, adding=%d", currentSize, len(data))
	}
	if err := os.MkdirAll(nsDir, dirPermissions); err != nil {
		return fmt.Errorf("failed to create namespace directory: %w", err)
	}
	if err := os.WriteFile(filePath, data, filePermissions); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}

// Load reads and returns the JSON from <namespace>/<key>.json.
func (s *SessionStore) Load(namespace, key string) ([]byte, error) {
	_, filePath, err := s.validatedPath(namespace, key)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	data, readErr := os.ReadFile(filePath) // #nosec G304 -- path validated above
	if readErr != nil {
		return nil, fmt.Errorf("key not found: %s/%s", namespace, key)
	}
	return data, nil
}

// List returns all keys in a namespace (file names without .json extension).
func (s *SessionStore) List(namespace string) ([]string, error) {
	nsDir, err := s.validatedNsDir(namespace)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return jsonKeysFromDir(nsDir)
}

// Delete removes the file for a given namespace/key.
func (s *SessionStore) Delete(namespace, key string) error {
	_, filePath, err := s.validatedPath(namespace, key)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete: %s/%s: %w", namespace, key, err)
	}
	return nil
}
