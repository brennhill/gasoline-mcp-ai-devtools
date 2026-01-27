// ai_persistence.go — File-based persistent key-value store for session data.
// Provides namespace-scoped storage that survives server restarts, enabling AI
// assistants to save and retrieve structured data across sessions.
// Design: Each namespace maps to a JSON file on disk. Individual values are capped
// at 1MB, total storage per project at 10MB. Operations are mutex-guarded for
// concurrent safety. Supports save, load, list, delete, and stats actions.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ============================================
// Constants
// ============================================

const (
	maxFileSize          = 1 * 1024 * 1024  // 1MB per file
	maxProjectSize       = 10 * 1024 * 1024 // 10MB per project
	maxErrorHistory      = 500
	staleErrorThreshold  = 30 * 24 * time.Hour // 30 days
	defaultFlushInterval = 30 * time.Second
	dirPermissions       = 0o755
	filePermissions      = 0o644
)

// ============================================
// Types
// ============================================

// SessionStore provides persistent cross-session memory backed by disk.
// Data is stored in .gasoline/ within the project directory.
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
	ProjectID    string                 `json:"project_id"`
	SessionCount int                    `json:"session_count"`
	Baselines    []string               `json:"baselines"`
	NoiseConfig  map[string]interface{} `json:"noise_config,omitempty"`
	ErrorHistory []ErrorHistoryEntry    `json:"error_history"`
	APISchema    map[string]interface{} `json:"api_schema,omitempty"`
	Performance  map[string]interface{} `json:"performance,omitempty"`
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

// ============================================
// Constructor
// ============================================

// NewSessionStore creates a new SessionStore with default flush interval.
// projectPath is the project root directory; data is stored in projectPath/.gasoline/
func NewSessionStore(projectPath string) (*SessionStore, error) {
	return NewSessionStoreWithInterval(projectPath, defaultFlushInterval)
}

// NewSessionStoreWithInterval creates a new SessionStore with a custom flush interval.
func NewSessionStoreWithInterval(projectPath string, flushInterval time.Duration) (*SessionStore, error) {
	// Validate and clean the path to prevent directory traversal
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, fmt.Errorf("invalid project path: %w", err)
	}

	// Ensure the resolved path doesn't contain suspicious patterns
	// (This catches .. and symlink traversal)
	if strings.Contains(absPath, "..") {
		return nil, fmt.Errorf("project path contains '..': %s", absPath)
	}

	projectDir := filepath.Join(absPath, ".gasoline")

	s := &SessionStore{
		projectPath:   absPath,
		projectDir:    projectDir,
		dirty:         make(map[string][]byte),
		flushInterval: flushInterval,
		stopCh:        make(chan struct{}),
	}

	// Create .gasoline directory
	if err := os.MkdirAll(projectDir, dirPermissions); err != nil {
		return nil, fmt.Errorf("failed to create .gasoline directory: %w", err)
	}

	// Ensure .gasoline is in .gitignore
	s.ensureGitignore()

	// Load or create meta
	if err := s.loadOrCreateMeta(); err != nil {
		return nil, fmt.Errorf("failed to load meta: %w", err)
	}

	// Start background flush goroutine
	go s.backgroundFlush()

	return s, nil
}

// ensureGitignore adds .gasoline/ to .gitignore if not already present.
func (s *SessionStore) ensureGitignore() {
	gitignorePath := filepath.Join(s.projectPath, ".gitignore")

	// Check if .gitignore exists and already contains .gasoline
	// #nosec G304 -- path is constructed from internal projectPath field
	if data, err := os.ReadFile(gitignorePath); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == ".gasoline" || line == ".gasoline/" {
				return // already present
			}
		}
		// Append to existing file
		f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, filePermissions) // #nosec G304 -- path is constructed from internal projectPath field
		if err != nil {
			return
		}
		// Add newline before if file doesn't end with one
		if len(data) > 0 && data[len(data)-1] != '\n' {
			_, _ = f.WriteString("\n")
		}
		_, _ = f.WriteString(".gasoline/\n")
		_ = f.Close()
	}
	// If .gitignore doesn't exist, don't create one (might not be a git repo)
}

// ============================================
// Meta Operations
// ============================================

func (s *SessionStore) loadOrCreateMeta() error {
	metaPath := filepath.Join(s.projectDir, "meta.json")
	data, err := os.ReadFile(metaPath) // #nosec G304 -- path is constructed from internal projectDir field
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

// ============================================
// Path Validation
// ============================================

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

// ============================================
// Core Operations
// ============================================

// Save writes data as JSON to <namespace>/<key>.json.
func (s *SessionStore) Save(namespace, key string, data []byte) error {
	if err := validateStoreInput(namespace, "namespace"); err != nil {
		return err
	}
	if err := validateStoreInput(key, "key"); err != nil {
		return err
	}

	if len(data) > maxFileSize {
		return fmt.Errorf("data exceeds maximum file size (1MB): %d bytes", len(data))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check project size limit
	currentSize, err := s.projectSize()
	if err == nil && currentSize+int64(len(data)) > maxProjectSize {
		return fmt.Errorf("project size limit exceeded (10MB): current=%d, adding=%d", currentSize, len(data))
	}

	nsDir := filepath.Join(s.projectDir, namespace)
	if err := validatePathInDir(s.projectDir, nsDir); err != nil {
		return err
	}

	if err := os.MkdirAll(nsDir, dirPermissions); err != nil {
		return fmt.Errorf("failed to create namespace directory: %w", err)
	}

	filePath := filepath.Join(nsDir, key+".json")
	if err := validatePathInDir(s.projectDir, filePath); err != nil {
		return err
	}

	if err := os.WriteFile(filePath, data, filePermissions); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Load reads and returns the JSON from <namespace>/<key>.json.
func (s *SessionStore) Load(namespace, key string) ([]byte, error) {
	if err := validateStoreInput(namespace, "namespace"); err != nil {
		return nil, err
	}
	if err := validateStoreInput(key, "key"); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	filePath := filepath.Join(s.projectDir, namespace, key+".json")
	if err := validatePathInDir(s.projectDir, filePath); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filePath) // #nosec G304 -- path validated above
	if err != nil {
		return nil, fmt.Errorf("key not found: %s/%s", namespace, key)
	}
	return data, nil
}

// List returns all keys in a namespace (file names without .json extension).
func (s *SessionStore) List(namespace string) ([]string, error) {
	if err := validateStoreInput(namespace, "namespace"); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	nsDir := filepath.Join(s.projectDir, namespace)
	if err := validatePathInDir(s.projectDir, nsDir); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(nsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var keys []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".json") {
			keys = append(keys, strings.TrimSuffix(name, ".json"))
		}
	}

	if keys == nil {
		keys = []string{}
	}
	return keys, nil
}

// Delete removes the file for a given namespace/key.
func (s *SessionStore) Delete(namespace, key string) error {
	if err := validateStoreInput(namespace, "namespace"); err != nil {
		return err
	}
	if err := validateStoreInput(key, "key"); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := filepath.Join(s.projectDir, namespace, key+".json")
	if err := validatePathInDir(s.projectDir, filePath); err != nil {
		return err
	}

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete: %s/%s: %w", namespace, key, err)
	}
	return nil
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
			// Count meta.json size
			info, err := entry.Info()
			if err == nil {
				stats.TotalBytes += info.Size()
			}
			continue
		}

		nsDir := filepath.Join(s.projectDir, entry.Name())
		nsEntries, err := os.ReadDir(nsDir)
		if err != nil {
			continue
		}

		count := 0
		for _, nsEntry := range nsEntries {
			if nsEntry.IsDir() {
				continue
			}
			info, err := nsEntry.Info()
			if err == nil {
				stats.TotalBytes += info.Size()
				count++
			}
		}
		stats.Namespaces[entry.Name()] = count
	}

	return stats, nil
}

// ============================================
// Session Context
// ============================================

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

	// Load baselines list
	baselineDir := filepath.Join(s.projectDir, "baselines")
	if entries, err := os.ReadDir(baselineDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				ctx.Baselines = append(ctx.Baselines, strings.TrimSuffix(e.Name(), ".json"))
			}
		}
	}

	// Load noise config
	noisePath := filepath.Join(s.projectDir, "noise", "config.json")
	// #nosec G304 -- path is constructed from internal projectDir field
	if data, err := os.ReadFile(noisePath); err == nil {
		var config map[string]interface{}
		if json.Unmarshal(data, &config) == nil {
			ctx.NoiseConfig = config
		}
	}

	// Load error history
	errPath := filepath.Join(s.projectDir, "errors", "history.json")
	// #nosec G304 -- path is constructed from internal projectDir field
	if data, err := os.ReadFile(errPath); err == nil {
		// Try to unmarshal as []ErrorHistoryEntry first
		var entries []ErrorHistoryEntry
		if json.Unmarshal(data, &entries) == nil {
			ctx.ErrorHistory = entries
		} else {
			// Try as generic JSON array (e.g., from tests saving raw maps)
			var rawEntries []map[string]interface{}
			if json.Unmarshal(data, &rawEntries) == nil {
				for _, raw := range rawEntries {
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
					ctx.ErrorHistory = append(ctx.ErrorHistory, entry)
				}
			}
		}
	}

	// Load API schema
	schemaPath := filepath.Join(s.projectDir, "api_schema", "schema.json")
	// #nosec G304 -- path is constructed from internal projectDir field
	if data, err := os.ReadFile(schemaPath); err == nil {
		var schema map[string]interface{}
		if json.Unmarshal(data, &schema) == nil {
			ctx.APISchema = schema
		}
	}

	// Load performance
	perfPath := filepath.Join(s.projectDir, "performance", "endpoints.json")
	// #nosec G304 -- path is constructed from internal projectDir field
	if data, err := os.ReadFile(perfPath); err == nil {
		var perf map[string]interface{}
		if json.Unmarshal(data, &perf) == nil {
			ctx.Performance = perf
		}
	}

	return ctx
}

// ============================================
// Dirty Data + Background Flush
// ============================================

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

// ============================================
// Size Calculation
// ============================================

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

// ============================================
// Error History Helpers
// ============================================

// enforceErrorHistoryCap ensures the error history doesn't exceed maxErrorHistory entries.
// It evicts the oldest entries (by FirstSeen) when the cap is exceeded.
func enforceErrorHistoryCap(entries []ErrorHistoryEntry) []ErrorHistoryEntry {
	if len(entries) <= maxErrorHistory {
		return entries
	}

	// Sort by FirstSeen (newest first) and keep only maxErrorHistory
	// Simple approach: find and remove the oldest
	for len(entries) > maxErrorHistory {
		oldestIdx := 0
		for i := 1; i < len(entries); i++ {
			if entries[i].FirstSeen.Before(entries[oldestIdx].FirstSeen) {
				oldestIdx = i
			}
		}
		entries = append(entries[:oldestIdx], entries[oldestIdx+1:]...)
	}

	return entries
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

// ============================================
// MCP Tool Handler
// ============================================

// HandleSessionStore handles the session_store MCP tool actions.
func (s *SessionStore) HandleSessionStore(args SessionStoreArgs) (json.RawMessage, error) {
	switch args.Action {
	case "save":
		if args.Namespace == "" {
			return nil, fmt.Errorf("namespace is required for save action")
		}
		if args.Key == "" {
			return nil, fmt.Errorf("key is required for save action")
		}
		if len(args.Data) == 0 {
			return nil, fmt.Errorf("data is required for save action")
		}
		if err := s.Save(args.Namespace, args.Key, []byte(args.Data)); err != nil {
			return nil, err
		}
		// Error impossible: map[string]interface{} with primitive values is always serializable
		result, _ := json.Marshal(map[string]interface{}{
			"status":    "saved",
			"namespace": args.Namespace,
			"key":       args.Key,
		})
		return result, nil

	case "load":
		if args.Namespace == "" {
			return nil, fmt.Errorf("namespace is required for load action")
		}
		if args.Key == "" {
			return nil, fmt.Errorf("key is required for load action")
		}
		data, err := s.Load(args.Namespace, args.Key)
		if err != nil {
			return nil, err
		}
		var parsed interface{}
		_ = json.Unmarshal(data, &parsed)
		// Error impossible: map[string]interface{} with primitive values is always serializable
		result, _ := json.Marshal(map[string]interface{}{
			"namespace": args.Namespace,
			"key":       args.Key,
			"data":      parsed,
		})
		return result, nil

	case "list":
		if args.Namespace == "" {
			return nil, fmt.Errorf("namespace is required for list action")
		}
		keys, err := s.List(args.Namespace)
		if err != nil {
			return nil, err
		}
		// Error impossible: map[string]interface{} with primitive values is always serializable
		result, _ := json.Marshal(map[string]interface{}{
			"namespace": args.Namespace,
			"keys":      keys,
		})
		return result, nil

	case "delete":
		if args.Namespace == "" {
			return nil, fmt.Errorf("namespace is required for delete action")
		}
		if args.Key == "" {
			return nil, fmt.Errorf("key is required for delete action")
		}
		if err := s.Delete(args.Namespace, args.Key); err != nil {
			return nil, err
		}
		// Error impossible: map[string]interface{} with primitive values is always serializable
		result, _ := json.Marshal(map[string]interface{}{
			"status":    "deleted",
			"namespace": args.Namespace,
			"key":       args.Key,
		})
		return result, nil

	case "stats":
		stats, err := s.Stats()
		if err != nil {
			return nil, err
		}
		// Error impossible: map[string]interface{} with primitive values is always serializable
		result, _ := json.Marshal(map[string]interface{}{
			"total_bytes":   stats.TotalBytes,
			"session_count": stats.SessionCount,
			"namespaces":    stats.Namespaces,
		})
		return result, nil

	default:
		return nil, fmt.Errorf("unknown action: %s (valid: save, load, list, delete, stats)", args.Action)
	}
}
