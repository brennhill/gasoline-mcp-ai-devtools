/**
 * Purpose: Facade that re-exports communication primitives (circuit breaker, batchers, server HTTP, log formatting, screenshot capture).
 * Why: Single import point for communication functions, avoiding scattered imports across consumers.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
/**
 * @fileoverview Communication - Facade that re-exports communication functions
 * from modular subcomponents: circuit-breaker.ts, batchers.ts, server.ts,
 * log-formatting.ts, and screenshot-capture.ts.
 */
// Re-export circuit breaker functions
export { createCircuitBreaker } from './circuit-breaker.js';
// Re-export batcher functions and types
export { createBatcherWithCircuitBreaker, createLogBatcher, RATE_LIMIT_CONFIG } from './batchers.js';
// Re-export server communication functions
export { sendLogsToServer, sendWSEventsToServer, sendNetworkBodiesToServer, sendEnhancedActionsToServer, sendPerformanceSnapshotsToServer, checkServerHealth, updateBadge, sendStatusPing } from './server.js';
// Re-export log formatting (split into its own module for coherence)
export { formatLogEntry, shouldCaptureLog } from './log-formatting.js';
// Re-export screenshot capture (split into its own module for coherence)
export { captureScreenshot } from './screenshot-capture.js';
//# sourceMappingURL=communication.js.map