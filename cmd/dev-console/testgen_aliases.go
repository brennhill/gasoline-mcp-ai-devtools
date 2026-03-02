// Purpose: Re-exports test generation aliases, constants, and helper bindings.
// Why: Keeps compatibility surfaces centralized so handler files stay focused on request flow.
// Docs: docs/features/feature/test-generation/index.md

package main

import "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/testgen"

type TestFromContextRequest = testgen.TestFromContextRequest
type GeneratedTest = testgen.GeneratedTest
type TestCoverage = testgen.TestCoverage
type TestGenMetadata = testgen.TestGenMetadata
type TestHealRequest = testgen.TestHealRequest
type HealedSelector = testgen.HealedSelector
type HealResult = testgen.HealResult
type HealSummary = testgen.HealSummary
type BatchHealResult = testgen.BatchHealResult
type FileHealResult = testgen.FileHealResult
type TestClassifyRequest = testgen.TestClassifyRequest
type TestFailure = testgen.TestFailure
type FailureClassification = testgen.FailureClassification
type SuggestedFix = testgen.SuggestedFix
type BatchClassifyResult = testgen.BatchClassifyResult

const (
	ErrNoErrorContext          = testgen.ErrNoErrorContext
	ErrNoActionsCaptured       = testgen.ErrNoActionsCaptured
	ErrNoBaseline              = testgen.ErrNoBaseline
	ErrTestFileNotFound        = testgen.ErrTestFileNotFound
	ErrSelectorInjection       = testgen.ErrSelectorInjection
	ErrInvalidSelectorSyntax   = testgen.ErrInvalidSelectorSyntax
	ErrClassificationUncertain = testgen.ErrClassificationUncertain
	ErrBatchTooLarge           = testgen.ErrBatchTooLarge
)

const (
	MaxFilesPerBatch    = testgen.MaxFilesPerBatch
	MaxFileSizeBytes    = testgen.MaxFileSizeBytes
	MaxTotalBatchSize   = testgen.MaxTotalBatchSize
	MaxSelectorsPerFile = testgen.MaxSelectorsPerFile
)

const (
	CategorySelectorBroken = testgen.CategorySelectorBroken
	CategoryTimingFlaky    = testgen.CategoryTimingFlaky
	CategoryNetworkFlaky   = testgen.CategoryNetworkFlaky
	CategoryRealBug        = testgen.CategoryRealBug
	CategoryTestBug        = testgen.CategoryTestBug
	CategoryUnknown        = testgen.CategoryUnknown
)

const maxFailuresPerBatch = testgen.MaxFailuresPerBatch

var (
	generateErrorID              = testgen.GenerateErrorID
	generateTestFilename         = testgen.GenerateTestFilename
	extractSelectorsFromActions  = testgen.ExtractSelectorsFromActions
	normalizeTimestamp           = testgen.NormalizeTimestamp
	targetSelector               = testgen.TargetSelector
	playwrightActionLine         = testgen.PlaywrightActionLine
	generatePlaywrightScript     = testgen.GeneratePlaywrightScript
	deriveInteractionTestName    = testgen.DeriveInteractionTestName
	buildRegressionAssertions    = testgen.BuildRegressionAssertions
	insertAssertionsBeforeClose  = testgen.InsertAssertionsBeforeClose
	matchClassificationPattern   = testgen.MatchClassificationPattern
	generateSuggestedFix         = testgen.GenerateSuggestedFix
	validateTestFilePath         = testgen.ValidateTestFilePath
	resolveTestPath              = testgen.ResolveTestPath
	containsDangerousPattern     = testgen.ContainsDangerousPattern
	validateSelector             = testgen.ValidateSelector
	extractSelectorsFromTestFile = testgen.ExtractSelectorsFromTestFile
	findTestFiles                = testgen.FindTestFiles
	formatHealSummary            = testgen.FormatHealSummary
	classifyHealedSelector       = testgen.ClassifyHealedSelector
	isTestFile                   = testgen.IsTestFile
)
