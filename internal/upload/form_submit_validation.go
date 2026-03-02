package upload

import (
	"fmt"
	"os"
	"strings"
)

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

func OpenAndValidateFile(resolvedPath, displayPath string) (*os.File, os.FileInfo, error) {
	// #nosec G304 -- file path validated by Security chain
	file, err := os.Open(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("file not found: %s: %w", displayPath, err)
		}
		return nil, nil, fmt.Errorf("failed to open file: %s: %w", displayPath, err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close() //nolint:errcheck // closing on error path
		return nil, nil, fmt.Errorf("failed to stat file: %s: %w", displayPath, err)
	}

	if err := CheckHardlink(info); err != nil {
		file.Close() //nolint:errcheck // closing on error path
		return nil, nil, err
	}

	return file, info, nil
}
