// ai_persistence_helpers.go — Stats, session context, dirty-data flush, and error-history helpers for SessionStore.
package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

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
