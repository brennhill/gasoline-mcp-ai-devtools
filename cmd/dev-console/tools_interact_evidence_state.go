// Purpose: Manages per-command evidence state lifecycle (store, retrieve, clear).
// Why: Isolates evidence state mutations behind helpers to prevent lock ordering bugs.

package main

func (h *interactActionHandler) clearEvidenceState(correlationID string) {
	h.parent.evidenceMu.Lock()
	defer h.parent.evidenceMu.Unlock()
	delete(h.parent.evidenceByCommand, correlationID)
}

func (h *interactActionHandler) storeEvidenceState(correlationID string, state *commandEvidenceState) {
	h.parent.evidenceMu.Lock()
	defer h.parent.evidenceMu.Unlock()
	if h.parent.evidenceByCommand == nil {
		h.parent.evidenceByCommand = make(map[string]*commandEvidenceState)
	}
	h.parent.evidenceByCommand[correlationID] = state
}

func (h *interactActionHandler) loadEvidenceAttachContext(correlationID string) (cached map[string]any, needsAfter bool, clientID string, done bool) {
	h.parent.evidenceMu.Lock()
	defer h.parent.evidenceMu.Unlock()

	state, ok := h.parent.evidenceByCommand[correlationID]
	if !ok {
		return nil, false, "", true
	}
	if state.finalized {
		return cloneAnyMap(state.cached), false, "", true
	}

	return nil, state.shouldCapture && state.maxCaptures > 1, state.clientID, false
}

func (h *interactActionHandler) finalizeEvidencePayload(correlationID string, needsAfter bool, after evidenceShot) (map[string]any, bool) {
	h.parent.evidenceMu.Lock()
	defer h.parent.evidenceMu.Unlock()

	state, ok := h.parent.evidenceByCommand[correlationID]
	if !ok {
		return nil, false
	}
	if !state.finalized {
		if needsAfter {
			state.after = after
		}
		state.cached = buildEvidencePayload(state)
		state.finalized = true
	}

	return cloneAnyMap(state.cached), true
}

func buildEvidencePayload(state *commandEvidenceState) map[string]any {
	if state == nil {
		return map[string]any{}
	}

	payload := map[string]any{
		"mode":   string(state.mode),
		"action": state.action,
	}

	if state.before.Path != "" {
		payload["before"] = state.before.Path
	}
	if state.after.Path != "" {
		payload["after"] = state.after.Path
	}

	files := map[string]any{}
	if state.before.Filename != "" {
		files["before"] = state.before.Filename
	}
	if state.after.Filename != "" {
		files["after"] = state.after.Filename
	}
	if len(files) > 0 {
		payload["filenames"] = files
	}

	errors := map[string]any{}
	if state.before.Error != "" {
		errors["before"] = state.before.Error
	}
	if state.after.Error != "" {
		errors["after"] = state.after.Error
	}
	if len(errors) > 0 {
		payload["errors"] = errors
	}

	if state.skipped != "" {
		payload["skipped"] = state.skipped
	}

	if len(errors) > 0 && (state.before.Path != "" || state.after.Path != "") {
		payload["partial"] = true
	}

	return payload
}

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		if nested, ok := v.(map[string]any); ok {
			out[k] = cloneAnyMap(nested)
			continue
		}
		out[k] = v
	}
	return out
}
