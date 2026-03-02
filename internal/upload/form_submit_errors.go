package upload

import (
	"fmt"
	"net/http"
)

func formSubmitStage3Error(msg string) StageResponse {
	return StageResponse{Success: false, Stage: 3, Error: msg}
}

var httpStatusErrors = map[int]string{
	401: "User not logged into platform (HTTP 401). Please log in and retry.",
	403: "CSRF token mismatch or forbidden (HTTP 403). Token may be expired.",
	422: "Form validation failed (HTTP 422). Check required fields.",
}

// Truncate returns s unchanged if len(s) <= maxLen. Otherwise, it truncates
// and appends "..." so the total output length equals maxLen.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..."[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func buildHTTPErrorResponse(resp *http.Response, fileName string, fileSize int64, elapsed int64) StageResponse {
	bodyBytes := make([]byte, 1024)
	n, _ := resp.Body.Read(bodyBytes)
	bodyPreview := string(bodyBytes[:n])

	errMsg, ok := httpStatusErrors[resp.StatusCode]
	if !ok {
		errMsg = fmt.Sprintf("Platform returned HTTP %d", resp.StatusCode)
	}

	return StageResponse{
		Success:       false,
		Stage:         3,
		Error:         errMsg,
		FileName:      fileName,
		FileSizeBytes: fileSize,
		DurationMs:    elapsed,
		Suggestions:   []string{"Check authentication", "Verify CSRF token", "Response: " + Truncate(bodyPreview, 200)},
	}
}
