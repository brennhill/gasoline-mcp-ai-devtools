// Purpose: Package upload — multi-stage file upload with security validation, SSRF protection, and OS automation.
// Why: Enforces upload safety boundaries against path traversal, SSRF, and injection attacks across all stages.
// Docs: docs/features/feature/file-upload/index.md

/*
Package upload implements the four-stage file upload pipeline with comprehensive
security validation at each stage.

Stages:
  - Stage 1 (File Read): validates path, reads file, returns base64-encoded content.
  - Stage 2 (Dialog Inject): validates path, queues file dialog injection.
  - Stage 3 (Form Submit): streams multipart form submission with SSRF-safe transport.
  - Stage 4 (OS Automation): injects file path into native dialogs via AppleScript/xdotool/SendKeys.

Key types:
  - Security: immutable upload security configuration (upload-dir, deny patterns).
  - PathValidationResult: validated, symlink-resolved absolute path safe to open.
  - StageResponse: generic response for all upload stage operations.

Key functions:
  - ValidateUploadDir: validates the --upload-dir flag at startup.
  - ValidateFilePath: runs the full path validation chain (clean, resolve, denylist, scope).
  - HandleFileRead: Stage 1 handler.
  - HandleFormSubmit: Stage 3 handler with streaming multipart upload.
  - HandleOSAutomation: Stage 4 handler with platform-specific automation.
  - NewSSRFSafeTransport: returns an HTTP transport that blocks private/internal targets.
*/
package upload
