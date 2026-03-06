// Purpose: Provides explicit reset semantics for all in-memory annotation state.
// Why: Ensures configure clear operations remove stale sessions/details and waiters deterministically.
// Docs: docs/features/feature/annotated-screenshots/index.md

package annotation

// ClearCounts reports how many annotation-store entries were removed by ClearAll.
type ClearCounts struct {
	Sessions      int
	Details       int
	NamedSessions int
	Waiters       int
}

// ClearAll removes all anonymous sessions, named sessions, detail entries, and waiters.
// It also resets draw-start time so future waits evaluate against fresh state.
func (s *Store) ClearAll() ClearCounts {
	type clearPlan struct {
		counts ClearCounts
		ch     chan struct{}
	}
	plan := func() clearPlan {
		s.mu.Lock()
		defer s.mu.Unlock()

		counts := ClearCounts{
			Sessions:      len(s.sessions),
			Details:       len(s.details),
			NamedSessions: len(s.named),
			Waiters:       len(s.waiters),
		}

		s.sessions = make(map[int]*sessionEntry)
		s.details = make(map[string]*detailEntry)
		s.named = make(map[string]*namedSessionEntry)
		s.waiters = nil
		s.lastDrawStartedAt = 0

		ch := s.sessionNotify
		s.sessionNotify = make(chan struct{})

		return clearPlan{counts: counts, ch: ch}
	}()

	close(plan.ch)
	return plan.counts
}
