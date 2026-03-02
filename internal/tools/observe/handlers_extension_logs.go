// Purpose: Observe handlers for extension debug log access.

package observe

import (
	"encoding/json"
	"time"

	"github.com/dev-console/dev-console/internal/buffers"
	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/mcp"
)

func buildExtensionLogEntries(allLogs []capture.ExtensionLog, limit int, level string) []map[string]any {
	matched := buffers.ReverseFilterLimit(allLogs, func(entry capture.ExtensionLog) bool {
		return level == "" || entry.Level == level
	}, limit)

	logs := make([]map[string]any, len(matched))
	for i, entry := range matched {
		logs[i] = map[string]any{
			"level":     entry.Level,
			"message":   entry.Message,
			"source":    entry.Source,
			"category":  entry.Category,
			"data":      entry.Data,
			"timestamp": entry.Timestamp,
		}
	}
	return logs
}

// GetExtensionLogs returns internal extension debug logs.
func GetExtensionLogs(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit int    `json:"limit"`
		Level string `json:"level"`
	}
	mcp.LenientUnmarshal(args, &params)
	params.Limit = clampLimit(params.Limit, 100)

	allLogs := deps.GetCapture().GetExtensionLogs()

	matched := buffers.ReverseFilterLimit(allLogs, func(entry capture.ExtensionLog) bool {
		return params.Level == "" || entry.Level == params.Level
	}, params.Limit)

	logs := make([]map[string]any, len(matched))
	for i, entry := range matched {
		logs[i] = map[string]any{
			"level":     entry.Level,
			"message":   entry.Message,
			"source":    entry.Source,
			"category":  entry.Category,
			"data":      entry.Data,
			"timestamp": entry.Timestamp,
		}
	}

	var newestTS time.Time
	if len(allLogs) > 0 {
		newestTS = allLogs[len(allLogs)-1].Timestamp
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Extension logs", map[string]any{
		"logs":     logs,
		"count":    len(logs),
		"metadata": BuildResponseMetadata(deps.GetCapture(), newestTS),
	})}
}
