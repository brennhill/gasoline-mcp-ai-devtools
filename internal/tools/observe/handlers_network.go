// Purpose: Observe handlers for network bodies and WebSocket events.

package observe

import (
	"encoding/json"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/buffers"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/mcp"
)

// GetNetworkBodies returns captured HTTP response bodies with optional filtering.
// #lizard forgives
func GetNetworkBodies(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit     int    `json:"limit"`
		URL       string `json:"url"`
		Method    string `json:"method"`
		StatusMin int    `json:"status_min"`
		StatusMax int    `json:"status_max"`
		BodyKey   string `json:"body_key"`
		BodyPath  string `json:"body_path"`
		Summary   bool   `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)
	if params.BodyKey != "" && params.BodyPath != "" {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrInvalidParam,
			"Only one body filter can be used at a time",
			"Use either 'body_key' or 'body_path', not both",
			mcp.WithParam("body_key"),
			mcp.WithParam("body_path"),
		)}
	}
	params.Limit = clampLimit(params.Limit, 100)

	allBodies := deps.GetCapture().GetNetworkBodies()
	var bodyFilterErr error
	filtered := buffers.ReverseFilterLimit(allBodies, func(b capture.NetworkBody) bool {
		if bodyFilterErr != nil {
			return false
		}
		if params.URL != "" && !ContainsIgnoreCase(b.URL, params.URL) {
			return false
		}
		if params.Method != "" && !ContainsIgnoreCase(b.Method, params.Method) {
			return false
		}
		if params.StatusMin > 0 && b.Status < params.StatusMin {
			return false
		}
		if params.StatusMax > 0 && b.Status > params.StatusMax {
			return false
		}
		_, include, err := ApplyNetworkBodyFilter(b, params.BodyKey, params.BodyPath)
		if err != nil {
			bodyFilterErr = err
			return false
		}
		return include
	}, params.Limit)

	if bodyFilterErr != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrInvalidParam,
			"Invalid network body filter: "+bodyFilterErr.Error(),
			"Use a valid body_path syntax like data.items[0].id",
			mcp.WithParam("body_path"),
		)}
	}

	// Re-apply body filter to transform matched entries (extract body_key/body_path).
	if params.BodyKey != "" || params.BodyPath != "" {
		for i, b := range filtered {
			filteredBody, _, _ := ApplyNetworkBodyFilter(b, params.BodyKey, params.BodyPath)
			filtered[i] = filteredBody
		}
	}
	var newestTS time.Time
	if len(allBodies) > 0 {
		newestTS, _ = time.Parse(time.RFC3339, allBodies[len(allBodies)-1].Timestamp)
	}

	waterfallCount := len(deps.GetCapture().GetNetworkWaterfallEntries())
	responseMeta := BuildResponseMetadata(deps.GetCapture(), newestTS)
	if params.Summary {
		summary := buildNetworkBodiesSummary(filtered, responseMeta)
		if len(filtered) == 0 {
			// TODO: Extend hints to reflect method/status/body_key/body_path filters, not just URL.
			summary["hint"] = networkBodiesEmptyHint(waterfallCount, len(allBodies), params.URL)
		}
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Network bodies", summary)}
	}

	response := map[string]any{
		"entries":  filtered,
		"count":    len(filtered),
		"metadata": responseMeta,
	}

	if len(filtered) == 0 {
		// TODO: Extend hints to reflect method/status/body_key/body_path filters, not just URL.
		response["hint"] = networkBodiesEmptyHint(waterfallCount, len(allBodies), params.URL)
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Network bodies", response)}
}

// GetWSEvents returns captured WebSocket events with optional filtering.
func GetWSEvents(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit        int    `json:"limit"`
		URL          string `json:"url"`
		ConnectionID string `json:"connection_id"`
		Direction    string `json:"direction"`
		Summary      bool   `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)
	params.Limit = clampLimit(params.Limit, 100)

	allEvents := deps.GetCapture().GetAllWebSocketEvents()
	filtered := buffers.ReverseFilterLimit(allEvents, func(evt capture.WebSocketEvent) bool {
		if params.URL != "" && !ContainsIgnoreCase(evt.URL, params.URL) {
			return false
		}
		if params.ConnectionID != "" && evt.ID != params.ConnectionID {
			return false
		}
		if params.Direction != "" && evt.Direction != params.Direction {
			return false
		}
		return true
	}, params.Limit)
	var newestTS time.Time
	if len(allEvents) > 0 {
		newestTS, _ = time.Parse(time.RFC3339, allEvents[len(allEvents)-1].Timestamp)
	}

	responseMeta := BuildResponseMetadata(deps.GetCapture(), newestTS)
	if params.Summary {
		summary := buildWSEventsSummary(filtered, responseMeta)
		if len(filtered) == 0 {
			summary["hint"] = wsEventsEmptyHint(len(allEvents), params.URL)
		}
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("WebSocket events", summary)}
	}

	response := map[string]any{
		"entries":  filtered,
		"count":    len(filtered),
		"metadata": responseMeta,
	}

	if len(filtered) == 0 {
		response["hint"] = wsEventsEmptyHint(len(allEvents), params.URL)
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("WebSocket events", response)}
}
