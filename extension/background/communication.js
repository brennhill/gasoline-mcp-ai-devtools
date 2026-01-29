/**
 * @fileoverview Communication - Handles server communication, circuit breaker,
 * batching, and all HTTP interactions with the Gasoline server.
 */
import { MAX_PENDING_BUFFER } from './state-manager.js';
// =============================================================================
// CONSTANTS
// =============================================================================
const DEFAULT_DEBOUNCE_MS = 100;
const DEFAULT_MAX_BATCH_SIZE = 50;
/** Rate limit configuration */
export const RATE_LIMIT_CONFIG = {
    maxFailures: 5,
    resetTimeout: 30000,
    backoffSchedule: [100, 500, 2000],
    retryBudget: 3,
};
// =============================================================================
// CIRCUIT BREAKER
// =============================================================================
/**
 * Circuit breaker with exponential backoff for server communication.
 * Prevents the extension from hammering a down/slow server.
 */
export function createCircuitBreaker(sendFn, options = {}) {
    const maxFailures = options.maxFailures ?? 5;
    const resetTimeout = options.resetTimeout ?? 30000;
    const initialBackoff = options.initialBackoff ?? 1000;
    const maxBackoff = options.maxBackoff ?? 30000;
    let state = 'closed';
    let consecutiveFailures = 0;
    let totalFailures = 0;
    let totalSuccesses = 0;
    let currentBackoff = 0;
    let lastFailureTime = 0;
    let probeInFlight = false;
    function getState() {
        if (state === 'open' && Date.now() - lastFailureTime >= resetTimeout) {
            state = 'half-open';
        }
        return state;
    }
    function getStats() {
        return {
            state: getState(),
            consecutiveFailures,
            totalFailures,
            totalSuccesses,
            currentBackoff,
        };
    }
    function reset() {
        state = 'closed';
        consecutiveFailures = 0;
        currentBackoff = 0;
        probeInFlight = false;
    }
    function onSuccess() {
        consecutiveFailures = 0;
        currentBackoff = 0;
        totalSuccesses++;
        state = 'closed';
        probeInFlight = false;
    }
    function onFailure() {
        consecutiveFailures++;
        totalFailures++;
        lastFailureTime = Date.now();
        probeInFlight = false;
        if (consecutiveFailures >= maxFailures) {
            state = 'open';
        }
        if (consecutiveFailures > 1) {
            currentBackoff = Math.min(initialBackoff * Math.pow(2, consecutiveFailures - 2), maxBackoff);
        }
        else {
            currentBackoff = 0;
        }
    }
    async function execute(args) {
        const currentState = getState();
        if (currentState === 'open') {
            throw new Error('Circuit breaker is open');
        }
        if (currentState === 'half-open') {
            if (probeInFlight) {
                throw new Error('Circuit breaker is open');
            }
            probeInFlight = true;
        }
        if (currentBackoff > 0) {
            await new Promise((r) => {
                setTimeout(r, currentBackoff);
            });
        }
        try {
            const result = (await sendFn(args));
            onSuccess();
            return result;
        }
        catch (err) {
            onFailure();
            throw err;
        }
    }
    function recordFailure() {
        consecutiveFailures++;
        totalFailures++;
        lastFailureTime = Date.now();
        if (consecutiveFailures >= maxFailures) {
            state = 'open';
        }
        currentBackoff =
            consecutiveFailures >= 2 ? Math.min(initialBackoff * Math.pow(2, consecutiveFailures - 2), maxBackoff) : 0;
    }
    return { execute, getState, getStats, reset, recordFailure };
}
// =============================================================================
// BATCHER WITH CIRCUIT BREAKER
// =============================================================================
/**
 * Creates a batcher wired with circuit breaker logic for rate limiting.
 */
export function createBatcherWithCircuitBreaker(sendFn, options = {}) {
    const debounceMs = options.debounceMs ?? DEFAULT_DEBOUNCE_MS;
    const maxBatchSize = options.maxBatchSize ?? DEFAULT_MAX_BATCH_SIZE;
    const retryBudget = options.retryBudget ?? RATE_LIMIT_CONFIG.retryBudget;
    const maxFailures = options.maxFailures ?? RATE_LIMIT_CONFIG.maxFailures;
    const resetTimeout = options.resetTimeout ?? RATE_LIMIT_CONFIG.resetTimeout;
    const backoffSchedule = RATE_LIMIT_CONFIG.backoffSchedule;
    const localConnectionStatus = { connected: true };
    const isSharedCB = !!options.sharedCircuitBreaker;
    const cb = options.sharedCircuitBreaker ||
        createCircuitBreaker(sendFn, {
            maxFailures,
            resetTimeout,
            initialBackoff: 0,
            maxBackoff: 0,
        });
    function getScheduledBackoff(failures) {
        if (failures <= 0)
            return 0;
        const idx = Math.min(failures - 1, backoffSchedule.length - 1);
        return backoffSchedule[idx];
    }
    const wrappedCircuitBreaker = {
        getState: () => cb.getState(),
        getStats: () => {
            const stats = cb.getStats();
            return {
                ...stats,
                currentBackoff: getScheduledBackoff(stats.consecutiveFailures),
            };
        },
        reset: () => cb.reset(),
    };
    async function attemptSend(entries) {
        if (!isSharedCB) {
            return await cb.execute(entries);
        }
        const state = cb.getState();
        if (state === 'open') {
            throw new Error('Circuit breaker is open');
        }
        try {
            const result = await sendFn(entries);
            cb.reset();
            return result;
        }
        catch (err) {
            cb.recordFailure();
            throw err;
        }
    }
    let pending = [];
    let timeoutId = null;
    async function flushWithCircuitBreaker() {
        if (pending.length === 0)
            return;
        const entries = pending;
        pending = [];
        if (timeoutId) {
            clearTimeout(timeoutId);
            timeoutId = null;
        }
        const currentState = cb.getState();
        if (currentState === 'open') {
            pending = entries.concat(pending).slice(0, MAX_PENDING_BUFFER);
            return;
        }
        try {
            await attemptSend(entries);
            localConnectionStatus.connected = true;
        }
        catch {
            localConnectionStatus.connected = false;
            if (cb.getState() === 'open') {
                pending = entries.concat(pending).slice(0, MAX_PENDING_BUFFER);
                return;
            }
            let retriesLeft = retryBudget - 1;
            while (retriesLeft > 0) {
                retriesLeft--;
                const stats = cb.getStats();
                const backoff = getScheduledBackoff(stats.consecutiveFailures);
                if (backoff > 0) {
                    await new Promise((r) => {
                        setTimeout(r, backoff);
                    });
                }
                try {
                    await attemptSend(entries);
                    localConnectionStatus.connected = true;
                    return;
                }
                catch {
                    localConnectionStatus.connected = false;
                    if (cb.getState() === 'open') {
                        pending = entries.concat(pending).slice(0, MAX_PENDING_BUFFER);
                        return;
                    }
                }
            }
        }
    }
    const scheduleFlush = () => {
        if (timeoutId)
            return;
        timeoutId = setTimeout(() => {
            timeoutId = null;
            flushWithCircuitBreaker();
        }, debounceMs);
    };
    const batcher = {
        add(entry) {
            if (pending.length >= MAX_PENDING_BUFFER)
                return;
            pending.push(entry);
            if (pending.length >= maxBatchSize) {
                flushWithCircuitBreaker();
            }
            else {
                scheduleFlush();
            }
        },
        async flush() {
            await flushWithCircuitBreaker();
        },
        clear() {
            pending = [];
            if (timeoutId) {
                clearTimeout(timeoutId);
                timeoutId = null;
            }
        },
        getPending() {
            return [...pending];
        },
    };
    return {
        batcher,
        circuitBreaker: wrappedCircuitBreaker,
        getConnectionStatus: () => ({ ...localConnectionStatus }),
    };
}
// =============================================================================
// LOG BATCHER (SIMPLE)
// =============================================================================
/**
 * Create a simple log batcher without circuit breaker
 */
export function createLogBatcher(flushFn, options = {}) {
    const debounceMs = options.debounceMs ?? DEFAULT_DEBOUNCE_MS;
    const maxBatchSize = options.maxBatchSize ?? DEFAULT_MAX_BATCH_SIZE;
    const memoryPressureGetter = options.memoryPressureGetter ?? null;
    let pending = [];
    let timeoutId = null;
    const getEffectiveMaxBatchSize = () => {
        if (memoryPressureGetter) {
            const state = memoryPressureGetter();
            if (state.reducedCapacities) {
                return Math.floor(maxBatchSize / 2);
            }
        }
        return maxBatchSize;
    };
    const flush = () => {
        if (pending.length === 0)
            return;
        const entries = pending;
        pending = [];
        if (timeoutId) {
            clearTimeout(timeoutId);
            timeoutId = null;
        }
        flushFn(entries);
    };
    const scheduleFlush = () => {
        if (timeoutId)
            return;
        timeoutId = setTimeout(() => {
            timeoutId = null;
            flush();
        }, debounceMs);
    };
    return {
        add(entry) {
            if (pending.length >= MAX_PENDING_BUFFER)
                return;
            pending.push(entry);
            const effectiveMax = getEffectiveMaxBatchSize();
            if (pending.length >= effectiveMax) {
                flush();
            }
            else {
                scheduleFlush();
            }
        },
        flush() {
            flush();
        },
        clear() {
            pending = [];
            if (timeoutId) {
                clearTimeout(timeoutId);
                timeoutId = null;
            }
        },
    };
}
// =============================================================================
// SERVER COMMUNICATION
// =============================================================================
/**
 * Send log entries to the server
 */
export async function sendLogsToServer(serverUrl, entries, debugLogFn) {
    if (debugLogFn)
        debugLogFn('connection', `Sending ${entries.length} entries to server`);
    const response = await fetch(`${serverUrl}/logs`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({ entries }),
    });
    if (!response.ok) {
        const error = `Server error: ${response.status} ${response.statusText}`;
        if (debugLogFn)
            debugLogFn('error', error);
        throw new Error(error);
    }
    const result = (await response.json());
    if (debugLogFn)
        debugLogFn('connection', `Server accepted entries, total: ${result.entries}`);
    return result;
}
/**
 * Send WebSocket events to the server
 */
export async function sendWSEventsToServer(serverUrl, events, debugLogFn) {
    if (debugLogFn)
        debugLogFn('connection', `Sending ${events.length} WS events to server`);
    const response = await fetch(`${serverUrl}/websocket-events`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({ events }),
    });
    if (!response.ok) {
        const error = `Server error (WS): ${response.status} ${response.statusText}`;
        if (debugLogFn)
            debugLogFn('error', error);
        throw new Error(error);
    }
    if (debugLogFn)
        debugLogFn('connection', `Server accepted ${events.length} WS events`);
}
/**
 * Send network bodies to the server
 */
export async function sendNetworkBodiesToServer(serverUrl, bodies, debugLogFn) {
    if (debugLogFn)
        debugLogFn('connection', `Sending ${bodies.length} network bodies to server`);
    const response = await fetch(`${serverUrl}/network-bodies`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({ bodies }),
    });
    if (!response.ok) {
        const error = `Server error (network bodies): ${response.status} ${response.statusText}`;
        if (debugLogFn)
            debugLogFn('error', error);
        throw new Error(error);
    }
    if (debugLogFn)
        debugLogFn('connection', `Server accepted ${bodies.length} network bodies`);
}
/**
 * Send network waterfall data to server
 */
export async function sendNetworkWaterfallToServer(serverUrl, payload, debugLogFn) {
    if (debugLogFn)
        debugLogFn('connection', `Sending ${payload.entries.length} waterfall entries to server`);
    const response = await fetch(`${serverUrl}/network-waterfall`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(payload),
    });
    if (!response.ok) {
        const error = `Server error (network waterfall): ${response.status} ${response.statusText}`;
        if (debugLogFn)
            debugLogFn('error', error);
        throw new Error(error);
    }
    if (debugLogFn)
        debugLogFn('connection', `Server accepted ${payload.entries.length} waterfall entries`);
}
/**
 * Send enhanced actions to server
 */
export async function sendEnhancedActionsToServer(serverUrl, actions, debugLogFn) {
    if (debugLogFn)
        debugLogFn('connection', `Sending ${actions.length} enhanced actions to server`);
    const response = await fetch(`${serverUrl}/enhanced-actions`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({ actions }),
    });
    if (!response.ok) {
        const error = `Server error (enhanced actions): ${response.status} ${response.statusText}`;
        if (debugLogFn)
            debugLogFn('error', error);
        throw new Error(error);
    }
    if (debugLogFn)
        debugLogFn('connection', `Server accepted ${actions.length} enhanced actions`);
}
/**
 * Send performance snapshots to server
 */
export async function sendPerformanceSnapshotsToServer(serverUrl, snapshots, debugLogFn) {
    if (debugLogFn)
        debugLogFn('connection', `Sending ${snapshots.length} performance snapshots to server`);
    const response = await fetch(`${serverUrl}/performance-snapshots`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ snapshots }),
    });
    if (!response.ok) {
        const error = `Server error (performance snapshots): ${response.status} ${response.statusText}`;
        if (debugLogFn)
            debugLogFn('error', error);
        throw new Error(error);
    }
    if (debugLogFn)
        debugLogFn('connection', `Server accepted ${snapshots.length} performance snapshots`);
}
/**
 * Check server health
 */
export async function checkServerHealth(serverUrl) {
    try {
        const response = await fetch(`${serverUrl}/health`);
        if (!response.ok) {
            return { connected: false, error: `HTTP ${response.status}` };
        }
        let data;
        try {
            data = (await response.json());
        }
        catch {
            return {
                connected: false,
                error: 'Server returned invalid response - check Server URL in options',
            };
        }
        return {
            ...data,
            connected: true,
        };
    }
    catch (error) {
        return {
            connected: false,
            error: error.message,
        };
    }
}
/**
 * Update extension badge
 */
export function updateBadge(status) {
    if (typeof chrome === 'undefined' || !chrome.action)
        return;
    if (status.connected) {
        const errorCount = status.errorCount || 0;
        chrome.action.setBadgeText({
            text: errorCount === 0 ? '' : errorCount > 99 ? '99+' : String(errorCount),
        });
        chrome.action.setBadgeBackgroundColor({
            color: '#3fb950',
        });
    }
    else {
        chrome.action.setBadgeText({ text: '!' });
        chrome.action.setBadgeBackgroundColor({
            color: '#f85149',
        });
    }
}
/**
 * Post query results back to the server
 */
export async function postQueryResult(serverUrl, queryId, type, result) {
    let endpoint;
    if (type === 'a11y') {
        endpoint = '/a11y-result';
    }
    else if (type === 'state') {
        endpoint = '/state-result';
    }
    else if (type === 'highlight') {
        endpoint = '/highlight-result';
    }
    else if (type === 'execute' || type === 'browser_action') {
        endpoint = '/execute-result';
    }
    else {
        endpoint = '/dom-result';
    }
    await fetch(`${serverUrl}${endpoint}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: queryId, result }),
    });
}
/**
 * POST async command result to server using correlation_id
 */
export async function postAsyncCommandResult(serverUrl, correlationId, status, result = null, error = null, debugLogFn) {
    const payload = {
        correlation_id: correlationId,
        status: status,
    };
    if (result !== null) {
        payload.result = result;
    }
    if (error !== null) {
        payload.error = error;
    }
    try {
        await fetch(`${serverUrl}/execute-result`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
        });
    }
    catch (err) {
        if (debugLogFn) {
            debugLogFn('connection', 'Failed to post async command result', {
                correlationId,
                status,
                error: err.message,
            });
        }
    }
}
/**
 * Post extension settings to server
 */
export async function postSettings(serverUrl, sessionId, settings, debugLogFn) {
    try {
        await fetch(`${serverUrl}/settings`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                session_id: sessionId,
                settings: settings,
            }),
        });
        if (debugLogFn)
            debugLogFn('connection', 'Posted settings to server', settings);
    }
    catch (err) {
        if (debugLogFn)
            debugLogFn('connection', 'Failed to post settings', { error: err.message });
    }
}
/**
 * Poll the server's /settings endpoint for AI capture overrides
 */
export async function pollCaptureSettings(serverUrl) {
    try {
        const response = await fetch(`${serverUrl}/settings`);
        if (!response.ok)
            return null;
        const data = (await response.json());
        return data.capture_overrides || {};
    }
    catch {
        return null;
    }
}
/**
 * Post extension logs to server
 */
export async function postExtensionLogs(serverUrl, logs) {
    if (logs.length === 0)
        return;
    try {
        await fetch(`${serverUrl}/extension-logs`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ logs }),
        });
    }
    catch (err) {
        console.error('[Gasoline] Failed to post extension logs', err);
    }
}
/**
 * Send status ping to server
 */
export async function sendStatusPing(serverUrl, statusMessage, diagnosticLogFn) {
    try {
        await fetch(`${serverUrl}/api/extension-status`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(statusMessage),
        });
    }
    catch (err) {
        if (diagnosticLogFn) {
            diagnosticLogFn('[Gasoline] Status ping error: ' + err.message);
        }
    }
}
/**
 * Poll server for pending queries
 */
export async function pollPendingQueries(serverUrl, sessionId, pilotState, diagnosticLogFn, debugLogFn) {
    try {
        if (diagnosticLogFn) {
            diagnosticLogFn(`[Diagnostic] Poll request: header=${pilotState}`);
        }
        const response = await fetch(`${serverUrl}/pending-queries`, {
            headers: {
                'X-Gasoline-Session': sessionId,
                'X-Gasoline-Pilot': pilotState,
            },
        });
        if (!response.ok) {
            if (debugLogFn)
                debugLogFn('connection', 'Poll pending-queries failed', { status: response.status });
            return [];
        }
        const data = (await response.json());
        if (!data.queries || data.queries.length === 0)
            return [];
        if (debugLogFn)
            debugLogFn('connection', 'Got pending queries', { count: data.queries.length });
        return data.queries;
    }
    catch (err) {
        if (debugLogFn)
            debugLogFn('connection', 'Poll pending-queries error', { error: err.message });
        return [];
    }
}
// =============================================================================
// LOG FORMATTING
// =============================================================================
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
// =============================================================================
// SCREENSHOT CAPTURE
// =============================================================================
/**
 * Capture a screenshot of the visible tab area
 */
export async function captureScreenshot(tabId, serverUrl, relatedErrorId, errorType, canTakeScreenshotFn, recordScreenshotFn, debugLogFn) {
    const rateCheck = canTakeScreenshotFn(tabId);
    if (!rateCheck.allowed) {
        if (debugLogFn) {
            debugLogFn('capture', `Screenshot rate limited: ${rateCheck.reason}`, {
                tabId,
                nextAllowedIn: rateCheck.nextAllowedIn,
            });
        }
        return {
            success: false,
            error: `Rate limited: ${rateCheck.reason}`,
            nextAllowedIn: rateCheck.nextAllowedIn,
        };
    }
    try {
        const tab = await chrome.tabs.get(tabId);
        const dataUrl = await chrome.tabs.captureVisibleTab(tab.windowId, {
            format: 'jpeg',
            quality: 80,
        });
        recordScreenshotFn(tabId);
        const response = await fetch(`${serverUrl}/screenshots`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                dataUrl,
                url: tab.url,
                errorId: relatedErrorId || '',
                errorType: errorType || '',
            }),
        });
        if (!response.ok) {
            throw new Error(`Server returned ${response.status}`);
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
            ...(relatedErrorId ? { relatedErrorId } : {}),
        };
        if (debugLogFn) {
            debugLogFn('capture', `Screenshot saved: ${result.filename}`, {
                trigger: relatedErrorId ? 'error' : 'manual',
                relatedErrorId,
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