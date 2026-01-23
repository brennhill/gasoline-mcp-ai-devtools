# Changelog

All notable changes to Dev Console will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2024-01-22

### Added

- Initial release
- **Server**
  - Zero-dependency Node.js server
  - JSONL log file format
  - Log rotation (configurable max entries)
  - CORS support for browser extension
  - Health check endpoint
  - Clear logs endpoint
- **Browser Extension**
  - Console capture (log, warn, error, info, debug)
  - Network error capture (4xx, 5xx responses)
  - Exception capture (onerror, unhandled rejections)
  - Configurable capture levels
  - Domain filtering
  - Connection status badge
- **Landing Page**
  - Quick start instructions
  - Feature overview
  - Privacy information
