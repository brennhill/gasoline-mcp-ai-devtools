// annotation_store.go — In-memory store for draw mode annotation sessions.
// Stores annotation data and element details with TTL-based expiration.
package main

import (
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/util"
)

// globalAnnotationStore is the shared annotation store used by both HTTP routes and tool handlers.
var globalAnnotationStore = NewAnnotationStore(10 * time.Minute)

// AnnotationRect represents a viewport-relative rectangle.
type AnnotationRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Annotation is a lightweight annotation returned by default.
type Annotation struct {
	ID             string         `json:"id"`
	Rect           AnnotationRect `json:"rect"`
	Text           string         `json:"text"`
	Timestamp      int64          `json:"timestamp"`
	PageURL        string         `json:"page_url"`
	ElementSummary string         `json:"element_summary"`
	CorrelationID  string         `json:"correlation_id"`
}

// AnnotationDetail contains full DOM/style detail for lazy retrieval.
type AnnotationDetail struct {
	CorrelationID  string            `json:"correlation_id"`
	Selector       string            `json:"selector"`
	Tag            string            `json:"tag"`
	TextContent    string            `json:"text_content"`
	Classes        []string          `json:"classes"`
	ID             string            `json:"id"`
	ComputedStyles map[string]string `json:"computed_styles"`
	ParentSelector string            `json:"parent_selector"`
	BoundingRect   AnnotationRect    `json:"bounding_rect"`
	A11yFlags      []string          `json:"a11y_flags,omitempty"`
}

// AnnotationSession represents a completed draw mode session.
type AnnotationSession struct {
	Annotations    []Annotation `json:"annotations"`
	ScreenshotPath string       `json:"screenshot"`
	PageURL        string       `json:"page_url"`
	TabID          int          `json:"tab_id"`
	Timestamp      int64        `json:"timestamp"`
}

// annotationDetailEntry wraps detail with expiration time.
type annotationDetailEntry struct {
	Detail    AnnotationDetail
	ExpiresAt time.Time
}

// annotationSessionEntry wraps session with expiration time.
type annotationSessionEntry struct {
	Session   *AnnotationSession
	ExpiresAt time.Time
}

const maxSessions = 100
const maxNamedSessions = 50

// NamedAnnotationSession accumulates annotations across multiple pages.
type NamedAnnotationSession struct {
	Name      string               `json:"name"`
	Pages     []*AnnotationSession `json:"pages"`
	UpdatedAt int64                `json:"updated_at"`
}

// namedSessionEntry wraps a named session with TTL.
type namedSessionEntry struct {
	Session   *NamedAnnotationSession
	ExpiresAt time.Time
}

// AnnotationStore manages annotation sessions and details in memory.
type AnnotationStore struct {
	mu       sync.RWMutex
	sessions map[int]*annotationSessionEntry   // tabID → session with TTL
	details  map[string]*annotationDetailEntry // correlationID → detail with TTL
	named    map[string]*namedSessionEntry     // session name → multi-page session

	detailTTL  time.Duration
	sessionTTL time.Duration
	done       chan struct{} // signals cleanup goroutine to stop
	closeOnce  sync.Once     // ensures Close() is safe to call concurrently

	// Blocking wait support
	sessionNotify     chan struct{} // closed on StoreSession, then recreated
	lastDrawStartedAt int64         // millis; set by MarkDrawStarted
}

// NewAnnotationStore creates a new store with the given detail TTL.
func NewAnnotationStore(detailTTL time.Duration) *AnnotationStore {
	s := &AnnotationStore{
		sessions:      make(map[int]*annotationSessionEntry),
		details:       make(map[string]*annotationDetailEntry),
		named:         make(map[string]*namedSessionEntry),
		detailTTL:     detailTTL,
		sessionTTL:    30 * time.Minute,
		done:          make(chan struct{}),
		sessionNotify: make(chan struct{}),
	}
	// Start background cleanup goroutine
	util.SafeGo(func() { s.cleanupLoop() })
	return s
}

// Close stops the background cleanup goroutine. Safe to call concurrently.
func (s *AnnotationStore) Close() {
	s.closeOnce.Do(func() {
		close(s.done)
	})
}

// StoreSession saves an annotation session, overwriting any previous session for the tab.
// Notifies any goroutines blocked in WaitForSession.
func (s *AnnotationStore) StoreSession(tabID int, session *AnnotationSession) {
	s.mu.Lock()
	s.sessions[tabID] = &annotationSessionEntry{
		Session:   session,
		ExpiresAt: time.Now().Add(s.sessionTTL),
	}
	// Evict oldest sessions if over cap
	if len(s.sessions) > maxSessions {
		s.evictOldestSessionLocked()
	}
	// Notify waiters: close current channel, create a fresh one
	ch := s.sessionNotify
	s.sessionNotify = make(chan struct{})
	s.mu.Unlock()
	close(ch)
}

// MarkDrawStarted records the current time so WaitForSession can skip stale sessions.
func (s *AnnotationStore) MarkDrawStarted() {
	s.mu.Lock()
	s.lastDrawStartedAt = time.Now().UnixMilli()
	s.mu.Unlock()
}

// WaitForSession blocks until a session newer than the last MarkDrawStarted arrives,
// or until timeout expires. Returns (session, timedOut).
// Loops on wake-ups to handle spurious notifications from unrelated sessions.
func (s *AnnotationStore) WaitForSession(timeout time.Duration) (*AnnotationSession, bool) {
	s.mu.RLock()
	sinceTs := s.lastDrawStartedAt
	s.mu.RUnlock()

	checker := func() any {
		if session := s.getSessionSince(sinceTs); session != nil {
			return session
		}
		return nil
	}

	result, timedOut := s.waitForCondition(timeout, checker)
	if result != nil {
		return result.(*AnnotationSession), false
	}
	return nil, timedOut
}

// getSessionSince returns the latest session with Timestamp > sinceTs.
func (s *AnnotationStore) getSessionSince(sinceTs int64) *AnnotationSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	var latest *annotationSessionEntry
	for _, entry := range s.sessions {
		if now.After(entry.ExpiresAt) {
			continue
		}
		if entry.Session.Timestamp <= sinceTs {
			continue
		}
		if latest == nil || entry.Session.Timestamp > latest.Session.Timestamp {
			latest = entry
		}
	}
	if latest == nil {
		return nil
	}
	return latest.Session
}

// GetSession returns the latest annotation session for a tab.
func (s *AnnotationStore) GetSession(tabID int) *AnnotationSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry := s.sessions[tabID]
	if entry == nil {
		return nil
	}
	if time.Now().After(entry.ExpiresAt) {
		return nil
	}
	return entry.Session
}

// GetLatestSession returns the most recent annotation session across all tabs.
func (s *AnnotationStore) GetLatestSession() *AnnotationSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	var latest *annotationSessionEntry
	for _, entry := range s.sessions {
		if now.After(entry.ExpiresAt) {
			continue
		}
		if latest == nil || entry.Session.Timestamp > latest.Session.Timestamp {
			latest = entry
		}
	}
	if latest == nil {
		return nil
	}
	return latest.Session
}

// StoreDetail saves element detail with TTL expiration.
func (s *AnnotationStore) StoreDetail(correlationID string, detail AnnotationDetail) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.details[correlationID] = &annotationDetailEntry{
		Detail:    detail,
		ExpiresAt: time.Now().Add(s.detailTTL),
	}
}

// GetDetail retrieves element detail if not expired.
func (s *AnnotationStore) GetDetail(correlationID string) (*AnnotationDetail, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.details[correlationID]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}
	return &entry.Detail, true
}

// AppendToNamedSession adds a page to a named multi-page session.
// Creates the session if it doesn't exist. Also fires the session notify.
func (s *AnnotationStore) AppendToNamedSession(name string, session *AnnotationSession) {
	s.mu.Lock()
	entry, ok := s.named[name]
	if !ok {
		entry = &namedSessionEntry{
			Session: &NamedAnnotationSession{
				Name: name,
			},
		}
		s.named[name] = entry
	}
	entry.Session.Pages = append(entry.Session.Pages, session)
	entry.Session.UpdatedAt = time.Now().UnixMilli()
	entry.ExpiresAt = time.Now().Add(s.sessionTTL)
	// Evict oldest named sessions if over cap
	if len(s.named) > maxNamedSessions {
		s.evictOldestNamedSessionLocked()
	}
	// Notify waiters
	ch := s.sessionNotify
	s.sessionNotify = make(chan struct{})
	s.mu.Unlock()
	close(ch)
}

// GetNamedSession returns a snapshot of the named multi-page session if not expired.
// Returns a shallow copy with its own Pages slice to avoid data races with concurrent
// AppendToNamedSession calls that modify the internal Pages slice under write lock.
func (s *AnnotationStore) GetNamedSession(name string) *NamedAnnotationSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.named[name]
	if !ok {
		return nil
	}
	if time.Now().After(entry.ExpiresAt) {
		return nil
	}
	// Return a copy so callers can safely iterate Pages without holding the lock
	copied := *entry.Session
	copied.Pages = make([]*AnnotationSession, len(entry.Session.Pages))
	copy(copied.Pages, entry.Session.Pages)
	return &copied
}

// ListNamedSessions returns the names of all non-expired named sessions.
func (s *AnnotationStore) ListNamedSessions() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	names := make([]string, 0, len(s.named))
	for name, entry := range s.named {
		if !now.After(entry.ExpiresAt) {
			names = append(names, name)
		}
	}
	return names
}

// ClearNamedSession removes a named session.
func (s *AnnotationStore) ClearNamedSession(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.named, name)
}

// evictOldestNamedSessionLocked removes the named session with the oldest UpdatedAt.
// Must be called with s.mu held.
func (s *AnnotationStore) evictOldestNamedSessionLocked() {
	var oldestKey string
	var oldestTime int64 = 1<<63 - 1
	for name, entry := range s.named {
		if entry.Session.UpdatedAt < oldestTime {
			oldestTime = entry.Session.UpdatedAt
			oldestKey = name
		}
	}
	delete(s.named, oldestKey)
}

// WaitForNamedSession blocks until the named session has been updated since sinceTs,
// or until timeout expires. Returns (session, timedOut).
// Loops on wake-ups to handle spurious notifications from unrelated sessions.
func (s *AnnotationStore) WaitForNamedSession(name string, timeout time.Duration) (*NamedAnnotationSession, bool) {
	s.mu.RLock()
	sinceTs := s.lastDrawStartedAt
	s.mu.RUnlock()

	checker := func() any {
		if ns := s.GetNamedSession(name); ns != nil && ns.UpdatedAt > sinceTs {
			return ns
		}
		return nil
	}

	result, timedOut := s.waitForCondition(timeout, checker)
	if result != nil {
		return result.(*NamedAnnotationSession), false
	}
	return nil, timedOut
}

// waitForCondition blocks until checker returns non-nil, timeout, or store close.
// Returns (result, timedOut). result is nil if timed out or store closed.
func (s *AnnotationStore) waitForCondition(timeout time.Duration, checker func() any) (any, bool) {
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

// evictOldestSessionLocked removes the session with the oldest timestamp.
// Must be called with s.mu held.
func (s *AnnotationStore) evictOldestSessionLocked() {
	var oldestKey int
	var oldestTime int64 = 1<<63 - 1
	for tabID, entry := range s.sessions {
		if entry.Session.Timestamp < oldestTime {
			oldestTime = entry.Session.Timestamp
			oldestKey = tabID
		}
	}
	delete(s.sessions, oldestKey)
}

// cleanupLoop periodically removes expired sessions and detail entries.
func (s *AnnotationStore) cleanupLoop() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.evictExpiredEntries()
		}
	}
}

// evictExpiredEntries removes all expired details, sessions, and named sessions.
func (s *AnnotationStore) evictExpiredEntries() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for id, entry := range s.details {
		if now.After(entry.ExpiresAt) {
			delete(s.details, id)
		}
	}
	for tabID, entry := range s.sessions {
		if now.After(entry.ExpiresAt) {
			delete(s.sessions, tabID)
		}
	}
	for name, entry := range s.named {
		if now.After(entry.ExpiresAt) {
			delete(s.named, name)
		}
	}
}
