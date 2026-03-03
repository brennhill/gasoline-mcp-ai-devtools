package checkpoint

import (
	"sort"
	"time"
)

func (cm *CheckpointManager) resolveCheckpoint(name, clientID string, now time.Time) (*Checkpoint, bool) {
	if name == "" {
		return cm.resolveAutoCheckpoint(now), false
	}

	namespacedName := name
	if clientID != "" {
		namespacedName = clientID + ":" + name
	}

	if named, ok := cm.namedCheckpoints[namespacedName]; ok {
		return named, true
	}
	if named, ok := cm.namedCheckpoints[name]; ok {
		return named, true
	}
	if cp := cm.resolveTimestampCheckpoint(name); cp != nil {
		return cp, true
	}
	return &Checkpoint{CreatedAt: now, KnownEndpoints: make(map[string]endpointState)}, true
}

func (cm *CheckpointManager) resolveAutoCheckpoint(now time.Time) *Checkpoint {
	if cm.autoCheckpoint != nil {
		return cm.autoCheckpoint
	}
	return &Checkpoint{
		CreatedAt:      now,
		KnownEndpoints: make(map[string]endpointState),
	}
}

func (cm *CheckpointManager) snapshotNow() *Checkpoint {
	logTotal := cm.server.GetLogTotalAdded()
	netTotal := cm.capture.GetNetworkTotalAdded()
	wsTotal := cm.capture.GetWebSocketTotalAdded()
	actTotal := cm.capture.GetActionTotalAdded()

	return &Checkpoint{
		CreatedAt:      time.Now(),
		LogTotal:       logTotal,
		NetworkTotal:   netTotal,
		WSTotal:        wsTotal,
		ActionTotal:    actTotal,
		KnownEndpoints: make(map[string]endpointState),
	}
}

func (cm *CheckpointManager) resolveTimestampCheckpoint(tsStr string) *Checkpoint {
	t, err := time.Parse(time.RFC3339Nano, tsStr)
	if err != nil {
		t, err = time.Parse(time.RFC3339, tsStr)
		if err != nil {
			return nil
		}
	}

	logTotal := cm.findPositionAtTime(cm.server.GetLogTimestamps(), cm.server.GetLogTotalAdded(), t)
	netTotal := cm.findPositionAtTime(cm.capture.GetNetworkTimestamps(), cm.capture.GetNetworkTotalAdded(), t)
	wsTotal := cm.findPositionAtTime(cm.capture.GetWebSocketTimestamps(), cm.capture.GetWebSocketTotalAdded(), t)
	actTotal := cm.findPositionAtTime(cm.capture.GetActionTimestamps(), cm.capture.GetActionTotalAdded(), t)

	return &Checkpoint{
		CreatedAt:      t,
		LogTotal:       logTotal,
		NetworkTotal:   netTotal,
		WSTotal:        wsTotal,
		ActionTotal:    actTotal,
		KnownEndpoints: make(map[string]endpointState),
	}
}

func (cm *CheckpointManager) findPositionAtTime(addedAt []time.Time, currentTotal int64, t time.Time) int64 {
	if len(addedAt) == 0 {
		return currentTotal
	}

	idx := sort.Search(len(addedAt), func(i int) bool {
		return addedAt[i].After(t)
	})

	entriesAfter := int64(len(addedAt) - idx)
	pos := currentTotal - entriesAfter
	if pos < 0 {
		pos = 0
	}
	return pos
}
