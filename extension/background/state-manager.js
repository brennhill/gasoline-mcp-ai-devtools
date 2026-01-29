/**
 * @fileoverview State Manager - Manages extension state including error groups,
 * screenshot rate limiting, memory pressure, context annotations, source maps,
 * and processing query tracking.
 */
// =============================================================================
// CONSTANTS
// =============================================================================
/** Error deduplication window in milliseconds */
const ERROR_DEDUP_WINDOW_MS = 5000;
/** Error group flush interval in milliseconds */
const ERROR_GROUP_FLUSH_MS = 10000;
/** Maximum tracked error groups */
const MAX_TRACKED_ERRORS = 100;
/** Error group max age - cleanup after 1 hour */
export const ERROR_GROUP_MAX_AGE_MS = 3600000;
/** Screenshot rate limit in milliseconds */
const SCREENSHOT_RATE_LIMIT_MS = 5000;
/** Maximum screenshots per session */
const SCREENSHOT_MAX_PER_SESSION = 10;
/** Source map cache size limit */
export const SOURCE_MAP_CACHE_SIZE = 50;
/** Source map fetch timeout */
const SOURCE_MAP_FETCH_TIMEOUT = 5000;
/** Memory limits */
export const MEMORY_SOFT_LIMIT = 20 * 1024 * 1024;
export const MEMORY_HARD_LIMIT = 50 * 1024 * 1024;
export const MEMORY_CHECK_INTERVAL_MS = 30000;
export const MEMORY_AVG_LOG_ENTRY_SIZE = 500;
export const MEMORY_AVG_WS_EVENT_SIZE = 300;
export const MEMORY_AVG_NETWORK_BODY_SIZE = 1000;
export const MEMORY_AVG_ACTION_SIZE = 400;
/** Maximum pending buffer size */
export const MAX_PENDING_BUFFER = 1000;
/** Context annotation thresholds */
const CONTEXT_SIZE_THRESHOLD = 20 * 1024;
const CONTEXT_WARNING_WINDOW_MS = 60000;
const CONTEXT_WARNING_COUNT = 3;
/** Debug log buffer size */
const DEBUG_LOG_MAX_ENTRIES = 200;
/** Processing query TTL */
const PROCESSING_QUERY_TTL_MS = 60000;
/** Stack frame regex patterns */
const STACK_FRAME_REGEX = /^\s*at\s+(?:(.+?)\s+\()?(?:(.+?):(\d+):(\d+)|(.+?):(\d+))\)?$/;
const ANONYMOUS_FRAME_REGEX = /^\s*at\s+(.+?):(\d+):(\d+)$/;
/** VLQ character mapping */
const VLQ_CHARS = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
const VLQ_CHAR_MAP = new Map(VLQ_CHARS.split('').map((c, i) => [c, i]));
// =============================================================================
// STATE
// =============================================================================
/** Error grouping state */
const errorGroups = new Map();
/** Screenshot rate limiting state */
const screenshotTimestamps = new Map();
/** Source map cache */
const sourceMapCache = new Map();
/** Memory pressure state */
let memoryPressureLevel = 'normal';
let lastMemoryCheck = 0;
let networkBodyCaptureDisabled = false;
let reducedCapacities = false;
/** Context annotation monitoring state */
let contextExcessiveTimestamps = [];
let contextWarningState = null;
/** Debug log buffer */
const debugLogBuffer = [];
/** Processing queries tracking */
const processingQueries = new Map();
/** Source map enabled flag */
let sourceMapEnabled = false;
// =============================================================================
// ERROR GROUPING
// =============================================================================
/**
 * Create a signature for an error to identify duplicates
 */
export function createErrorSignature(entry) {
    const parts = [];
    parts.push(entry.type || 'unknown');
    parts.push(entry.level || 'error');
    if (entry.type === 'exception') {
        const exEntry = entry;
        parts.push(exEntry.message || '');
        if (exEntry.stack) {
            const firstFrame = exEntry.stack.split('\n')[1] || '';
            parts.push(firstFrame.trim());
        }
    }
    else if (entry.type === 'network') {
        const netEntry = entry;
        parts.push(netEntry.method || 'GET');
        try {
            const url = new URL(netEntry.url || '', 'http://localhost');
            parts.push(url.pathname);
        }
        catch {
            parts.push(netEntry.url || '');
        }
        parts.push(String(netEntry.status || 0));
    }
    else if (entry.type === 'console') {
        const consEntry = entry;
        if (consEntry.args && consEntry.args.length > 0) {
            const firstArg = consEntry.args[0];
            parts.push(typeof firstArg === 'string' ? firstArg.slice(0, 200) : JSON.stringify(firstArg).slice(0, 200));
        }
    }
    return parts.join('|');
}
/**
 * Process an error through the grouping system
 */
export function processErrorGroup(entry) {
    if (entry.level !== 'error' && entry.level !== 'warn') {
        return { shouldSend: true, entry };
    }
    const signature = createErrorSignature(entry);
    const now = Date.now();
    if (errorGroups.has(signature)) {
        const group = errorGroups.get(signature);
        if (now - group.lastSeen < ERROR_DEDUP_WINDOW_MS) {
            group.count++;
            group.lastSeen = now;
            return { shouldSend: false };
        }
        const countToReport = group.count;
        group.count = 1;
        group.lastSeen = now;
        group.firstSeen = now;
        if (countToReport > 1) {
            return {
                shouldSend: true,
                entry: { ...entry, _previousOccurrences: countToReport - 1 },
            };
        }
        return { shouldSend: true, entry };
    }
    if (errorGroups.size >= MAX_TRACKED_ERRORS) {
        let oldestSig = null;
        let oldestTime = Infinity;
        for (const [sig, group] of errorGroups) {
            if (group.lastSeen < oldestTime) {
                oldestTime = group.lastSeen;
                oldestSig = sig;
            }
        }
        if (oldestSig) {
            errorGroups.delete(oldestSig);
        }
    }
    errorGroups.set(signature, {
        entry,
        count: 1,
        firstSeen: now,
        lastSeen: now,
    });
    return { shouldSend: true, entry };
}
/**
 * Get current state of error groups (for testing)
 */
export function getErrorGroupsState() {
    return errorGroups;
}
/**
 * Clean up stale error groups older than ERROR_GROUP_MAX_AGE_MS
 */
export function cleanupStaleErrorGroups(debugLogFn) {
    const now = Date.now();
    for (const [signature, group] of errorGroups) {
        if (now - group.lastSeen > ERROR_GROUP_MAX_AGE_MS) {
            errorGroups.delete(signature);
            if (debugLogFn) {
                debugLogFn('error', 'Cleaned up stale error group', {
                    signature: signature.slice(0, 50) + '...',
                    age: Math.round((now - group.lastSeen) / 60000) + ' min',
                });
            }
        }
    }
}
/**
 * Flush error groups - send any accumulated counts
 */
export function flushErrorGroups() {
    const now = Date.now();
    const entriesToSend = [];
    for (const [signature, group] of errorGroups) {
        if (group.count > 1) {
            // Spread the readonly entry and add our mutable properties
            const processedEntry = {
                ...group.entry,
                _aggregatedCount: group.count,
                _firstSeen: new Date(group.firstSeen).toISOString(),
                _lastSeen: new Date(group.lastSeen).toISOString(),
            };
            // Override ts with fresh timestamp (cast to mutable)
            processedEntry.ts = new Date().toISOString();
            entriesToSend.push(processedEntry);
            group.count = 0;
        }
        if (now - group.lastSeen > ERROR_GROUP_FLUSH_MS * 2) {
            errorGroups.delete(signature);
        }
    }
    return entriesToSend;
}
// =============================================================================
// SCREENSHOT RATE LIMITING
// =============================================================================
/**
 * Check if a screenshot is allowed based on rate limiting
 */
export function canTakeScreenshot(tabId) {
    const now = Date.now();
    if (!screenshotTimestamps.has(tabId)) {
        screenshotTimestamps.set(tabId, []);
    }
    const timestamps = screenshotTimestamps.get(tabId);
    const recentTimestamps = timestamps.filter((t) => now - t < 60000);
    if (recentTimestamps.length >= SCREENSHOT_MAX_PER_SESSION) {
        return { allowed: false, reason: 'session_limit', nextAllowedIn: null };
    }
    const lastTimestamp = recentTimestamps[recentTimestamps.length - 1];
    if (lastTimestamp && now - lastTimestamp < SCREENSHOT_RATE_LIMIT_MS) {
        return {
            allowed: false,
            reason: 'rate_limit',
            nextAllowedIn: SCREENSHOT_RATE_LIMIT_MS - (now - lastTimestamp),
        };
    }
    return { allowed: true };
}
/**
 * Record a screenshot timestamp
 */
export function recordScreenshot(tabId) {
    if (!screenshotTimestamps.has(tabId)) {
        screenshotTimestamps.set(tabId, []);
    }
    screenshotTimestamps.get(tabId).push(Date.now());
}
/**
 * Clear screenshot timestamps for a tab
 */
export function clearScreenshotTimestamps(tabId) {
    screenshotTimestamps.delete(tabId);
}
// =============================================================================
// MEMORY ENFORCEMENT
// =============================================================================
/**
 * Estimate total buffer memory usage from buffer contents
 */
export function estimateBufferMemory(buffers) {
    let total = 0;
    total += buffers.logEntries.length * MEMORY_AVG_LOG_ENTRY_SIZE;
    for (const event of buffers.wsEvents) {
        total += MEMORY_AVG_WS_EVENT_SIZE;
        if (event.data && typeof event.data === 'string') {
            total += event.data.length;
        }
    }
    for (const body of buffers.networkBodies) {
        total += MEMORY_AVG_NETWORK_BODY_SIZE;
        if (body.requestBody && typeof body.requestBody === 'string') {
            total += body.requestBody.length;
        }
        if (body.responseBody && typeof body.responseBody === 'string') {
            total += body.responseBody.length;
        }
    }
    total += buffers.enhancedActions.length * MEMORY_AVG_ACTION_SIZE;
    return total;
}
/**
 * Check memory pressure and take appropriate action
 */
export function checkMemoryPressure(buffers) {
    const estimatedMemory = estimateBufferMemory(buffers);
    lastMemoryCheck = Date.now();
    if (estimatedMemory >= MEMORY_HARD_LIMIT) {
        const alreadyApplied = memoryPressureLevel === 'hard';
        memoryPressureLevel = 'hard';
        networkBodyCaptureDisabled = true;
        reducedCapacities = true;
        return {
            level: 'hard',
            action: 'disable_network_capture',
            estimatedMemory,
            alreadyApplied,
        };
    }
    if (estimatedMemory >= MEMORY_SOFT_LIMIT) {
        const alreadyApplied = memoryPressureLevel === 'soft' || memoryPressureLevel === 'hard';
        memoryPressureLevel = 'soft';
        reducedCapacities = true;
        if (networkBodyCaptureDisabled && estimatedMemory < MEMORY_HARD_LIMIT) {
            networkBodyCaptureDisabled = false;
        }
        return {
            level: 'soft',
            action: 'reduce_capacities',
            estimatedMemory,
            alreadyApplied,
        };
    }
    memoryPressureLevel = 'normal';
    reducedCapacities = false;
    networkBodyCaptureDisabled = false;
    return {
        level: 'normal',
        action: 'none',
        estimatedMemory,
        alreadyApplied: false,
    };
}
/**
 * Get the current memory pressure state
 */
export function getMemoryPressureState() {
    return {
        memoryPressureLevel,
        lastMemoryCheck,
        networkBodyCaptureDisabled,
        reducedCapacities,
    };
}
/**
 * Reset memory pressure state to initial values (for testing)
 */
export function resetMemoryPressureState() {
    memoryPressureLevel = 'normal';
    lastMemoryCheck = 0;
    networkBodyCaptureDisabled = false;
    reducedCapacities = false;
}
/**
 * Check if network body capture is disabled
 */
export function isNetworkBodyCaptureDisabled() {
    return networkBodyCaptureDisabled;
}
// =============================================================================
// CONTEXT ANNOTATION MONITORING
// =============================================================================
/**
 * Measure the serialized byte size of _context in a log entry
 */
export function measureContextSize(entry) {
    const context = entry._context;
    if (!context || typeof context !== 'object')
        return 0;
    const keys = Object.keys(context);
    if (keys.length === 0)
        return 0;
    return JSON.stringify(context).length;
}
/**
 * Check a batch of entries for excessive context annotation usage
 */
export function checkContextAnnotations(entries) {
    const now = Date.now();
    for (const entry of entries) {
        const size = measureContextSize(entry);
        if (size > CONTEXT_SIZE_THRESHOLD) {
            contextExcessiveTimestamps.push({ ts: now, size });
        }
    }
    contextExcessiveTimestamps = contextExcessiveTimestamps.filter((t) => now - t.ts < CONTEXT_WARNING_WINDOW_MS);
    if (contextExcessiveTimestamps.length >= CONTEXT_WARNING_COUNT) {
        const avgSize = contextExcessiveTimestamps.reduce((sum, t) => sum + t.size, 0) / contextExcessiveTimestamps.length;
        contextWarningState = {
            sizeKB: Math.round(avgSize / 1024),
            count: contextExcessiveTimestamps.length,
            triggeredAt: now,
        };
    }
    else if (contextWarningState && contextExcessiveTimestamps.length === 0) {
        contextWarningState = null;
    }
}
/**
 * Get the current context annotation warning state
 */
export function getContextWarning() {
    return contextWarningState;
}
/**
 * Reset the context annotation warning (for testing)
 */
export function resetContextWarning() {
    contextExcessiveTimestamps = [];
    contextWarningState = null;
}
// =============================================================================
// SOURCE MAP HANDLING
// =============================================================================
/**
 * Set source map enabled state
 */
export function setSourceMapEnabled(enabled) {
    sourceMapEnabled = enabled;
}
/**
 * Check if source maps are enabled
 */
export function isSourceMapEnabled() {
    return sourceMapEnabled;
}
/**
 * Set an entry in the source map cache with LRU eviction
 */
export function setSourceMapCacheEntry(url, map) {
    if (!sourceMapCache.has(url) && sourceMapCache.size >= SOURCE_MAP_CACHE_SIZE) {
        const firstKey = sourceMapCache.keys().next().value;
        if (firstKey) {
            sourceMapCache.delete(firstKey);
        }
    }
    sourceMapCache.delete(url);
    sourceMapCache.set(url, map);
}
/**
 * Get an entry from the source map cache
 */
export function getSourceMapCacheEntry(url) {
    return sourceMapCache.get(url) || null;
}
/**
 * Get the current size of the source map cache
 */
export function getSourceMapCacheSize() {
    return sourceMapCache.size;
}
/**
 * Clear the source map cache
 */
export function clearSourceMapCache() {
    sourceMapCache.clear();
}
/**
 * Decode a VLQ-encoded string into an array of integers
 */
export function decodeVLQ(str) {
    const result = [];
    let shift = 0;
    let value = 0;
    for (const char of str) {
        const digit = VLQ_CHAR_MAP.get(char);
        if (digit === undefined) {
            throw new Error(`Invalid VLQ character: ${char}`);
        }
        const continued = digit & 32;
        value += (digit & 31) << shift;
        if (continued) {
            shift += 5;
        }
        else {
            const negate = value & 1;
            value = value >> 1;
            result.push(negate ? -value : value);
            value = 0;
            shift = 0;
        }
    }
    return result;
}
/**
 * Parse a source map's mappings string into a structured format
 */
export function parseMappings(mappingsStr) {
    const lines = mappingsStr.split(';');
    const parsed = [];
    for (const line of lines) {
        const segments = [];
        if (line.length > 0) {
            const segmentStrs = line.split(',');
            for (const segmentStr of segmentStrs) {
                if (segmentStr.length > 0) {
                    segments.push(decodeVLQ(segmentStr));
                }
            }
        }
        parsed.push(segments);
    }
    return parsed;
}
/**
 * Parse a stack trace line into components
 */
export function parseStackFrame(line) {
    const match = line.match(STACK_FRAME_REGEX);
    if (match) {
        const [, functionName, file1, line1, col1, file2, line2] = match;
        return {
            functionName: functionName || '<anonymous>',
            fileName: file1 || file2 || '',
            lineNumber: parseInt(line1 || line2 || '0', 10),
            columnNumber: col1 ? parseInt(col1, 10) : 0,
            raw: line,
        };
    }
    const anonMatch = line.match(ANONYMOUS_FRAME_REGEX);
    if (anonMatch) {
        return {
            functionName: '<anonymous>',
            fileName: anonMatch[1] || '',
            lineNumber: parseInt(anonMatch[2] || '0', 10),
            columnNumber: parseInt(anonMatch[3] || '0', 10),
            raw: line,
        };
    }
    return null;
}
/**
 * Extract sourceMappingURL from script content
 */
export function extractSourceMapUrl(content) {
    const regex = /\/\/[#@]\s*sourceMappingURL=(.+?)(?:\s|$)/;
    const match = content.match(regex);
    return match && match[1] ? match[1].trim() : null;
}
/**
 * Parse source map data into a usable format
 */
export function parseSourceMapData(sourceMap) {
    const mappings = parseMappings(sourceMap.mappings || '');
    return {
        sources: sourceMap.sources || [],
        names: sourceMap.names || [],
        sourceRoot: sourceMap.sourceRoot || '',
        mappings,
        sourcesContent: sourceMap.sourcesContent || [],
    };
}
/**
 * Find original location from source map
 */
export function findOriginalLocation(sourceMap, line, column) {
    if (!sourceMap || !sourceMap.mappings)
        return null;
    const lineIndex = line - 1;
    if (lineIndex < 0 || lineIndex >= sourceMap.mappings.length)
        return null;
    const lineSegments = sourceMap.mappings[lineIndex];
    if (!lineSegments || lineSegments.length === 0)
        return null;
    let genCol = 0;
    let sourceIndex = 0;
    let origLine = 0;
    let origCol = 0;
    let nameIndex = 0;
    let bestMatch = null;
    for (let li = 0; li <= lineIndex; li++) {
        genCol = 0;
        const segments = sourceMap.mappings[li];
        if (!segments)
            continue;
        for (const segment of segments) {
            if (segment.length >= 1)
                genCol += segment[0];
            if (segment.length >= 2)
                sourceIndex += segment[1];
            if (segment.length >= 3)
                origLine += segment[2];
            if (segment.length >= 4)
                origCol += segment[3];
            if (segment.length >= 5)
                nameIndex += segment[4];
            if (li === lineIndex && genCol <= column) {
                bestMatch = {
                    source: sourceMap.sources[sourceIndex] || '',
                    line: origLine + 1,
                    column: origCol,
                    name: segment.length >= 5 ? sourceMap.names[nameIndex] || null : null,
                };
            }
        }
    }
    return bestMatch;
}
/**
 * Fetch a source map for a script URL
 */
export async function fetchSourceMap(scriptUrl, debugLogFn) {
    if (sourceMapCache.has(scriptUrl)) {
        return sourceMapCache.get(scriptUrl) || null;
    }
    try {
        if (sourceMapCache.size >= SOURCE_MAP_CACHE_SIZE) {
            const firstKey = sourceMapCache.keys().next().value;
            if (firstKey) {
                sourceMapCache.delete(firstKey);
            }
        }
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), SOURCE_MAP_FETCH_TIMEOUT);
        const scriptResponse = await fetch(scriptUrl, { signal: controller.signal });
        clearTimeout(timeoutId);
        if (!scriptResponse.ok) {
            sourceMapCache.set(scriptUrl, null);
            return null;
        }
        const scriptContent = await scriptResponse.text();
        let sourceMapUrl = extractSourceMapUrl(scriptContent);
        if (!sourceMapUrl) {
            sourceMapCache.set(scriptUrl, null);
            return null;
        }
        if (sourceMapUrl.startsWith('data:')) {
            const base64Match = sourceMapUrl.match(/^data:application\/json;base64,(.+)$/);
            if (base64Match && base64Match[1]) {
                let jsonStr;
                try {
                    jsonStr = atob(base64Match[1]);
                }
                catch {
                    if (debugLogFn)
                        debugLogFn('sourcemap', 'Invalid base64 in inline source map', { scriptUrl });
                    sourceMapCache.set(scriptUrl, null);
                    return null;
                }
                let sourceMap;
                try {
                    sourceMap = JSON.parse(jsonStr);
                }
                catch {
                    if (debugLogFn)
                        debugLogFn('sourcemap', 'Invalid JSON in inline source map', { scriptUrl });
                    sourceMapCache.set(scriptUrl, null);
                    return null;
                }
                const parsed = parseSourceMapData(sourceMap);
                sourceMapCache.set(scriptUrl, parsed);
                return parsed;
            }
            sourceMapCache.set(scriptUrl, null);
            return null;
        }
        if (!sourceMapUrl.startsWith('http')) {
            const base = scriptUrl.substring(0, scriptUrl.lastIndexOf('/') + 1);
            sourceMapUrl = new URL(sourceMapUrl, base).href;
        }
        const mapController = new AbortController();
        const mapTimeoutId = setTimeout(() => mapController.abort(), SOURCE_MAP_FETCH_TIMEOUT);
        const mapResponse = await fetch(sourceMapUrl, { signal: mapController.signal });
        clearTimeout(mapTimeoutId);
        if (!mapResponse.ok) {
            sourceMapCache.set(scriptUrl, null);
            return null;
        }
        let sourceMap;
        try {
            sourceMap = await mapResponse.json();
        }
        catch {
            if (debugLogFn)
                debugLogFn('sourcemap', 'Invalid JSON in external source map', { scriptUrl, sourceMapUrl });
            sourceMapCache.set(scriptUrl, null);
            return null;
        }
        const parsed = parseSourceMapData(sourceMap);
        sourceMapCache.set(scriptUrl, parsed);
        return parsed;
    }
    catch (err) {
        if (debugLogFn) {
            debugLogFn('sourcemap', 'Source map fetch failed', {
                scriptUrl,
                error: err.message,
            });
        }
        sourceMapCache.set(scriptUrl, null);
        return null;
    }
}
/**
 * Resolve a single stack frame to original location
 */
export async function resolveStackFrame(frame, debugLogFn) {
    if (!frame.fileName || !frame.fileName.startsWith('http')) {
        return frame;
    }
    const sourceMap = await fetchSourceMap(frame.fileName, debugLogFn);
    if (!sourceMap) {
        return frame;
    }
    const original = findOriginalLocation(sourceMap, frame.lineNumber, frame.columnNumber);
    if (!original) {
        return frame;
    }
    return {
        ...frame,
        originalFileName: original.source,
        originalLineNumber: original.line,
        originalColumnNumber: original.column,
        originalFunctionName: original.name || frame.functionName,
        resolved: true,
    };
}
/**
 * Resolve an entire stack trace
 */
export async function resolveStackTrace(stack, debugLogFn) {
    if (!stack || !sourceMapEnabled)
        return stack;
    const lines = stack.split('\n');
    const resolvedLines = [];
    for (const line of lines) {
        const frame = parseStackFrame(line);
        if (!frame) {
            resolvedLines.push(line);
            continue;
        }
        try {
            const resolved = await resolveStackFrame(frame, debugLogFn);
            if (resolved.resolved) {
                const funcName = resolved.originalFunctionName || resolved.functionName;
                const fileName = resolved.originalFileName;
                const lineNum = resolved.originalLineNumber;
                const colNum = resolved.originalColumnNumber;
                resolvedLines.push(`    at ${funcName} (${fileName}:${lineNum}:${colNum}) [resolved from ${resolved.fileName}:${resolved.lineNumber}:${resolved.columnNumber}]`);
            }
            else {
                resolvedLines.push(line);
            }
        }
        catch {
            resolvedLines.push(line);
        }
    }
    return resolvedLines.join('\n');
}
// =============================================================================
// PROCESSING QUERY TRACKING
// =============================================================================
/**
 * Get current state of processing queries (for testing)
 */
export function getProcessingQueriesState() {
    return processingQueries;
}
/**
 * Add a query to the processing set with timestamp
 */
export function addProcessingQuery(queryId, timestamp = Date.now()) {
    processingQueries.set(queryId, timestamp);
}
/**
 * Remove a query from the processing set
 */
export function removeProcessingQuery(queryId) {
    processingQueries.delete(queryId);
}
/**
 * Check if a query is currently being processed
 */
export function isQueryProcessing(queryId) {
    return processingQueries.has(queryId);
}
/**
 * Clean up stale processing queries that have exceeded the TTL
 */
export function cleanupStaleProcessingQueries(debugLogFn) {
    const now = Date.now();
    for (const [queryId, timestamp] of processingQueries) {
        if (now - timestamp > PROCESSING_QUERY_TTL_MS) {
            processingQueries.delete(queryId);
            if (debugLogFn) {
                debugLogFn('connection', 'Cleaned up stale processing query', {
                    queryId,
                    age: Math.round((now - timestamp) / 1000) + 's',
                });
            }
        }
    }
}
// =============================================================================
// DEBUG LOG BUFFER
// =============================================================================
/**
 * Get all debug log entries
 */
export function getDebugLog() {
    return [...debugLogBuffer];
}
/**
 * Add entry to debug log buffer
 */
export function addDebugLogEntry(entry) {
    debugLogBuffer.push(entry);
    if (debugLogBuffer.length > DEBUG_LOG_MAX_ENTRIES) {
        debugLogBuffer.shift();
    }
}
/**
 * Clear debug log buffer
 */
export function clearDebugLog() {
    debugLogBuffer.length = 0;
}
//# sourceMappingURL=state-manager.js.map