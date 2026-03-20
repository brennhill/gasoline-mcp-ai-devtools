// scaffold_runner.go — Wires the scaffold API endpoint to the scaffold engine.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/scaffold"
)

// ScaffoldRunner manages scaffold job execution and progress broadcasting.
type ScaffoldRunner struct {
	broadcaster *scaffold.Broadcaster
	inflight    sync.Map // in-flight project names → struct{}
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

	// Limit request body to 1 MB.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

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

	// Concurrency guard: reject if same project name is already scaffolding.
	if _, loaded := sr.inflight.LoadOrStore(req.Name, struct{}{}); loaded {
		jsonResponse(w, http.StatusConflict, map[string]string{"error": fmt.Sprintf("project %q is already being scaffolded", req.Name)})
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
	// Always release the concurrency guard when done.
	defer sr.inflight.Delete(cfg.Name)

	// Panic recovery: broadcast error if a panic occurs.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("scaffold panic for %q: %v\n%s", cfg.Name, r, debug.Stack())
			sr.broadcaster.Broadcast(channel, scaffold.StepEvent{
				Step:   "scaffold",
				Status: "error",
				Label:  "Scaffold failed (internal error)",
				Error:  fmt.Sprintf("panic: %v", r),
			})
		}
	}()

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

	// Timeout: 5 minutes for the entire scaffold process.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

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
