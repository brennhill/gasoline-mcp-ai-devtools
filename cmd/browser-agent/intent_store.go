// intent_store.go — In-memory store for user-initiated intents (e.g., "Find Problems" button).
// Why: Bridges the UI trigger to the AI session — the intent persists until the AI picks it up.
// Docs: docs/features/feature/auto-fix/index.md

package main

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"sync/atomic"
	"time"
)

const (
	intentTTL      = 5 * time.Minute
	intentMaxCount = 3
	// Number of tool responses to nudge before giving up and discarding.
	intentMaxNudges    = 3
	intentActionQAScan = "qa_scan"
)

type intent struct {
	CorrelationID string `json:"correlation_id"`
	PageURL       string `json:"page_url"`
	Action        string `json:"action"`
	CreatedAt     int64  `json:"created_at"`
	NudgeCount    int    `json:"-"`
}

type intentStore struct {
	mu    sync.Mutex
	items []intent
	count atomic.Int32 // Fast-path: skip lock when empty
}

func newIntentStore() *intentStore {
	return &intentStore{}
}

func (s *intentStore) Add(pageURL, action string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanExpiredLocked()

	for len(s.items) >= intentMaxCount {
		s.items = s.items[1:]
	}

	id := generateCorrelationID()
	s.items = append(s.items, intent{
		CorrelationID: id,
		PageURL:       pageURL,
		Action:        action,
		CreatedAt:     time.Now().Unix(),
	})
	s.count.Store(int32(len(s.items)))
	return id
}

// Consume removes and returns the intent with the given correlation ID.
func (s *intentStore) Consume(correlationID string) *intent {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanExpiredLocked()

	for i, it := range s.items {
		if it.CorrelationID == correlationID {
			s.items = append(s.items[:i], s.items[i+1:]...)
			s.count.Store(int32(len(s.items)))
			return &it
		}
	}
	return nil
}

// Pending returns all non-expired intents without consuming them.
func (s *intentStore) Pending() []intent {
	if s.count.Load() == 0 {
		return []intent{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanExpiredLocked()
	out := make([]intent, len(s.items))
	copy(out, s.items)
	return out
}

// NudgeAndClean increments the nudge count on all pending intents and removes
// any that have exceeded intentMaxNudges. Returns true if there are still
// pending intents that should be surfaced to the AI.
func (s *intentStore) NudgeAndClean() bool {
	if s.count.Load() == 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanExpiredLocked()

	n := 0
	for i := range s.items {
		s.items[i].NudgeCount++
		if s.items[i].NudgeCount <= intentMaxNudges {
			s.items[n] = s.items[i]
			n++
		}
	}
	s.items = s.items[:n]
	s.count.Store(int32(n))
	return n > 0
}

// ConsumeAll removes and returns all non-expired intents.
func (s *intentStore) ConsumeAll() []intent {
	if s.count.Load() == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]intent, len(s.items))
	copy(out, s.items)
	s.items = s.items[:0]
	s.count.Store(0)
	return out
}

func (s *intentStore) cleanExpiredLocked() {
	now := time.Now().Unix()
	cutoff := now - int64(intentTTL.Seconds())
	n := 0
	for _, it := range s.items {
		if it.CreatedAt >= cutoff {
			s.items[n] = it
			n++
		}
	}
	s.items = s.items[:n]
	s.count.Store(int32(n))
}

func generateCorrelationID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "intent_" + hex.EncodeToString([]byte(time.Now().String()[:19]))
	}
	return "intent_" + hex.EncodeToString(b)
}
