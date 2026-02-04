/**
 * @fileoverview Serialization utilities for safe value handling.
 * Provides safe serialization with circular reference detection, DOM element
 * selector generation, and sensitive input detection.
 *
 * NOTE: This module has NO mutable state. All functions are pure and stateless.
 * No resetForTesting() function is needed.
 */
import type { JsonValue } from '../types/index';
/**
 * Safely serialize a value, handling circular references and special types
 */
export declare function safeSerialize(value: unknown, depth?: number, seen?: WeakSet<object>): JsonValue;
/**
 * Get element selector for identification
 */
export declare function getElementSelector(element: Element | null): string;
/**
 * Check if an input contains sensitive data
 */
export declare function isSensitiveInput(element: Element | null): boolean;
//# sourceMappingURL=serialize.d.ts.map