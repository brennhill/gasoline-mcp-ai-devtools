// Purpose: Implements dirty-write tracking and background flush for deferred persistence writes.
// Why: Separates write-coalescing logic from immediate CRUD operations.
package persistence

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *SessionStore) MarkDirty(namespace, key string, data []byte) {
	if validateStoreInput(namespace, "namespace") != nil || validateStoreInput(key, "key") != nil {
		return
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
	toFlush := func() map[string][]byte {
		s.dirtyMu.Lock()
		defer s.dirtyMu.Unlock()
		if len(s.dirty) == 0 {
			return nil
		}

		copied := make(map[string][]byte, len(s.dirty))
		for k, v := range s.dirty {
			copied[k] = v
		}
		s.dirty = make(map[string][]byte)
		return copied
	}()
	if len(toFlush) == 0 {
		return
	}

	for key, data := range toFlush {
		parts := strings.SplitN(key, "/", 2)
		if len(parts) != 2 {
			continue
		}
		namespace, name := parts[0], parts[1]

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

func (s *SessionStore) Shutdown() {
	shouldShutdown := func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.stopped {
			return false
		}
		s.stopped = true
		return true
	}()
	if !shouldShutdown {
		return
	}

	close(s.stopCh)
	s.flushDirty()

	func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.meta.LastSession = time.Now()
		_ = s.saveMeta()
	}()
}
