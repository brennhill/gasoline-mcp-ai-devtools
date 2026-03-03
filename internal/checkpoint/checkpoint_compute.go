// Purpose: Implements checkpoint diff computation over console, network, websocket, and action buffers.
// Why: Produces compact change sets optimized for AI context windows and incident triage.
// Docs: docs/features/feature/push-alerts/index.md

package checkpoint

func (cm *CheckpointManager) computeConsoleDiff(cp *Checkpoint, severity string) *ConsoleDiff {
	snapshot := cm.server.GetLogSnapshot()
	newEntries := recentSlice(len(snapshot.Entries), int(snapshot.TotalAdded-cp.LogTotal))
	if newEntries < 0 {
		return &ConsoleDiff{}
	}
	entries := snapshot.Entries[len(snapshot.Entries)-newEntries:]

	classified := classifyLogEntries(entries, severity)
	return &ConsoleDiff{
		TotalNew: classified.totalNew,
		Errors:   buildConsoleEntries(classified.errorMap, classified.errorOrder),
		Warnings: buildConsoleEntries(classified.warningMap, classified.warningOrder),
	}
}

func (cm *CheckpointManager) computeNetworkDiff(cp *Checkpoint) *NetworkDiff {
	allBodies := cm.capture.GetNetworkBodies()
	count := recentSlice(len(allBodies), int(cm.capture.GetNetworkTotalAdded()-cp.NetworkTotal))
	if count < 0 {
		return &NetworkDiff{}
	}
	newBodies := allBodies[len(allBodies)-count:]

	diff := &NetworkDiff{TotalNew: len(newBodies)}
	for _, body := range newBodies {
		classifyNetworkBody(diff, body, cp.KnownEndpoints)
	}
	capNetworkDiff(diff)
	return diff
}

func (cm *CheckpointManager) computeWebSocketDiff(cp *Checkpoint, severity string) *WebSocketDiff {
	allEvents := cm.capture.GetAllWebSocketEvents()
	count := recentSlice(len(allEvents), int(cm.capture.GetWebSocketTotalAdded()-cp.WSTotal))
	if count < 0 {
		return &WebSocketDiff{}
	}
	newEvents := allEvents[len(allEvents)-count:]

	diff := &WebSocketDiff{TotalNew: len(newEvents)}
	for i := range newEvents {
		classifyWSEvent(diff, &newEvents[i], severity)
	}
	capWSDiff(diff)
	return diff
}

func (cm *CheckpointManager) computeActionsDiff(cp *Checkpoint) *ActionsDiff {
	allActions := cm.capture.GetAllEnhancedActions()
	count := recentSlice(len(allActions), int(cm.capture.GetActionTotalAdded()-cp.ActionTotal))
	if count < 0 {
		return &ActionsDiff{}
	}
	newActions := allActions[len(allActions)-count:]

	diff := &ActionsDiff{TotalNew: len(newActions)}
	for i := range newActions {
		if i >= maxDiffEntriesPerCat {
			break
		}
		diff.Actions = append(diff.Actions, ActionEntry{
			Type:      newActions[i].Type,
			URL:       newActions[i].URL,
			Timestamp: newActions[i].Timestamp,
		})
	}
	return diff
}
