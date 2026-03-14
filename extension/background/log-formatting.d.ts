/**
 * Purpose: Log entry formatting and level-based capture filtering.
 * Why: Separates log formatting concerns from communication/transport to keep each module single-purpose.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import type { LogEntry } from '../types/index.js';
/**
 * Format a log entry with timestamp and truncation
 */
export declare function formatLogEntry(entry: LogEntry): LogEntry;
/**
 * Determine if a log should be captured based on level filter
 */
export declare function shouldCaptureLog(logLevel: string, filterLevel: string, logType?: string): boolean;
//# sourceMappingURL=log-formatting.d.ts.map