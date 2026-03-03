/**
 * Purpose: Conversational side panel for in-page chat with AI.
 * Why: Enables bi-directional conversation — user messages, AI responses, and annotations visible in-browser.
 * Docs: docs/features/feature/chat-panel/index.md
 */
/**
 * Toggle the chat panel visibility.
 * If visible, closes it. If hidden, opens it.
 */
export declare function toggleChatPanel(serverUrl: string): void;
/**
 * Inject annotation data into the chat panel as a message.
 * Called when draw mode completes while the panel is open.
 */
export declare function injectAnnotationMessage(data: {
    annotation_count: number;
    details?: string;
}): void;
/** Check if the chat panel is currently open. */
export declare function isChatPanelOpen(): boolean;
//# sourceMappingURL=chat-panel.d.ts.map