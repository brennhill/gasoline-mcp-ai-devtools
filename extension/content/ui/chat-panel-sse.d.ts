/**
 * Purpose: SSE client for real-time chat message streaming.
 * Why: Receives AI responses and annotation messages from the daemon via Server-Sent Events.
 * Docs: docs/features/feature/chat-panel/index.md
 */
/** Message type matching the Go ChatMessage struct. */
export interface ChatMessage {
    role: 'user' | 'assistant' | 'annotation';
    text: string;
    timestamp: number;
    conversation_id: string;
    annotations?: unknown[];
}
/** SSE connection handle. */
export interface SSEConnection {
    close: () => void;
}
/**
 * Connect to the chat SSE stream endpoint.
 * Sends existing history as an initial burst, then streams new messages.
 * Auto-reconnects on disconnect.
 */
export declare function connectChatStream(serverUrl: string, conversationId: string, onMessage: (msg: ChatMessage) => void, onHistory: (msgs: ChatMessage[]) => void, onError: (err: string) => void): SSEConnection;
//# sourceMappingURL=chat-panel-sse.d.ts.map