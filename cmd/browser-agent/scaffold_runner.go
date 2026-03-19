// scaffold_runner.go — Wires the scaffold API endpoint to the scaffold engine.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/scaffold"
)

// ScaffoldRunner manages scaffold job execution and progress broadcasting.
type ScaffoldRunner struct {
	broadcaster *scaffold.Broadcaster
}

// NewScaffoldRunner creates a new scaffold runner.
func NewScaffoldRunner() *ScaffoldRunner {
	return &ScaffoldRunner{
		broadcaster: scaffold.NewBroadcaster(),
	}
}

// HandleScaffold handles POST /api/scaffold.
func (sr *ScaffoldRunner) HandleScaffold(w http.ResponseWriter, r *http.Request) {
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

	// Create engine config.
	cfg := scaffold.Config{
		Description:  req.Description,
		Audience:     req.Audience,
		FirstFeature: req.FirstFeature,
		Name:         req.Name,
	}

	// Start scaffold in background.
	go sr.runScaffold(cfg, channel)

	jsonResponse(w, http.StatusAccepted, scaffoldResponse{
		Status:  "accepted",
		Channel: channel,
	})
}

// runScaffold executes the scaffold steps in the background.
func (sr *ScaffoldRunner) runScaffold(cfg scaffold.Config, channel string) {
	eng, err := scaffold.NewEngine(cfg)
	if err != nil {
		sr.broadcaster.Broadcast(channel, scaffold.StepEvent{
			Step:   "init",
			Status: "error",
			Label:  "Initialization failed",
			Error:  err.Error(),
		})
		return
	}

	eng.OnProgress(func(evt scaffold.StepEvent) {
		sr.broadcaster.Broadcast(channel, evt)
	})

	steps := scaffold.DefaultSteps()
	ctx := context.Background()

	if err := eng.RunAll(ctx, steps); err != nil {
		sr.broadcaster.Broadcast(channel, scaffold.StepEvent{
			Step:   "scaffold",
			Status: "error",
			Label:  "Scaffold failed",
			Error:  err.Error(),
		})
		return
	}

	// Write AI context files.
	if err := scaffold.WriteAIContext(eng.ProjectDir(), cfg); err != nil {
		sr.broadcaster.Broadcast(channel, scaffold.StepEvent{
			Step:   "ai_context",
			Status: "error",
			Label:  "AI context generation failed",
			Error:  err.Error(),
		})
		return
	}

	sr.broadcaster.Broadcast(channel, scaffold.StepEvent{
		Step:   "scaffold",
		Status: "complete",
		Label:  "Scaffold complete",
	})
}
