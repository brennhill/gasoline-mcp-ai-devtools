/**
 * Purpose: Executes in-page actions and query handlers within the page context.
 * Why: Executes page-context actions safely while preserving deterministic command results.
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/query-dom/index.md
 */
import type { ExecuteJsResult } from '../types/index.js';
export declare function safeSerializeForExecute(value: unknown, depth?: number, seen?: WeakSet<object>): unknown;
/**
 * Execute arbitrary JavaScript in the page context with timeout handling.
 */
export declare function executeJavaScript(script: string, timeoutMs?: number): Promise<ExecuteJsResult>;
//# sourceMappingURL=execute-js.d.ts.map