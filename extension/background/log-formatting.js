/**
 * Purpose: Log entry formatting and level-based capture filtering.
 * Why: Separates log formatting concerns from communication/transport to keep each module single-purpose.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
/**
 * Truncate a single argument if too large
 */
function truncateArg(arg, maxSize = 10240) {
    if (arg === null || arg === undefined)
        return arg;
    try {
        const serialized = JSON.stringify(arg);
        if (serialized.length > maxSize) {
            if (typeof arg === 'string') {
                return arg.slice(0, maxSize) + '... [truncated]';
            }
            return serialized.slice(0, maxSize) + '...[truncated]';
        }
        return arg;
    }
    catch {
        if (typeof arg === 'object') {
            return '[Circular or unserializable object]';
        }
        return String(arg);
    }
}
/**
 * Format a log entry with timestamp and truncation
 */
export function formatLogEntry(entry) {
    const formatted = { ...entry };
    if (!formatted.ts) {
        ;
        formatted.ts = new Date().toISOString();
    }
    if ('args' in formatted && Array.isArray(formatted.args)) {
        formatted.args = formatted.args.map((arg) => truncateArg(arg));
    }
    return formatted;
}
/**
 * Determine if a log should be captured based on level filter
 */
export function shouldCaptureLog(logLevel, filterLevel, logType) {
    if (logType === 'network' || logType === 'exception') {
        return true;
    }
    const levels = ['debug', 'log', 'info', 'warn', 'error'];
    const logIndex = levels.indexOf(logLevel);
    const filterIndex = levels.indexOf(filterLevel === 'all' ? 'debug' : filterLevel);
    return logIndex >= filterIndex;
}
//# sourceMappingURL=log-formatting.js.map