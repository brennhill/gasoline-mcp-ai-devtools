/**
 * Purpose: Fetches and caches source maps, parses stack frames with VLQ decoding, and resolves stack traces for better error messages.
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
/**
 * Resolve an entire stack trace
 */
export declare function resolveStackTrace(stack: string, debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<string>;
//# sourceMappingURL=snapshots.d.ts.map