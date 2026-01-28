// temporal_graph.go — Cross-session event history with causal links and deduplication.
// Records significant events (errors, regressions, resolutions, deploys, fixes) to a
// persistent JSONL file (.gasoline/history/events.jsonl). Events are deduplicated by
// fingerprint (type + source + normalized message) — repeat errors increment a counter
// rather than creating new entries. Causal links between events form a sparse directed graph.
// Design: Append-only writes for durability. 90-day retention with eviction on startup.
// Events distinguish origin ("system" from browser data vs "agent" from AI recordings)
// so future sessions can weight ground-truth events more heavily.
package main

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// --- Types ---

// EventLink represents a causal relationship between events.
type EventLink struct {
	Target       string `json:"target"`
	Relationship string `json:"relationship"`
	Confidence   string `json:"confidence"` // "explicit" or "inferred"
}

// TemporalEvent represents a significant event in the project's history.
type TemporalEvent struct {
	ID              string      `json:"id"`
	Type            string      `json:"type"` // error, regression, resolution, baseline_shift, deploy, fix
	Timestamp       string      `json:"timestamp"`
	Description     string      `json:"description"`
	Source          string      `json:"source,omitempty"`
	Origin          string      `json:"origin"` // "system" or "agent"
	Agent           string      `json:"agent,omitempty"`
	Status          string      `json:"status"` // active, resolved, superseded
	Links           []EventLink `json:"links,omitempty"`
	OccurrenceCount int         `json:"occurrence_count,omitempty"`
}

// TemporalQuery specifies filters for querying the event history.
type TemporalQuery struct {
	Type      string `json:"type,omitempty"`
	Since     string `json:"since,omitempty"` // "1h", "1d", "7d", "30d"
	RelatedTo string `json:"related_to,omitempty"`
	Pattern   string `json:"pattern,omitempty"`
}

// TemporalQueryResponse is returned by analyze(target: "history").
type TemporalQueryResponse struct {
	Events      []TemporalEvent `json:"events"`
	TotalEvents int             `json:"total_events"`
	TimeRange   string          `json:"time_range"`
	Summary     string          `json:"summary"`
}

// --- Temporal Graph ---

// TemporalGraph manages the cross-session event history.
type TemporalGraph struct {
	mu              sync.RWMutex
	events          []TemporalEvent
	fingerprints    map[string]int // fingerprint -> index in events slice
	dir             string         // base directory (.gasoline/)
	file            *os.File
	retentionDays   int
}

// NewTemporalGraph creates or loads a temporal graph from the given project directory.
func NewTemporalGraph(projectDir string) *TemporalGraph {
	histDir := filepath.Join(projectDir, "history")
	_ = os.MkdirAll(histDir, 0755) // #nosec G301 -- 0755 for log directory is appropriate

	tg := &TemporalGraph{
		events:        make([]TemporalEvent, 0),
		fingerprints:  make(map[string]int),
		dir:           projectDir,
		retentionDays: 90,
	}

	// Load existing events
	eventsPath := filepath.Join(histDir, "events.jsonl")
	tg.loadFromFile(eventsPath)

	// Evict old events
	tg.evict()

	// Open file for appending
	f, err := os.OpenFile(eventsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644) // #nosec G302 G304 -- log files are intentionally world-readable; path from internal field
	if err == nil {
		tg.file = f
	}

	return tg
}

// loadFromFile reads events from the JSONL file.
func (tg *TemporalGraph) loadFromFile(path string) {
	f, err := os.Open(path) // #nosec G304 -- path is constructed from internal projectDir field
	if err != nil {
		return // No file yet
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event TemporalEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue // Skip corrupted lines
		}
		tg.events = append(tg.events, event)
		fp := eventFingerprint(event)
		if fp != "" {
			tg.fingerprints[fp] = len(tg.events) - 1
		}
	}
}

// evict removes events older than retentionDays and rewrites the file.
func (tg *TemporalGraph) evict() {
	cutoff := time.Now().Add(-time.Duration(tg.retentionDays) * 24 * time.Hour)
	kept := make([]TemporalEvent, 0, len(tg.events))
	for _, e := range tg.events {
		ts, err := time.Parse(time.RFC3339, e.Timestamp)
		if err != nil {
			kept = append(kept, e) // Keep unparseable (shouldn't happen)
			continue
		}
		if ts.After(cutoff) {
			kept = append(kept, e)
		}
	}

	if len(kept) < len(tg.events) {
		tg.events = kept
		// Rebuild fingerprint index
		tg.fingerprints = make(map[string]int)
		for i, e := range tg.events {
			fp := eventFingerprint(e)
			if fp != "" {
				tg.fingerprints[fp] = i
			}
		}
		// Rewrite file
		tg.rewriteFile()
	}
}

// rewriteFile writes all current events to the JSONL file.
func (tg *TemporalGraph) rewriteFile() {
	path := filepath.Join(tg.dir, "history", "events.jsonl")
	f, err := os.Create(path) // #nosec G304 -- path is constructed from internal dir field
	if err != nil {
		return
	}
	defer f.Close()
	for _, e := range tg.events {
		data, _ := json.Marshal(e)
		_, _ = f.Write(append(data, '\n')) // #nosec G104 -- best-effort write to log file
	}
}

// RecordEvent adds a new event to the graph.
func (tg *TemporalGraph) RecordEvent(event TemporalEvent) {
	tg.mu.Lock()
	defer tg.mu.Unlock()

	// Auto-fill fields
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	if event.ID == "" {
		event.ID = generateEventID()
	}
	if event.Status == "" {
		event.Status = "active"
	}
	if event.OccurrenceCount == 0 {
		event.OccurrenceCount = 1
	}

	// Deduplication by fingerprint
	fp := eventFingerprint(event)
	if fp != "" {
		if idx, exists := tg.fingerprints[fp]; exists {
			// Increment count on existing event
			tg.events[idx].OccurrenceCount++
			tg.events[idx].Timestamp = event.Timestamp // Update last seen
			return
		}
	}

	// Append new event
	tg.events = append(tg.events, event)
	if fp != "" {
		tg.fingerprints[fp] = len(tg.events) - 1
	}

	// Persist to file
	tg.writeEvent(event)
}

// appendEvent adds an event without deduplication (for testing/internal use).
func (tg *TemporalGraph) appendEvent(event TemporalEvent) {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	tg.events = append(tg.events, event)
	fp := eventFingerprint(event)
	if fp != "" {
		tg.fingerprints[fp] = len(tg.events) - 1
	}
	tg.writeEvent(event)
}

// writeEvent appends a single event to the file.
func (tg *TemporalGraph) writeEvent(event TemporalEvent) {
	if tg.file == nil {
		return
	}
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	_, _ = tg.file.Write(append(data, '\n')) // #nosec G104 -- best-effort write to log file
}

// Query returns events matching the given filters.
func (tg *TemporalGraph) Query(q TemporalQuery) TemporalQueryResponse {
	tg.mu.RLock()
	defer tg.mu.RUnlock()

	sinceStr := q.Since
	if sinceStr == "" {
		sinceStr = "7d"
	}
	sinceDuration := parseSinceDuration(sinceStr)
	cutoff := time.Now().Add(-sinceDuration)

	result := make([]TemporalEvent, 0)
	for _, e := range tg.events {
		// Time filter
		ts, err := time.Parse(time.RFC3339, e.Timestamp)
		if err != nil {
			continue
		}
		if ts.Before(cutoff) {
			continue
		}

		// Type filter
		if q.Type != "" && e.Type != q.Type {
			continue
		}

		// Pattern filter
		if q.Pattern != "" && !strings.Contains(e.Description, q.Pattern) {
			continue
		}

		// RelatedTo filter — find events that link to the specified ID
		if q.RelatedTo != "" {
			linked := false
			for _, link := range e.Links {
				if link.Target == q.RelatedTo {
					linked = true
					break
				}
			}
			if !linked {
				continue
			}
		}

		result = append(result, e)
	}

	summary := fmt.Sprintf("%d events in last %s.", len(result), sinceStr)

	return TemporalQueryResponse{
		Events:      result,
		TotalEvents: len(result),
		TimeRange:   sinceStr,
		Summary:     summary,
	}
}

// Close persists dedup counts and closes the file handle.
func (tg *TemporalGraph) Close() {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	if tg.file != nil {
		_ = tg.file.Close() // #nosec G104 -- best-effort close
		tg.file = nil
	}
	// Rewrite to persist dedup counts updated during this session
	tg.rewriteFile()
}

// --- Helpers ---

// eventFingerprint generates a deduplication key for an event.
// Only error events are deduplicated (same type + source + normalized description).
func eventFingerprint(e TemporalEvent) string {
	if e.Type != "error" {
		return "" // Only deduplicate errors
	}
	return e.Type + "|" + e.Source + "|" + normalizeErrorMessage(e.Description)
}

// generateEventID creates a unique event ID.
func generateEventID() string {
	ts := time.Now().UnixMilli()
	var b [2]byte
	_, _ = rand.Read(b[:]) // #nosec G104 -- best-effort randomness for non-security event ID
	r := binary.LittleEndian.Uint16(b[:]) % 10000
	return fmt.Sprintf("evt_%d_%04d", ts, r)
}

// parseSinceDuration parses a since string like "1h", "1d", "7d", "30d".
func parseSinceDuration(s string) time.Duration {
	if s == "" {
		return 7 * 24 * time.Hour // default: 7 days
	}

	if strings.HasSuffix(s, "h") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "h"))
		if err == nil {
			return time.Duration(n) * time.Hour
		}
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err == nil {
			return time.Duration(n) * 24 * time.Hour
		}
	}

	return 7 * 24 * time.Hour // fallback default
}

// --- Configure record_event Handler ---

// handleRecordEvent processes configure(action: "record_event", event: {...}).
func handleRecordEvent(tg *TemporalGraph, eventData map[string]interface{}, agent string) (string, string) {
	eventType, _ := eventData["type"].(string)
	if eventType == "" {
		return "", "Required field 'type' is missing. Valid types: error, regression, resolution, baseline_shift, deploy, fix."
	}

	description, _ := eventData["description"].(string)
	if description == "" {
		return "", "Required field 'description' is missing."
	}

	event := TemporalEvent{
		Type:        eventType,
		Description: description,
		Origin:      "agent",
		Agent:       agent,
	}

	// Optional source
	if source, ok := eventData["source"].(string); ok {
		event.Source = source
	}

	// Optional related_to link
	if relatedTo, ok := eventData["related_to"].(string); ok && relatedTo != "" {
		event.Links = []EventLink{
			{Target: relatedTo, Relationship: "related_to", Confidence: "explicit"},
		}
	}

	tg.RecordEvent(event)
	return fmt.Sprintf("Event recorded: %s - %s", eventType, description), ""
}

// --- MCP Tool Handler ---

// toolAnalyzeHistory handles analyze(target: "history") MCP calls.
func (h *ToolHandler) toolAnalyzeHistory(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if h.temporalGraph == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "No history recorded yet", "Navigate and interact with a page first")}
	}

	var params struct {
		Query TemporalQuery `json:"query"`
	}
	_ = json.Unmarshal(args, &params)

	resp := h.temporalGraph.Query(params.Query)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Temporal analysis", resp)}
}

// toolConfigureRecordEvent handles configure(action: "record_event") MCP calls.
func (h *ToolHandler) toolConfigureRecordEvent(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if h.temporalGraph == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Temporal graph not initialized", "Internal server error — do not retry")}
	}

	var params struct {
		Event map[string]interface{} `json:"event"`
	}
	if err := json.Unmarshal(args, &params); err != nil || params.Event == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam,
			"Required parameter 'event' is missing. Provide {type, description, [source], [related_to]}.",
			"Add the 'event' parameter and call again", withParam("event"))}
	}

	agent := "unknown"
	result, errMsg := handleRecordEvent(h.temporalGraph, params.Event, agent)
	if errMsg != "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, errMsg, "Fix the event parameters and call again")}
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(result)}
}
