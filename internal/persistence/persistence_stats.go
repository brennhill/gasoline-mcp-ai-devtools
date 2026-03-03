// Purpose: Counts files and bytes per namespace for persistence store statistics.
// Why: Isolates storage accounting from CRUD operations and maintenance.
package persistence

import (
	"os"
	"path/filepath"
	"strings"
)

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

func isSafeDirName(name string) bool {
	return name != ".." && !filepath.IsAbs(name) && !strings.Contains(name, "..")
}

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
