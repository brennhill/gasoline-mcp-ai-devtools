// tools_errors_test.go — Tests for structured error retryable field and retry_after_ms.
package main

import (
	"testing"
)

// ============================================
// Retryable Error Field Tests
// ============================================

func TestStructuredError_RetryableErrors_SerializeCorrectly(t *testing.T) {
	t.Parallel()

	result := mcpStructuredError(
		ErrExtTimeout, "Extension timed out", "Retry the command",
		withRetryable(true), withRetryAfterMs(1000),
	)

	se := extractStructuredErrorJSON(t, result)

	retryable, ok := se["retryable"].(bool)
	if !ok {
		t.Fatal("retryable field missing or not a bool")
	}
	if !retryable {
		t.Error("retryable should be true for ErrExtTimeout")
	}

	retryAfterMs, ok := se["retry_after_ms"].(float64)
	if !ok {
		t.Fatal("retry_after_ms field missing or not a number")
	}
	if retryAfterMs != 1000 {
		t.Errorf("retry_after_ms = %v, want 1000", retryAfterMs)
	}
}

func TestStructuredError_NonRetryableErrors_OmitRetryAfterMs(t *testing.T) {
	t.Parallel()

	result := mcpStructuredError(
		ErrInvalidParam, "Bad parameter", "Fix the parameter",
		withRetryable(false),
	)

	se := extractStructuredErrorJSON(t, result)

	retryable, ok := se["retryable"].(bool)
	if !ok {
		t.Fatal("retryable field missing or not a bool")
	}
	if retryable {
		t.Error("retryable should be false for ErrInvalidParam")
	}

	if _, exists := se["retry_after_ms"]; exists {
		t.Error("retry_after_ms should not be present for non-retryable errors")
	}
}

func TestStructuredError_DefaultRetryable_IsFalse(t *testing.T) {
	t.Parallel()

	// No withRetryable option — should default to false
	result := mcpStructuredError(
		ErrInternal, "Internal error", "Do not retry",
	)

	se := extractStructuredErrorJSON(t, result)

	// retryable should still be present (zero value = false)
	retryable, ok := se["retryable"].(bool)
	if !ok {
		t.Fatal("retryable field missing or not a bool")
	}
	if retryable {
		t.Error("retryable should default to false")
	}
}

func TestStructuredError_ErrorCodes_RetryableDefaults(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		code      string
		retryable bool
		retryMs   int
	}{
		{ErrExtTimeout, true, 1000},
		{ErrExtError, true, 2000},
		{ErrRateLimited, true, 1000},
		{ErrInvalidParam, false, 0},
		{ErrMissingParam, false, 0},
		{ErrInternal, false, 0},
		{ErrUnknownMode, false, 0},
		{ErrNoData, true, 2000},
	}

	for _, tc := range testCases {
		t.Run(tc.code, func(t *testing.T) {
			opts := retryDefaultsForCode(tc.code)
			result := mcpStructuredError(tc.code, "test", "test", opts...)

			se := extractStructuredErrorJSON(t, result)

			retryable, _ := se["retryable"].(bool)
			if retryable != tc.retryable {
				t.Errorf("code %s: retryable = %v, want %v", tc.code, retryable, tc.retryable)
			}

			if tc.retryMs > 0 {
				retryAfterMs, ok := se["retry_after_ms"].(float64)
				if !ok {
					t.Errorf("code %s: retry_after_ms missing", tc.code)
				} else if int(retryAfterMs) != tc.retryMs {
					t.Errorf("code %s: retry_after_ms = %v, want %v", tc.code, retryAfterMs, tc.retryMs)
				}
			} else {
				if _, exists := se["retry_after_ms"]; exists {
					t.Errorf("code %s: retry_after_ms should not be present", tc.code)
				}
			}
		})
	}
}

// Helpers: extractStructuredErrorJSON and extractJSONFromText are in tools_test_helpers_test.go.
