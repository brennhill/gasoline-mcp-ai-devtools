// form_submit.go — Stage 3 form submission logic with multipart streaming.
package upload

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UploadHTTPClient is a shared client for Stage 3 form submissions.
// Reuses connections via the default transport pool.
var UploadHTTPClient = &http.Client{
	Timeout: 10 * time.Minute, // Large file uploads can take a while
	Transport: NewSSRFSafeTransport(func() bool {
		return SkipSSRFCheck
	}),
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		// Prevent redirect to private IPs (SSRF via redirect)
		if err := ValidateFormActionURL(req.URL.String()); err != nil {
			return fmt.Errorf("redirect blocked: %w", err)
		}
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

// HandleFormSubmit is the core logic for form submission, testable without HTTP.
// Stage 3 requires --upload-dir.
func HandleFormSubmit(req FormSubmitRequest, sec *Security) StageResponse {
	return HandleFormSubmitCtx(context.Background(), req, sec)
}

func formSubmitStage3Error(msg string) StageResponse {
	return StageResponse{Success: false, Stage: 3, Error: msg}
}

// ValidateFormSubmitFields validates all form submission fields.
func ValidateFormSubmitFields(req *FormSubmitRequest, sec *Security) (*PathValidationResult, error) {
	if req.FormAction == "" {
		return nil, fmt.Errorf("missing required parameter: form_action")
	}
	if req.FilePath == "" {
		return nil, fmt.Errorf("missing required parameter: file_path")
	}
	if req.FileInputName == "" {
		return nil, fmt.Errorf("missing required parameter: file_input_name")
	}

	pathResult, pathErr := sec.ValidateFilePath(req.FilePath, true)
	if pathErr != nil {
		return nil, pathErr
	}

	if err := ValidateFormActionURL(req.FormAction); err != nil {
		return nil, fmt.Errorf("invalid form_action URL: %w", err)
	}

	if req.Method == "" {
		req.Method = "POST"
	}
	if err := ValidateHTTPMethod(req.Method); err != nil {
		return nil, err
	}

	if err := ValidateCookieHeader(req.Cookies); err != nil {
		return nil, err
	}

	for k := range req.Fields {
		if strings.ContainsAny(k, "\r\n\x00\"") {
			return nil, fmt.Errorf("form field name %q contains invalid characters", k)
		}
	}

	return pathResult, nil
}

// OpenAndValidateFile opens and validates a file for upload.
func OpenAndValidateFile(resolvedPath, displayPath string) (*os.File, os.FileInfo, error) {
	// #nosec G304 -- file path validated by Security chain
	file, err := os.Open(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("file not found: %s", displayPath)
		}
		return nil, nil, fmt.Errorf("failed to open file: %s", displayPath)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close() //nolint:errcheck // closing on error path
		return nil, nil, fmt.Errorf("failed to stat file: %s", displayPath)
	}

	if err := CheckHardlink(info); err != nil {
		file.Close() //nolint:errcheck // closing on error path
		return nil, nil, err
	}

	return file, info, nil
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

// StreamMultipartForm writes the multipart form data to the pipe writer.
func StreamMultipartForm(pw *io.PipeWriter, writer *multipart.Writer, req FormSubmitRequest, file *os.File) error {
	defer pw.Close() //nolint:errcheck // pipe close

	if req.CSRFToken != "" {
		if err := writer.WriteField("csrf_token", req.CSRFToken); err != nil {
			return err
		}
	}

	for k, v := range req.Fields {
		if err := writer.WriteField(k, v); err != nil {
			return err
		}
	}

	fileName := filepath.Base(req.FilePath)
	mimeType := DetectMimeType(fileName)
	partHeader := make(textproto.MIMEHeader)
	safeName := SanitizeForContentDisposition(req.FileInputName)
	safeFileName := SanitizeForContentDisposition(fileName)
	partHeader.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`, safeName, safeFileName))
	partHeader.Set("Content-Type", mimeType)

	fw, err := writer.CreatePart(partHeader)
	if err != nil {
		return err
	}

	if _, err := io.Copy(fw, file); err != nil {
		return err
	}

	return writer.Close()
}

// ExecuteFormSubmit performs the actual HTTP form submission.
func ExecuteFormSubmit(ctx context.Context, req FormSubmitRequest, file *os.File, info os.FileInfo, writer *multipart.Writer, pr *io.PipeReader, pw *io.PipeWriter, start time.Time) StageResponse {
	writeErrCh := make(chan error, 1)
	go func() { // lint:allow-bare-goroutine — short-lived pipe writer, error captured via channel
		writeErrCh <- StreamMultipartForm(pw, writer, req, file)
	}()

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.FormAction, pr)
	if err != nil {
		_ = pr.Close()
		<-writeErrCh
		return formSubmitStage3Error("Failed to create HTTP request: " + err.Error())
	}

	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	if req.Cookies != "" {
		httpReq.Header.Set("Cookie", req.Cookies)
	}

	// #nosec G704 -- req.FormAction is pre-validated by ValidateFormActionURL and redirect callback revalidates
	httpResp, err := UploadHTTPClient.Do(httpReq)
	if err != nil {
		<-writeErrCh
		return StageResponse{
			Success: false, Stage: 3,
			Error: "Form submission failed: " + err.Error(), FileName: filepath.Base(req.FilePath),
			FileSizeBytes: info.Size(), DurationMs: time.Since(start).Milliseconds(),
		}
	}
	defer httpResp.Body.Close() //nolint:errcheck // deferred close

	if writeErr := <-writeErrCh; writeErr != nil {
		return StageResponse{
			Success: false, Stage: 3,
			Error: "Error writing form data: " + writeErr.Error(), DurationMs: time.Since(start).Milliseconds(),
		}
	}

	if httpResp.StatusCode >= 400 {
		return buildHTTPErrorResponse(httpResp, filepath.Base(req.FilePath), info.Size(), time.Since(start).Milliseconds())
	}

	return StageResponse{
		Success: true, Stage: 3,
		Status:        fmt.Sprintf("Form interception: %s submitted to platform (HTTP %d)", req.Method, httpResp.StatusCode),
		FileName:      filepath.Base(req.FilePath),
		FileSizeBytes: info.Size(), DurationMs: time.Since(start).Milliseconds(),
	}
}

// HandleFormSubmitCtx is the context-aware form submission handler.
func HandleFormSubmitCtx(ctx context.Context, req FormSubmitRequest, sec *Security) StageResponse {
	start := time.Now()

	pathResult, err := ValidateFormSubmitFields(&req, sec)
	if err != nil {
		return formSubmitStage3Error(err.Error())
	}

	file, info, err := OpenAndValidateFile(pathResult.ResolvedPath, req.FilePath)
	if err != nil {
		return formSubmitStage3Error(err.Error())
	}
	defer file.Close() //nolint:errcheck // deferred close

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	return ExecuteFormSubmit(ctx, req, file, info, writer, pr, pw, start)
}
