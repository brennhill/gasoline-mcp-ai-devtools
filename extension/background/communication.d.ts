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
export { createCircuitBreaker, type CircuitBreakerOptions, type CircuitBreaker } from './circuit-breaker.js';
export { createBatcherWithCircuitBreaker, createLogBatcher, RATE_LIMIT_CONFIG, type Batcher, type BatcherWithCircuitBreaker, type BatcherConfig, type LogBatcherOptions } from './batchers.js';
export { sendLogsToServer, sendWSEventsToServer, sendNetworkBodiesToServer, sendEnhancedActionsToServer, sendPerformanceSnapshotsToServer, checkServerHealth, updateBadge, sendStatusPing, type ServerHealthResponse } from './server.js';
export { formatLogEntry, shouldCaptureLog } from './log-formatting.js';
export { captureScreenshot } from './screenshot-capture.js';
//# sourceMappingURL=communication.d.ts.map