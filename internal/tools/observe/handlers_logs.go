// Purpose: Observe handlers for browser logs with cursor pagination.

package observe

import (
	"encoding/json"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/mcp"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/pagination"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
)

// logQueryParams holds the validated parameters for a browser logs query.
type logQueryParams struct {
	Limit             int
	Level             string
	MinLevel          string
	Source            string
	URL               string
	Scope             string
	AfterCursor       string
	BeforeCursor      string
	SinceCursor       string
	RestartOnEviction bool
	IncludeInternal   bool
	IncludeExtension  bool
	ExtensionLimit    int
	Summary           bool
}

// parseLogParams unmarshals and validates log query parameters, returning
// a normalized params struct and any user-facing parameter hints.
func parseLogParams(args json.RawMessage) (logQueryParams, string) {
	var raw struct {
		Limit             int    `json:"limit"`
		Level             string `json:"level"`
		MinLevel          string `json:"min_level"`
		Source            string `json:"source"`
		URL               string `json:"url"`
		Scope             string `json:"scope"`
		AfterCursor       string `json:"after_cursor"`
		BeforeCursor      string `json:"before_cursor"`
		SinceCursor       string `json:"since_cursor"`
		RestartOnEviction bool   `json:"restart_on_eviction"`
		IncludeInternal   bool   `json:"include_internal"`
		IncludeExtension  bool   `json:"include_extension_logs"`
		ExtensionLimit    int    `json:"extension_limit"`
		Summary           bool   `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &raw)

	p := logQueryParams{
		Limit:             clampLimit(raw.Limit, 100),
		Level:             raw.Level,
		MinLevel:          raw.MinLevel,
		Source:            raw.Source,
		URL:               raw.URL,
		Scope:             raw.Scope,
		AfterCursor:       raw.AfterCursor,
		BeforeCursor:      raw.BeforeCursor,
		SinceCursor:       raw.SinceCursor,
		RestartOnEviction: raw.RestartOnEviction,
		IncludeInternal:   raw.IncludeInternal,
		IncludeExtension:  raw.IncludeExtension,
		ExtensionLimit:    raw.ExtensionLimit,
		Summary:           raw.Summary,
	}

	// Quiet alias: level → min_level (threshold, not exact match).
	if p.Level != "" && p.MinLevel == "" {
		p.MinLevel = p.Level
		p.Level = ""
	}

	var paramHint string
	if p.MinLevel != "" && LogLevelRank(p.MinLevel) < 0 {
		paramHint = "Unknown min_level " + p.MinLevel + " ignored (using default=all). Valid values: debug, log, info, warn, error."
		p.MinLevel = ""
	}

	if p.Scope == "" {
		p.Scope = "current_page"
	}
	if p.Scope != "current_page" && p.Scope != "all" {
		hint := "Unknown scope " + p.Scope + " ignored (using default=current_page). Valid values: current_page, all."
		if paramHint != "" {
			paramHint += " " + hint
		} else {
			paramHint = hint
		}
		p.Scope = "current_page"
	}

	return p, paramHint
}

// logFilterCriteria bundles the filtering inputs for filterLogEntries.
type logFilterCriteria struct {
	IncludeInternal bool
	Scope           string
	TrackedTabID    int
	Level           string
	MinLevel        string
	Source          string
	URL             string
}

// logFilterResult holds filtered entries and suppression counts.
type logFilterResult struct {
	Entries         []pagination.LogEntryWithSequence
	NoiseSuppressed int
}

// filterLogEntries applies scope, level, source, URL, and noise filters to enriched log entries.
func filterLogEntries(deps Deps, enriched []pagination.LogEntryWithSequence, c logFilterCriteria) logFilterResult {
	filtered := make([]pagination.LogEntryWithSequence, 0, len(enriched))
	noiseSuppressed := 0

	for _, e := range enriched {
		entryType, _ := e.Entry["type"].(string)
		if !c.IncludeInternal && isInternalLogType(entryType) {
			continue
		}

		if deps.IsConsoleNoise(e.Entry) {
			noiseSuppressed++
			continue
		}

		if c.Scope == "current_page" && c.TrackedTabID != 0 {
			if !(c.IncludeInternal && isInternalLogType(entryType)) {
				entryTabID, _ := e.Entry["tabId"].(float64)
				if int(entryTabID) != c.TrackedTabID {
					continue
				}
			}
		}

		level, _ := e.Entry["level"].(string)
		if level == "" && isInternalLogType(entryType) {
			level = "info"
		}
		if c.Level != "" && level != c.Level {
			continue
		}
		if c.MinLevel != "" && LogLevelRank(level) < LogLevelRank(c.MinLevel) {
			continue
		}

		if c.Source != "" {
			source, _ := e.Entry["source"].(string)
			if source != c.Source {
				continue
			}
		}

		if c.URL != "" {
			entryURL, _ := e.Entry["url"].(string)
			if !ContainsIgnoreCase(entryURL, c.URL) {
				continue
			}
		}

		filtered = append(filtered, e)
	}

	return logFilterResult{Entries: filtered, NoiseSuppressed: noiseSuppressed}
}

// GetBrowserLogs returns console log entries with cursor-based pagination.
func GetBrowserLogs(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	params, paramHint := parseLogParams(args)

	_, trackedTabID, trackedTabURL := deps.GetCapture().GetTrackingStatus()

	// Default URL filter to the tracked page URL so logs are scoped to
	// the current page, not stale entries from previous navigations.
	if params.URL == "" && params.Scope == "current_page" && trackedTabURL != "" {
		params.URL = trackedTabURL
	}

	rawEntries, _ := deps.GetLogEntries()
	totalAdded := deps.GetLogTotalAdded()

	// Convert to []map[string]any for pagination package.
	allEntries := make([]map[string]any, len(rawEntries))
	for i, e := range rawEntries {
		allEntries[i] = e
	}

	enriched := pagination.EnrichLogEntries(allEntries, totalAdded)

	fr := filterLogEntries(deps, enriched, logFilterCriteria{
		IncludeInternal: params.IncludeInternal,
		Scope:           params.Scope,
		TrackedTabID:    trackedTabID,
		Level:           params.Level,
		MinLevel:        params.MinLevel,
		Source:          params.Source,
		URL:             params.URL,
	})

	paginated, pMeta, err := pagination.ApplyLogCursorPagination(
		fr.Entries,
		params.AfterCursor, params.BeforeCursor, params.SinceCursor,
		params.Limit,
		params.RestartOnEviction,
	)
	if err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrInvalidParam, err.Error(), "Check cursor format or use restart_on_eviction:true")}
	}

	logs := make([]map[string]any, len(paginated))
	for i, e := range paginated {
		logs[i] = normalizeBrowserLogEntry(e.Entry)
	}

	var newestTS time.Time
	if len(paginated) > 0 {
		last := paginated[len(paginated)-1]
		if ts := logEntryTimestamp(last.Entry); ts != "" {
			newestTS = util.ParseTimestamp(ts)
		}
	}

	isFirstPage := params.AfterCursor == "" && params.BeforeCursor == "" && params.SinceCursor == ""
	meta := BuildPaginatedMetadataWithSummary(deps.GetCapture(), newestTS, pMeta, isFirstPage, func() map[string]any {
		return quickLogsSummary(logs)
	})
	meta["scope"] = params.Scope
	if fr.NoiseSuppressed > 0 {
		meta["noise_suppressed"] = fr.NoiseSuppressed
	}

	if params.Summary {
		summaryResp := buildLogsSummary(logs, meta)
		if paramHint != "" {
			summaryResp["param_hint"] = paramHint
		}
		return mcp.Succeed(req, "Browser logs", summaryResp)
	}

	response := map[string]any{
		"logs":     logs,
		"count":    len(logs),
		"metadata": meta,
	}
	if paramHint != "" {
		response["param_hint"] = paramHint
	}
	if len(logs) == 0 {
		response["hint"] = logsEmptyHint(params.Scope, params.MinLevel)
	}

	if params.IncludeExtension {
		limit := params.ExtensionLimit
		if limit <= 0 {
			limit = params.Limit
		}
		limit = clampLimit(limit, 100)
		extLogs := buildExtensionLogEntries(deps.GetCapture().GetExtensionLogs(), limit, params.Level, params.MinLevel)
		response["extension_logs"] = extLogs
		response["extension_logs_count"] = len(extLogs)
	}

	return mcp.Succeed(req, "Browser logs", response)
}
