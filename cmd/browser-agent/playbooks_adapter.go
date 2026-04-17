// playbooks_adapter.go — Bridges the playbooks sub-package into the main package namespace.
// Why: Allows callers in the main package to use playbook functions without qualifying every call.

package main

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/playbooks"

// Re-export package-level vars for main-package callers.
var (
	capabilityIndex   = playbooks.CapabilityIndex
	playbookMap       = playbooks.Playbooks
	guideContent      = playbooks.GuideContent
	quickstartContent = playbooks.QuickstartContent
	demoScripts       = playbooks.DemoScripts
)

// interactFailurePlaybook is a type alias for the sub-package type.
type interactFailurePlaybook = playbooks.InteractFailurePlaybook

// resolveResourceContent delegates to the playbooks sub-package.
func resolveResourceContent(uri string) (string, string, bool) {
	return playbooks.ResolveResourceContent(uri)
}

// lookupInteractFailurePlaybook delegates to the playbooks sub-package.
func lookupInteractFailurePlaybook(rawCode string) (string, interactFailurePlaybook, bool) {
	return playbooks.LookupInteractFailurePlaybook(rawCode)
}

// normalizeInteractFailureCode delegates to the playbooks sub-package.
func normalizeInteractFailureCode(raw string) string {
	return playbooks.NormalizeInteractFailureCode(raw)
}

// tutorialFailureRecoveryPlaybooks delegates to the playbooks sub-package.
func tutorialFailureRecoveryPlaybooks() map[string]any {
	return playbooks.TutorialFailureRecoveryPlaybooks()
}
