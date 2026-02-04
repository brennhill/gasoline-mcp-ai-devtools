/**
 * @fileoverview Message Handlers - Handles messages from content script including
 * settings, state management, JavaScript execution, and DOM/accessibility queries.
 */
import type { BrowserStateSnapshot, ExecuteJsResult } from '../types/index';
/**
 * Safe serialization for complex objects returned from executeJavaScript.
 */
export declare function safeSerializeForExecute(value: unknown, depth?: number, seen?: WeakSet<object>): unknown;
/**
 * Execute arbitrary JavaScript in the page context with timeout handling.
 */
export declare function executeJavaScript(script: string, timeoutMs?: number): Promise<ExecuteJsResult>;
/**
 * Install message listener for handling content script messages
 */
export declare function installMessageListener(captureStateFn: () => BrowserStateSnapshot, restoreStateFn: (state: BrowserStateSnapshot, includeUrl: boolean) => unknown): void;
//# sourceMappingURL=message-handlers.d.ts.map