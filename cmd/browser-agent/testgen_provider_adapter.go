// Purpose: Adapts testGenHandler deps to the internal testgen DataProvider API.
// Why: Isolates data access and wrapper delegation from request parsing/response formatting.
// Docs: docs/features/feature/test-generation/index.md

package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testgen"
)

// toolHandlerDataProvider adapts testGenHandlerDeps to testgen.DataProvider.
type toolHandlerDataProvider struct {
	deps testGenHandlerDeps
}

func (a *toolHandlerDataProvider) GetLogEntries() []map[string]any {
	entries, _ := a.deps.GetLogEntries()
	return entries
}

func (a *toolHandlerDataProvider) GetAllEnhancedActions() []capture.EnhancedAction {
	return a.deps.GetCapture().GetAllEnhancedActions()
}

func (a *toolHandlerDataProvider) GetNetworkBodies() []capture.NetworkBody {
	return a.deps.GetCapture().GetNetworkBodies()
}

// dataProvider returns a testgen.DataProvider backed by this test-generation handler.
func (h *testGenHandler) dataProvider() testgen.DataProvider {
	return &toolHandlerDataProvider{deps: h.deps}
}

func (h *testGenHandler) generateTestFromError(req TestFromContextRequest) (*GeneratedTest, error) {
	return testgen.GenerateTestFromError(h.dataProvider(), req)
}

func (h *testGenHandler) generateTestFromInteraction(req TestFromContextRequest) (*GeneratedTest, error) {
	return testgen.GenerateTestFromInteraction(h.dataProvider(), req)
}

func (h *testGenHandler) generateTestFromRegression(req TestFromContextRequest) (*GeneratedTest, error) {
	return testgen.GenerateTestFromRegression(h.dataProvider(), req)
}

func (h *testGenHandler) analyzeTestFile(req TestHealRequest, projectDir string) ([]string, error) {
	return testgen.AnalyzeTestFile(req, projectDir)
}

func (h *testGenHandler) repairSelectors(req TestHealRequest, _ string) (*HealResult, error) {
	return testgen.RepairSelectors(req)
}

func (h *testGenHandler) healTestBatch(req TestHealRequest, projectDir string) (*BatchHealResult, error) {
	return testgen.HealTestBatch(req, projectDir)
}

func (h *testGenHandler) classifyFailure(failure *TestFailure) *FailureClassification {
	return testgen.ClassifyFailure(failure)
}

func (h *testGenHandler) classifyFailureBatch(failures []TestFailure) *BatchClassifyResult {
	return testgen.ClassifyFailureBatch(failures)
}
