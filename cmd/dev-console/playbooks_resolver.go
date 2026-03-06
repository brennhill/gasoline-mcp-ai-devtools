// Purpose: Resolves playbook/demo resource URIs to canonical URIs and markdown content.
// Why: Isolates URI parsing and alias normalization from large static documentation payloads.

package main

import "strings"

// canonicalPlaybookCapability normalizes capability aliases to canonical playbook keys.
func canonicalPlaybookCapability(capability string) string {
	switch strings.ToLower(strings.TrimSpace(capability)) {
	case "performance", "performance_analysis":
		return "performance"
	case "accessibility", "accessibility_audit":
		return "accessibility"
	case "security", "security_audit":
		return "security"
	case "automation", "browser_automation", "interact":
		return "automation"
	default:
		return ""
	}
}

// resolvePlaybookKey resolves "{capability}/{level}" and bare "{capability}" to canonical keys.
func resolvePlaybookKey(raw string) string {
	trimmed := strings.Trim(strings.ToLower(strings.TrimSpace(raw)), "/")
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	switch len(parts) {
	case 1:
		capability := canonicalPlaybookCapability(parts[0])
		if capability == "" {
			return ""
		}
		return capability + "/quick"
	case 2:
		capability := canonicalPlaybookCapability(parts[0])
		level := strings.TrimSpace(parts[1])
		if capability == "" || level == "" {
			return ""
		}
		return capability + "/" + level
	default:
		return ""
	}
}

// resolveResourceContent resolves a gasoline resource URI into canonical URI + markdown.
func resolveResourceContent(uri string) (string, string, bool) {
	switch {
	case uri == "gasoline://capabilities":
		return uri, capabilityIndex, true
	case uri == "gasoline://guide":
		return uri, guideContent, true
	case uri == "gasoline://quickstart":
		return uri, quickstartContent, true
	case strings.HasPrefix(uri, "gasoline://playbook/"):
		key := resolvePlaybookKey(strings.TrimPrefix(uri, "gasoline://playbook/"))
		text, ok := playbooks[key]
		if !ok {
			return "", "", false
		}
		return "gasoline://playbook/" + key, text, true
	case strings.HasPrefix(uri, "gasoline://demo/"):
		name := strings.TrimPrefix(uri, "gasoline://demo/")
		text, ok := demoScripts[name]
		if !ok {
			return "", "", false
		}
		return uri, text, true
	default:
		return "", "", false
	}
}
