// telemetry.go — Unified GET /telemetry endpoint with basic pagination.
// Replaces 6 separate GET endpoints for reading telemetry buffers.
// Supports: logs, network_waterfall, network_bodies, websocket_events,
// actions, performance_snapshots, extension_logs, websocket_status.
package main

import (
	"net/http"
	"strconv"

	"github.com/dev-console/dev-console/internal/capture"
)

// handleTelemetry returns an http.HandlerFunc that serves GET /telemetry.
// Dispatches to the appropriate buffer getter based on the type query param.
func handleTelemetry(server *Server, cap *capture.Capture) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		q := r.URL.Query()
		telType := q.Get("type")
		if telType == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{
				"error": "Missing required 'type' parameter",
				"hint":  "Valid types: logs, network_waterfall, network_bodies, websocket_events, actions, performance_snapshots, extension_logs, websocket_status",
			})
			return
		}

		limit := 0
		if ls := q.Get("limit"); ls != "" {
			if v, err := strconv.Atoi(ls); err == nil && v > 0 {
				limit = v
			}
		}

		var result any
		var count int

		switch telType {
		case "logs":
			entries := server.getEntries()
			if limit > 0 && len(entries) > limit {
				entries = entries[len(entries)-limit:]
			}
			result = entries
			count = len(entries)

		case "network_waterfall":
			entries := cap.GetNetworkWaterfallEntries()
			if limit > 0 && len(entries) > limit {
				entries = entries[len(entries)-limit:]
			}
			result = entries
			count = len(entries)

		case "network_bodies":
			bodies := cap.GetNetworkBodies()
			if limit > 0 && len(bodies) > limit {
				bodies = bodies[len(bodies)-limit:]
			}
			result = bodies
			count = len(bodies)

		case "websocket_events":
			events := cap.GetWebSocketEvents(capture.WebSocketEventFilter{})
			if limit > 0 && len(events) > limit {
				events = events[len(events)-limit:]
			}
			result = events
			count = len(events)

		case "actions":
			actions := cap.GetAllEnhancedActions()
			if limit > 0 && len(actions) > limit {
				actions = actions[len(actions)-limit:]
			}
			result = actions
			count = len(actions)

		case "performance_snapshots":
			snapshots := cap.GetPerformanceSnapshots()
			if limit > 0 && len(snapshots) > limit {
				snapshots = snapshots[len(snapshots)-limit:]
			}
			result = snapshots
			count = len(snapshots)

		case "extension_logs":
			elogs := cap.GetExtensionLogs()
			if limit > 0 && len(elogs) > limit {
				elogs = elogs[len(elogs)-limit:]
			}
			result = elogs
			count = len(elogs)

		case "websocket_status":
			status := cap.GetWebSocketStatus(capture.WebSocketStatusFilter{})
			jsonResponse(w, http.StatusOK, map[string]any{
				"type":        telType,
				"connections": status.Connections,
				"closed":      status.Closed,
				"count":       len(status.Connections),
			})
			return

		default:
			jsonResponse(w, http.StatusBadRequest, map[string]string{
				"error": "Unknown telemetry type: " + telType,
				"hint":  "Valid types: logs, network_waterfall, network_bodies, websocket_events, actions, performance_snapshots, extension_logs, websocket_status",
			})
			return
		}

		jsonResponse(w, http.StatusOK, map[string]any{
			"type":  telType,
			"items": result,
			"count": count,
		})
	}
}
