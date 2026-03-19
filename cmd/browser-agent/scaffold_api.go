// scaffold_api.go — Handles POST /api/scaffold to initiate project scaffolding.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// scaffoldRequest is the JSON body for POST /api/scaffold.
type scaffoldRequest struct {
	Description  string `json:"description"`
	Audience     string `json:"audience"`
	FirstFeature string `json:"first_feature"`
	Name         string `json:"name"`
}

// scaffoldResponse is the JSON response for POST /api/scaffold.
type scaffoldResponse struct {
	Status  string `json:"status"`
	Channel string `json:"channel"`
}

// validAudiences is the set of allowed audience values.
var validAudiences = map[string]bool{
	"just_me": true,
	"my_team": true,
	"public":  true,
}

// handleScaffoldAPI handles POST /api/scaffold.
func handleScaffoldAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var req scaffoldRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON body"})
		return
	}

	// Validate required fields.
	if req.Description == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "description is required"})
		return
	}
	if req.Name == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if req.Audience != "" && !validAudiences[req.Audience] {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "audience must be one of: just_me, my_team, public"})
		return
	}

	// Generate channel ID.
	channel := fmt.Sprintf("scaffold-%s-%d", req.Name, time.Now().Unix())

	jsonResponse(w, http.StatusAccepted, scaffoldResponse{
		Status:  "accepted",
		Channel: channel,
	})
}
