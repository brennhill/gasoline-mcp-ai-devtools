// Purpose: Shared sequence load/filter helpers used by configure sequence commands.

package main

import "encoding/json"

// loadSequence loads a sequence from the session store and returns it.
func (h *ToolHandler) loadSequence(req JSONRPCRequest, name string) (*Sequence, *JSONRPCResponse) {
	if resp, blocked := h.requireSessionStore(req); blocked {
		return nil, &resp
	}

	data, err := h.sessionStoreImpl.Load(sequenceNamespace, name)
	if err != nil {
		resp := fail(req, ErrNoData, "Sequence not found: "+name, "Use list_sequences to see available sequences")
		return nil, &resp
	}

	var seq Sequence
	if err := json.Unmarshal(data, &seq); err != nil {
		resp := fail(req, ErrInvalidJSON, "Corrupted sequence data: "+err.Error(), "Delete and re-save the sequence")
		return nil, &resp
	}

	return &seq, nil
}

// hasAllTags returns true if seqTags contains all of requiredTags.
func hasAllTags(seqTags, requiredTags []string) bool {
	tagSet := make(map[string]bool, len(seqTags))
	for _, t := range seqTags {
		tagSet[t] = true
	}
	for _, req := range requiredTags {
		if !tagSet[req] {
			return false
		}
	}
	return true
}
