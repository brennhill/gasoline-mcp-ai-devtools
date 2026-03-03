// Purpose: Stores and retrieves retry-contract state keyed by command correlation id.
// Why: Centralizes locking and lifecycle rules for retry context across interact command results.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
	"strings"
	"time"
)

func (h *interactActionHandler) armRetryContract(correlationID, action string, args json.RawMessage) {
	if h == nil || correlationID == "" {
		return
	}

	if action == "" {
		action = canonicalActionFromInteractArgs(args)
	}
	action = strings.ToLower(strings.TrimSpace(action))
	strategy, fingerprint := deriveRetryStrategy(action, args)
	parentCorrID := parseRetryParentCorrelationID(args)

	state := &commandRetryState{
		Attempt:             1,
		MaxAttempts:         maxRetryAttemptsPerStep,
		Action:              action,
		Strategy:            strategy,
		StrategyFingerprint: fingerprint,
		ChangedStrategy:     true,
		ParentCorrelationID: parentCorrID,
		CreatedAt:           time.Now(),
	}

	if parentCorrID != "" {
		if parent, ok := h.getRetryState(parentCorrID); ok {
			state.Attempt = parent.Attempt + 1
			if state.Attempt > state.MaxAttempts {
				state.Attempt = state.MaxAttempts
				state.PolicyViolation = "attempt_limit_exceeded"
			}
			state.ChangedStrategy = state.StrategyFingerprint != parent.StrategyFingerprint
			if !state.ChangedStrategy {
				state.PolicyViolation = "strategy_unchanged"
			}
		} else {
			// Treat explicit parent chaining as retry attempt even if parent context has expired.
			state.Attempt = 2
			state.PolicyViolation = "parent_context_missing"
		}
	}

	h.storeRetryState(correlationID, state)
}

func (h *interactActionHandler) getRetryState(correlationID string) (*commandRetryState, bool) {
	h.parent.retryContractMu.Lock()
	defer h.parent.retryContractMu.Unlock()
	state, ok := h.parent.retryByCommand[correlationID]
	return state, ok
}

func (h *interactActionHandler) storeRetryState(correlationID string, state *commandRetryState) {
	h.parent.retryContractMu.Lock()
	defer h.parent.retryContractMu.Unlock()

	if h.parent.retryByCommand == nil {
		h.parent.retryByCommand = make(map[string]*commandRetryState)
	}
	h.parent.retryByCommand[correlationID] = state
	h.pruneRetryStatesLocked(2048)
}

func (h *interactActionHandler) pruneRetryStatesLocked(maxEntries int) {
	if len(h.parent.retryByCommand) <= maxEntries {
		return
	}

	var oldestKey string
	var oldestTime time.Time
	for key, st := range h.parent.retryByCommand {
		if oldestKey == "" || st.CreatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = st.CreatedAt
		}
	}
	if oldestKey != "" {
		delete(h.parent.retryByCommand, oldestKey)
	}
}
