/**
 * Purpose: SSE client for real-time chat message streaming.
 * Why: Receives AI responses and annotation messages from the daemon via Server-Sent Events.
 * Docs: docs/features/feature/chat-panel/index.md
 */
// chat-panel-sse.ts — SSE client using fetch + ReadableStream for custom headers.
import { getRequestHeaders } from '../../background/server.js';
/**
 * Connect to the chat SSE stream endpoint.
 * Sends existing history as an initial burst, then streams new messages.
 * Auto-reconnects on disconnect.
 */
export function connectChatStream(serverUrl, conversationId, onMessage, onHistory, onError) {
    let abortController = null;
    let closed = false;
    let reconnectTimer = null;
    function connect() {
        if (closed)
            return;
        abortController = new AbortController();
        const url = `${serverUrl}/chat/stream?conversation_id=${encodeURIComponent(conversationId)}`;
        fetch(url, {
            method: 'GET',
            headers: {
                ...getRequestHeaders(),
                Accept: 'text/event-stream'
            },
            signal: abortController.signal
        })
            .then((response) => {
            if (!response.ok) {
                onError(`SSE connection failed: HTTP ${response.status}`);
                scheduleReconnect();
                return;
            }
            if (!response.body) {
                onError('SSE connection: no response body');
                scheduleReconnect();
                return;
            }
            readSSEStream(response.body);
        })
            .catch((err) => {
            if (closed || err.name === 'AbortError')
                return;
            onError(`SSE connection error: ${err.message}`);
            scheduleReconnect();
        });
    }
    function readSSEStream(body) {
        const reader = body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        function processChunk() {
            reader
                .read()
                .then(({ done, value }) => {
                if (done || closed) {
                    if (!closed)
                        scheduleReconnect();
                    return;
                }
                buffer += decoder.decode(value, { stream: true });
                // Split on double-newline (SSE event boundary)
                const events = buffer.split('\n\n');
                // Keep the last incomplete chunk in the buffer
                buffer = events.pop() ?? '';
                for (const event of events) {
                    if (!event.trim())
                        continue;
                    parseSSEEvent(event);
                }
                processChunk();
            })
                .catch((err) => {
                if (closed || err.name === 'AbortError')
                    return;
                onError(`SSE read error: ${err.message}`);
                scheduleReconnect();
            });
        }
        processChunk();
    }
    function parseSSEEvent(raw) {
        let eventType = '';
        let data = '';
        for (const line of raw.split('\n')) {
            if (line.startsWith('event: ')) {
                eventType = line.slice(7);
            }
            else if (line.startsWith('data: ')) {
                data = line.slice(6);
            }
            else if (line.startsWith(':')) {
                // Comment (heartbeat), ignore
            }
        }
        if (!eventType || !data)
            return;
        try {
            if (eventType === 'history') {
                const msgs = JSON.parse(data);
                onHistory(msgs);
            }
            else if (eventType === 'message') {
                const msg = JSON.parse(data);
                onMessage(msg);
            }
        }
        catch {
            // Malformed JSON, skip
        }
    }
    function scheduleReconnect() {
        if (closed || reconnectTimer)
            return;
        reconnectTimer = setTimeout(() => {
            reconnectTimer = null;
            connect();
        }, 1000);
    }
    // Start connection
    connect();
    return {
        close() {
            closed = true;
            if (abortController)
                abortController.abort();
            if (reconnectTimer) {
                clearTimeout(reconnectTimer);
                reconnectTimer = null;
            }
        }
    };
}
//# sourceMappingURL=chat-panel-sse.js.map