// Purpose: Groups and fingerprints log entries into aggregated summaries for the summarized_logs observe mode.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/json"
	"math"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/mcp"
)

// GetSummarizedLogs handles observe(what="summarized_logs").
func GetSummarizedLogs(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	params := parseSummarizedLogsParams(args)
	if params.Scope != "current_page" && params.Scope != "all" {
		return mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcp.StructuredErrorResponse(
				mcp.ErrInvalidParam,
				"Invalid scope: "+params.Scope,
				"Use 'current_page' (default) or 'all'",
				mcp.WithParam("scope"),
			),
		}
	}

	_, trackedTabID, trackedTabURL := deps.GetCapture().GetTrackingStatus()
	if params.URL == "" && params.Scope == "current_page" && trackedTabURL != "" {
		params.URL = trackedTabURL
	}
	rawEntries, _ := deps.GetLogEntries()
	views, noiseSuppressed := filterSummarizedLogViews(rawEntries, deps, params, trackedTabID)

	groups, anomalies := groupLogs(views, params.MinGroupSize)
	if groups == nil {
		groups = []LogGroup{}
	}
	if anomalies == nil {
		anomalies = []LogAnomaly{}
	}
	detectPeriodicity(groups)

	timeStart, timeEnd := summarizedLogsTimeRange(views)
	summary := summarizedLogsSummary(views, groups, anomalies, noiseSuppressed, timeStart, timeEnd)
	responseMeta := BuildResponseMetadata(deps.GetCapture(), time.Time{})

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Summarized logs", map[string]any{
		"groups":    cleanSummarizedLogGroups(groups),
		"anomalies": anomalies,
		"summary":   summary,
		"metadata":  responseMeta,
	})}
}

type summarizedLogsParams struct {
	Limit        int
	MinLevel     string
	Level        string
	Source       string
	URL          string
	Scope        string
	MinGroupSize int
}

func parseSummarizedLogsParams(args json.RawMessage) summarizedLogsParams {
	var params summarizedLogsParams
	mcp.LenientUnmarshal(args, &params)
	if params.Scope == "" {
		params.Scope = "current_page"
	}
	params.Limit = clampLimit(params.Limit, 100)
	if params.MinGroupSize <= 0 {
		params.MinGroupSize = 2
	}
	return params
}

func filterSummarizedLogViews(rawEntries []map[string]any, deps Deps, params summarizedLogsParams, trackedTabID int) ([]logEntryView, int) {
	noiseSuppressed := 0
	views := make([]logEntryView, 0, min(params.Limit, len(rawEntries)))
	count := 0

	// Scan newest first (tail to head), up to limit.
	for i := len(rawEntries) - 1; i >= 0 && count < params.Limit; i-- {
		entry := rawEntries[i]
		entryType, _ := entry["type"].(string)
		if entryType == "lifecycle" || entryType == "tracking" || entryType == "extension" {
			continue
		}
		if deps.IsConsoleNoise(entry) {
			noiseSuppressed++
			continue
		}
		if params.Scope == "current_page" && trackedTabID != 0 {
			entryTabID, _ := entry["tabId"].(float64)
			if int(entryTabID) != trackedTabID {
				continue
			}
		}
		level, _ := entry["level"].(string)
		if params.Level != "" && level != params.Level {
			continue
		}
		if params.MinLevel != "" && LogLevelRank(level) < LogLevelRank(params.MinLevel) {
			continue
		}
		if params.Source != "" {
			source, _ := entry["source"].(string)
			if source != params.Source {
				continue
			}
		}
		if params.URL != "" {
			entryURL, _ := entry["url"].(string)
			if !ContainsIgnoreCase(entryURL, params.URL) {
				continue
			}
		}

		message, _ := entry["message"].(string)
		source, _ := entry["source"].(string)
		url, _ := entry["url"].(string)
		ts, _ := entry["ts"].(string)
		views = append(views, logEntryView{
			Level:   level,
			Message: message,
			Source:  source,
			URL:     url,
			Line:    entry["line"],
			Column:  entry["column"],
			TS:      ts,
			TabID:   entry["tabId"],
		})
		count++
	}

	return views, noiseSuppressed
}

func cleanSummarizedLogGroups(groups []LogGroup) []map[string]any {
	cleanGroups := make([]map[string]any, len(groups))
	for i, g := range groups {
		m := map[string]any{
			"fingerprint":     g.Fingerprint,
			"sample_message":  g.SampleMessage,
			"count":           g.Count,
			"level_breakdown": g.LevelBreakdown,
			"first_seen":      g.FirstSeen,
			"last_seen":       g.LastSeen,
			"is_periodic":     g.IsPeriodic,
			"source":          g.Source,
		}
		if g.PeriodSeconds > 0 {
			m["period_seconds"] = g.PeriodSeconds
		}
		if len(g.Sources) > 1 {
			m["sources"] = g.Sources
		}
		cleanGroups[i] = m
	}
	return cleanGroups
}

func summarizedLogsSummary(views []logEntryView, groups []LogGroup, anomalies []LogAnomaly, noiseSuppressed int, timeStart, timeEnd string) map[string]any {
	totalEntries := len(views)
	compressionRatio := 0.0
	if totalEntries > 0 {
		compressionRatio = 1.0 - float64(len(groups)+len(anomalies))/float64(totalEntries)
		compressionRatio = math.Round(compressionRatio*100) / 100
	}

	return map[string]any{
		"total_entries":     totalEntries,
		"groups":            len(groups),
		"anomalies":         len(anomalies),
		"noise_suppressed":  noiseSuppressed,
		"compression_ratio": compressionRatio,
		"time_range": map[string]any{
			"start": timeStart,
			"end":   timeEnd,
		},
	}
}

func summarizedLogsTimeRange(views []logEntryView) (string, string) {
	var timeStart, timeEnd string
	for _, v := range views {
		if v.TS == "" {
			continue
		}
		if timeStart == "" || v.TS < timeStart {
			timeStart = v.TS
		}
		if timeEnd == "" || v.TS > timeEnd {
			timeEnd = v.TS
		}
	}
	return timeStart, timeEnd
}
