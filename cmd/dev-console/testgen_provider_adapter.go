// Purpose: Adapts ToolHandler state to the internal testgen DataProvider API.
// Why: Isolates data access and wrapper delegation from request parsing/response formatting.
// Docs: docs/features/feature/test-generation/index.md

package main

import (
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/testgen"
)

// toolHandlerDataProvider adapts *ToolHandler to testgen.DataProvider.
type toolHandlerDataProvider struct {
	h *ToolHandler
}

func (a *toolHandlerDataProvider) GetLogEntries() []map[string]any {
	a.h.server.mu.RLock()
	entries := make([]LogEntry, len(a.h.server.entries))
	copy(entries, a.h.server.entries)
	a.h.server.mu.RUnlock()
	return entries
}

func (a *toolHandlerDataProvider) GetAllEnhancedActions() []capture.EnhancedAction {
	return a.h.capture.GetAllEnhancedActions()
}

func (a *toolHandlerDataProvider) GetNetworkBodies() []capture.NetworkBody {
	return a.h.capture.GetNetworkBodies()
}

// dataProvider returns a testgen.DataProvider backed by this ToolHandler.
func (h *ToolHandler) dataProvider() testgen.DataProvider {
	return &toolHandlerDataProvider{h: h}
}

func (h *ToolHandler) findTargetError(errorID string) (LogEntry, string, int64) {
	return testgen.FindTargetError(h.dataProvider(), errorID)
}

func (h *ToolHandler) getActionsInTimeWindow(centerTimestamp int64, windowMs int64) ([]capture.EnhancedAction, error) {
	return testgen.GetActionsInTimeWindow(h.dataProvider(), centerTimestamp, windowMs)
}

func (h *ToolHandler) countNetworkAssertions() int {
	return testgen.CountNetworkAssertions(h.dataProvider())
}

func (h *ToolHandler) collectErrorMessages(limit int) []string {
	return testgen.CollectErrorMessages(h.dataProvider(), limit)
}

func (h *ToolHandler) generateTestFromError(req TestFromContextRequest) (*GeneratedTest, error) {
	return testgen.GenerateTestFromError(h.dataProvider(), req)
}

func (h *ToolHandler) generateTestFromInteraction(req TestFromContextRequest) (*GeneratedTest, error) {
	return testgen.GenerateTestFromInteraction(h.dataProvider(), req)
}

func (h *ToolHandler) generateTestFromRegression(req TestFromContextRequest) (*GeneratedTest, error) {
	return testgen.GenerateTestFromRegression(h.dataProvider(), req)
}

func (h *ToolHandler) analyzeTestFile(req TestHealRequest, projectDir string) ([]string, error) {
	return testgen.AnalyzeTestFile(req, projectDir)
}

func (h *ToolHandler) repairSelectors(req TestHealRequest, _ string) (*HealResult, error) {
	return testgen.RepairSelectors(req)
}

func (h *ToolHandler) healSelector(oldSelector string) (*HealedSelector, error) {
	return testgen.HealSelector(oldSelector)
}

func (h *ToolHandler) healTestBatch(req TestHealRequest, projectDir string) (*BatchHealResult, error) {
	return testgen.HealTestBatch(req, projectDir)
}

func (h *ToolHandler) classifyFailure(failure *TestFailure) *FailureClassification {
	return testgen.ClassifyFailure(failure)
}

func (h *ToolHandler) classifyFailureBatch(failures []TestFailure) *BatchClassifyResult {
	return testgen.ClassifyFailureBatch(failures)
}
