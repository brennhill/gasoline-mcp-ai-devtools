// types.go — Types and constants for test generation, healing, and classification.
package testgen

// ============================================
// Test Generation Types
// ============================================

// TestFromContextRequest represents generate {type: "test_from_context"} parameters.
type TestFromContextRequest struct {
	Context      string `json:"context"`       // "error", "interaction", "regression"
	ErrorID      string `json:"error_id"`      // Optional: specific error to reproduce
	Framework    string `json:"framework"`     // "playwright", "vitest", "jest"
	OutputFormat string `json:"output_format"` // "file", "inline"
	BaseURL      string `json:"base_url"`
	IncludeMocks bool   `json:"include_mocks"`
	TestName     string `json:"test_name"` // Optional: override filename base
}

// GeneratedTest represents the output of test generation.
type GeneratedTest struct {
	Framework  string          `json:"framework"`
	Filename   string          `json:"filename"`
	Content    string          `json:"content"`
	Selectors  []string        `json:"selectors"`
	Assertions int             `json:"assertions"`
	Coverage   TestCoverage    `json:"coverage"`
	Metadata   TestGenMetadata `json:"metadata"`
}

// TestCoverage describes what the generated test covers.
type TestCoverage struct {
	ErrorReproduced bool `json:"error_reproduced"`
	NetworkMocked   bool `json:"network_mocked"`
	StateCaptured   bool `json:"state_captured"`
}

// TestGenMetadata provides traceability.
type TestGenMetadata struct {
	SourceError string   `json:"source_error,omitempty"`
	GeneratedAt string   `json:"generated_at"`
	ContextUsed []string `json:"context_used"`
}

// ============================================
// Test Heal Types
// ============================================

// TestHealRequest represents generate {type: "test_heal"} parameters.
type TestHealRequest struct {
	Action          string   `json:"action"`           // "analyze" | "repair" | "batch"
	TestFile        string   `json:"test_file"`        // For analyze/repair
	TestDir         string   `json:"test_dir"`         // For batch
	BrokenSelectors []string `json:"broken_selectors"` // For repair
	AutoApply       bool     `json:"auto_apply"`       // For repair
}

// HealedSelector represents a repaired selector.
type HealedSelector struct {
	OldSelector string  `json:"old_selector"`
	NewSelector string  `json:"new_selector"`
	Confidence  float64 `json:"confidence"`
	Strategy    string  `json:"strategy"`
	LineNumber  int     `json:"line_number"`
}

// HealResult represents selector healing output.
type HealResult struct {
	Healed         []HealedSelector `json:"healed"`
	Unhealed       []string         `json:"unhealed"`
	UpdatedContent string           `json:"updated_content,omitempty"`
	Summary        HealSummary      `json:"summary"`
}

// HealSummary provides statistics on healing results.
type HealSummary struct {
	TotalBroken  int `json:"total_broken"`
	HealedAuto   int `json:"healed_auto"`
	HealedManual int `json:"healed_manual"`
	Unhealed     int `json:"unhealed"`
}

// BatchHealResult represents results from healing a batch of test files.
type BatchHealResult struct {
	FilesProcessed int              `json:"files_processed"`
	FilesSkipped   int              `json:"files_skipped"`
	TotalSelectors int              `json:"total_selectors"`
	TotalHealed    int              `json:"total_healed"`
	TotalUnhealed  int              `json:"total_unhealed"`
	FileResults    []FileHealResult `json:"file_results"`
	Warnings       []string         `json:"warnings,omitempty"`
}

// FileHealResult represents healing results for a single file.
type FileHealResult struct {
	FilePath string `json:"file_path"`
	Healed   int    `json:"healed"`
	Unhealed int    `json:"unhealed"`
	Skipped  bool   `json:"skipped"`
	Reason   string `json:"reason,omitempty"`
}

// ============================================
// Test Classify Types
// ============================================

// TestClassifyRequest represents generate {type: "test_classify"} parameters.
type TestClassifyRequest struct {
	Action     string        `json:"action"` // "failure", "batch"
	Failure    *TestFailure  `json:"failure"`
	Failures   []TestFailure `json:"failures"`
	TestOutput string        `json:"test_output"`
}

// TestFailure represents a single test failure to classify.
type TestFailure struct {
	TestName   string `json:"test_name"`
	Error      string `json:"error"`
	Screenshot string `json:"screenshot"` // base64, optional
	Trace      string `json:"trace"`      // stack trace
	DurationMs int64  `json:"duration_ms"`
}

// FailureClassification represents the result of classifying a test failure.
type FailureClassification struct {
	Category          string        `json:"category"`
	Confidence        float64       `json:"confidence"`
	Evidence          []string      `json:"evidence"`
	RecommendedAction string        `json:"recommended_action"`
	IsRealBug         bool          `json:"is_real_bug"`
	IsFlaky           bool          `json:"is_flaky"`
	IsEnvironment     bool          `json:"is_environment"`
	SuggestedFix      *SuggestedFix `json:"suggested_fix,omitempty"`
}

// SuggestedFix provides actionable fix suggestion.
type SuggestedFix struct {
	Type string `json:"type"` // "selector_update", "add_wait", "mock_network", etc.
	Old  string `json:"old,omitempty"`
	New  string `json:"new,omitempty"`
	Code string `json:"code,omitempty"`
}

// BatchClassifyResult represents the result of classifying multiple failures.
type BatchClassifyResult struct {
	TotalClassified int                     `json:"total_classified"`
	RealBugs        int                     `json:"real_bugs"`
	FlakyTests      int                     `json:"flaky_tests"`
	TestBugs        int                     `json:"test_bugs"`
	Uncertain       int                     `json:"uncertain"`
	Classifications []FailureClassification `json:"classifications"`
	Summary         map[string]int          `json:"summary"` // category -> count
}

// ============================================
// Constants
// ============================================

// Error codes for test generation.
const (
	ErrNoErrorContext          = "no_error_context"
	ErrNoActionsCaptured       = "no_actions_captured"
	ErrNoBaseline              = "no_baseline"
	ErrTestFileNotFound        = "test_file_not_found"
	ErrSelectorInjection       = "selector_injection_detected"
	ErrInvalidSelectorSyntax   = "invalid_selector_syntax"
	ErrClassificationUncertain = "classification_uncertain"
	ErrBatchTooLarge           = "batch_too_large"
)

// Batch limits.
const (
	MaxFilesPerBatch    = 20
	MaxFileSizeBytes    = 500 * 1024      // 500KB
	MaxTotalBatchSize   = 5 * 1024 * 1024 // 5MB
	MaxSelectorsPerFile = 50
)

// Classification categories.
const (
	CategorySelectorBroken = "selector_broken"
	CategoryTimingFlaky    = "timing_flaky"
	CategoryNetworkFlaky   = "network_flaky"
	CategoryRealBug        = "real_bug"
	CategoryTestBug        = "test_bug"
	CategoryUnknown        = "unknown"
)

// MaxFailuresPerBatch is the limit for batch failure classification.
const MaxFailuresPerBatch = 20
