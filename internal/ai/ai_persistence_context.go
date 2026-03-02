package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func loadJSONFileAs(path string) map[string]any {
	data, err := os.ReadFile(path) // #nosec G304 -- callers construct path from internal projectDir field // nosemgrep: go_filesystem_rule-fileread -- local persistence store I/O
	if err != nil {
		return nil
	}
	var result map[string]any
	if json.Unmarshal(data, &result) != nil {
		return nil
	}
	return result
}

func parseRawErrorEntry(raw map[string]any) ErrorHistoryEntry {
	entry := ErrorHistoryEntry{}
	if fp, ok := raw["fingerprint"].(string); ok {
		entry.Fingerprint = fp
	}
	if c, ok := raw["count"].(float64); ok {
		entry.Count = int(c)
	}
	if r, ok := raw["resolved"].(bool); ok {
		entry.Resolved = r
	}
	return entry
}

func loadErrorHistory(path string) []ErrorHistoryEntry {
	data, err := os.ReadFile(path) // #nosec G304 -- callers construct path from internal projectDir field
	if err != nil {
		return nil
	}

	var entries []ErrorHistoryEntry
	if json.Unmarshal(data, &entries) == nil {
		return entries
	}

	var rawEntries []map[string]any
	if json.Unmarshal(data, &rawEntries) != nil {
		return nil
	}

	result := make([]ErrorHistoryEntry, 0, len(rawEntries))
	for _, raw := range rawEntries {
		result = append(result, parseRawErrorEntry(raw))
	}
	return result
}

func (s *SessionStore) LoadSessionContext() SessionContext {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ctx := SessionContext{
		ProjectID:    s.projectPath,
		SessionCount: s.meta.SessionCount,
		Baselines:    []string{},
		ErrorHistory: []ErrorHistoryEntry{},
	}

	baselineDir := filepath.Join(s.projectDir, "baselines")
	if keys, err := jsonKeysFromDir(baselineDir); err == nil && len(keys) > 0 {
		ctx.Baselines = keys
	}

	ctx.NoiseConfig = loadJSONFileAs(filepath.Join(s.projectDir, "noise", "config.json"))
	if entries := loadErrorHistory(filepath.Join(s.projectDir, "errors", "history.json")); entries != nil {
		ctx.ErrorHistory = entries
	}
	ctx.APISchema = loadJSONFileAs(filepath.Join(s.projectDir, "api_schema", "schema.json"))
	ctx.Performance = loadJSONFileAs(filepath.Join(s.projectDir, "performance", "endpoints.json"))

	return ctx
}
