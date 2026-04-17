// Purpose: Implements aggregate security scanning across captured network/log evidence.
// Why: Centralizes security checks so risk findings are produced with one coherent severity model.
// Docs: docs/features/feature/security-hardening/index.md

package security

import (
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

type SecurityFinding struct {
	Check       string `json:"check"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Location    string `json:"location"`
	Evidence    string `json:"evidence"`
	Remediation string `json:"remediation"`
}

type LogEntry = types.LogEntry

type SecurityScanInput struct {
	NetworkBodies    []capture.NetworkBody
	WaterfallEntries []capture.NetworkWaterfallEntry
	ConsoleEntries   []LogEntry
	PageURLs         []string
	URLFilter        string
	Checks           []string
	SeverityMin      string
}

type SecurityScanResult struct {
	Findings  []SecurityFinding `json:"findings"`
	Summary   ScanSummary       `json:"summary"`
	ScannedAt time.Time         `json:"scanned_at"`
}

type ScanSummary struct {
	TotalFindings int            `json:"total_findings"`
	BySeverity    map[string]int `json:"by_severity"`
	ByCheck       map[string]int `json:"by_check"`
	URLsScanned   int            `json:"urls_scanned"`
}

type SecurityScanner struct {
	mu sync.RWMutex
}

var defaultSecurityChecks = []string{"credentials", "pii", "headers", "cookies", "transport", "auth", "network"}

func NewSecurityScanner() *SecurityScanner {
	return &SecurityScanner{}
}
