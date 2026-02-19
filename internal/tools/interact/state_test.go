// state_test.go — Tests for state management pure functions.
package interact

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================
// ParseCapturedStatePayload
// ============================================

func TestParseCapturedStatePayload_WithSuccessEnvelope(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{"success":true,"result":{"form_values":{"x":"1"},"local_storage":{"a":"b"}}}`)
	data, err := ParseCapturedStatePayload(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := data["form_values"]; !ok {
		t.Error("should have form_values")
	}
	if _, ok := data["local_storage"]; !ok {
		t.Error("should have local_storage")
	}
}

func TestParseCapturedStatePayload_DirectResult(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{"form_values":{},"local_storage":{"k":"v"}}`)
	data, err := ParseCapturedStatePayload(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := data["local_storage"]; !ok {
		t.Error("should have local_storage")
	}
}

func TestParseCapturedStatePayload_FailureEnvelope(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{"success":false,"error":"script_error","message":"access denied"}`)
	_, err := ParseCapturedStatePayload(raw)
	if err == nil {
		t.Fatal("should return error on failure envelope")
	}
	if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("error should contain message, got: %v", err)
	}
}

func TestParseCapturedStatePayload_Empty(t *testing.T) {
	t.Parallel()
	_, err := ParseCapturedStatePayload(json.RawMessage(``))
	if err == nil {
		t.Fatal("should return error on empty payload")
	}
}

func TestParseCapturedStatePayload_Null(t *testing.T) {
	t.Parallel()
	_, err := ParseCapturedStatePayload(json.RawMessage(`null`))
	if err == nil {
		t.Fatal("should return error on null payload")
	}
}

func TestParseCapturedStatePayload_MissingExpectedFields(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{"status":"ok","unrelated":"data"}`)
	_, err := ParseCapturedStatePayload(raw)
	if err == nil {
		t.Fatal("should return error when no expected fields present")
	}
}

func TestParseCapturedStatePayload_SuccessNoResult(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{"success":true}`)
	_, err := ParseCapturedStatePayload(raw)
	if err == nil {
		t.Fatal("should return error when success=true but no result object")
	}
}

func TestParseCapturedStatePayload_SuccessWithFormValuesAtTopLevel(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{"success":true,"form_values":{"x":"1"}}`)
	data, err := ParseCapturedStatePayload(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := data["form_values"]; !ok {
		t.Error("should have form_values")
	}
}

func TestParseCapturedStatePayload_FailureWithErrorCode(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{"success":false,"error":"timeout_error"}`)
	_, err := ParseCapturedStatePayload(raw)
	if err == nil {
		t.Fatal("should return error on failure")
	}
	if !strings.Contains(err.Error(), "timeout_error") {
		t.Errorf("error should contain error code, got: %v", err)
	}
}

func TestParseCapturedStatePayload_FailureNoMessage(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{"success":false}`)
	_, err := ParseCapturedStatePayload(raw)
	if err == nil {
		t.Fatal("should return error on failure with no message")
	}
	if !strings.Contains(err.Error(), "execute_js failed") {
		t.Errorf("error should be generic, got: %v", err)
	}
}

// ============================================
// BuildStateRestoreScript
// ============================================

func TestBuildStateRestoreScript_ContainsFormValues(t *testing.T) {
	t.Parallel()
	formValues := map[string]any{"email": "a@b.com", "name": "Test"}
	scrollPos := map[string]any{"x": 10.0, "y": 200.0}

	script := BuildStateRestoreScript(formValues, scrollPos, nil, nil, nil)

	if !strings.Contains(script, "a@b.com") {
		t.Error("script should contain form value 'a@b.com'")
	}
	if !strings.Contains(script, "Test") {
		t.Error("script should contain form value 'Test'")
	}
	if !strings.Contains(script, "200") {
		t.Error("script should contain scroll position")
	}
}

func TestBuildStateRestoreScript_IncludesStorage(t *testing.T) {
	t.Parallel()
	localStorage := map[string]any{"theme": "dark", "lang": "en"}
	sessionStorage := map[string]any{"cart_id": "abc123"}
	cookies := map[string]any{"prefs": "compact", "_ga": "GA1.2.123"}

	script := BuildStateRestoreScript(nil, nil, localStorage, sessionStorage, cookies)

	// localStorage restore
	if !strings.Contains(script, "localStorage.setItem") {
		t.Error("script should contain localStorage.setItem")
	}
	if !strings.Contains(script, "dark") {
		t.Error("script should contain localStorage value 'dark'")
	}
	if !strings.Contains(script, "en") {
		t.Error("script should contain localStorage value 'en'")
	}

	// sessionStorage restore
	if !strings.Contains(script, "sessionStorage.setItem") {
		t.Error("script should contain sessionStorage.setItem")
	}
	if !strings.Contains(script, "abc123") {
		t.Error("script should contain sessionStorage value 'abc123'")
	}

	// cookie restore
	if !strings.Contains(script, "document.cookie") {
		t.Error("script should contain document.cookie assignment")
	}
	if !strings.Contains(script, "compact") {
		t.Error("script should contain cookie value 'compact'")
	}
}

func TestBuildStateRestoreScript_EmptyValues(t *testing.T) {
	t.Parallel()
	script := BuildStateRestoreScript(nil, nil, nil, nil, nil)
	if script == "" {
		t.Error("should return a valid script even with empty values")
	}
	// All data objects should be present as empty
	if !strings.Contains(script, "const formValues = {}") {
		t.Error("script should contain empty formValues")
	}
	if !strings.Contains(script, "const lsData = {}") {
		t.Error("script should contain empty lsData")
	}
	if !strings.Contains(script, "const ssData = {}") {
		t.Error("script should contain empty ssData")
	}
	if !strings.Contains(script, "const cookieData = {}") {
		t.Error("script should contain empty cookieData")
	}
}

func TestBuildStateRestoreScript_HandlesRadioEntries(t *testing.T) {
	t.Parallel()
	formValues := map[string]any{
		"radio::plan": map[string]any{
			"kind":  "radio",
			"name":  "billing.plan",
			"value": "pro",
		},
	}

	script := BuildStateRestoreScript(formValues, nil, nil, nil, nil)

	if !strings.Contains(script, "val.kind === 'radio'") {
		t.Error("script should include the radio restore branch")
	}
	if !strings.Contains(script, "\"kind\":\"radio\"") {
		t.Error("script should embed serialized radio metadata")
	}
}

// ============================================
// StateCaptureScript — content verification
// ============================================

func TestStateCaptureScript_FiltersStorageKeys(t *testing.T) {
	t.Parallel()

	if !strings.Contains(StateCaptureScript, "localStorage.key(i)") {
		t.Error("capture script should iterate localStorage keys")
	}
	if !strings.Contains(StateCaptureScript, "localStorage.getItem(k)") {
		t.Error("capture script should read localStorage values")
	}
	if !strings.Contains(StateCaptureScript, "sensitiveKeyPattern.test(k)") {
		t.Error("capture script should filter storage keys with sensitiveKeyPattern")
	}

	if !strings.Contains(StateCaptureScript, "sessionStorage.key(i)") {
		t.Error("capture script should iterate sessionStorage keys")
	}
	if !strings.Contains(StateCaptureScript, "sessionStorage.getItem(k)") {
		t.Error("capture script should read sessionStorage values")
	}

	if !strings.Contains(StateCaptureScript, "document.cookie.split") {
		t.Error("capture script should split document.cookie")
	}

	if !strings.Contains(StateCaptureScript, "local_storage: ls") {
		t.Error("capture script should return local_storage")
	}
	if !strings.Contains(StateCaptureScript, "session_storage: ss") {
		t.Error("capture script should return session_storage")
	}
	if !strings.Contains(StateCaptureScript, "cookies: cookies") {
		t.Error("capture script should return cookies")
	}
}

func TestStateCaptureScript_CapturesRadioAsStructuredValue(t *testing.T) {
	t.Parallel()

	if !strings.Contains(StateCaptureScript, "forms['radio::' + groupName]") {
		t.Error("capture script should store radio values under radio::<group>")
	}
	if !strings.Contains(StateCaptureScript, "kind: 'radio'") {
		t.Error("capture script should persist radio metadata kind")
	}
}

func TestStateCaptureScript_StorageTryCatch(t *testing.T) {
	t.Parallel()

	storageBlocks := []string{"localStorage.length", "sessionStorage.length", "document.cookie"}
	for _, block := range storageBlocks {
		idx := strings.Index(StateCaptureScript, block)
		if idx < 0 {
			t.Errorf("capture script should access %s", block)
			continue
		}
		preceding := StateCaptureScript[:idx]
		lastTry := strings.LastIndex(preceding, "try {")
		if lastTry < 0 {
			t.Errorf("access to %s should be inside a try block", block)
		}
	}
}

// ============================================
// State constants
// ============================================

func TestStateConstants(t *testing.T) {
	t.Parallel()

	if StateCaptureStatusCaptured != "captured" {
		t.Errorf("StateCaptureStatusCaptured = %q", StateCaptureStatusCaptured)
	}
	if StateCaptureStatusPilotDisabled != "skipped_pilot_disabled" {
		t.Errorf("StateCaptureStatusPilotDisabled = %q", StateCaptureStatusPilotDisabled)
	}
	if StateRestoreStatusQueued != "queued" {
		t.Errorf("StateRestoreStatusQueued = %q", StateRestoreStatusQueued)
	}
	if StateNamespace != "saved_states" {
		t.Errorf("StateNamespace = %q", StateNamespace)
	}
}
