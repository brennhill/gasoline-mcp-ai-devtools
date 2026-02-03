// Package capture provides real-time browser telemetry capture and buffering.
//
// Core functionality includes:
//   - WebSocket event capture with connection lifecycle tracking
//   - Network request/response body capture with binary format detection
//   - User action capture (clicks, inputs, navigation) with multi-strategy selectors
//   - Performance timing data (PerformanceResourceTiming API)
//   - Console log capture with structured filtering
//   - Recording/playback for test generation and debugging
//
// The Capture type maintains ring buffers with configurable capacity, memory-based
// eviction, and TTL filtering. All methods are thread-safe using a single mutex.
//
// Memory management enforces soft/hard/critical limits to prevent unbounded growth:
//   - Soft (50MB): Evict 25% of oldest entries
//   - Hard (100MB): Evict 50% of oldest entries
//   - Critical (150MB): Clear all buffers, enter minimal mode
package capture
