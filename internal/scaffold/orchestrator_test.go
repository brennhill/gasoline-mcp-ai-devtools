// orchestrator_test.go — Tests for the scaffold orchestrator that wires engine + progress + API.

package scaffold

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// ============================================
// Orchestrator: Full scaffold flow with mock steps
// ============================================

func TestOrchestrator_RunsAllMockSteps(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Description:  "test app",
		Audience:     "just_me",
		FirstFeature: "testing",
		Name:         "test-app",
		BaseDir:      dir,
	}

	eng, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	projectDir := eng.ProjectDir()
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create mock steps that create marker files.
	steps := []Step{
		{
			Name:  "step_1",
			Label: "Step 1",
			Run: func(ctx context.Context, pd string) error {
				return os.WriteFile(filepath.Join(pd, "step1.marker"), []byte("done"), 0644)
			},
			Verify: func(pd string) error {
				return VerifyFileExists(filepath.Join(pd, "step1.marker"))
			},
		},
		{
			Name:  "step_2",
			Label: "Step 2",
			Run: func(ctx context.Context, pd string) error {
				return os.WriteFile(filepath.Join(pd, "step2.marker"), []byte("done"), 0644)
			},
			Verify: func(pd string) error {
				return VerifyFileExists(filepath.Join(pd, "step2.marker"))
			},
		},
	}

	var events []StepEvent
	eng.OnProgress(func(evt StepEvent) {
		events = append(events, evt)
	})

	if err := eng.RunAll(context.Background(), steps); err != nil {
		t.Fatalf("RunAll: %v", err)
	}

	// Verify both marker files exist.
	for _, f := range []string{"step1.marker", "step2.marker"} {
		if _, err := os.Stat(filepath.Join(projectDir, f)); err != nil {
			t.Errorf("missing marker file: %s", f)
		}
	}

	// Verify we got events for both steps.
	if len(events) < 4 { // 2 running + 2 done
		t.Errorf("expected at least 4 events, got %d", len(events))
	}
}

func TestOrchestrator_StopsOnError(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Description:  "test app",
		Audience:     "just_me",
		FirstFeature: "testing",
		Name:         "test-app",
		BaseDir:      dir,
	}

	eng, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	if err := os.MkdirAll(eng.ProjectDir(), 0755); err != nil {
		t.Fatal(err)
	}

	step2Ran := false
	steps := []Step{
		{
			Name:  "failing_step",
			Label: "Failing Step",
			Run: func(ctx context.Context, pd string) error {
				return nil // runs, but verify will fail
			},
			Verify: func(pd string) error {
				return VerifyFileExists(filepath.Join(pd, "never-exists"))
			},
		},
		{
			Name:  "should_not_run",
			Label: "Should Not Run",
			Run: func(ctx context.Context, pd string) error {
				step2Ran = true
				return nil
			},
			Verify: func(pd string) error { return nil },
		},
	}

	err = eng.RunAll(context.Background(), steps)
	if err == nil {
		t.Error("RunAll should fail when a step fails")
	}

	if step2Ran {
		t.Error("step 2 should not have run after step 1 failed")
	}
}

func TestOrchestrator_ContextCancellation(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Description:  "test app",
		Audience:     "just_me",
		FirstFeature: "testing",
		Name:         "test-app",
		BaseDir:      dir,
	}

	eng, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	if err := os.MkdirAll(eng.ProjectDir(), 0755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	steps := []Step{
		{
			Name:  "cancels",
			Label: "Cancels Mid-Run",
			Run: func(ctx context.Context, pd string) error {
				cancel()
				return ctx.Err()
			},
			Verify: func(pd string) error { return nil },
		},
	}

	err = eng.RunAll(ctx, steps)
	if err == nil {
		t.Error("RunAll should fail when context is cancelled")
	}
}
