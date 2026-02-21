package main

import "testing"

func TestDaemonProcessArgv0ForVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		exePath  string
		version  string
		expected string
	}{
		{
			name:     "semver appends compact digits",
			exePath:  "/usr/local/bin/gasoline-mcp",
			version:  "0.7.6",
			expected: "gasoline-mcp-076",
		},
		{
			name:     "v-prefixed semver supported",
			exePath:  "/usr/local/bin/gasoline-mcp",
			version:  "v1.2.3",
			expected: "gasoline-mcp-123",
		},
		{
			name:     "pre release keeps major minor patch only",
			exePath:  "/usr/local/bin/gasoline-mcp",
			version:  "0.7.6-beta.1",
			expected: "gasoline-mcp-076",
		},
		{
			name:     "empty version leaves basename",
			exePath:  "/usr/local/bin/gasoline-mcp",
			version:  "",
			expected: "gasoline-mcp",
		},
		{
			name:     "missing executable falls back to default",
			exePath:  "",
			version:  "0.7.6",
			expected: "gasoline-mcp-076",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := daemonProcessArgv0ForVersion(tc.exePath, tc.version)
			if got != tc.expected {
				t.Fatalf("daemonProcessArgv0ForVersion(%q, %q) = %q, want %q", tc.exePath, tc.version, got, tc.expected)
			}
		})
	}
}

func TestCompactVersionTag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{input: "0.7.6", expected: "076"},
		{input: "v0.7.6", expected: "076"},
		{input: "0.7.6-beta.1", expected: "076"},
		{input: "release-2026-02-21", expected: "20260221"},
		{input: "", expected: ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got := compactVersionTag(tc.input)
			if got != tc.expected {
				t.Fatalf("compactVersionTag(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}
