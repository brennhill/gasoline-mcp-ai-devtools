// Purpose: Atomically checks all filters and emits MCP notifications for qualifying alerts.
// Why: Separates the emit path from dedup, filter, and rate-limit logic.
package streaming

import (
	"encoding/json"
	"io"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/types"
)

// EmitAlert atomically checks all filters and emits an MCP notification if appropriate.
func (s *StreamState) EmitAlert(alert types.Alert) {
	type emitPlan struct {
		emit         bool
		writer       io.Writer
		notification MCPNotification
	}
	plan := func() emitPlan {
		s.Mu.Lock()
		defer s.Mu.Unlock()

		if !s.passesFiltersLocked(alert) {
			return emitPlan{}
		}

		now := time.Now()
		if !s.canEmitAtLocked(now) {
			if len(s.PendingBatch) < MaxPendingBatch {
				s.PendingBatch = append(s.PendingBatch, alert)
			}
			return emitPlan{}
		}

		dedupKey := alert.Category + ":" + alert.Title
		if s.isDuplicateLocked(dedupKey, now) {
			return emitPlan{}
		}

		s.recordEmissionLocked(dedupKey, now)
		return emitPlan{
			emit:         true,
			writer:       s.Writer,
			notification: FormatMCPNotification(alert),
		}
	}()
	if !plan.emit {
		return
	}

	if plan.writer != nil {
		data, err := json.Marshal(plan.notification)
		if err == nil {
			_, _ = plan.writer.Write(data)
			_, _ = plan.writer.Write([]byte{'\n'})
		}
	}
}

// FormatMCPNotification creates an MCP notification from an alert.
func FormatMCPNotification(alert types.Alert) MCPNotification {
	return MCPNotification{
		JSONRPC: "2.0",
		Method:  "notifications/message",
		Params: NotificationParams{
			Level:  alert.Severity,
			Logger: NotificationLoggerName,
			Data: map[string]any{
				"category":  alert.Category,
				"severity":  alert.Severity,
				"title":     alert.Title,
				"detail":    alert.Detail,
				"timestamp": alert.Timestamp,
				"source":    alert.Source,
			},
		},
	}
}
