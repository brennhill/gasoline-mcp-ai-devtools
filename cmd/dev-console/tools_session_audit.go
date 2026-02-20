// tools_session_audit.go — Session snapshot adapter and audit-trail recording helpers for MCP tools.
package main

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/audit"
	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/performance"
	"github.com/dev-console/dev-console/internal/session"
)

// toolCaptureStateReader adapts ToolHandler state to session.CaptureStateReader.
type toolCaptureStateReader struct {
	h *ToolHandler
}

func newToolCaptureStateReader(h *ToolHandler) session.CaptureStateReader {
	return &toolCaptureStateReader{h: h}
}

func (r *toolCaptureStateReader) GetConsoleErrors() []session.SnapshotError {
	return r.collectConsoleByLevel(map[string]bool{"error": true})
}

func (r *toolCaptureStateReader) GetConsoleWarnings() []session.SnapshotError {
	return r.collectConsoleByLevel(map[string]bool{"warn": true, "warning": true})
}

func (r *toolCaptureStateReader) collectConsoleByLevel(levels map[string]bool) []session.SnapshotError {
	if r.h == nil || r.h.server == nil {
		return []session.SnapshotError{}
	}

	r.h.server.mu.RLock()
	entries := append([]LogEntry(nil), r.h.server.entries...)
	r.h.server.mu.RUnlock()

	type key struct {
		level string
		msg   string
	}
	counts := make(map[key]int)
	for _, entry := range entries {
		level, _ := entry["level"].(string)
		if !levels[level] {
			continue
		}
		msg, _ := entry["message"].(string)
		msg = strings.TrimSpace(msg)
		if msg == "" {
			continue
		}
		k := key{level: level, msg: msg}
		counts[k]++
	}

	keys := make([]key, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].level != keys[j].level {
			return keys[i].level < keys[j].level
		}
		return keys[i].msg < keys[j].msg
	})

	out := make([]session.SnapshotError, 0, len(keys))
	for _, k := range keys {
		out = append(out, session.SnapshotError{
			Type:    k.level,
			Message: k.msg,
			Count:   counts[k],
		})
	}
	return out
}

func (r *toolCaptureStateReader) GetNetworkRequests() []session.SnapshotNetworkRequest {
	if r.h == nil || r.h.capture == nil {
		return []session.SnapshotNetworkRequest{}
	}
	bodies := r.h.capture.GetNetworkBodies()
	out := make([]session.SnapshotNetworkRequest, 0, len(bodies))
	for _, body := range bodies {
		out = append(out, session.SnapshotNetworkRequest{
			Method:       body.Method,
			URL:          body.URL,
			Status:       body.Status,
			Duration:     body.Duration,
			ResponseSize: len(body.ResponseBody),
			ContentType:  body.ContentType,
		})
	}
	return out
}

func (r *toolCaptureStateReader) GetWSConnections() []session.SnapshotWSConnection {
	if r.h == nil || r.h.capture == nil {
		return []session.SnapshotWSConnection{}
	}
	status := r.h.capture.GetWebSocketStatus(capture.WebSocketStatusFilter{})
	out := make([]session.SnapshotWSConnection, 0, len(status.Connections))
	for _, conn := range status.Connections {
		out = append(out, session.SnapshotWSConnection{
			URL:         conn.URL,
			State:       conn.State,
			MessageRate: conn.MessageRate.Incoming.PerSecond + conn.MessageRate.Outgoing.PerSecond,
		})
	}
	return out
}

func (r *toolCaptureStateReader) GetPerformance() *performance.PerformanceSnapshot {
	if r.h == nil || r.h.capture == nil {
		return nil
	}
	snapshots := r.h.capture.GetPerformanceSnapshots()
	if len(snapshots) == 0 {
		return nil
	}

	var best *performance.PerformanceSnapshot
	var bestTS time.Time
	for i := range snapshots {
		s := snapshots[i]
		ts, err := time.Parse(time.RFC3339Nano, s.Timestamp)
		if err != nil {
			ts, _ = time.Parse(time.RFC3339, s.Timestamp)
		}
		if best == nil || ts.After(bestTS) {
			copied := s
			best = &copied
			bestTS = ts
		}
	}
	return best
}

func (r *toolCaptureStateReader) GetCurrentPageURL() string {
	if r.h == nil || r.h.capture == nil {
		return ""
	}
	_, _, trackedURL := r.h.capture.GetTrackingStatus()
	if trackedURL != "" {
		return trackedURL
	}
	if snap := r.GetPerformance(); snap != nil && snap.URL != "" {
		return snap.URL
	}
	bodies := r.h.capture.GetNetworkBodies()
	if len(bodies) > 0 {
		return bodies[len(bodies)-1].URL
	}
	return ""
}

func (h *ToolHandler) recordAuditToolCall(
	req JSONRPCRequest,
	toolName string,
	args json.RawMessage,
	resp JSONRPCResponse,
	started time.Time,
) {
	if h == nil || h.auditTrail == nil {
		return
	}
	if shouldSkipAuditRecording(toolName, args) {
		return
	}

	sessionID := h.auditSessionForClient(req.ClientID)
	if sessionID == "" {
		return
	}

	success := resp.Error == nil && !isToolResultError(resp.Result)
	entry := audit.AuditEntry{
		AuditSessionID: sessionID,
		ClientID:       normalizeAuditClientID(req.ClientID),
		ToolName:       toolName,
		Parameters:     string(args),
		ResponseSize:   len(resp.Result),
		Duration:       time.Since(started).Milliseconds(),
		Success:        success,
	}
	if !success {
		entry.ErrorMessage = auditErrorMessage(resp)
	}

	h.auditTrail.Record(entry)
}

func (h *ToolHandler) auditSessionForClient(clientID string) string {
	if h == nil || h.auditTrail == nil {
		return ""
	}

	client := normalizeAuditClientID(clientID)

	h.auditMu.Lock()
	defer h.auditMu.Unlock()

	if sid, ok := h.auditSessionMap[client]; ok && sid != "" {
		if h.auditTrail.GetAuditSession(sid) != nil {
			return sid
		}
		delete(h.auditSessionMap, client)
	}

	info := h.auditTrail.CreateAuditSession(audit.ClientIdentifier{Name: client})
	if info == nil || info.ID == "" {
		return ""
	}
	h.auditSessionMap[client] = info.ID
	return info.ID
}

func normalizeAuditClientID(clientID string) string {
	trimmed := strings.TrimSpace(clientID)
	if trimmed == "" {
		return "unknown"
	}
	return trimmed
}

func auditErrorMessage(resp JSONRPCResponse) string {
	if resp.Error != nil && resp.Error.Message != "" {
		return resp.Error.Message
	}
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return ""
	}
	if len(result.Content) == 0 {
		return ""
	}
	return result.Content[0].Text
}

func shouldSkipAuditRecording(toolName string, args json.RawMessage) bool {
	if toolName != "configure" || len(args) == 0 {
		return false
	}
	var params struct {
		What      string `json:"what"`
		Action    string `json:"action"`
		Operation string `json:"operation"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return false
	}
	dispatch := params.What
	if dispatch == "" {
		dispatch = params.Action
	}
	return strings.EqualFold(strings.TrimSpace(dispatch), "audit_log") &&
		strings.EqualFold(strings.TrimSpace(params.Operation), "clear")
}
