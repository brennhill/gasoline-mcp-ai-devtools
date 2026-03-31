// uploadhandler_test.go — Unit tests for the uploadhandler sub-package exported API.

package uploadhandler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Re-exported function variables are non-nil
// ---------------------------------------------------------------------------

func TestReExportedFunctions_NonNil(t *testing.T) {
	functions := map[string]any{
		"HandleFileRead":     HandleFileRead,
		"HandleDialogInject": HandleDialogInject,
		"HandleFormSubmit":   HandleFormSubmit,
		"HandleFormSubmitCtx": HandleFormSubmitCtx,
		"ValidateFormSubmitFields": ValidateFormSubmitFields,
		"OpenAndValidateFile":      OpenAndValidateFile,
		"StreamMultipartForm":      StreamMultipartForm,
		"ExecuteFormSubmit":        ExecuteFormSubmit,
		"HandleOSAutomation":       HandleOSAutomation,
		"DetectBrowserPID":         DetectBrowserPID,
		"DismissFileDialog":        DismissFileDialog,
		"ExecuteOSAutomation":      ExecuteOSAutomation,
		"GetProgressTier":          GetProgressTier,
		"DetectMimeType":           DetectMimeType,
	}
	for name, fn := range functions {
		if fn == nil {
			t.Errorf("re-exported function %s should not be nil", name)
		}
	}
}

func TestSecurityFunctions_NonNil(t *testing.T) {
	functions := map[string]any{
		"ValidateUploadDir":     ValidateUploadDir,
		"MatchesDenylist":       MatchesDenylist,
		"MatchesUserDenylist":   MatchesUserDenylist,
		"IsWithinDir":           IsWithinDir,
		"PathsEqualFold":        PathsEqualFold,
		"PathHasPrefixFold":     PathHasPrefixFold,
		"IsPrivateIP":           IsPrivateIP,
		"NewSecurity":           NewSecurity,
		"SetSkipSSRFCheck":      SetSkipSSRFCheck,
		"SetSSRFAllowedHosts":   SetSSRFAllowedHosts,
		"NewSSRFSafeTransport":  NewSSRFSafeTransport,
	}
	for name, fn := range functions {
		if fn == nil {
			t.Errorf("security function %s should not be nil", name)
		}
	}
}

func TestValidatorFunctions_NonNil(t *testing.T) {
	functions := map[string]any{
		"ValidatePathForOSAutomation":   ValidatePathForOSAutomation,
		"ValidateHTTPMethod":            ValidateHTTPMethod,
		"ValidateFormActionURL":         ValidateFormActionURL,
		"ValidateCookieHeader":          ValidateCookieHeader,
		"SanitizeForContentDisposition": SanitizeForContentDisposition,
		"SanitizeForAppleScript":        SanitizeForAppleScript,
		"SanitizeForSendKeys":           SanitizeForSendKeys,
	}
	for name, fn := range functions {
		if fn == nil {
			t.Errorf("validator function %s should not be nil", name)
		}
	}
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

func TestConstants_SaneValues(t *testing.T) {
	if MaxBase64FileSize <= 0 {
		t.Errorf("MaxBase64FileSize should be positive, got %d", MaxBase64FileSize)
	}
	if DefaultEscalationTimeoutMs <= 0 {
		t.Errorf("DefaultEscalationTimeoutMs should be positive, got %d", DefaultEscalationTimeoutMs)
	}
}

func TestProgressTierConstants(t *testing.T) {
	// Verify the tier constants are distinct.
	tiers := map[ProgressTier]bool{
		ProgressTierSimple:   true,
		ProgressTierPeriodic: true,
		ProgressTierDetailed: true,
	}
	if len(tiers) != 3 {
		t.Error("expected 3 distinct ProgressTier values")
	}
}

// ---------------------------------------------------------------------------
// HTTP handlers: method enforcement
// ---------------------------------------------------------------------------

func testJSONResponder(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func TestHandleFileReadHTTP_WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/file/read", nil)
	w := httptest.NewRecorder()

	HandleFileReadHTTP(w, req, nil, testJSONResponder)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should return 405, got %d", w.Code)
	}
}

func TestHandleFileDialogInjectHTTP_WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/file/dialog/inject", nil)
	w := httptest.NewRecorder()

	HandleFileDialogInjectHTTP(w, req, nil, testJSONResponder)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should return 405, got %d", w.Code)
	}
}

func TestHandleFormSubmitHTTP_WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/form/submit", nil)
	w := httptest.NewRecorder()

	HandleFormSubmitHTTP(w, req, nil, testJSONResponder)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should return 405, got %d", w.Code)
	}
}

func TestHandleOSAutomationHTTP_WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/os-automation/inject", nil)
	w := httptest.NewRecorder()

	HandleOSAutomationHTTP(w, req, true, nil, testJSONResponder)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should return 405, got %d", w.Code)
	}
}

func TestHandleOSAutomationHTTP_Disabled(t *testing.T) {
	body := strings.NewReader(`{}`)
	req := httptest.NewRequest("POST", "/api/os-automation/inject", body)
	w := httptest.NewRecorder()

	HandleOSAutomationHTTP(w, req, false, nil, testJSONResponder)

	if w.Code != http.StatusForbidden {
		t.Errorf("disabled OS automation should return 403, got %d", w.Code)
	}
}

func TestHandleOSAutomationDismissHTTP_WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/os-automation/dismiss", nil)
	w := httptest.NewRecorder()

	HandleOSAutomationDismissHTTP(w, req, true, testJSONResponder)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should return 405, got %d", w.Code)
	}
}

func TestHandleOSAutomationDismissHTTP_Disabled(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/os-automation/dismiss", nil)
	w := httptest.NewRecorder()

	HandleOSAutomationDismissHTTP(w, req, false, testJSONResponder)

	if w.Code != http.StatusForbidden {
		t.Errorf("disabled should return 403, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// HTTP handlers: invalid JSON
// ---------------------------------------------------------------------------

func TestHandleFileReadHTTP_InvalidJSON(t *testing.T) {
	body := strings.NewReader(`not-json`)
	req := httptest.NewRequest("POST", "/api/file/read", body)
	w := httptest.NewRecorder()

	HandleFileReadHTTP(w, req, nil, testJSONResponder)

	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON should return 400, got %d", w.Code)
	}
}

func TestHandleFormSubmitHTTP_InvalidJSON(t *testing.T) {
	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest("POST", "/api/form/submit", body)
	w := httptest.NewRecorder()

	HandleFormSubmitHTTP(w, req, nil, testJSONResponder)

	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON should return 400, got %d", w.Code)
	}
}
