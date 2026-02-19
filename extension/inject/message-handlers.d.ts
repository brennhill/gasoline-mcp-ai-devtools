/**
 * @fileoverview Message Handlers - Dispatches messages from content script to
 * specialized modules for settings, state management, JavaScript execution,
 * and DOM/accessibility queries.
 */
import type { BrowserStateSnapshot } from '../types/index';
export { executeJavaScript, safeSerializeForExecute } from './execute-js';
/**
 * Link health query request message from content script
 */
interface LinkHealthQueryRequestMessageData {
    type: 'GASOLINE_LINK_HEALTH_QUERY';
    requestId: number | string;
    params?: Record<string, unknown>;
}
/**
 * Handle link health check request from content script
 */
export declare function handleLinkHealthQuery(data: LinkHealthQueryRequestMessageData): Promise<unknown>;
export declare function installMessageListener(captureStateFn: () => BrowserStateSnapshot, restoreStateFn: (state: BrowserStateSnapshot, includeUrl: boolean) => unknown): void;
//# sourceMappingURL=message-handlers.d.ts.map