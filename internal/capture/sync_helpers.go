// Purpose: Provides sync command/response helper builders used by the /sync transport handler.
// Why: Keeps serialization and snapshot gating logic separate from request mutation flow.
// Docs: docs/features/feature/query-service/index.md

package capture

import (
	"strings"

	"github.com/dev-console/dev-console/internal/queries"
)

// buildSyncCommands converts pending queries to sync commands.
func buildSyncCommands(pending []queries.PendingQueryResponse) []SyncCommand {
	commands := make([]SyncCommand, len(pending))
	for i, q := range pending {
		commands[i] = SyncCommand{
			ID:            q.ID,
			Type:          q.Type,
			Params:        q.Params,
			TabID:         q.TabID,
			CorrelationID: q.CorrelationID,
			TraceID:       q.TraceID,
		}
	}
	return commands
}

// shouldEmitSyncSnapshot determines whether lifecycle telemetry should include a sync snapshot.
func shouldEmitSyncSnapshot(req SyncRequest, state syncConnectionState, commandsOut int) bool {
	if state.isReconnect || state.wasDisconnected || !state.wasConnected {
		return true
	}
	if len(req.CommandResults) > 0 || commandsOut > 0 {
		return true
	}
	if req.LastCommandAck != "" {
		return true
	}
	return false
}

func (c *Capture) buildCaptureOverrides() map[string]string {
	mode, productionParity, rewrites := c.GetSecurityMode()
	if mode == SecurityModeNormal {
		return map[string]string{}
	}

	overrides := map[string]string{
		"security_mode":     mode,
		"production_parity": "false",
	}
	if productionParity {
		overrides["production_parity"] = "true"
	}
	if len(rewrites) > 0 {
		overrides["insecure_rewrites_applied"] = strings.Join(rewrites, ",")
	}
	return overrides
}
