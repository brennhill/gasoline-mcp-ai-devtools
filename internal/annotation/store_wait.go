// Purpose: Provides shared blocking wait primitive for session and named-session updates.
// Why: Centralizes timeout and wakeup semantics to keep wait paths consistent and testable.
// Docs: docs/features/feature/annotated-screenshots/index.md

package annotation

import "time"

// waitForCondition blocks until checker returns non-nil, timeout, or store close.
// Returns (result, timedOut). result is nil if timed out or store closed.
func (s *Store) waitForCondition(timeout time.Duration, checker func() any) (any, bool) {
	if result := checker(); result != nil {
		return result, false
	}

	deadline := time.Now().Add(timeout)
	for {
		s.mu.RLock()
		ch := s.sessionNotify
		s.mu.RUnlock()

		remaining := time.Until(deadline)
		if remaining <= 0 {
			if result := checker(); result != nil {
				return result, false
			}
			return nil, true
		}

		timer := time.NewTimer(remaining)
		select {
		case <-ch:
			timer.Stop()
			if result := checker(); result != nil {
				return result, false
			}
			continue
		case <-timer.C:
			if result := checker(); result != nil {
				return result, false
			}
			return nil, true
		case <-s.done:
			timer.Stop()
			return nil, false
		}
	}
}
