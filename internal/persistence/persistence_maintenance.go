// Purpose: Calculates project storage size and enforces size-based eviction of oldest namespaces.
// Why: Separates storage maintenance and eviction from CRUD and initialization.
package persistence

import (
	"os"
	"path/filepath"
)

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
