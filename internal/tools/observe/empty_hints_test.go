// empty_hints_test.go — Unit tests for diagnostic hint builders.
package observe

import (
	"strings"
	"testing"
)

// ============================================
// networkBodiesEmptyHint
// ============================================

func TestNetworkBodiesEmptyHint_FilterMismatch(t *testing.T) {
	t.Parallel()
	hint := networkBodiesEmptyHint(0, 5, NetworkBodiesHintFilters{URL: "github.com"})

	if !strings.Contains(hint, "github.com") {
		t.Errorf("hint should mention the URL filter, got: %s", hint)
	}
	if !strings.Contains(hint, "5 bodies exist in the buffer") {
		t.Errorf("hint should mention unfiltered count with 'in the buffer' wording, got: %s", hint)
	}
}

func TestNetworkBodiesEmptyHint_WaterfallOnly(t *testing.T) {
	t.Parallel()
	hint := networkBodiesEmptyHint(10, 0, NetworkBodiesHintFilters{})

	if !strings.Contains(hint, "waterfall") {
		t.Errorf("hint should mention waterfall, got: %s", hint)
	}
	if !strings.Contains(hint, "10 requests") {
		t.Errorf("hint should mention waterfall count, got: %s", hint)
	}
	if !strings.Contains(hint, "after") {
		t.Errorf("hint should explain prospective-only capture, got: %s", hint)
	}
}

func TestNetworkBodiesEmptyHint_NothingCaptured(t *testing.T) {
	t.Parallel()
	hint := networkBodiesEmptyHint(0, 0, NetworkBodiesHintFilters{})

	if !strings.Contains(hint, "pilot") {
		t.Errorf("hint should suggest checking pilot status, got: %s", hint)
	}
	if !strings.Contains(hint, "No network bodies captured") {
		t.Errorf("hint should state no bodies captured, got: %s", hint)
	}
}

func TestNetworkBodiesEmptyHint_FilterWithZeroUnfiltered(t *testing.T) {
	t.Parallel()
	// URL filter present but unfilteredCount is 0 — should fall through to case 2/3
	hint := networkBodiesEmptyHint(0, 0, NetworkBodiesHintFilters{URL: "github.com"})

	// Should NOT mention the filter (case 1 requires unfilteredCount > 0)
	if strings.Contains(hint, "filter") {
		t.Errorf("hint should not mention filter when unfiltered count is 0, got: %s", hint)
	}
}

func TestNetworkBodiesEmptyHint_FilterMismatch_MultipleFilters(t *testing.T) {
	t.Parallel()
	hint := networkBodiesEmptyHint(0, 3, NetworkBodiesHintFilters{
		URL:       "api.example.com",
		Method:    "post",
		StatusMin: 400,
		StatusMax: 499,
		BodyPath:  "data.items[0].id",
	})

	for _, expected := range []string{
		`url~"api.example.com"`,
		"method=POST",
		"status=400..499",
		"body_path=data.items[0].id",
	} {
		if !strings.Contains(hint, expected) {
			t.Errorf("hint should mention %q, got: %s", expected, hint)
		}
	}
}

// ============================================
// wsEventsEmptyHint
// ============================================

func TestWSEventsEmptyHint_FilterMismatch(t *testing.T) {
	t.Parallel()
	hint := wsEventsEmptyHint(8, "stream.example.com")

	if !strings.Contains(hint, "stream.example.com") {
		t.Errorf("hint should mention the URL filter, got: %s", hint)
	}
	if !strings.Contains(hint, "8 events exist") {
		t.Errorf("hint should mention unfiltered count, got: %s", hint)
	}
}

func TestWSEventsEmptyHint_NoEvents(t *testing.T) {
	t.Parallel()
	hint := wsEventsEmptyHint(0, "")

	if !strings.Contains(strings.ToLower(hint), "websocket") {
		t.Errorf("hint should mention WebSocket, got: %s", hint)
	}
	if !strings.Contains(hint, "before connections open") {
		t.Errorf("hint should explain prospective interception, got: %s", hint)
	}
}

// ============================================
// wsStatusEmptyHint
// ============================================

func TestWSStatusEmptyHint_Content(t *testing.T) {
	t.Parallel()
	hint := wsStatusEmptyHint()

	if !strings.Contains(strings.ToLower(hint), "websocket") {
		t.Errorf("hint should mention WebSocket, got: %s", hint)
	}
	if !strings.Contains(hint, "before connections open") {
		t.Errorf("hint should explain prospective interception, got: %s", hint)
	}
	if hint == "" {
		t.Fatal("hint should not be empty")
	}
}

// ============================================
// errorsEmptyHint
// ============================================

func TestErrorsEmptyHint_CurrentPage(t *testing.T) {
	t.Parallel()
	hint := errorsEmptyHint("current_page")
	if !strings.Contains(hint, "current page") {
		t.Errorf("hint should mention current page scope, got: %s", hint)
	}
	if !strings.Contains(hint, "scope") {
		t.Errorf("hint should suggest scope:all, got: %s", hint)
	}
}

func TestErrorsEmptyHint_All(t *testing.T) {
	t.Parallel()
	hint := errorsEmptyHint("all")
	if !strings.Contains(hint, "any tab") {
		t.Errorf("hint should mention all tabs, got: %s", hint)
	}
	if !strings.Contains(hint, "pilot") {
		t.Errorf("hint should suggest checking pilot, got: %s", hint)
	}
}

// ============================================
// logsEmptyHint
// ============================================

func TestLogsEmptyHint_WithMinLevel(t *testing.T) {
	t.Parallel()
	hint := logsEmptyHint("current_page", "error")
	if !strings.Contains(hint, "error") {
		t.Errorf("hint should mention the min_level filter, got: %s", hint)
	}
	if !strings.Contains(hint, "debug") {
		t.Errorf("hint should suggest lowering threshold, got: %s", hint)
	}
}

func TestLogsEmptyHint_CurrentPage(t *testing.T) {
	t.Parallel()
	hint := logsEmptyHint("current_page", "")
	if !strings.Contains(hint, "current page") {
		t.Errorf("hint should mention current page scope, got: %s", hint)
	}
}

func TestLogsEmptyHint_All(t *testing.T) {
	t.Parallel()
	hint := logsEmptyHint("all", "")
	if !strings.Contains(hint, "pilot") {
		t.Errorf("hint should suggest checking pilot, got: %s", hint)
	}
}

// ============================================
// actionsEmptyHint
// ============================================

func TestActionsEmptyHint_Content(t *testing.T) {
	t.Parallel()
	hint := actionsEmptyHint()
	if hint == "" {
		t.Fatal("hint should not be empty")
	}
	if !strings.Contains(hint, "actions") || !strings.Contains(hint, "clicks") {
		t.Errorf("hint should describe what actions are, got: %s", hint)
	}
}

// ============================================
// timelineEmptyHint
// ============================================

func TestTimelineEmptyHint_Content(t *testing.T) {
	t.Parallel()
	hint := timelineEmptyHint()
	if hint == "" {
		t.Fatal("hint should not be empty")
	}
	if !strings.Contains(hint, "timeline") {
		t.Errorf("hint should mention timeline, got: %s", hint)
	}
}

// ============================================
// errorBundlesEmptyHint
// ============================================

func TestErrorBundlesEmptyHint_Content(t *testing.T) {
	t.Parallel()
	hint := errorBundlesEmptyHint()
	if hint == "" {
		t.Fatal("hint should not be empty")
	}
	if !strings.Contains(hint, "errors") {
		t.Errorf("hint should mention errors, got: %s", hint)
	}
}

// ============================================
// transientsEmptyHint
// ============================================

func TestTransientsEmptyHint_WithClassification(t *testing.T) {
	t.Parallel()
	hint := transientsEmptyHint("toast")
	if !strings.Contains(hint, "toast") {
		t.Errorf("hint should mention the classification filter, got: %s", hint)
	}
}

func TestTransientsEmptyHint_NoFilter(t *testing.T) {
	t.Parallel()
	hint := transientsEmptyHint("")
	if !strings.Contains(hint, "transient") || !strings.Contains(hint, "toasts") {
		t.Errorf("hint should describe transients, got: %s", hint)
	}
}

// ============================================
// networkWaterfallEmptyHint
// ============================================

func TestNetworkWaterfallEmptyHint_WithURL(t *testing.T) {
	t.Parallel()
	hint := networkWaterfallEmptyHint("api.example.com")
	if !strings.Contains(hint, "api.example.com") {
		t.Errorf("hint should mention the URL filter, got: %s", hint)
	}
}

func TestNetworkWaterfallEmptyHint_NoFilter(t *testing.T) {
	t.Parallel()
	hint := networkWaterfallEmptyHint("")
	if !strings.Contains(hint, "Performance API") {
		t.Errorf("hint should explain capture source, got: %s", hint)
	}
}
