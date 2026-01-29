/**
 * @fileoverview WebSocket capture.
 * Wraps the WebSocket constructor to intercept lifecycle events and messages,
 * with adaptive sampling, schema detection, and truncation.
 */
import { WS_MAX_BODY_SIZE, WS_PREVIEW_LIMIT } from './constants.js';
// Cached TextEncoder instance to avoid per-call allocation in getSize() hot path
const _textEncoder = typeof TextEncoder !== 'undefined' ? new TextEncoder() : null;
// WebSocket capture state
let originalWebSocket = null;
let webSocketCaptureEnabled = false;
let webSocketCaptureMode = 'lifecycle';
/**
 * Get the byte size of a WebSocket message
 */
export function getSize(data) {
    if (typeof data === 'string') {
        return _textEncoder ? _textEncoder.encode(data).length : data.length;
    }
    if (data instanceof ArrayBuffer)
        return data.byteLength;
    if (data && typeof data === 'object' && 'size' in data)
        return data.size;
    return 0;
}
/**
 * Format a WebSocket payload for logging
 */
export function formatPayload(data) {
    if (typeof data === 'string')
        return data;
    if (data instanceof ArrayBuffer) {
        const bytes = new Uint8Array(data);
        if (data.byteLength < 256) {
            // Small binary: hex preview
            let hex = '';
            for (let i = 0; i < bytes.length; i++) {
                const byte = bytes[i];
                if (byte !== undefined) {
                    hex += byte.toString(16).padStart(2, '0');
                }
            }
            return `[Binary: ${data.byteLength}B] ${hex}`;
        }
        else {
            // Large binary: size + magic bytes (first 4 bytes)
            let magic = '';
            for (let i = 0; i < Math.min(4, bytes.length); i++) {
                const byte = bytes[i];
                if (byte !== undefined) {
                    magic += byte.toString(16).padStart(2, '0');
                }
            }
            return `[Binary: ${data.byteLength}B, magic:${magic}]`;
        }
    }
    // Blob or Blob-like
    if (data && typeof data === 'object' && 'size' in data) {
        return `[Binary: ${data.size}B]`;
    }
    return String(data);
}
/**
 * Truncate a WebSocket message to the size limit
 */
export function truncateWsMessage(message) {
    if (typeof message === 'string' && message.length > WS_MAX_BODY_SIZE) {
        return { data: message.slice(0, WS_MAX_BODY_SIZE), truncated: true };
    }
    return { data: message, truncated: false };
}
/**
 * Create a connection tracker for adaptive sampling and schema detection
 */
export function createConnectionTracker(id, url) {
    const tracker = {
        id,
        url,
        messageCount: 0,
        _sampleCounter: 0,
        _messageRate: 0,
        _messageTimestamps: [],
        _schemaKeys: [],
        _schemaVariants: new Map(),
        _schemaConsistent: true,
        _schemaDetected: false,
        stats: {
            incoming: { count: 0, bytes: 0, lastPreview: null, lastAt: null },
            outgoing: { count: 0, bytes: 0, lastPreview: null, lastAt: null },
        },
        /**
         * Record a message for stats and schema detection
         */
        recordMessage(direction, data) {
            this.messageCount++;
            const size = data ? (typeof data === 'string' ? data.length : getSize(data)) : 0;
            const now = Date.now();
            this.stats[direction].count++;
            this.stats[direction].bytes += size;
            this.stats[direction].lastAt = now;
            if (data && typeof data === 'string') {
                this.stats[direction].lastPreview = data.length > WS_PREVIEW_LIMIT ? data.slice(0, WS_PREVIEW_LIMIT) : data;
            }
            // Track timestamps for rate calculation
            this._messageTimestamps.push(now);
            // Keep only last 5 seconds
            const cutoff = now - 5000;
            this._messageTimestamps = this._messageTimestamps.filter((t) => t >= cutoff);
            // Schema detection from first 5 incoming JSON messages
            if (direction === 'incoming' && data && typeof data === 'string' && this._schemaKeys.length < 5) {
                try {
                    const parsed = JSON.parse(data);
                    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
                        const keys = Object.keys(parsed).sort();
                        const keyStr = keys.join(',');
                        this._schemaKeys.push(keyStr);
                        // Track variants
                        this._schemaVariants.set(keyStr, (this._schemaVariants.get(keyStr) || 0) + 1);
                        // Check consistency after 2+ messages
                        if (this._schemaKeys.length >= 2) {
                            const first = this._schemaKeys[0];
                            this._schemaConsistent = this._schemaKeys.every((k) => k === first);
                        }
                        if (this._schemaKeys.length >= 5) {
                            this._schemaDetected = true;
                        }
                    }
                }
                catch {
                    // Not JSON, no schema
                }
            }
            // Track variants for messages beyond the first 5 (cap at 50 to bound memory)
            if (direction === 'incoming' && data && typeof data === 'string' && this._schemaDetected) {
                try {
                    const parsed = JSON.parse(data);
                    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
                        const keys = Object.keys(parsed).sort();
                        const keyStr = keys.join(',');
                        // Only add new variants if under cap; always increment existing
                        if (this._schemaVariants.has(keyStr) || this._schemaVariants.size < 50) {
                            this._schemaVariants.set(keyStr, (this._schemaVariants.get(keyStr) || 0) + 1);
                        }
                    }
                }
                catch {
                    // Not JSON
                }
            }
        },
        /**
         * Determine if a message should be sampled (logged)
         */
        shouldSample(_direction) {
            this._sampleCounter++;
            // Always log first 5 messages on a connection
            if (this.messageCount > 0 && this.messageCount <= 5)
                return true;
            const rate = this._messageRate || this.getMessageRate();
            if (rate < 10)
                return true;
            if (rate < 50) {
                // Target ~10 msg/s
                const n = Math.max(1, Math.round(rate / 10));
                return this._sampleCounter % n === 0;
            }
            if (rate < 200) {
                // Target ~5 msg/s
                const n = Math.max(1, Math.round(rate / 5));
                return this._sampleCounter % n === 0;
            }
            // > 200: target ~2 msg/s
            const n = Math.max(1, Math.round(rate / 2));
            return this._sampleCounter % n === 0;
        },
        /**
         * Lifecycle events should always be logged
         */
        shouldLogLifecycle() {
            return true;
        },
        /**
         * Get sampling info
         */
        getSamplingInfo() {
            const rate = this._messageRate || this.getMessageRate();
            let targetRate = rate;
            if (rate >= 10 && rate < 50)
                targetRate = 10;
            else if (rate >= 50 && rate < 200)
                targetRate = 5;
            else if (rate >= 200)
                targetRate = 2;
            return {
                rate: `${rate}/s`,
                logged: `${targetRate}/${Math.round(rate)}`,
                window: '5s',
            };
        },
        /**
         * Get the current message rate (messages per second)
         */
        getMessageRate() {
            if (this._messageTimestamps.length < 2)
                return this._messageTimestamps.length;
            const lastTime = this._messageTimestamps[this._messageTimestamps.length - 1];
            const firstTime = this._messageTimestamps[0];
            if (lastTime === undefined || firstTime === undefined)
                return this._messageTimestamps.length;
            const window = (lastTime - firstTime) / 1000;
            return window > 0 ? this._messageTimestamps.length / window : this._messageTimestamps.length;
        },
        /**
         * Set the message rate manually (for testing)
         */
        setMessageRate(rate) {
            this._messageRate = rate;
        },
        /**
         * Get the detected schema info
         */
        getSchema() {
            if (this._schemaKeys.length === 0) {
                return { detectedKeys: null, consistent: true };
            }
            // Get union of all detected keys
            const allKeys = new Set();
            for (const keyStr of this._schemaKeys) {
                for (const k of keyStr.split(',')) {
                    if (k)
                        allKeys.add(k);
                }
            }
            // Build variants list
            const variants = [];
            for (const [keyStr, count] of this._schemaVariants) {
                if (count > 0)
                    variants.push(keyStr);
            }
            return {
                detectedKeys: allKeys.size > 0 ? Array.from(allKeys).sort() : null,
                consistent: this._schemaConsistent,
                variants: variants.length > 1 ? variants : undefined,
            };
        },
        /**
         * Check if a message represents a schema change
         */
        isSchemaChange(data) {
            if (!this._schemaDetected || !data || typeof data !== 'string')
                return false;
            try {
                const parsed = JSON.parse(data);
                if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed))
                    return false;
                const keys = Object.keys(parsed).sort().join(',');
                // It's a change if none of the first 5 schemas match
                return !this._schemaKeys.includes(keys);
            }
            catch {
                return false;
            }
        },
    };
    return tracker;
}
/**
 * Install WebSocket capture by wrapping the WebSocket constructor
 */
export function installWebSocketCapture() {
    if (typeof window === 'undefined')
        return;
    if (!window.WebSocket)
        return; // No WebSocket support
    if (originalWebSocket)
        return; // Already installed
    originalWebSocket = window.WebSocket;
    const OriginalWS = window.WebSocket;
    function GasolineWebSocket(url, protocols) {
        const ws = new OriginalWS(url, protocols);
        const connectionId = crypto.randomUUID();
        const urlString = url.toString();
        ws.addEventListener('open', () => {
            if (!webSocketCaptureEnabled)
                return;
            window.postMessage({
                type: 'GASOLINE_WS',
                payload: { type: 'websocket', event: 'open', id: connectionId, url: urlString, ts: new Date().toISOString() },
            }, '*');
        });
        ws.addEventListener('close', (event) => {
            if (!webSocketCaptureEnabled)
                return;
            window.postMessage({
                type: 'GASOLINE_WS',
                payload: {
                    type: 'websocket',
                    event: 'close',
                    id: connectionId,
                    url: urlString,
                    code: event.code,
                    reason: event.reason,
                    ts: new Date().toISOString(),
                },
            }, '*');
        });
        ws.addEventListener('error', () => {
            if (!webSocketCaptureEnabled)
                return;
            window.postMessage({
                type: 'GASOLINE_WS',
                payload: { type: 'websocket', event: 'error', id: connectionId, url: urlString, ts: new Date().toISOString() },
            }, '*');
        });
        ws.addEventListener('message', (event) => {
            if (!webSocketCaptureEnabled)
                return;
            if (webSocketCaptureMode !== 'messages')
                return;
            const data = event.data;
            const size = getSize(data);
            const formatted = formatPayload(data);
            const { data: truncatedData, truncated } = truncateWsMessage(formatted);
            window.postMessage({
                type: 'GASOLINE_WS',
                payload: {
                    type: 'websocket',
                    event: 'message',
                    id: connectionId,
                    url: urlString,
                    direction: 'incoming',
                    data: truncatedData,
                    size,
                    truncated: truncated || undefined,
                    ts: new Date().toISOString(),
                },
            }, '*');
        });
        // Wrap send() to capture outgoing messages
        const originalSend = ws.send.bind(ws);
        ws.send = function (data) {
            if (webSocketCaptureEnabled && webSocketCaptureMode === 'messages') {
                const size = getSize(data);
                const formatted = formatPayload(data);
                const { data: truncatedData, truncated } = truncateWsMessage(formatted);
                window.postMessage({
                    type: 'GASOLINE_WS',
                    payload: {
                        type: 'websocket',
                        event: 'message',
                        id: connectionId,
                        url: urlString,
                        direction: 'outgoing',
                        data: truncatedData,
                        size,
                        truncated: truncated || undefined,
                        ts: new Date().toISOString(),
                    },
                }, '*');
            }
            return originalSend(data);
        };
        return ws;
    }
    // Set up prototype chain and static properties
    GasolineWebSocket.prototype = OriginalWS.prototype;
    Object.defineProperty(GasolineWebSocket, 'CONNECTING', { value: OriginalWS.CONNECTING, writable: false });
    Object.defineProperty(GasolineWebSocket, 'OPEN', { value: OriginalWS.OPEN, writable: false });
    Object.defineProperty(GasolineWebSocket, 'CLOSING', { value: OriginalWS.CLOSING, writable: false });
    Object.defineProperty(GasolineWebSocket, 'CLOSED', { value: OriginalWS.CLOSED, writable: false });
    window.WebSocket = GasolineWebSocket;
}
/**
 * Set the WebSocket capture mode
 */
export function setWebSocketCaptureMode(mode) {
    webSocketCaptureMode = mode;
}
/**
 * Set WebSocket capture enabled state
 */
export function setWebSocketCaptureEnabled(enabled) {
    webSocketCaptureEnabled = enabled;
}
/**
 * Get the current WebSocket capture mode
 */
export function getWebSocketCaptureMode() {
    return webSocketCaptureMode;
}
/**
 * Uninstall WebSocket capture, restoring the original constructor
 */
export function uninstallWebSocketCapture() {
    if (typeof window === 'undefined')
        return;
    if (originalWebSocket) {
        window.WebSocket = originalWebSocket;
        originalWebSocket = null;
    }
}
/**
 * Reset all module state for testing purposes
 * Restores original WebSocket if installed, resets capture settings to defaults.
 * Call this in beforeEach/afterEach test hooks to prevent test pollution.
 */
export function resetForTesting() {
    uninstallWebSocketCapture();
    webSocketCaptureEnabled = false;
    webSocketCaptureMode = 'lifecycle';
    originalWebSocket = null;
}
//# sourceMappingURL=websocket.js.map