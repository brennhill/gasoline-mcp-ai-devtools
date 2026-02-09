// upload_types.go — Types and constants for file upload feature.
// Defines request/response types, escalation states, MIME detection, and progress tracking.
package main

import (
	"path/filepath"
	"strings"
	"time"
)

// ============================================
// Security Flags
// ============================================

// Upload automation security constants
const (
	// ErrUploadDisabled is returned when upload automation is not enabled
	ErrUploadDisabled = "upload_disabled"
)

// ============================================
// Request/Response Types: Stage 1 (File Read)
// ============================================

// FileReadRequest is the request body for POST /api/file/read
type FileReadRequest struct {
	FilePath string `json:"file_path"`
}

// FileReadResponse is the response body for POST /api/file/read
type FileReadResponse struct {
	Success    bool   `json:"success"`
	FileName   string `json:"file_name,omitempty"`
	FileSize   int64  `json:"file_size,omitempty"`
	MimeType   string `json:"mime_type,omitempty"`
	DataBase64 string `json:"data_base64,omitempty"`
	Error      string `json:"error,omitempty"`
}

// ============================================
// Request/Response Types: Stage 2 (File Dialog)
// ============================================

// FileDialogInjectRequest is the request body for POST /api/file/dialog/inject
type FileDialogInjectRequest struct {
	FilePath   string `json:"file_path"`
	BrowserPID int    `json:"browser_pid"`
}

// ============================================
// Request/Response Types: Stage 3 (Form Submit)
// ============================================

// FormSubmitRequest is the request body for POST /api/form/submit
type FormSubmitRequest struct {
	FormAction    string            `json:"form_action"`
	Method        string            `json:"method,omitempty"`
	Fields        map[string]string `json:"fields,omitempty"`
	FileInputName string            `json:"file_input_name"`
	FilePath      string            `json:"file_path"`
	CSRFToken     string            `json:"csrf_token,omitempty"`
	Cookies       string            `json:"cookies,omitempty"`
}

// ============================================
// Request/Response Types: Stage 4 (OS Automation)
// ============================================

// OSAutomationInjectRequest is the request body for POST /api/os-automation/inject
type OSAutomationInjectRequest struct {
	FilePath   string `json:"file_path"`
	BrowserPID int    `json:"browser_pid"`
	RetryCount int    `json:"retry_count,omitempty"`
}

// ============================================
// Generic Upload Stage Response
// ============================================

// UploadStageResponse is the generic response for upload stage operations
type UploadStageResponse struct {
	Success          bool     `json:"success"`
	Stage            int      `json:"stage,omitempty"`
	Status           string   `json:"status,omitempty"`
	Error            string   `json:"error,omitempty"`
	FileName         string   `json:"file_name,omitempty"`
	FileSizeBytes    int64    `json:"file_size_bytes,omitempty"`
	DurationMs       int64    `json:"duration_ms,omitempty"`
	EscalationReason string   `json:"escalation_reason,omitempty"`
	Suggestions      []string `json:"suggestions,omitempty"`
	// Progress fields (for large files)
	BytesSent  int64   `json:"bytes_sent,omitempty"`
	TotalBytes int64   `json:"total_bytes,omitempty"`
	Percent    float64 `json:"percent,omitempty"`
	ETASeconds int     `json:"eta_seconds,omitempty"`
	SpeedMBPS  float64 `json:"speed_mbps,omitempty"`
}

// ============================================
// Progress Tracking
// ============================================

// ProgressTier defines the progress reporting strategy based on file size
type ProgressTier string

const (
	ProgressTierSimple   ProgressTier = "simple"   // < 100MB: result only
	ProgressTierPeriodic ProgressTier = "periodic" // 100MB - 2GB: 10% chunks
	ProgressTierDetailed ProgressTier = "detailed" // > 2GB: byte-level + ETA
)

const (
	progressThresholdSmall = 100 * 1024 * 1024       // 100MB
	progressThresholdLarge = 2 * 1024 * 1024 * 1024   // 2GB
)

// getProgressTier returns the appropriate progress tier based on file size
func getProgressTier(fileSize int64) ProgressTier {
	if fileSize < progressThresholdSmall {
		return ProgressTierSimple
	}
	if fileSize < progressThresholdLarge {
		return ProgressTierPeriodic
	}
	return ProgressTierDetailed
}

// UploadProgress tracks progress for an ongoing upload
type UploadProgress struct {
	Tier       ProgressTier `json:"tier"`
	BytesSent  int64        `json:"bytes_sent"`
	TotalBytes int64        `json:"total_bytes"`
	Percent    float64      `json:"percent"`
	StartTime  time.Time    `json:"start_time"`
	ETASeconds int          `json:"eta_seconds"`
	SpeedMBPS  float64      `json:"speed_mbps"`
}

// ============================================
// Escalation State Machine
// ============================================

// UploadStage represents the current stage in the escalation state machine
type UploadStage string

const (
	StageIdle              UploadStage = "idle"
	StageDragDrop          UploadStage = "stage_1_dragdrop"
	StageFileDialog        UploadStage = "stage_2_file_dialog"
	StageFormInterception  UploadStage = "stage_3_form_interception"
	StageOSAutomation      UploadStage = "stage_4_os_automation"
	StageComplete          UploadStage = "complete"
	StageError             UploadStage = "error"
)

// UploadEscalationState tracks the current state of the escalation state machine
type UploadEscalationState struct {
	CurrentStage     UploadStage `json:"current_stage"`
	EscalationReason string      `json:"escalation_reason,omitempty"`
	LastError        string      `json:"last_error,omitempty"`
	RetryCount       int         `json:"retry_count"`
	StartTime        time.Time   `json:"start_time"`
}

// NewUploadEscalationState creates a new escalation state starting at idle
func NewUploadEscalationState() *UploadEscalationState {
	return &UploadEscalationState{
		CurrentStage: StageIdle,
		StartTime:    time.Now(),
	}
}

// Advance moves to the next stage with an optional reason
func (s *UploadEscalationState) Advance(nextStage UploadStage, reason string) {
	s.CurrentStage = nextStage
	if reason != "" {
		s.EscalationReason = reason
	}
}

// Complete marks the upload as successfully completed
func (s *UploadEscalationState) Complete() {
	s.CurrentStage = StageComplete
}

// Fail marks the upload as failed with an error message
func (s *UploadEscalationState) Fail(lastError string) {
	s.CurrentStage = StageError
	s.LastError = lastError
}

// StageNumber returns the numeric stage (1-4) or 0 for non-stage states
func (s *UploadEscalationState) StageNumber() int {
	switch s.CurrentStage {
	case StageDragDrop:
		return 1
	case StageFileDialog:
		return 2
	case StageFormInterception:
		return 3
	case StageOSAutomation:
		return 4
	default:
		return 0
	}
}

// ============================================
// MIME Type Detection
// ============================================

// mimeTypes maps file extensions to MIME types (zero-dependency alternative to mime package)
var mimeTypes = map[string]string{
	// Video
	".mp4":  "video/mp4",
	".webm": "video/webm",
	".avi":  "video/x-msvideo",
	".mov":  "video/quicktime",
	".mkv":  "video/x-matroska",
	".wmv":  "video/x-ms-wmv",
	".flv":  "video/x-flv",
	".m4v":  "video/x-m4v",
	".3gp":  "video/3gpp",

	// Audio
	".mp3":  "audio/mpeg",
	".wav":  "audio/wav",
	".ogg":  "audio/ogg",
	".flac": "audio/flac",
	".aac":  "audio/aac",
	".m4a":  "audio/x-m4a",
	".wma":  "audio/x-ms-wma",

	// Image
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
	".bmp":  "image/bmp",
	".ico":  "image/x-icon",
	".tiff": "image/tiff",
	".tif":  "image/tiff",
	".avif": "image/avif",

	// Document
	".pdf":  "application/pdf",
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".ppt":  "application/vnd.ms-powerpoint",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",

	// Text
	".txt":  "text/plain",
	".html": "text/html",
	".htm":  "text/html",
	".css":  "text/css",
	".csv":  "text/csv",
	".md":   "text/markdown",
	".xml":  "application/xml",

	// Code/Data
	".js":   "application/javascript",
	".ts":   "application/typescript",
	".json": "application/json",
	".yaml": "application/x-yaml",
	".yml":  "application/x-yaml",

	// Archive
	".zip":  "application/zip",
	".gz":   "application/gzip",
	".tar":  "application/x-tar",
	".rar":  "application/x-rar-compressed",
	".7z":   "application/x-7z-compressed",
	".bz2":  "application/x-bzip2",

	// Other
	".wasm": "application/wasm",
	".apk":  "application/vnd.android.package-archive",
	".dmg":  "application/x-apple-diskimage",
}

// detectMimeType returns the MIME type for a given filename based on its extension.
// Falls back to "application/octet-stream" for unknown extensions.
func detectMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// ============================================
// Upload File Size Constants
// ============================================

const (
	// maxBase64FileSize is the maximum file size for base64 encoding (100MB)
	maxBase64FileSize = 100 * 1024 * 1024

	// streamChunkSize is the chunk size for streaming large files (64MB)
	streamChunkSize = 64 * 1024 * 1024

	// osAutomationMaxRetries is the maximum number of retries for stage 4
	osAutomationMaxRetries = 3

	// defaultEscalationTimeoutMs is the default timeout before auto-escalating
	defaultEscalationTimeoutMs = 5000
)
