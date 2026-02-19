import type { ExecuteJsResult } from '../types/index';
export declare function safeSerializeForExecute(value: unknown, depth?: number, seen?: WeakSet<object>): unknown;
/**
 * Execute arbitrary JavaScript in the page context with timeout handling.
 */
export declare function executeJavaScript(script: string, timeoutMs?: number): Promise<ExecuteJsResult>;
//# sourceMappingURL=execute-js.d.ts.map