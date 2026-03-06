/**
 * Purpose: Dispatches window.postMessage commands from the content script to specialized inject-context handlers (settings, state, JS execution, DOM/a11y queries).
 * Docs: docs/features/feature/interact-explore/index.md
 */
/**
 * @fileoverview Message Handlers - Dispatches messages from content script to
 * specialized modules for settings, state management, JavaScript execution,
 * and DOM/accessibility queries.
 */
import type { BrowserStateSnapshot } from '../types/index.js';
export { executeJavaScript, safeSerializeForExecute } from './execute-js.js';
/**
 * Link health query request message from content script
 */
interface LinkHealthQueryRequestMessageData {
    type: 'gasoline_link_health_query';
    requestId: number | string;
    params?: Record<string, unknown>;
}
/**
 * Handle link health check request from content script
 */
export declare function handleLinkHealthQuery(data: LinkHealthQueryRequestMessageData): Promise<unknown>;
export declare function installMessageListener(captureStateFn: () => BrowserStateSnapshot, restoreStateFn: (state: BrowserStateSnapshot, includeUrl: boolean) => unknown): void;
//# sourceMappingURL=message-handlers.d.ts.map