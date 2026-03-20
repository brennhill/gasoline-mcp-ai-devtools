// engine.go — Scaffold engine: executes scaffold steps with verification gates.

package scaffold

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the user's wizard inputs for scaffolding.
type Config struct {
	Description  string `json:"description"`
	Audience     string `json:"audience"`
	FirstFeature string `json:"first_feature"`
	Name         string `json:"name"`
	BaseDir      string `json:"base_dir"` // defaults to ~/strum-projects
}

// Validate checks that required fields are present.
func (c *Config) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	if c.Description == "" {
		return fmt.Errorf("description is required")
	}
	return nil
}

// StepEvent is a progress update for a single scaffold step.
type StepEvent struct {
	Step   string `json:"step"`
	Status string `json:"status"` // running, done, error, retrying
	Label  string `json:"label"`
	Error  string `json:"error,omitempty"`
}

// Step defines a single scaffold step with a run function and verification gate.
type Step struct {
	Name   string
	Label  string
	Run    func(ctx context.Context, projectDir string) error
	Verify func(projectDir string) error
}

// ProgressFunc is called for each step progress update.
type ProgressFunc func(StepEvent)

// Engine orchestrates scaffold step execution.
type Engine struct {
	cfg        Config
	projectDir string
	onProgress ProgressFunc
}

// NewEngine creates a scaffold engine with the given config.
func NewEngine(cfg Config) (*Engine, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	baseDir := cfg.BaseDir
	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot determine home directory: %w", err)
		}
		baseDir = filepath.Join(home, "strum-projects")
	}

	return &Engine{
		cfg:        cfg,
		projectDir: filepath.Join(baseDir, cfg.Name),
	}, nil
}

// ProjectDir returns the full path to the project directory.
func (e *Engine) ProjectDir() string {
	return e.projectDir
}

// OnProgress sets the progress callback.
func (e *Engine) OnProgress(fn ProgressFunc) {
	e.onProgress = fn
}

// emitProgress sends a progress event if a callback is registered.
func (e *Engine) emitProgress(evt StepEvent) {
	if e.onProgress != nil {
		e.onProgress(evt)
	}
}

// RunStep executes a single step with verification. Retries once on verify failure.
func (e *Engine) RunStep(ctx context.Context, step Step) error {
	e.emitProgress(StepEvent{Step: step.Name, Status: "running", Label: step.Label})

	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			e.emitProgress(StepEvent{Step: step.Name, Status: "retrying", Label: step.Label})
		}

		if err := step.Run(ctx, e.projectDir); err != nil {
			e.emitProgress(StepEvent{Step: step.Name, Status: "error", Label: step.Label, Error: err.Error()})
			return fmt.Errorf("step %q run failed: %w", step.Name, err)
		}

		if err := step.Verify(e.projectDir); err != nil {
			if attempt == 0 {
				// First failure: retry.
				continue
			}
			e.emitProgress(StepEvent{Step: step.Name, Status: "error", Label: step.Label, Error: err.Error()})
			return fmt.Errorf("step %q verification failed after retry: %w", step.Name, err)
		}

		e.emitProgress(StepEvent{Step: step.Name, Status: "done", Label: step.Label})
		return nil
	}

	// Unreachable, but satisfies the compiler.
	return fmt.Errorf("step %q: unexpected loop exit", step.Name)
}

// RunAll executes all steps in sequence.
// Returns an error if the project directory already exists.
func (e *Engine) RunAll(ctx context.Context, steps []Step) error {
	if _, err := os.Stat(e.projectDir); err == nil {
		return fmt.Errorf("project directory already exists: %s", e.projectDir)
	}
	for _, step := range steps {
		if err := e.RunStep(ctx, step); err != nil {
			return err
		}
	}
	return nil
}
