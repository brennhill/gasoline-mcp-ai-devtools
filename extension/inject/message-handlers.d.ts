/**
 * @fileoverview Message Handlers - Handles messages from content script including
 * settings, state management, JavaScript execution, and DOM/accessibility queries.
 */
import type { BrowserStateSnapshot, ExecuteJsResult } from '../types/index';
/**
 * Link health query request message from content script
 */
interface LinkHealthQueryRequestMessageData {
    type: 'GASOLINE_LINK_HEALTH_QUERY';
    requestId: number | string;
    params?: Record<string, unknown>;
}
export declare function safeSerializeForExecute(value: unknown, depth?: number, seen?: WeakSet<object>): unknown;
/**
 * Execute arbitrary JavaScript in the page context with timeout handling.
 */
export declare function executeJavaScript(script: string, timeoutMs?: number): Promise<ExecuteJsResult>;
/**
 * Handle link health check request from content script
 */
export declare function handleLinkHealthQuery(data: LinkHealthQueryRequestMessageData): Promise<unknown>;
export declare function installMessageListener(captureStateFn: () => BrowserStateSnapshot, restoreStateFn: (state: BrowserStateSnapshot, includeUrl: boolean) => unknown): void;
export {};
//# sourceMappingURL=message-handlers.d.ts.map