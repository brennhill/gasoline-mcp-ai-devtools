// @ts-nocheck
/**
 * @fileoverview Reproduction script generation and enhanced action recording.
 * Captures user interactions with multi-strategy selectors (testId, role, aria,
 * text, CSS path) and generates Playwright test scripts for reproducing issues.
 */

import {
  ENHANCED_ACTION_BUFFER_SIZE,
  CSS_PATH_MAX_DEPTH,
  SELECTOR_TEXT_MAX_LENGTH,
  SCRIPT_MAX_SIZE,
  CLICKABLE_TAGS,
} from './constants.js'
import { isSensitiveInput } from './serialize.js'

// Enhanced action buffer (separate from v3 action buffer)
let enhancedActionBuffer = []

/**
 * Get the implicit ARIA role for an element
 * @param {Object} element - The DOM element
 * @returns {string|null} The implicit role or null
 */
export function getImplicitRole(element) {
  if (!element || !element.tagName) return null

  const tag = element.tagName.toLowerCase()
  const type = element.getAttribute ? element.getAttribute('type') : null

  switch (tag) {
    case 'button':
      return 'button'
    case 'a':
      return element.getAttribute && element.getAttribute('href') !== null ? 'link' : null
    case 'textarea':
      return 'textbox'
    case 'select':
      return 'combobox'
    case 'nav':
      return 'navigation'
    case 'main':
      return 'main'
    case 'header':
      return 'banner'
    case 'footer':
      return 'contentinfo'
    case 'input': {
      const inputType = type || 'text'
      switch (inputType) {
        case 'text':
        case 'email':
        case 'password':
        case 'tel':
        case 'url':
          return 'textbox'
        case 'checkbox':
          return 'checkbox'
        case 'radio':
          return 'radio'
        case 'search':
          return 'searchbox'
        case 'number':
          return 'spinbutton'
        case 'range':
          return 'slider'
        default:
          return 'textbox'
      }
    }
    default:
      return null
  }
}

/**
 * Detect if a CSS class name is dynamically generated (CSS-in-JS)
 * @param {string} className - The class name to check
 * @returns {boolean} True if the class is likely dynamic
 */
export function isDynamicClass(className) {
  if (!className) return false
  // Known CSS-in-JS prefixes
  if (/^(css|sc|emotion|styled|chakra)-/.test(className)) return true
  // Random hash classes: 5-8 lowercase-only chars
  if (/^[a-z]{5,8}$/.test(className)) return true
  return false
}

/**
 * Compute a CSS path for an element
 * @param {Object} element - The DOM element
 * @returns {string} The CSS path
 */
export function computeCssPath(element) {
  if (!element) return ''

  const parts = []
  let current = element

  while (current && parts.length < CSS_PATH_MAX_DEPTH) {
    let selector = current.tagName ? current.tagName.toLowerCase() : ''

    // Stop at element with ID
    if (current.id) {
      selector = `#${current.id}`
      parts.unshift(selector)
      break
    }

    // Add non-dynamic classes (max 2)
    const classList =
      current.className && typeof current.className === 'string'
        ? current.className
            .trim()
            .split(/\s+/)
            .filter((c) => c && !isDynamicClass(c))
        : []
    if (classList.length > 0) {
      selector += '.' + classList.slice(0, 2).join('.')
    }

    parts.unshift(selector)
    current = current.parentElement
  }

  return parts.join(' > ')
}

/**
 * Compute multi-strategy selectors for an element
 * @param {Object} element - The DOM element
 * @returns {Object} Selector strategies
 */
export function computeSelectors(element) {
  if (!element) return { cssPath: '' }

  const selectors = {}

  // Priority 1: Test ID
  const testId =
    (element.getAttribute &&
      (element.getAttribute('data-testid') ||
        element.getAttribute('data-test-id') ||
        element.getAttribute('data-cy'))) ||
    undefined
  if (testId) selectors.testId = testId

  // Priority 2: ARIA label
  const ariaLabel = element.getAttribute && element.getAttribute('aria-label')
  if (ariaLabel) selectors.ariaLabel = ariaLabel

  // Priority 3: Role + accessible name
  const explicitRole = element.getAttribute && element.getAttribute('role')
  const role = explicitRole || getImplicitRole(element)
  const name = ariaLabel || (element.textContent && element.textContent.trim().slice(0, SELECTOR_TEXT_MAX_LENGTH))
  if (role && name) {
    selectors.role = { role, name: ariaLabel || name }
  }

  // Priority 4: ID
  if (element.id) selectors.id = element.id

  // Priority 5: Text content (for clickable elements only)
  if (element.tagName && CLICKABLE_TAGS.has(element.tagName.toUpperCase())) {
    const text = (element.textContent || element.innerText || '').trim()
    if (text && text.length > 0) {
      selectors.text = text.slice(0, SELECTOR_TEXT_MAX_LENGTH)
    }
  } else if (element.getAttribute && element.getAttribute('role') === 'button') {
    const text = (element.textContent || element.innerText || '').trim()
    if (text && text.length > 0) {
      selectors.text = text.slice(0, SELECTOR_TEXT_MAX_LENGTH)
    }
  }

  // Priority 6: CSS path (always computed as fallback)
  selectors.cssPath = computeCssPath(element)

  return selectors
}

/**
 * Record an enhanced action with multi-strategy selectors
 * @param {string} type - Action type (click, input, keypress, navigate, select, scroll)
 * @param {Object|null} element - The target DOM element
 * @param {Object} opts - Additional options (value, key, fromUrl, toUrl, etc.)
 * @returns {Object} The recorded action
 */
export function recordEnhancedAction(type, element, opts = {}) {
  const action = {
    type,
    timestamp: Date.now(),
    url: typeof window !== 'undefined' && window.location ? window.location.href : '',
  }

  // Compute selectors for element (if provided)
  if (element) {
    action.selectors = computeSelectors(element)
  }

  // Type-specific data
  switch (type) {
    case 'input': {
      const inputType = element && element.getAttribute ? element.getAttribute('type') : 'text'
      action.inputType = inputType || 'text'
      // Redact sensitive values
      if (inputType === 'password' || (element && isSensitiveInput(element))) {
        action.value = '[redacted]'
      } else {
        action.value = opts.value || ''
      }
      break
    }
    case 'keypress':
      action.key = opts.key || ''
      break
    case 'navigate':
      action.fromUrl = opts.fromUrl || ''
      action.toUrl = opts.toUrl || ''
      break
    case 'select':
      action.selectedValue = opts.selectedValue || ''
      action.selectedText = opts.selectedText || ''
      break
    case 'scroll':
      action.scrollY = opts.scrollY || 0
      break
  }

  // Add to buffer
  enhancedActionBuffer.push(action)
  if (enhancedActionBuffer.length > ENHANCED_ACTION_BUFFER_SIZE) {
    enhancedActionBuffer.shift()
  }

  // Emit to content script for server relay
  if (typeof window !== 'undefined' && window.postMessage) {
    window.postMessage({ type: 'GASOLINE_ENHANCED_ACTION', payload: action }, '*')
  }

  return action
}

/**
 * Get the enhanced action buffer
 * @returns {Array} Copy of the enhanced action buffer
 */
export function getEnhancedActionBuffer() {
  return [...enhancedActionBuffer]
}

/**
 * Clear the enhanced action buffer
 */
export function clearEnhancedActionBuffer() {
  enhancedActionBuffer = []
}

/**
 * Generate a Playwright test script from captured actions
 * @param {Array} actions - Array of enhanced action objects
 * @param {Object} opts - Options: { errorMessage, baseUrl, lastNActions }
 * @returns {string} Complete Playwright test script
 */
export function generatePlaywrightScript(actions, opts = {}) {
  const { errorMessage, baseUrl, lastNActions } = opts

  // Apply lastNActions filter
  let filteredActions = actions
  if (lastNActions && lastNActions > 0 && actions.length > lastNActions) {
    filteredActions = actions.slice(-lastNActions)
  }

  // Determine start URL
  let startUrl = ''
  if (filteredActions.length > 0) {
    startUrl = filteredActions[0].url || ''
  }
  if (baseUrl && startUrl) {
    try {
      const parsed = new URL(startUrl)
      startUrl = baseUrl + parsed.pathname
    } catch {
      startUrl = baseUrl
    }
  }

  // Build test name
  const testName = errorMessage ? `reproduction: ${errorMessage.slice(0, 80)}` : 'reproduction: captured user actions'

  // Generate step code
  const steps = []
  let prevTimestamp = null

  for (const action of filteredActions) {
    // Add pause comment for long gaps
    if (prevTimestamp && action.timestamp - prevTimestamp > 2000) {
      const gap = Math.round((action.timestamp - prevTimestamp) / 1000)
      steps.push(`  // [${gap}s pause]`)
    }
    prevTimestamp = action.timestamp

    const locator = getPlaywrightLocator(action.selectors || {})

    switch (action.type) {
      case 'click':
        if (locator) {
          steps.push(`  await page.${locator}.click();`)
        } else {
          steps.push(`  // click action - no selector available`)
        }
        break
      case 'input': {
        const value = action.value === '[redacted]' ? '[user-provided]' : action.value || ''
        if (locator) {
          steps.push(`  await page.${locator}.fill('${escapeString(value)}');`)
        }
        break
      }
      case 'keypress':
        steps.push(`  await page.keyboard.press('${escapeString(action.key || '')}');`)
        break
      case 'navigate': {
        let toUrl = action.toUrl || ''
        if (baseUrl && toUrl) {
          try {
            const parsed = new URL(toUrl)
            toUrl = baseUrl + parsed.pathname
          } catch {
            /* use as-is */
          }
        }
        steps.push(`  await page.waitForURL('${escapeString(toUrl)}');`)
        break
      }
      case 'select':
        if (locator) {
          steps.push(`  await page.${locator}.selectOption('${escapeString(action.selectedValue || '')}');`)
        }
        break
      case 'scroll':
        steps.push(`  // User scrolled to y=${action.scrollY || 0}`)
        break
    }
  }

  // Assemble script
  let script = `import { test, expect } from '@playwright/test';\n\n`
  script += `test('${escapeString(testName)}', async ({ page }) => {\n`
  if (startUrl) {
    script += `  await page.goto('${escapeString(startUrl)}');\n\n`
  }
  script += steps.join('\n')
  if (steps.length > 0) script += '\n'
  if (errorMessage) {
    script += `\n  // Error occurred here: ${errorMessage}\n`
  }
  script += `});\n`

  // Cap output size
  if (script.length > SCRIPT_MAX_SIZE) {
    script = script.slice(0, SCRIPT_MAX_SIZE)
  }

  return script
}

/**
 * Get the best Playwright locator for a set of selectors
 * Priority: testId > role > ariaLabel > text > id > cssPath
 * @param {Object} selectors - The selector strategies
 * @returns {string} The Playwright locator expression
 */
function getPlaywrightLocator(selectors) {
  if (selectors.testId) {
    return `getByTestId('${escapeString(selectors.testId)}')`
  }
  if (selectors.role && selectors.role.role) {
    if (selectors.role.name) {
      return `getByRole('${escapeString(selectors.role.role)}', { name: '${escapeString(selectors.role.name)}' })`
    }
    return `getByRole('${escapeString(selectors.role.role)}')`
  }
  if (selectors.ariaLabel) {
    return `getByLabel('${escapeString(selectors.ariaLabel)}')`
  }
  if (selectors.text) {
    return `getByText('${escapeString(selectors.text)}')`
  }
  if (selectors.id) {
    return `locator('#${escapeString(selectors.id)}')`
  }
  if (selectors.cssPath) {
    return `locator('${escapeString(selectors.cssPath)}')`
  }
  return null
}

/**
 * Escape a string for use in JavaScript string literals
 * @param {string} str - The string to escape
 * @returns {string} Escaped string
 */
function escapeString(str) {
  if (!str) return ''
  return str.replace(/\\/g, '\\\\').replace(/'/g, "\\'").replace(/\n/g, '\\n')
}
