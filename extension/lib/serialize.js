// @ts-nocheck
/**
 * @fileoverview Serialization utilities for safe value handling.
 * Provides safe serialization with circular reference detection, DOM element
 * selector generation, and sensitive input detection.
 */

import { MAX_STRING_LENGTH, MAX_DEPTH, SENSITIVE_INPUT_TYPES } from './constants.js'

/**
 * Safely serialize a value, handling circular references and special types
 */
export function safeSerialize(value, depth = 0, seen = new WeakSet()) {
  // Handle null/undefined
  if (value === null) return null
  if (value === undefined) return undefined

  // Handle primitives
  const type = typeof value
  if (type === 'string') {
    if (value.length > MAX_STRING_LENGTH) {
      return value.slice(0, MAX_STRING_LENGTH) + '... [truncated]'
    }
    return value
  }
  if (type === 'number' || type === 'boolean') {
    return value
  }

  // Handle functions
  if (type === 'function') {
    return `[Function: ${value.name || 'anonymous'}]`
  }

  // Handle Error objects specially
  if (value instanceof Error) {
    return {
      name: value.name,
      message: value.message,
      stack: value.stack,
    }
  }

  // Depth limit
  if (depth >= MAX_DEPTH) {
    return '[Max depth exceeded]'
  }

  // Handle objects
  if (type === 'object') {
    // Circular reference check
    if (seen.has(value)) {
      return '[Circular]'
    }
    seen.add(value)

    // Handle DOM elements
    if (value.nodeType) {
      const tag = value.tagName ? value.tagName.toLowerCase() : 'node'
      const id = value.id ? `#${value.id}` : ''
      const className = value.className ? `.${value.className.split(' ').join('.')}` : ''
      return `[${tag}${id}${className}]`
    }

    // Handle arrays (cap at 100 elements to prevent OOM)
    if (Array.isArray(value)) {
      return value.slice(0, 100).map((item) => safeSerialize(item, depth + 1, seen))
    }

    // Handle plain objects (cap at 50 keys to prevent OOM)
    const result = {}
    for (const key of Object.keys(value).slice(0, 50)) {
      try {
        result[key] = safeSerialize(value[key], depth + 1, seen)
      } catch {
        result[key] = '[Unserializable]'
      }
    }
    return result
  }

  return String(value)
}

/**
 * Get element selector for identification
 * @param {Element} element - The DOM element
 * @returns {string} A selector string for the element
 */
export function getElementSelector(element) {
  if (!element || !element.tagName) return ''

  const tag = element.tagName.toLowerCase()
  const id = element.id ? `#${element.id}` : ''
  const classes =
    element.className && typeof element.className === 'string'
      ? '.' + element.className.trim().split(/\s+/).slice(0, 2).join('.')
      : ''

  // Add data-testid if present
  const testId = element.getAttribute('data-testid')
  const testIdStr = testId ? `[data-testid="${testId}"]` : ''

  return `${tag}${id}${classes}${testIdStr}`.slice(0, 100)
}

/**
 * Check if an input contains sensitive data
 * @param {Element} element - The input element
 * @returns {boolean} True if the input is sensitive
 */
export function isSensitiveInput(element) {
  if (!element) return false

  const type = (element.type || '').toLowerCase()
  const autocomplete = (element.autocomplete || '').toLowerCase()
  const name = (element.name || '').toLowerCase()

  // Check type attribute
  if (SENSITIVE_INPUT_TYPES.includes(type)) return true

  // Check autocomplete attribute
  if (autocomplete.includes('password') || autocomplete.includes('cc-') || autocomplete.includes('credit-card'))
    return true

  // Check name attribute for common patterns
  if (
    name.includes('password') ||
    name.includes('passwd') ||
    name.includes('secret') ||
    name.includes('token') ||
    name.includes('credit') ||
    name.includes('card') ||
    name.includes('cvv') ||
    name.includes('cvc') ||
    name.includes('ssn')
  )
    return true

  return false
}
