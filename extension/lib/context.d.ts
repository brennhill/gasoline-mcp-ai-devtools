/**
 * Purpose: Provides shared runtime utilities used by extension and server workflows.
 * Docs: docs/features/feature/observe/index.md
 */
import type { JsonValue } from '../types/index';
/**
 * Get current context annotations as an object
 */
export declare function getContextAnnotations(): Record<string, JsonValue> | null;
/**
 * Set a context annotation
 */
export declare function setContextAnnotation(key: string, value: unknown): boolean;
/**
 * Remove a context annotation
 */
export declare function removeContextAnnotation(key: string): boolean;
/**
 * Clear all context annotations
 */
export declare function clearContextAnnotations(): void;
//# sourceMappingURL=context.d.ts.map