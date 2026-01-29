/**
 * @fileoverview Type Index - Barrel export for all Gasoline Extension types
 *
 * This is the single entry point for importing types in the extension.
 * Usage: import type { LogEntry, BackgroundMessage } from './types.js';
 */
// Re-export type guards from utils
export { isObject, isNonEmptyString, hasType, isJsonValue, createTypeGuard, } from './utils.js';
//# sourceMappingURL=index.js.map