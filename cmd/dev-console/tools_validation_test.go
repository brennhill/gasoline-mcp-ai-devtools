// tools_validation_test.go — Unit tests for validation helpers.
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCheckLogQuality(t *testing.T) {
	t.Parallel()

	t.Run("all good entries", func(t *testing.T) {
		entries := []LogEntry{
			{"ts": "2026-02-11T10:00:00Z", "message": "hello", "source": "console"},
			{"ts": "2026-02-11T10:00:01Z", "message": "world", "source": "network"},
		}
		if got := checkLogQuality(entries); got != "" {
			t.Fatalf("expected empty warning for good entries, got %q", got)
		}
	})

	t.Run("missing ts", func(t *testing.T) {
		entries := []LogEntry{
			{"message": "no ts", "source": "test"},
		}
		got := checkLogQuality(entries)
		if !strings.Contains(got, "missing 'ts'") {
			t.Fatalf("expected ts warning, got %q", got)
		}
	})

	t.Run("missing message", func(t *testing.T) {
		entries := []LogEntry{
			{"ts": "2026-02-11T10:00:00Z", "source": "test"},
		}
		got := checkLogQuality(entries)
		if !strings.Contains(got, "missing 'message'") {
			t.Fatalf("expected message warning, got %q", got)
		}
	})

	t.Run("missing source", func(t *testing.T) {
		entries := []LogEntry{
			{"ts": "2026-02-11T10:00:00Z", "message": "hi"},
		}
		got := checkLogQuality(entries)
		if !strings.Contains(got, "missing 'source'") {
			t.Fatalf("expected source warning, got %q", got)
		}
	})

	t.Run("mixed good and bad", func(t *testing.T) {
		entries := []LogEntry{
			{"ts": "2026-02-11T10:00:00Z", "message": "ok", "source": "a"},
			{"message": "no ts", "source": "b"},
			{"ts": "2026-02-11T10:00:02Z", "message": "ok2", "source": "c"},
		}
		got := checkLogQuality(entries)
		if !strings.Contains(got, "1/3") {
			t.Fatalf("expected 1/3 bad entries, got %q", got)
		}
	})

	t.Run("empty entries", func(t *testing.T) {
		if got := checkLogQuality(nil); got != "" {
			t.Fatalf("expected empty for nil, got %q", got)
		}
		if got := checkLogQuality([]LogEntry{}); got != "" {
			t.Fatalf("expected empty for empty slice, got %q", got)
		}
	})
}

func TestValidateLogEntry(t *testing.T) {
	t.Parallel()

	t.Run("valid entries", func(t *testing.T) {
		for _, level := range []string{"error", "warn", "info", "debug", "log"} {
			entry := LogEntry{"level": level, "msg": "test"}
			if !validateLogEntry(entry) {
				t.Errorf("validateLogEntry with level=%q should be valid", level)
			}
		}
	})

	t.Run("missing level", func(t *testing.T) {
		if validateLogEntry(LogEntry{"msg": "no level"}) {
			t.Fatal("missing level should be invalid")
		}
	})

	t.Run("unknown level", func(t *testing.T) {
		if validateLogEntry(LogEntry{"level": "critical"}) {
			t.Fatal("unknown level should be invalid")
		}
	})

	t.Run("non-string level", func(t *testing.T) {
		if validateLogEntry(LogEntry{"level": 42}) {
			t.Fatal("non-string level should be invalid")
		}
	})

	t.Run("oversized entry rejected", func(t *testing.T) {
		huge := strings.Repeat("x", maxEntrySize)
		entry := LogEntry{"level": "info", "data": huge}
		if validateLogEntry(entry) {
			t.Fatal("oversized entry should be invalid")
		}
	})

	t.Run("small entry fast path", func(t *testing.T) {
		entry := LogEntry{"level": "info", "msg": "small"}
		if !validateLogEntry(entry) {
			t.Fatal("small entry should be valid")
		}
	})
}

func TestValidateLogEntries(t *testing.T) {
	t.Parallel()

	entries := []LogEntry{
		{"level": "info", "msg": "ok"},
		{"level": "bad_level"},
		{"level": "error", "msg": "also ok"},
		{"msg": "no level"},
	}

	valid, rejected := validateLogEntries(entries)
	if len(valid) != 2 {
		t.Fatalf("valid count = %d, want 2", len(valid))
	}
	if rejected != 2 {
		t.Fatalf("rejected count = %d, want 2", rejected)
	}
}

func TestGetJSONFieldNames(t *testing.T) {
	t.Parallel()

	type sample struct {
		Name    string `json:"name"`
		Age     int    `json:"age,omitempty"`
		Ignored string `json:"-"`
		NoTag   string
	}

	names := getJSONFieldNames(sample{})
	if !names["name"] || !names["age"] || !names["NoTag"] {
		t.Fatalf("expected name, age, NoTag; got %v", names)
	}
	if names["Ignored"] || names["-"] {
		t.Fatalf("Ignored field should not be in names; got %v", names)
	}
}

func TestUnmarshalWithWarnings(t *testing.T) {
	t.Parallel()

	type params struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	t.Run("no warnings for known fields", func(t *testing.T) {
		data := json.RawMessage(`{"name":"test","age":5}`)
		var p params
		warnings, err := unmarshalWithWarnings(data, &p)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(warnings) != 0 {
			t.Fatalf("expected no warnings, got %v", warnings)
		}
		if p.Name != "test" || p.Age != 5 {
			t.Fatalf("unexpected values: %+v", p)
		}
	})

	t.Run("warns on unknown fields", func(t *testing.T) {
		data := json.RawMessage(`{"name":"test","typo_field":"oops"}`)
		var p params
		warnings, err := unmarshalWithWarnings(data, &p)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(warnings) != 1 || !strings.Contains(warnings[0], "typo_field") {
			t.Fatalf("expected warning about typo_field, got %v", warnings)
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		data := json.RawMessage(`{bad}`)
		var p params
		_, err := unmarshalWithWarnings(data, &p)
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}
