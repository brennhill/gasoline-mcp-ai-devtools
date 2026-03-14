/**
 * Purpose: Monitors context annotation sizes in log entries and warns when annotations are excessively large.
 * Docs: docs/features/feature/observe/index.md
 */
import type { LogEntry, ContextWarning } from '../types/index.js';
/**
 * Measure the serialized byte size of _context in a log entry
 */
export declare function measureContextSize(entry: LogEntry): number;
/**
 * Check a batch of entries for excessive context annotation usage
 */
export declare function checkContextAnnotations(entries: LogEntry[]): void;
/**
 * Get the current context annotation warning state
 */
export declare function getContextWarning(): ContextWarning | null;
/**
 * Reset the context annotation warning (for testing)
 */
export declare function resetContextWarning(): void;
//# sourceMappingURL=context-monitor.d.ts.map