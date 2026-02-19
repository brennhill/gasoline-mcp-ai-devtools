// store.go — In-memory store for draw mode annotation sessions.
// Stores annotation data and element details with TTL-based expiration.
package annotation

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/util"
)

// Rect represents a viewport-relative rectangle.
type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Annotation is a lightweight annotation returned by default.
type Annotation struct {
	ID             string `json:"id"`
	Rect           Rect   `json:"rect"`
	Text           string `json:"text"`
	Timestamp      int64  `json:"timestamp"`
	PageURL        string `json:"page_url"`
	ElementSummary string `json:"element_summary"`
	CorrelationID  string `json:"correlation_id"`
}

// Detail contains full DOM/style detail for lazy retrieval.
type Detail struct {
	CorrelationID  string            `json:"correlation_id"`
	Selector       string            `json:"selector"`
	Tag            string            `json:"tag"`
	TextContent    string            `json:"text_content"`
	OuterHTML      string            `json:"outer_html,omitempty"`
	Classes        []string          `json:"classes"`
	ID             string            `json:"id"`
	ComputedStyles map[string]string `json:"computed_styles"`
	ParentSelector string            `json:"parent_selector"`
	BoundingRect   Rect              `json:"bounding_rect"`
	A11yFlags      []string          `json:"a11y_flags,omitempty"`
	ShadowDOM      json.RawMessage   `json:"shadow_dom,omitempty"`
	AllElements    json.RawMessage   `json:"all_elements,omitempty"`
	ElementCount   int               `json:"element_count,omitempty"`
	IframeContent  json.RawMessage   `json:"iframe_content,omitempty"`
}

// Session represents a completed draw mode session.
type Session struct {
	Annotations    []Annotation `json:"annotations"`
	ScreenshotPath string       `json:"screenshot"`
	PageURL        string       `json:"page_url"`
	TabID          int          `json:"tab_id"`
	Timestamp      int64        `json:"timestamp"`
}

// detailEntry wraps detail with expiration time.
type detailEntry struct {
	Detail    Detail
	ExpiresAt time.Time
}

// sessionEntry wraps session with expiration time.
type sessionEntry struct {
	Session   *Session
	ExpiresAt time.Time
}

const MaxSessions = 100
const MaxNamedSessions = 50
const MaxDetails = 500

// NamedSession accumulates annotations across multiple pages.
type NamedSession struct {
	Name      string     `json:"name"`
	Pages     []*Session `json:"pages"`
	UpdatedAt int64      `json:"updated_at"`
}

// namedSessionEntry wraps a named session with TTL.
type namedSessionEntry struct {
	Session   *NamedSession
	ExpiresAt time.Time
}

// waiter is a pending correlation_id waiting for annotations to arrive.
// When annotations are stored, all matching waiters are completed via the callback.
type waiter struct {
	CorrelationID    string // command tracker correlation_id
	AnnotSessionName string // "" for anonymous, non-empty for named session
}

// Store manages annotation sessions and details in memory.
type Store struct {
	mu       sync.RWMutex
	sessions map[int]*sessionEntry    // tabID → session with TTL
	details  map[string]*detailEntry  // correlationID → detail with TTL
	named    map[string]*namedSessionEntry // session name → multi-page session

	detailTTL  time.Duration
	sessionTTL time.Duration
	done       chan struct{} // signals cleanup goroutine to stop
	closeOnce  sync.Once    // ensures Close() is safe to call concurrently

	// Blocking wait support
	sessionNotify     chan struct{} // closed on StoreSession, then recreated
	lastDrawStartedAt int64        // millis; set by MarkDrawStarted

	// Async wait support — LLM polls via observe({what: "command_result"})
	waiters         []waiter
	completeCommand func(correlationID string, result json.RawMessage) // callback to complete CommandTracker
}

// NewStore creates a new store with the given detail TTL.
func NewStore(detailTTL time.Duration) *Store {
	s := &Store{
		sessions:      make(map[int]*sessionEntry),
		details:       make(map[string]*detailEntry),
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
func (s *Store) Close() {
	s.closeOnce.Do(func() {
		close(s.done)
	})
}

// SetCommandCompleter sets the callback used to complete async annotation waiters.
// Must be called before any waiters are registered (typically at server startup).
func (s *Store) SetCommandCompleter(fn func(correlationID string, result json.RawMessage)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completeCommand = fn
}

// RegisterWaiter registers a correlation_id to be completed when annotations arrive.
// sessionName is "" for anonymous sessions, or a name for named sessions.
func (s *Store) RegisterWaiter(correlationID string, sessionName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.waiters = append(s.waiters, waiter{
		CorrelationID:    correlationID,
		AnnotSessionName: sessionName,
	})
}

// GetLatestSessionSinceDraw returns the latest session newer than MarkDrawStarted, or nil.
func (s *Store) GetLatestSessionSinceDraw() *Session {
	s.mu.RLock()
	sinceTs := s.lastDrawStartedAt
	s.mu.RUnlock()
	return s.getSessionSince(sinceTs)
}

// GetNamedSessionSinceDraw returns the named session if updated after MarkDrawStarted, or nil.
func (s *Store) GetNamedSessionSinceDraw(name string) *NamedSession {
	s.mu.RLock()
	sinceTs := s.lastDrawStartedAt
	s.mu.RUnlock()
	ns := s.GetNamedSession(name)
	if ns != nil && ns.UpdatedAt > sinceTs {
		return ns
	}
	return nil
}

// StoreSession saves an annotation session, overwriting any previous session for the tab.
// Notifies any goroutines blocked in WaitForSession and completes async waiters.
func (s *Store) StoreSession(tabID int, session *Session) {
	s.mu.Lock()
	s.sessions[tabID] = &sessionEntry{
		Session:   session,
		ExpiresAt: time.Now().Add(s.sessionTTL),
	}
	// Evict oldest sessions if over cap
	if len(s.sessions) > MaxSessions {
		s.evictOldestSessionLocked()
	}
	// Collect and clear anonymous waiters (sessionName == "")
	var toComplete []waiter
	remaining := s.waiters[:0]
	for _, w := range s.waiters {
		if w.AnnotSessionName == "" {
			toComplete = append(toComplete, w)
		} else {
			remaining = append(remaining, w)
		}
	}
	s.waiters = remaining
	completeFn := s.completeCommand
	// Notify blocking waiters: close current channel, create a fresh one
	ch := s.sessionNotify
	s.sessionNotify = make(chan struct{})
	s.mu.Unlock()
	close(ch)

	// Complete async waiters outside the lock
	if completeFn != nil {
		result := BuildSessionResult(session)
		for _, w := range toComplete {
			completeFn(w.CorrelationID, result)
		}
	}
}

// MarkDrawStarted records the current time so WaitForSession can skip stale sessions.
func (s *Store) MarkDrawStarted() {
	s.mu.Lock()
	s.lastDrawStartedAt = time.Now().UnixMilli()
	s.mu.Unlock()
}

// WaitForSession blocks until a session newer than the last MarkDrawStarted arrives,
// or until timeout expires. Returns (session, timedOut).
// Loops on wake-ups to handle spurious notifications from unrelated sessions.
func (s *Store) WaitForSession(timeout time.Duration) (*Session, bool) {
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
		return result.(*Session), false
	}
	return nil, timedOut
}

// getSessionSince returns the latest session with Timestamp > sinceTs.
func (s *Store) getSessionSince(sinceTs int64) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	var latest *sessionEntry
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
func (s *Store) GetSession(tabID int) *Session {
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
func (s *Store) GetLatestSession() *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	var latest *sessionEntry
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
// Evicts oldest entries if the detail map exceeds MaxDetails.
func (s *Store) StoreDetail(correlationID string, detail Detail) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.details[correlationID] = &detailEntry{
		Detail:    detail,
		ExpiresAt: time.Now().Add(s.detailTTL),
	}
	if len(s.details) > MaxDetails {
		s.evictOldestDetailLocked()
	}
}

// GetDetail retrieves element detail if not expired.
func (s *Store) GetDetail(correlationID string) (*Detail, bool) {
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
// Creates the session if it doesn't exist. Also fires the session notify and completes async waiters.
func (s *Store) AppendToNamedSession(name string, session *Session) {
	s.mu.Lock()
	entry, ok := s.named[name]
	if !ok {
		entry = &namedSessionEntry{
			Session: &NamedSession{
				Name: name,
			},
		}
		s.named[name] = entry
	}
	entry.Session.Pages = append(entry.Session.Pages, session)
	entry.Session.UpdatedAt = time.Now().UnixMilli()
	entry.ExpiresAt = time.Now().Add(s.sessionTTL)
	// Evict oldest named sessions if over cap
	if len(s.named) > MaxNamedSessions {
		s.evictOldestNamedSessionLocked()
	}
	// Collect and clear matching named waiters
	var toComplete []waiter
	remaining := s.waiters[:0]
	for _, w := range s.waiters {
		if w.AnnotSessionName == name {
			toComplete = append(toComplete, w)
		} else {
			remaining = append(remaining, w)
		}
	}
	s.waiters = remaining
	completeFn := s.completeCommand
	// Snapshot the session for completing waiters
	nsCopy := *entry.Session
	nsCopy.Pages = make([]*Session, len(entry.Session.Pages))
	copy(nsCopy.Pages, entry.Session.Pages)
	// Notify blocking waiters
	ch := s.sessionNotify
	s.sessionNotify = make(chan struct{})
	s.mu.Unlock()
	close(ch)

	// Complete async waiters outside the lock
	if completeFn != nil {
		for _, w := range toComplete {
			result := BuildNamedSessionResult(&nsCopy)
			completeFn(w.CorrelationID, result)
		}
	}
}

// GetNamedSession returns a snapshot of the named multi-page session if not expired.
// Returns a shallow copy with its own Pages slice to avoid data races with concurrent
// AppendToNamedSession calls that modify the internal Pages slice under write lock.
func (s *Store) GetNamedSession(name string) *NamedSession {
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
	copied.Pages = make([]*Session, len(entry.Session.Pages))
	copy(copied.Pages, entry.Session.Pages)
	return &copied
}

// ListNamedSessions returns the names of all non-expired named sessions.
func (s *Store) ListNamedSessions() []string {
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
func (s *Store) ClearNamedSession(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.named, name)
}

// evictOldestNamedSessionLocked removes the named session with the oldest UpdatedAt.
// Must be called with s.mu held.
func (s *Store) evictOldestNamedSessionLocked() {
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
func (s *Store) WaitForNamedSession(name string, timeout time.Duration) (*NamedSession, bool) {
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
		return result.(*NamedSession), false
	}
	return nil, timedOut
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

// evictOldestDetailLocked removes the detail entry closest to expiration.
// Must be called with s.mu held.
func (s *Store) evictOldestDetailLocked() {
	var oldestKey string
	var oldestExpiry time.Time
	first := true
	for id, entry := range s.details {
		if first || entry.ExpiresAt.Before(oldestExpiry) {
			oldestExpiry = entry.ExpiresAt
			oldestKey = id
			first = false
		}
	}
	if !first {
		delete(s.details, oldestKey)
	}
}

// evictOldestSessionLocked removes the session with the oldest timestamp.
// Must be called with s.mu held.
func (s *Store) evictOldestSessionLocked() {
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
func (s *Store) cleanupLoop() {
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

// BuildSessionResult serializes an annotation session for the CommandTracker.
func BuildSessionResult(session *Session) json.RawMessage {
	result := map[string]any{
		"status":      "complete",
		"annotations": session.Annotations,
		"count":       len(session.Annotations),
		"page_url":    session.PageURL,
	}
	if session.ScreenshotPath != "" {
		result["screenshot"] = session.ScreenshotPath
	}
	// Error impossible: map of primitive types
	data, _ := json.Marshal(result)
	return data
}

// BuildNamedSessionResult serializes a named session for the CommandTracker.
func BuildNamedSessionResult(ns *NamedSession) json.RawMessage {
	totalCount := 0
	pages := make([]map[string]any, 0, len(ns.Pages))
	for _, page := range ns.Pages {
		totalCount += len(page.Annotations)
		p := map[string]any{
			"page_url":    page.PageURL,
			"annotations": page.Annotations,
			"count":       len(page.Annotations),
			"tab_id":      page.TabID,
		}
		if page.ScreenshotPath != "" {
			p["screenshot"] = page.ScreenshotPath
		}
		pages = append(pages, p)
	}
	result := map[string]any{
		"status":             "complete",
		"annot_session_name": ns.Name,
		"pages":              pages,
		"page_count":         len(ns.Pages),
		"total_count":        totalCount,
	}
	// Error impossible: map of primitive types
	data, _ := json.Marshal(result)
	return data
}

// evictExpiredEntries removes all expired details, sessions, and named sessions.
func (s *Store) evictExpiredEntries() {
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
