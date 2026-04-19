// page_issues_summary_test.go — Unit tests for page-issue summary helpers.

package toolanalyze

import "testing"

func TestExtractIssueMessage_PriorityOrder(t *testing.T) {
	t.Parallel()

	// message wins over title/description/rule/url.
	if got := extractIssueMessage(map[string]any{
		"message": "msg", "title": "ttl", "url": "/foo",
	}); got != "msg" {
		t.Errorf("message priority: got %q, want %q", got, "msg")
	}

	// title wins when message absent.
	if got := extractIssueMessage(map[string]any{
		"title": "ttl", "url": "/foo",
	}); got != "ttl" {
		t.Errorf("title priority: got %q, want %q", got, "ttl")
	}

	// description wins when message and title absent.
	if got := extractIssueMessage(map[string]any{
		"description": "desc", "rule": "r", "url": "/foo",
	}); got != "desc" {
		t.Errorf("description priority: got %q, want %q", got, "desc")
	}

	// rule wins when message, title, description absent.
	if got := extractIssueMessage(map[string]any{
		"rule": "r", "url": "/foo",
	}); got != "r" {
		t.Errorf("rule priority: got %q, want %q", got, "r")
	}

	// url is the final fallback.
	if got := extractIssueMessage(map[string]any{
		"url": "/foo",
	}); got != "/foo" {
		t.Errorf("url fallback: got %q, want %q", got, "/foo")
	}

	// Empty map returns empty string.
	if got := extractIssueMessage(map[string]any{}); got != "" {
		t.Errorf("empty map: got %q, want empty", got)
	}

	// Non-string values are skipped (falls back to later keys).
	if got := extractIssueMessage(map[string]any{
		"message": 42, "title": "ttl",
	}); got != "ttl" {
		t.Errorf("non-string skipped: got %q, want %q", got, "ttl")
	}
}
