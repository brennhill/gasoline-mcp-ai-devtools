/**
 * Purpose: Provides shared runtime utilities used by extension and server workflows.
 * Why: Avoids duplicated logic across runtime layers and keeps behavior consistent.
 * Docs: docs/features/feature/observe/index.md
 */
import type { JsonValue } from '../types/index.js';
/**
 * Safely serialize a value, handling circular references and special types
 */
export declare function safeSerialize(value: unknown, depth?: number, seen?: WeakSet<object>): JsonValue;
/**
 * Get element selector for identification
 */
export declare function getElementSelector(element: Element | null): string;
export declare function isSensitiveInput(element: Element | null): boolean;
//# sourceMappingURL=serialize.d.ts.map