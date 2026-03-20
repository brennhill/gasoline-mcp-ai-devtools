// engine_test.go — Tests for the scaffold engine step execution and verification.

package scaffold

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// ============================================
// Step definitions and verification gates
// ============================================

func TestStepDefinitions_AllStepsHaveVerification(t *testing.T) {
	steps := DefaultSteps()
	if len(steps) == 0 {
		t.Fatal("DefaultSteps must return at least one step")
	}
	for _, s := range steps {
		if s.Name == "" {
			t.Error("step has empty Name")
		}
		if s.Label == "" {
			t.Errorf("step %q has empty Label", s.Name)
		}
		if s.Verify == nil {
			t.Errorf("step %q has no Verify function", s.Name)
		}
	}
}

func TestStepDefinitions_ExpectedStepNames(t *testing.T) {
	steps := DefaultSteps()
	names := make(map[string]bool)
	for _, s := range steps {
		names[s.Name] = true
	}

	expected := []string{
		"create_project",
		"install_deps",
		"add_tailwind",
		"add_shadcn",
		"quality_baseline",
		"git_init",
	}

	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing expected step %q", name)
		}
	}
}

// ============================================
// Verification gates (unit-testable)
// ============================================

func TestVerifyDirectoryExists(t *testing.T) {
	dir := t.TempDir()

	if err := VerifyDirectoryExists(dir); err != nil {
		t.Errorf("VerifyDirectoryExists on existing dir: %v", err)
	}

	if err := VerifyDirectoryExists(filepath.Join(dir, "nonexistent")); err == nil {
		t.Error("VerifyDirectoryExists on nonexistent dir: want error, got nil")
	}
}

func TestVerifyFileExists(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := VerifyFileExists(file); err != nil {
		t.Errorf("VerifyFileExists on existing file: %v", err)
	}

	if err := VerifyFileExists(filepath.Join(dir, "nope.txt")); err == nil {
		t.Error("VerifyFileExists on nonexistent file: want error, got nil")
	}
}

func TestVerifyPackageInstalled(t *testing.T) {
	dir := t.TempDir()

	// No node_modules at all.
	if err := VerifyPackageInstalled(dir, "react"); err == nil {
		t.Error("VerifyPackageInstalled with no node_modules: want error, got nil")
	}

	// Create fake node_modules/react.
	pkgDir := filepath.Join(dir, "node_modules", "react")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := VerifyPackageInstalled(dir, "react"); err != nil {
		t.Errorf("VerifyPackageInstalled with package present: %v", err)
	}
}

func TestVerifyGitInitialized(t *testing.T) {
	dir := t.TempDir()

	if err := VerifyGitInitialized(dir); err == nil {
		t.Error("VerifyGitInitialized with no .git: want error, got nil")
	}

	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := VerifyGitInitialized(dir); err != nil {
		t.Errorf("VerifyGitInitialized with .git: %v", err)
	}
}

// ============================================
// Config validation
// ============================================

func TestNewConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Description:  "a todo app",
				Audience:     "just_me",
				FirstFeature: "drag and drop",
				Name:         "todo-app",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			cfg: Config{
				Description:  "a todo app",
				Audience:     "just_me",
				FirstFeature: "drag and drop",
			},
			wantErr: true,
		},
		{
			name: "missing description",
			cfg: Config{
				Name:         "todo-app",
				Audience:     "just_me",
				FirstFeature: "drag and drop",
			},
			wantErr: true,
		},
		{
			name:    "empty config",
			cfg:     Config{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr && err == nil {
				t.Error("want error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("want no error, got %v", err)
			}
		})
	}
}

// ============================================
// Engine creation
// ============================================

func TestNewEngine_SetsProjectDir(t *testing.T) {
	cfg := Config{
		Description:  "test app",
		Audience:     "just_me",
		FirstFeature: "testing",
		Name:         "test-app",
		BaseDir:      t.TempDir(),
	}
	eng, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	expected := filepath.Join(cfg.BaseDir, "test-app")
	if eng.ProjectDir() != expected {
		t.Errorf("ProjectDir: want %q, got %q", expected, eng.ProjectDir())
	}
}

func TestNewEngine_DefaultBaseDir(t *testing.T) {
	cfg := Config{
		Description:  "test app",
		Audience:     "just_me",
		FirstFeature: "testing",
		Name:         "test-app",
	}
	eng, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, "strum-projects", "test-app")
	if eng.ProjectDir() != expected {
		t.Errorf("ProjectDir: want %q, got %q", expected, eng.ProjectDir())
	}
}

// ============================================
// RunAll: directory existence check
// ============================================

func TestEngine_RunAll_RejectsExistingDirectory(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Description:  "test app",
		Audience:     "just_me",
		FirstFeature: "testing",
		Name:         "existing-app",
		BaseDir:      dir,
	}
	eng, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// Pre-create the project directory.
	if err := os.MkdirAll(eng.ProjectDir(), 0755); err != nil {
		t.Fatal(err)
	}

	steps := []Step{
		{
			Name:  "should_not_run",
			Label: "Should Not Run",
			Run: func(ctx context.Context, pd string) error {
				t.Error("step should not have been executed")
				return nil
			},
			Verify: func(pd string) error { return nil },
		},
	}

	err = eng.RunAll(context.Background(), steps)
	if err == nil {
		t.Error("RunAll should fail when project directory already exists")
	}
}

// ============================================
// Progress callback
// ============================================

func TestEngine_ProgressCallback(t *testing.T) {
	cfg := Config{
		Description:  "test app",
		Audience:     "just_me",
		FirstFeature: "testing",
		Name:         "test-app",
		BaseDir:      t.TempDir(),
	}
	eng, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	var events []StepEvent
	eng.OnProgress(func(evt StepEvent) {
		events = append(events, evt)
	})

	// Emit a test event.
	eng.emitProgress(StepEvent{Step: "test", Status: "running", Label: "Testing"})

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Step != "test" || events[0].Status != "running" {
		t.Errorf("unexpected event: %+v", events[0])
	}
}

// ============================================
// Step execution with mock runner
// ============================================

func TestEngine_RunStep_SuccessPath(t *testing.T) {
	cfg := Config{
		Description:  "test app",
		Audience:     "just_me",
		FirstFeature: "testing",
		Name:         "test-app",
		BaseDir:      t.TempDir(),
	}
	eng, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// Create the project dir so verify passes.
	if err := os.MkdirAll(eng.ProjectDir(), 0755); err != nil {
		t.Fatal(err)
	}

	step := Step{
		Name:  "test_step",
		Label: "Test Step",
		Run: func(ctx context.Context, projectDir string) error {
			return nil
		},
		Verify: func(projectDir string) error {
			return VerifyDirectoryExists(projectDir)
		},
	}

	var events []StepEvent
	eng.OnProgress(func(evt StepEvent) {
		events = append(events, evt)
	})

	err = eng.RunStep(context.Background(), step)
	if err != nil {
		t.Fatalf("RunStep: %v", err)
	}

	// Should have "running" and "done" events.
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}
	if events[0].Status != "running" {
		t.Errorf("first event status: want 'running', got %q", events[0].Status)
	}
	if events[len(events)-1].Status != "done" {
		t.Errorf("last event status: want 'done', got %q", events[len(events)-1].Status)
	}
}

func TestEngine_RunStep_VerifyFailRetry(t *testing.T) {
	cfg := Config{
		Description:  "test app",
		Audience:     "just_me",
		FirstFeature: "testing",
		Name:         "test-app",
		BaseDir:      t.TempDir(),
	}
	eng, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	runCount := 0
	step := Step{
		Name:  "flaky_step",
		Label: "Flaky Step",
		Run: func(ctx context.Context, projectDir string) error {
			runCount++
			if runCount == 2 {
				// Second run: create the directory so verify passes.
				return os.MkdirAll(filepath.Join(projectDir, "success"), 0755)
			}
			return nil
		},
		Verify: func(projectDir string) error {
			return VerifyDirectoryExists(filepath.Join(projectDir, "success"))
		},
	}

	if err := os.MkdirAll(eng.ProjectDir(), 0755); err != nil {
		t.Fatal(err)
	}

	err = eng.RunStep(context.Background(), step)
	if err != nil {
		t.Fatalf("RunStep with retry: %v", err)
	}
	if runCount != 2 {
		t.Errorf("expected 2 runs (1 retry), got %d", runCount)
	}
}

func TestEngine_RunStep_VerifyFailTwice_ReturnsError(t *testing.T) {
	cfg := Config{
		Description:  "test app",
		Audience:     "just_me",
		FirstFeature: "testing",
		Name:         "test-app",
		BaseDir:      t.TempDir(),
	}
	eng, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	if err := os.MkdirAll(eng.ProjectDir(), 0755); err != nil {
		t.Fatal(err)
	}

	step := Step{
		Name:  "always_fails",
		Label: "Always Fails",
		Run: func(ctx context.Context, projectDir string) error {
			return nil
		},
		Verify: func(projectDir string) error {
			return VerifyDirectoryExists(filepath.Join(projectDir, "never-exists"))
		},
	}

	err = eng.RunStep(context.Background(), step)
	if err == nil {
		t.Error("RunStep should fail when verify fails twice")
	}
}
