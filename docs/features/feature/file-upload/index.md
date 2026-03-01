# File Upload

Automates file upload interactions in the browser via the `interact` tool's `upload` action.

## Overview

Provides secure, cross-platform file upload automation using OS-level dialog handling. Supports uploading files to standard `<input type="file">` elements and custom dropzones, with optional form submission after upload.

## Key Capabilities

- Upload files by absolute path to file input elements
- OS-level dialog automation for native file pickers
- Security validation of file paths (no traversal, size limits)
- Optional form submission after upload
- Cross-platform support (macOS, Linux, Windows)

## Code References

- `internal/upload/` — Core upload handlers, security validation, OS automation
- `src/` — Extension-side upload coordination

## Status

**Shipped** — Active in production with code references in `internal/upload/`.
