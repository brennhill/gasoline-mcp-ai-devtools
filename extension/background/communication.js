/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * @fileoverview Communication - Facade that re-exports communication functions
 * from modular subcomponents: circuit-breaker.ts, batchers.ts, and server.ts
 */
// Re-export circuit breaker functions
export { createCircuitBreaker } from './circuit-breaker.js';
// Re-export batcher functions and types
export { createBatcherWithCircuitBreaker, createLogBatcher, RATE_LIMIT_CONFIG } from './batchers.js';
// Re-export server communication functions
// NOTE: postSettings and pollCaptureSettings removed - use /sync for all communication
export { sendLogsToServer, sendWSEventsToServer, sendNetworkBodiesToServer, sendNetworkWaterfallToServer, sendEnhancedActionsToServer, sendPerformanceSnapshotsToServer, checkServerHealth, updateBadge, postQueryResult, postAsyncCommandResult, postExtensionLogs, sendStatusPing, pollPendingQueries } from './server.js';
import { getRequestHeaders } from './server.js';
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
/**
 * Capture a screenshot of the visible tab area
 */
export async function captureScreenshot(tabId, serverUrl, relatedErrorId, errorType, canTakeScreenshotFn, recordScreenshotFn, debugLogFn) {
    const rateCheck = canTakeScreenshotFn(tabId);
    if (!rateCheck.allowed) {
        if (debugLogFn) {
            debugLogFn('capture', `Screenshot rate limited: ${rateCheck.reason}`, {
                tabId,
                nextAllowedIn: rateCheck.nextAllowedIn
            });
        }
        return {
            success: false,
            error: `Rate limited: ${rateCheck.reason}`,
            nextAllowedIn: rateCheck.nextAllowedIn
        };
    }
    try {
        const tab = await chrome.tabs.get(tabId);
        const dataUrl = await chrome.tabs.captureVisibleTab(tab.windowId, {
            format: 'jpeg',
            quality: 80
        });
        recordScreenshotFn(tabId);
        const response = await fetch(`${serverUrl}/screenshots`, {
            method: 'POST',
            headers: getRequestHeaders(),
            body: JSON.stringify({
                dataUrl,
                url: tab.url,
                errorId: relatedErrorId || '',
                errorType: errorType || ''
            })
        });
        if (!response.ok) {
            throw new Error(`Failed to upload screenshot: server returned HTTP ${response.status} ${response.statusText}`);
        }
        const result = (await response.json());
        const screenshotEntry = {
            ts: new Date().toISOString(),
            type: 'screenshot',
            level: 'info',
            url: tab.url,
            _enrichments: ['screenshot'],
            screenshotFile: result.filename,
            trigger: relatedErrorId ? 'error' : 'manual',
            ...(relatedErrorId ? { relatedErrorId } : {})
        };
        if (debugLogFn) {
            debugLogFn('capture', `Screenshot saved: ${result.filename}`, {
                trigger: relatedErrorId ? 'error' : 'manual',
                relatedErrorId
            });
        }
        return { success: true, entry: screenshotEntry };
    }
    catch (error) {
        if (debugLogFn) {
            debugLogFn('error', 'Screenshot capture failed', { error: error.message });
        }
        return { success: false, error: error.message };
    }
}
//# sourceMappingURL=communication.js.map