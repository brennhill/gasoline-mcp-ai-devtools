package main

import (
	"strings"
	"testing"
)

func TestQuickstartContent_IncludesInteractFailureRecoveryExamples(t *testing.T) {
	t.Parallel()

	uri, text, ok := resolveResourceContent("gasoline://quickstart")
	if !ok {
		t.Fatal("resolveResourceContent(gasoline://quickstart) should succeed")
	}
	if uri != "gasoline://quickstart" {
		t.Fatalf("canonical uri = %q, want gasoline://quickstart", uri)
	}

	requiredTokens := []string{
		"element_not_found",
		"ambiguous_target",
		"stale_element_id",
		"scope_not_found",
		"Stop and report evidence",
	}
	for _, token := range requiredTokens {
		if !strings.Contains(strings.ToLower(text), strings.ToLower(token)) {
			t.Fatalf("quickstart missing token %q", token)
		}
	}
}
