package main

import (
	"path/filepath"
	"strings"
)

const defaultProcessBaseName = "gasoline-mcp"

// daemonProcessArgv0 returns a versioned process title for daemon child processes.
// Example: gasoline-mcp-076
func daemonProcessArgv0(exePath string) string {
	return daemonProcessArgv0ForVersion(exePath, version)
}

func daemonProcessArgv0ForVersion(exePath string, versionValue string) string {
	base := strings.TrimSpace(filepath.Base(exePath))
	if base == "" || base == "." || base == string(filepath.Separator) {
		base = defaultProcessBaseName
	}
	tag := compactVersionTag(versionValue)
	if tag == "" {
		return base
	}
	return base + "-" + tag
}

func compactVersionTag(versionValue string) string {
	v := strings.TrimSpace(versionValue)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	if v == "" {
		return ""
	}

	parts := strings.Split(v, ".")
	var b strings.Builder
	for i := 0; i < len(parts) && i < 3; i++ {
		digits := leadingDigits(parts[i])
		if digits == "" {
			break
		}
		b.WriteString(digits)
	}
	if b.Len() > 0 {
		return b.String()
	}

	// Fallback for non-semver strings.
	for _, r := range v {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func leadingDigits(value string) string {
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		if r < '0' || r > '9' {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}
