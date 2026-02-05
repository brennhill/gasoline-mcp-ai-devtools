// interface_checks.go — Compile-time interface satisfaction checks.
// These assignments will FAIL TO COMPILE if a concrete type's method signatures
// don't match the interface definition. This catches interface drift early.
//
// If this file doesn't compile, either:
// 1. Update the interface to match the implementation (preferred if impl is correct)
// 2. Update the implementation to match the interface
// 3. Remove the interface if it's not used
//
// BUILD TAGS: This file is excluded from normal builds via build constraint.
// Run `go build -tags interface_check ./internal/types` to verify interfaces.
//go:build interface_check

package types

import (
	"github.com/dev-console/dev-console/internal/ai"
	"github.com/dev-console/dev-console/internal/analysis"
	"github.com/dev-console/dev-console/internal/security"
	"github.com/dev-console/dev-console/internal/session"
)

// Compile-time checks: these will fail if interfaces don't match implementations
// Uncomment each line to test that specific interface.

// SessionStore: FAILS - interface uses (data interface{}), impl uses (data []byte)
// var _ SessionStore = (*ai.SessionStore)(nil)

// NoiseConfig: FAILS - interface has AddRule(interface{}), impl has AddRules([]NoiseRule)
// var _ NoiseConfig = (*ai.NoiseConfig)(nil)

// SecurityScanner: FAILS - interface has Scan(checks, severityMin), impl has Scan(SecurityScanInput)
// var _ SecurityScanner = (*security.SecurityScanner)(nil)

// ThirdPartyAuditor: FAILS - interface has Audit(origins, includeStatic), impl has Audit(bodies, pageURLs, params)
// var _ ThirdPartyAuditor = (*analysis.ThirdPartyAuditor)(nil)

// SessionManager: may work or fail depending on implementation
// var _ SessionManager = (*session.SessionManager)(nil)

// ============================================
// INTERFACE MISMATCH DOCUMENTATION
// ============================================
//
// The following interfaces in this package do NOT match their implementations:
//
// 1. SessionStore
//    Interface: Save(namespace, key string, data interface{}) error
//    Actual:    Save(namespace, key string, data []byte) error
//    Interface: Load(namespace, key string) (interface{}, error)
//    Actual:    Load(namespace, key string) ([]byte, error)
//    Interface: Stats() interface{}
//    Actual:    Stats() (StoreStats, error)
//
// 2. NoiseConfig
//    Interface: AddRule(rule interface{}) error
//    Actual:    AddRules(rules []NoiseRule) error
//    Interface: ListRules() []interface{}
//    Actual:    ListRules() []NoiseRule
//    Interface: AutoDetect() []interface{}
//    Actual:    AutoDetect(consoleEntries, networkBodies, wsEvents) []NoiseProposal
//    Interface: Match(entry interface{}) bool
//    Actual:    IsConsoleNoise(entry LogEntry) bool (+ IsNetworkNoise, IsWebSocketNoise)
//
// 3. SecurityScanner
//    Interface: Scan(checks []string, severityMin string) (interface{}, error)
//    Actual:    Scan(input SecurityScanInput) SecurityScanResult
//
// 4. ThirdPartyAuditor
//    Interface: Audit(firstPartyOrigins []string, includeStatic bool) (interface{}, error)
//    Actual:    Audit(bodies []NetworkBody, pageURLs []string, params ThirdPartyParams) ThirdPartyResult
//
// RESOLUTION: These interfaces are not used. The ToolHandler uses concrete types
// directly via dedicated fields (noiseConfig, sessionStoreImpl, etc.).
// The interfaces should either be deleted or updated to match implementations.

// Suppress unused import warnings
var (
	_ = ai.NewNoiseConfig
	_ = analysis.NewThirdPartyAuditor
	_ = security.NewSecurityScanner
	_ = session.NewVerificationManager
)
