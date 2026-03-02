package ai

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/server"
)

func NewCheckpointManager(serverReader server.LogReader, capture *capture.Capture) *CheckpointManager {
	return &CheckpointManager{
		namedCheckpoints: make(map[string]*Checkpoint),
		namedOrder:       make([]string, 0),
		server:           serverReader,
		capture:          capture,
	}
}

func (cm *CheckpointManager) CreateCheckpoint(name string, clientID string) error {
	if name == "" {
		return fmt.Errorf("checkpoint name cannot be empty")
	}
	if len(name) > maxCheckpointNameLen {
		return fmt.Errorf("checkpoint name exceeds %d characters", maxCheckpointNameLen)
	}

	storedName := name
	if clientID != "" {
		storedName = clientID + ":" + name
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cp := cm.snapshotNow()
	cp.Name = name
	cp.AlertDelivery = cm.alertDelivery

	if _, exists := cm.namedCheckpoints[storedName]; !exists {
		cm.namedOrder = append(cm.namedOrder, storedName)
	}
	cm.namedCheckpoints[storedName] = cp

	for len(cm.namedCheckpoints) > maxNamedCheckpoints {
		oldest := cm.namedOrder[0]
		newOrder := make([]string, len(cm.namedOrder)-1)
		copy(newOrder, cm.namedOrder[1:])
		cm.namedOrder = newOrder
		delete(cm.namedCheckpoints, oldest)
	}

	return nil
}

func (cm *CheckpointManager) GetNamedCheckpointCount() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return len(cm.namedCheckpoints)
}

func (cm *CheckpointManager) GetChangesSince(params GetChangesSinceParams, clientID string) DiffResponse {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	cp, isNamedQuery := cm.resolveCheckpoint(params.Checkpoint, clientID, now)

	resp := cm.computeDiffs(cp, params, now)
	cm.applySeverityFilter(&resp, params.Severity)
	cm.pruneEmptyDiffs(&resp)
	cm.attachAlerts(&resp, cp.AlertDelivery)

	jsonBytes, _ := json.Marshal(resp)
	resp.TokenCount = len(jsonBytes) / 4

	if !isNamedQuery {
		cm.markAlertsDelivered()
		cm.autoCheckpoint = cm.snapshotNow()
		cm.autoCheckpoint.KnownEndpoints = cm.buildKnownEndpoints(cp.KnownEndpoints)
		cm.autoCheckpoint.AlertDelivery = cm.alertDelivery
	}

	return resp
}

func (cm *CheckpointManager) computeDiffs(cp *Checkpoint, params GetChangesSinceParams, now time.Time) DiffResponse {
	resp := DiffResponse{
		From:       cp.CreatedAt,
		To:         now,
		DurationMs: now.Sub(cp.CreatedAt).Milliseconds(),
	}
	if cm.shouldInclude(params.Include, "console") {
		resp.Console = cm.computeConsoleDiff(cp, params.Severity)
	}
	if cm.shouldInclude(params.Include, "network") {
		resp.Network = cm.computeNetworkDiff(cp)
	}
	if cm.shouldInclude(params.Include, "websocket") {
		resp.WebSocket = cm.computeWebSocketDiff(cp, params.Severity)
	}
	if cm.shouldInclude(params.Include, "actions") {
		resp.Actions = cm.computeActionsDiff(cp)
	}
	return resp
}

func (cm *CheckpointManager) applySeverityFilter(resp *DiffResponse, severity string) {
	if severity == "errors_only" {
		if resp.Console != nil {
			resp.Console.Warnings = nil
		}
		if resp.WebSocket != nil && len(resp.WebSocket.Errors) == 0 {
			resp.WebSocket = nil
		}
	}
	resp.Severity = cm.determineSeverity(*resp)
	resp.Summary = cm.buildSummary(*resp)
}

func (cm *CheckpointManager) pruneEmptyDiffs(resp *DiffResponse) {
	if resp.Console != nil && resp.Console.TotalNew == 0 {
		resp.Console = nil
	}
	if resp.Network != nil && resp.Network.TotalNew == 0 && len(resp.Network.Failures) == 0 && len(resp.Network.NewEndpoints) == 0 && len(resp.Network.Degraded) == 0 {
		resp.Network = nil
	}
	if resp.WebSocket != nil && resp.WebSocket.TotalNew == 0 {
		resp.WebSocket = nil
	}
	if resp.Actions != nil && resp.Actions.TotalNew == 0 {
		resp.Actions = nil
	}
}

func (cm *CheckpointManager) attachAlerts(resp *DiffResponse, checkpointDelivery int64) {
	alerts := cm.getPendingAlerts(checkpointDelivery)
	if len(alerts) > 0 {
		resp.PerformanceAlerts = alerts
	}
}

func (cm *CheckpointManager) shouldInclude(include []string, category string) bool {
	if len(include) == 0 {
		return true
	}
	for _, inc := range include {
		if inc == category {
			return true
		}
	}
	return false
}
