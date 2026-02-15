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
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/state"
)

// Constants

const (
	maxFileSize          = 1 * 1024 * 1024  // 1MB per file
	maxProjectSize       = 10 * 1024 * 1024 // 10MB per project
	maxErrorHistory      = 500
	staleErrorThreshold  = 30 * 24 * time.Hour // 30 days
	defaultFlushInterval = 30 * time.Second
	dirPermissions       = 0o755
	filePermissions      = 0o644
)

// Types

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

// Constructor

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

// Meta Operations

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

// Path Validation

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

// Core Operations

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

// Stats

// countNamespaceFiles counts files and their total size within a namespace
// directory. Returns the file count and total byte size.
func countNamespaceFiles(nsDir string) (count int, bytes int64) {
	nsEntries, err := os.ReadDir(nsDir)
	if err != nil {
		return 0, 0
	}
	for _, nsEntry := range nsEntries {
		if nsEntry.IsDir() {
			continue
		}
		info, err := nsEntry.Info()
		if err == nil {
			bytes += info.Size()
			count++
		}
	}
	return count, bytes
}

// isSafeDirName returns true if the directory entry name is safe for path
// construction (rejects path traversal and absolute paths).
func isSafeDirName(name string) bool {
	return name != ".." && !filepath.IsAbs(name) && !strings.Contains(name, "..")
}

// Stats returns storage statistics.
func (s *SessionStore) Stats() (StoreStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := StoreStats{
		Namespaces:   make(map[string]int),
		SessionCount: s.meta.SessionCount,
	}

	entries, err := os.ReadDir(s.projectDir)
	if err != nil {
		return stats, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			info, err := entry.Info()
			if err == nil {
				stats.TotalBytes += info.Size()
			}
			continue
		}

		name := entry.Name()
		if !isSafeDirName(name) {
			continue
		}

		nsDir := filepath.Join(s.projectDir, name)
		count, bytes := countNamespaceFiles(nsDir)
		stats.TotalBytes += bytes
		stats.Namespaces[name] = count
	}

	return stats, nil
}

// Session Context

// loadJSONFileAs reads a JSON file and unmarshals it into a map. Returns nil
// if the file does not exist or contains invalid JSON.
func loadJSONFileAs(path string) map[string]any {
	// #nosec G304 -- callers construct path from internal projectDir field
	data, err := os.ReadFile(path) // nosemgrep: go_filesystem_rule-fileread -- local persistence store I/O
	if err != nil {
		return nil
	}
	var result map[string]any
	if json.Unmarshal(data, &result) != nil {
		return nil
	}
	return result
}

// parseRawErrorEntry converts a generic JSON map to an ErrorHistoryEntry,
// extracting only the fields that are present and correctly typed.
func parseRawErrorEntry(raw map[string]any) ErrorHistoryEntry {
	entry := ErrorHistoryEntry{}
	if fp, ok := raw["fingerprint"].(string); ok {
		entry.Fingerprint = fp
	}
	if c, ok := raw["count"].(float64); ok {
		entry.Count = int(c)
	}
	if r, ok := raw["resolved"].(bool); ok {
		entry.Resolved = r
	}
	return entry
}

// loadErrorHistory reads and parses the error history file. It first tries
// to unmarshal as typed entries, then falls back to generic map parsing.
func loadErrorHistory(path string) []ErrorHistoryEntry {
	// #nosec G304 -- callers construct path from internal projectDir field
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []ErrorHistoryEntry
	if json.Unmarshal(data, &entries) == nil {
		return entries
	}
	// Fallback: try as generic JSON array (e.g., from tests saving raw maps)
	var rawEntries []map[string]any
	if json.Unmarshal(data, &rawEntries) != nil {
		return nil
	}
	result := make([]ErrorHistoryEntry, 0, len(rawEntries))
	for _, raw := range rawEntries {
		result = append(result, parseRawErrorEntry(raw))
	}
	return result
}

// LoadSessionContext reads all namespace summaries and returns a combined context.
func (s *SessionStore) LoadSessionContext() SessionContext {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ctx := SessionContext{
		ProjectID:    s.projectPath,
		SessionCount: s.meta.SessionCount,
		Baselines:    []string{},
		ErrorHistory: []ErrorHistoryEntry{},
	}

	baselineDir := filepath.Join(s.projectDir, "baselines")
	if keys, err := jsonKeysFromDir(baselineDir); err == nil && len(keys) > 0 {
		ctx.Baselines = keys
	}

	ctx.NoiseConfig = loadJSONFileAs(filepath.Join(s.projectDir, "noise", "config.json"))

	if entries := loadErrorHistory(filepath.Join(s.projectDir, "errors", "history.json")); entries != nil {
		ctx.ErrorHistory = entries
	}

	ctx.APISchema = loadJSONFileAs(filepath.Join(s.projectDir, "api_schema", "schema.json"))
	ctx.Performance = loadJSONFileAs(filepath.Join(s.projectDir, "performance", "endpoints.json"))

	return ctx
}

// Dirty Data + Background Flush

// MarkDirty buffers data for background flush.
func (s *SessionStore) MarkDirty(namespace, key string, data []byte) {
	if validateStoreInput(namespace, "namespace") != nil || validateStoreInput(key, "key") != nil {
		return // silently drop invalid inputs
	}
	s.dirtyMu.Lock()
	defer s.dirtyMu.Unlock()
	dirtyKey := namespace + "/" + key
	s.dirty[dirtyKey] = data
}

func (s *SessionStore) backgroundFlush() {
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.flushDirty()
		case <-s.stopCh:
			return
		}
	}
}

func (s *SessionStore) flushDirty() {
	s.dirtyMu.Lock()
	if len(s.dirty) == 0 {
		s.dirtyMu.Unlock()
		return
	}
	// Copy dirty map and clear
	toFlush := make(map[string][]byte, len(s.dirty))
	for k, v := range s.dirty {
		toFlush[k] = v
	}
	s.dirty = make(map[string][]byte)
	s.dirtyMu.Unlock()

	// Write each dirty entry
	for key, data := range toFlush {
		parts := strings.SplitN(key, "/", 2)
		if len(parts) != 2 {
			continue
		}
		namespace, name := parts[0], parts[1]

		// Defense-in-depth: validate path stays within project dir
		nsDir := filepath.Join(s.projectDir, namespace)
		filePath := filepath.Join(nsDir, name+".json")
		if validatePathInDir(s.projectDir, filePath) != nil {
			continue
		}

		if err := os.MkdirAll(nsDir, dirPermissions); err != nil {
			continue
		}

		_ = os.WriteFile(filePath, data, filePermissions)
	}
}

// Shutdown flushes dirty data, saves meta, and stops the background goroutine.
func (s *SessionStore) Shutdown() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	s.mu.Unlock()

	// Stop background flush
	close(s.stopCh)

	// Flush any remaining dirty data
	s.flushDirty()

	// Save final meta
	s.mu.Lock()
	s.meta.LastSession = time.Now()
	_ = s.saveMeta()
	s.mu.Unlock()
}

// Size Calculation

func (s *SessionStore) projectSize() (int64, error) {
	var total int64
	err := filepath.Walk(s.projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // intentionally skip errors to continue walking
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

// Error History Helpers

// enforceErrorHistoryCap ensures the error history doesn't exceed maxErrorHistory entries.
// It evicts the oldest entries (by FirstSeen) when the cap is exceeded.
func enforceErrorHistoryCap(entries []ErrorHistoryEntry) []ErrorHistoryEntry {
	if len(entries) <= maxErrorHistory {
		return entries
	}

	// Sort by FirstSeen (oldest first) and keep only the newest maxErrorHistory entries
	// O(n log n) sort is more efficient than O(n*k) loop-remove pattern
	slices.SortFunc(entries, func(a, b ErrorHistoryEntry) int {
		return a.FirstSeen.Compare(b.FirstSeen)
	})

	// Keep only the newest entries (after sorting, they're at the end)
	return entries[len(entries)-maxErrorHistory:]
}

// evictStaleErrors removes entries whose LastSeen is older than the given threshold.
func evictStaleErrors(entries []ErrorHistoryEntry, threshold time.Duration) []ErrorHistoryEntry {
	cutoff := time.Now().Add(-threshold)
	var result []ErrorHistoryEntry
	for _, e := range entries {
		if e.LastSeen.After(cutoff) {
			result = append(result, e)
		}
	}
	if result == nil {
		result = []ErrorHistoryEntry{}
	}
	return result
}

// MCP Tool Handler

// requireFields validates that required string fields are non-empty for a
// given action. Returns an error naming the first missing field.
func requireFields(action string, fields map[string]string) error {
	for name, value := range fields {
		if value == "" {
			return fmt.Errorf("%s is required for %s action", name, action)
		}
	}
	return nil
}

func (s *SessionStore) handleSave(args SessionStoreArgs) (json.RawMessage, error) {
	if err := requireFields("save", map[string]string{
		"namespace": args.Namespace,
		"key":       args.Key,
	}); err != nil {
		return nil, err
	}
	if len(args.Data) == 0 {
		return nil, fmt.Errorf("data is required for save action")
	}
	if err := s.Save(args.Namespace, args.Key, []byte(args.Data)); err != nil {
		return nil, err
	}
	// Error impossible: map contains only string values
	result, _ := json.Marshal(map[string]any{
		"status":    "saved",
		"namespace": args.Namespace,
		"key":       args.Key,
	})
	return result, nil
}

func (s *SessionStore) handleLoad(args SessionStoreArgs) (json.RawMessage, error) {
	if err := requireFields("load", map[string]string{
		"namespace": args.Namespace,
		"key":       args.Key,
	}); err != nil {
		return nil, err
	}
	data, err := s.Load(args.Namespace, args.Key)
	if err != nil {
		return nil, err
	}
	var parsed any
	_ = json.Unmarshal(data, &parsed)
	// Error impossible: map contains only primitive types and pre-parsed JSON data
	result, _ := json.Marshal(map[string]any{
		"namespace": args.Namespace,
		"key":       args.Key,
		"data":      parsed,
	})
	return result, nil
}

func (s *SessionStore) handleList(args SessionStoreArgs) (json.RawMessage, error) {
	if args.Namespace == "" {
		return nil, fmt.Errorf("namespace is required for list action")
	}
	keys, err := s.List(args.Namespace)
	if err != nil {
		return nil, err
	}
	// Error impossible: map contains only string values and string slices
	result, _ := json.Marshal(map[string]any{
		"namespace": args.Namespace,
		"keys":      keys,
	})
	return result, nil
}

func (s *SessionStore) handleDelete(args SessionStoreArgs) (json.RawMessage, error) {
	if err := requireFields("delete", map[string]string{
		"namespace": args.Namespace,
		"key":       args.Key,
	}); err != nil {
		return nil, err
	}
	if err := s.Delete(args.Namespace, args.Key); err != nil {
		return nil, err
	}
	// Error impossible: map contains only string values
	result, _ := json.Marshal(map[string]any{
		"status":    "deleted",
		"namespace": args.Namespace,
		"key":       args.Key,
	})
	return result, nil
}

func (s *SessionStore) handleStats() (json.RawMessage, error) {
	stats, err := s.Stats()
	if err != nil {
		return nil, err
	}
	// Error impossible: map contains only primitive types and string slices
	result, _ := json.Marshal(map[string]any{
		"total_bytes":   stats.TotalBytes,
		"session_count": stats.SessionCount,
		"namespaces":    stats.Namespaces,
	})
	return result, nil
}

// HandleSessionStore handles the session_store MCP tool actions.
func (s *SessionStore) HandleSessionStore(args SessionStoreArgs) (json.RawMessage, error) {
	switch args.Action {
	case "save":
		return s.handleSave(args)
	case "load":
		return s.handleLoad(args)
	case "list":
		return s.handleList(args)
	case "delete":
		return s.handleDelete(args)
	case "stats":
		return s.handleStats()
	default:
		return nil, fmt.Errorf("unknown action: %s (valid: save, load, list, delete, stats)", args.Action)
	}
}
