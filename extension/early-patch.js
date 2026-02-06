"use strict";
// early-patch.ts — Lightweight WebSocket constructor patch.
// Runs in MAIN world at document_start before any page scripts.
// Saves original WebSocket and buffers connections for handoff to inject.bundled.js.
// Must be self-contained: no imports, no chrome.* APIs (MAIN world).
;
(function () {
    'use strict';
    if (typeof window === 'undefined' || !window.WebSocket)
        return;
    // Guard: only install once (extension reloads, multiple frames)
    if (window.__GASOLINE_ORIGINAL_WS__)
        return;
    const OriginalWS = window.WebSocket;
    // Store original for inject script to retrieve
    window.__GASOLINE_ORIGINAL_WS__ = OriginalWS;
    // Buffer for early connections
    const earlyConnections = [];
    window.__GASOLINE_EARLY_WS__ = earlyConnections;
    // Thin wrapper: creates real WebSocket + buffers lifecycle events
    function EarlyWebSocket(url, protocols) {
        const ws = protocols !== undefined ? new OriginalWS(url, protocols) : new OriginalWS(url);
        const conn = { ws, url: url.toString(), createdAt: Date.now(), events: [] };
        ws.addEventListener('open', () => {
            conn.events.push({ type: 'open', ts: Date.now() });
        });
        ws.addEventListener('close', (e) => {
            conn.events.push({ type: 'close', ts: Date.now(), code: e.code, reason: e.reason });
        });
        ws.addEventListener('error', () => {
            conn.events.push({ type: 'error', ts: Date.now() });
        });
        earlyConnections.push(conn);
        // Cap buffer to bound memory
        if (earlyConnections.length > 50) {
            earlyConnections.shift();
        }
        return ws;
    }
    // Preserve prototype chain: instanceof WebSocket still works
    EarlyWebSocket.prototype = OriginalWS.prototype;
    // Preserve static constants
    Object.defineProperty(EarlyWebSocket, 'CONNECTING', { value: 0, writable: false });
    Object.defineProperty(EarlyWebSocket, 'OPEN', { value: 1, writable: false });
    Object.defineProperty(EarlyWebSocket, 'CLOSING', { value: 2, writable: false });
    Object.defineProperty(EarlyWebSocket, 'CLOSED', { value: 3, writable: false });
    window.WebSocket = EarlyWebSocket;
})();
//# sourceMappingURL=early-patch.js.map