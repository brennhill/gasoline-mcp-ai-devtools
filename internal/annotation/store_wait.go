// Purpose: Provides shared blocking wait primitive for session and named-session updates.
// Why: Centralizes timeout and wakeup semantics to keep wait paths consistent and testable.
// Docs: docs/features/feature/annotated-screenshots/index.md

package annotation

import "time"

// TakeWaiter removes and returns a pending async annotation waiter by correlation_id.
// Returns (sessionName, true) when found, or ("", false) when missing.
func (s *Store) TakeWaiter(correlationID string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, w := range s.waiters {
		if w.CorrelationID != correlationID {
			continue
		}
		s.waiters = append(s.waiters[:i], s.waiters[i+1:]...)
		return w.AnnotSessionName, true
	}
	return "", false
}

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
