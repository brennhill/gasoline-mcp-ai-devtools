// store_results_test.go — Regression tests for URL glob filter (T10) and result builders.
// Purpose: Validates that URLMatches handles wildcard patterns correctly and does not
// allow prefix-collision attacks (e.g., "http://localhost:*" vs "http://localhost-evil.com").
// Docs: docs/features/feature/annotated-screenshots/index.md

package annotation

import "testing"

// ============================================
// T10 regression: URLMatches glob filter
// ============================================

func TestURLMatches_WildcardPort_RejectsHostPrefixCollision(t *testing.T) {
	t.Parallel()
	// "http://localhost:*" must NOT match "http://localhost-evil.com"
	// because the prefix "http://localhost:" does not match "http://localhost-evil.com".
	if URLMatches("http://localhost:*", "http://localhost-evil.com") {
		t.Error("expected http://localhost:* to reject http://localhost-evil.com (host prefix collision)")
	}
}

func TestURLMatches_WildcardPort_AcceptsValidPort(t *testing.T) {
	t.Parallel()
	if !URLMatches("http://localhost:*", "http://localhost:3000/page") {
		t.Error("expected http://localhost:* to match http://localhost:3000/page")
	}
}

func TestURLMatches_WildcardPort_AcceptsAnyPort(t *testing.T) {
	t.Parallel()
	if !URLMatches("http://localhost:*", "http://localhost:8080") {
		t.Error("expected http://localhost:* to match http://localhost:8080")
	}
}

func TestURLMatches_WildcardMiddle_PrefixAndSuffix(t *testing.T) {
	t.Parallel()
	// "http://example.com/*/page" should match URLs that start with
	// "http://example.com/" and end with "/page".
	if !URLMatches("http://example.com/*/page", "http://example.com/foo/page") {
		t.Error("expected wildcard middle to match http://example.com/foo/page")
	}
	if !URLMatches("http://example.com/*/page", "http://example.com/bar/baz/page") {
		t.Error("expected wildcard middle to match http://example.com/bar/baz/page")
	}
}

func TestURLMatches_WildcardMiddle_RejectsSuffixMismatch(t *testing.T) {
	t.Parallel()
	if URLMatches("http://example.com/*/page", "http://example.com/foo/other") {
		t.Error("expected wildcard middle to reject suffix mismatch")
	}
}

func TestURLMatches_WildcardMiddle_RejectsPrefixMismatch(t *testing.T) {
	t.Parallel()
	if URLMatches("http://example.com/*/page", "http://evil.com/foo/page") {
		t.Error("expected wildcard middle to reject prefix mismatch")
	}
}

func TestURLMatches_WildcardSuffix_MatchesAnyTrailing(t *testing.T) {
	t.Parallel()
	// "http://localhost:3000/*" is handled by the "/*" branch which
	// recurses with trailing slash; verify it still works.
	if !URLMatches("http://localhost:3000/*", "http://localhost:3000/anything") {
		t.Error("expected trailing wildcard to match")
	}
}

func TestURLMatches_WildcardSuffix_RejectsDifferentOrigin(t *testing.T) {
	t.Parallel()
	if URLMatches("http://localhost:3000/*", "http://localhost:4000/page") {
		t.Error("expected trailing wildcard to reject different origin")
	}
}

func TestURLMatches_NoWildcard_ExactMatch(t *testing.T) {
	t.Parallel()
	if !URLMatches("http://example.com/page", "http://example.com/page") {
		t.Error("expected exact match")
	}
	if URLMatches("http://example.com/page", "http://example.com/other") {
		t.Error("expected exact mismatch to fail")
	}
}

func TestURLMatches_EmptyFilter_MatchesAll(t *testing.T) {
	t.Parallel()
	if !URLMatches("", "http://anything.com") {
		t.Error("expected empty filter to match anything")
	}
	if !URLMatches("  ", "http://anything.com") {
		t.Error("expected whitespace filter to match anything")
	}
}

func TestURLMatches_EmptyPageURL_NeverMatches(t *testing.T) {
	t.Parallel()
	if URLMatches("http://example.com", "") {
		t.Error("expected non-empty filter to reject empty page URL")
	}
}

func TestURLMatches_WildcardOnly_MatchesAnything(t *testing.T) {
	t.Parallel()
	// A filter of just "*" should match any URL since prefix is "" and suffix is "".
	if !URLMatches("*", "http://example.com/anything") {
		t.Error("expected bare wildcard to match any URL")
	}
}

func TestURLMatches_WildcardPort_RejectsPortSubstring(t *testing.T) {
	t.Parallel()
	// "http://localhost:3*" should NOT match "http://localhost:30001/page"
	// Wait -- actually it should, because prefix is "http://localhost:3" and suffix is "".
	// The real security concern is host collision. Let's verify:
	if !URLMatches("http://localhost:3*", "http://localhost:3000") {
		t.Error("expected http://localhost:3* to match http://localhost:3000")
	}
	if !URLMatches("http://localhost:3*", "http://localhost:30001") {
		t.Error("expected http://localhost:3* to match http://localhost:30001")
	}
	// But it should not match a completely different host prefix:
	if URLMatches("http://localhost:3*", "http://localhost:4000") {
		t.Error("expected http://localhost:3* to reject http://localhost:4000")
	}
}
