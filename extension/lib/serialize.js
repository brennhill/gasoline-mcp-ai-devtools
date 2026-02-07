/**
 * @fileoverview Serialization utilities for safe value handling.
 * Provides safe serialization with circular reference detection, DOM element
 * selector generation, and sensitive input detection.
 *
 * NOTE: This module has NO mutable state. All functions are pure and stateless.
 * No resetForTesting() function is needed.
 */
import { MAX_STRING_LENGTH, MAX_DEPTH, SENSITIVE_INPUT_TYPES } from './constants.js';
/**
 * Safely serialize a value, handling circular references and special types
 */
export function safeSerialize(value, depth = 0, seen = new WeakSet()) {
    // Handle null/undefined
    if (value === null)
        return null;
    if (value === undefined)
        return null;
    // Handle primitives
    const type = typeof value;
    if (type === 'string') {
        const strValue = value;
        if (strValue.length > MAX_STRING_LENGTH) {
            return strValue.slice(0, MAX_STRING_LENGTH) + '... [truncated]';
        }
        return strValue;
    }
    if (type === 'number') {
        return value;
    }
    if (type === 'boolean') {
        return value;
    }
    // Handle functions
    if (type === 'function') {
        const fn = value;
        return `[Function: ${fn.name || 'anonymous'}]`;
    }
    // Handle Error objects specially
    if (value instanceof Error) {
        return {
            name: value.name,
            message: value.message,
            stack: value.stack || null,
        };
    }
    // Depth limit
    if (depth >= MAX_DEPTH) {
        return '[Max depth exceeded]';
    }
    // Handle objects
    if (type === 'object') {
        const objValue = value;
        // Circular reference check
        if (seen.has(objValue)) {
            return '[Circular]';
        }
        seen.add(objValue);
        // Handle DOM elements
        const domLike = value;
        if (domLike.nodeType) {
            const tag = domLike.tagName ? domLike.tagName.toLowerCase() : 'node';
            const id = domLike.id ? `#${domLike.id}` : '';
            const classNameValue = domLike.className;
            let className = '';
            if (typeof classNameValue === 'string' && classNameValue) {
                className = `.${classNameValue.split(' ').join('.')}`;
            }
            return `[${tag}${id}${className}]`;
        }
        // Handle arrays (cap at 100 elements to prevent OOM)
        if (Array.isArray(value)) {
            return value.slice(0, 100).map((item) => safeSerialize(item, depth + 1, seen));
        }
        // Handle plain objects (cap at 50 keys to prevent OOM)
        const result = {};
        for (const key of Object.keys(objValue).slice(0, 50)) {
            try {
                result[key] = safeSerialize(objValue[key], depth + 1, seen);
            }
            catch {
                result[key] = '[Unserializable]';
            }
        }
        return result;
    }
    return String(value);
}
/**
 * Get element selector for identification
 */
export function getElementSelector(element) {
    if (!element || !element.tagName)
        return '';
    const tag = element.tagName.toLowerCase();
    const id = element.id ? `#${element.id}` : '';
    let classes = '';
    const classNameValue = element.className;
    if (classNameValue && typeof classNameValue === 'string') {
        classes = '.' + classNameValue.trim().split(/\s+/).slice(0, 2).join('.');
    }
    // Add data-testid if present
    const testId = element.getAttribute('data-testid');
    const testIdStr = testId ? `[data-testid="${testId}"]` : '';
    return `${tag}${id}${classes}${testIdStr}`.slice(0, 100);
}
/**
 * Check if an input contains sensitive data
 */
export function isSensitiveInput(element) {
    if (!element)
        return false;
    const inputElement = element;
    const type = (inputElement.type || '').toLowerCase();
    const autocomplete = (inputElement.autocomplete || '').toLowerCase();
    const name = (inputElement.name || '').toLowerCase();
    // Check type attribute
    if (SENSITIVE_INPUT_TYPES.includes(type))
        return true;
    // Check autocomplete attribute
    if (autocomplete.includes('password') || autocomplete.includes('cc-') || autocomplete.includes('credit-card'))
        return true;
    // Check name attribute for common patterns
    if (name.includes('password') ||
        name.includes('passwd') ||
        name.includes('secret') ||
        name.includes('token') ||
        name.includes('credit') ||
        name.includes('card') ||
        name.includes('cvv') ||
        name.includes('cvc') ||
        name.includes('ssn'))
        return true;
    return false;
}
//# sourceMappingURL=serialize.js.map