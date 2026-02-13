// dom-primitives.ts — Pre-compiled DOM interaction functions for chrome.scripting.executeScript.
// These bypass CSP restrictions because they use the `func` parameter (no eval/new Function).
// Each function MUST be self-contained — no closures over external variables.

import type { PendingQuery } from '../types/queries'
import type { SyncClient } from './sync-client'

// Result shape returned by domPrimitive (compile-time only — erased at runtime)
interface DOMResult {
  success: boolean
  action: string
  selector: string
  value?: unknown
  error?: string
  message?: string
  dom_summary?: string
  timing?: { total_ms: number }
  dom_changes?: { added: number; removed: number; modified: number; summary: string }
  analysis?: string
}

type SendAsyncResult = (
  syncClient: SyncClient,
  queryId: string,
  correlationId: string,
  status: 'complete' | 'error' | 'timeout',
  result?: unknown,
  error?: string
) => void

type ActionToast = (
  tabId: number,
  text: string,
  detail?: string,
  state?: 'trying' | 'success' | 'warning' | 'error',
  durationMs?: number
) => void

/**
 * Single self-contained function for all DOM primitives.
 * Passed to chrome.scripting.executeScript({ func: domPrimitive, args: [...] }).
 * MUST NOT reference any module-level variables — Chrome serializes the function source only.
 */
export function domPrimitive(
  action: string,
  selector: string,
  options: {
    text?: string
    value?: string
    clear?: boolean
    checked?: boolean
    name?: string
    timeout_ms?: number
    analyze?: boolean
  }
): DOMResult | Promise<DOMResult> | { success: boolean; elements: unknown[] } {
  // ---------------------------------------------------------------
  // Selector resolver: CSS or semantic (text=, role=, placeholder=, label=, aria-label=)
  // All semantic selectors prefer visible elements over hidden ones.
  // ---------------------------------------------------------------

  // Visibility check: skip display:none, visibility:hidden, zero-size elements
  function isVisible(el: Element): boolean {
    if (!(el instanceof HTMLElement)) return true
    if (el.offsetParent === null && el.style.position !== 'fixed' && el.style.position !== 'sticky') return false
    const style = getComputedStyle(el)
    if (style.visibility === 'hidden' || style.display === 'none') return false
    return true
  }

  // Return first visible match from a NodeList, falling back to first match
  function firstVisible(els: NodeListOf<Element>): Element | null {
    let fallback: Element | null = null
    for (const el of els) {
      if (!fallback) fallback = el
      if (isVisible(el)) return el
    }
    return fallback
  }

  function resolveByText(searchText: string): Element | null {
    const walker = document.createTreeWalker(document.body || document.documentElement, NodeFilter.SHOW_TEXT)
    let fallback: Element | null = null
    while (walker.nextNode()) {
      const node = walker.currentNode
      if (node.textContent && node.textContent.trim().includes(searchText)) {
        const parent = node.parentElement
        if (!parent) continue
        const interactive = parent.closest('a, button, [role="button"], [role="link"], label, summary')
        const target = interactive || parent
        if (!fallback) fallback = target
        if (isVisible(target)) return target
      }
    }
    return fallback
  }

  function resolveByLabel(labelText: string): Element | null {
    const labels = document.querySelectorAll('label')
    for (const label of labels) {
      if (label.textContent && label.textContent.trim().includes(labelText)) {
        const forAttr = label.getAttribute('for')
        if (forAttr) {
          const target = document.getElementById(forAttr)
          if (target) return target
        }
        const nested = label.querySelector('input, select, textarea')
        if (nested) return nested
        return label
      }
    }
    return null
  }

  function resolveByAriaLabel(al: string): Element | null {
    const exact = document.querySelectorAll(`[aria-label="${CSS.escape(al)}"]`)
    if (exact.length > 0) return firstVisible(exact)
    const all = document.querySelectorAll('[aria-label]')
    let fallback: Element | null = null
    for (const el of all) {
      const label = el.getAttribute('aria-label') || ''
      if (label.startsWith(al)) {
        if (!fallback) fallback = el
        if (isVisible(el)) return el
      }
    }
    return fallback
  }

  // Semantic selector prefix resolvers
  const selectorResolvers: [string, (value: string) => Element | null][] = [
    ['text=', (v) => resolveByText(v)],
    ['role=', (v) => firstVisible(document.querySelectorAll(`[role="${CSS.escape(v)}"]`))],
    ['placeholder=', (v) => firstVisible(document.querySelectorAll(`[placeholder="${CSS.escape(v)}"]`))],
    ['label=', (v) => resolveByLabel(v)],
    ['aria-label=', (v) => resolveByAriaLabel(v)]
  ]

  function resolveElement(sel: string): Element | null {
    if (!sel) return null

    for (const [prefix, resolver] of selectorResolvers) {
      if (sel.startsWith(prefix)) return resolver(sel.slice(prefix.length))
    }

    return document.querySelector(sel)
  }

  function buildUniqueSelector(el: Element, htmlEl: HTMLElement, fallbackSelector: string): string {
    if (el.id) return `#${el.id}`
    if (el instanceof HTMLInputElement && el.name) return `input[name="${el.name}"]`
    const ariaLabel = el.getAttribute('aria-label')
    if (ariaLabel) return `aria-label=${ariaLabel}`
    const placeholder = el.getAttribute('placeholder')
    if (placeholder) return `placeholder=${placeholder}`
    const text = (htmlEl.textContent || '').trim().slice(0, 40)
    if (text) return `text=${text}`
    return fallbackSelector
  }

  // ---------------------------------------------------------------
  // list_interactive: scan the page for interactive elements
  // ---------------------------------------------------------------
  if (action === 'list_interactive') {
    const interactiveSelectors = [
      'a[href]',
      'button',
      'input',
      'select',
      'textarea',
      '[role="button"]',
      '[role="link"]',
      '[role="tab"]',
      '[role="menuitem"]',
      '[contenteditable="true"]',
      '[onclick]',
      '[tabindex]'
    ]
    const seen = new Set<Element>()
    const elements: {
      tag: string
      type?: string
      selector: string
      label: string
      role?: string
      placeholder?: string
      visible: boolean
    }[] = []

    for (const cssSelector of interactiveSelectors) {
      const matches = document.querySelectorAll(cssSelector)
      for (const el of matches) {
        if (seen.has(el)) continue
        seen.add(el)

        const htmlEl = el as HTMLElement
        const rect = htmlEl.getBoundingClientRect()
        const visible = rect.width > 0 && rect.height > 0 && htmlEl.offsetParent !== null

        const uniqueSelector = buildUniqueSelector(el, htmlEl, cssSelector)

        // Build human-readable label
        const label =
          el.getAttribute('aria-label') ||
          el.getAttribute('title') ||
          el.getAttribute('placeholder') ||
          (htmlEl.textContent || '').trim().slice(0, 60) ||
          el.tagName.toLowerCase()

        elements.push({
          tag: el.tagName.toLowerCase(),
          type: el instanceof HTMLInputElement ? el.type : undefined,
          selector: uniqueSelector,
          label,
          role: el.getAttribute('role') || undefined,
          placeholder: el.getAttribute('placeholder') || undefined,
          visible
        })

        if (elements.length >= 100) break // Cap at 100 elements
      }
      if (elements.length >= 100) break
    }

    return { success: true, elements }
  }

  // ---------------------------------------------------------------
  // Resolve element for all other actions
  // ---------------------------------------------------------------
  const el = resolveElement(selector)
  if (!el) {
    return {
      success: false,
      action,
      selector,
      error: 'element_not_found',
      message: `No element matches selector: ${selector}`
    }
  }

  // ---------------------------------------------------------------
  // Mutation tracking: wraps an action with MutationObserver to capture DOM changes.
  // Returns a compact dom_summary (always) and detailed dom_changes (when analyze:true).
  // ---------------------------------------------------------------
  function withMutationTracking(fn: () => DOMResult): Promise<DOMResult> {
    const t0 = performance.now()
    const mutations: MutationRecord[] = []
    const observer = new MutationObserver((records) => {
      mutations.push(...records)
    })
    observer.observe(document.body || document.documentElement, {
      childList: true,
      subtree: true,
      attributes: true
    })

    const result = fn()

    if (!result.success) {
      observer.disconnect()
      return Promise.resolve(result)
    }

    return new Promise((resolve) => {
      let resolved = false
      function finish() {
        if (resolved) return
        resolved = true
        observer.disconnect()
        const totalMs = Math.round(performance.now() - t0)
        const added = mutations.reduce((s, m) => s + m.addedNodes.length, 0)
        const removed = mutations.reduce((s, m) => s + m.removedNodes.length, 0)
        const modified = mutations.filter((m) => m.type === 'attributes').length
        const parts: string[] = []
        if (added > 0) parts.push(`${added} added`)
        if (removed > 0) parts.push(`${removed} removed`)
        if (modified > 0) parts.push(`${modified} modified`)
        const summary = parts.length > 0 ? parts.join(', ') : 'no DOM changes'

        const enriched: DOMResult = { ...result, dom_summary: summary }

        if (options.analyze) {
          enriched.timing = { total_ms: totalMs }
          enriched.dom_changes = { added, removed, modified, summary }
          enriched.analysis = `${result.action} completed in ${totalMs}ms. ${summary}.`
        }

        resolve(enriched)
      }

      // setTimeout fallback — always fires, even in backgrounded/headless tabs
      // where requestAnimationFrame is suppressed
      setTimeout(finish, 80)

      // Try rAF for better timing when tab is visible, but don't depend on it
      if (typeof requestAnimationFrame === 'function') {
        requestAnimationFrame(() => setTimeout(finish, 50))
      }
    })
  }

  // ---------------------------------------------------------------
  // Action dispatch
  // ---------------------------------------------------------------
  switch (action) {
    case 'click': {
      return withMutationTracking(() => {
        if (!(el instanceof HTMLElement)) {
          return {
            success: false,
            action,
            selector,
            error: 'not_interactive',
            message: `Element is not an HTMLElement: ${el.tagName}`
          }
        }
        el.click()
        return { success: true, action, selector }
      })
    }

    case 'type': {
      return withMutationTracking(() => {
        const text = options.text || ''

        // Contenteditable elements (Gmail compose body, rich text editors)
        if (el instanceof HTMLElement && el.isContentEditable) {
          el.focus()
          if (options.clear) {
            const selection = document.getSelection()
            if (selection) {
              selection.selectAllChildren(el)
              selection.deleteFromDocument()
            }
          }
          document.execCommand('insertText', false, text)
          return { success: true, action, selector, value: el.textContent }
        }

        if (!(el instanceof HTMLInputElement) && !(el instanceof HTMLTextAreaElement)) {
          return {
            success: false,
            action,
            selector,
            error: 'not_typeable',
            message: `Element is not an input, textarea, or contenteditable: ${el.tagName}`
          }
        }
        const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement : HTMLInputElement
        const nativeSetter = Object.getOwnPropertyDescriptor(proto.prototype, 'value')?.set
        if (nativeSetter) {
          const newValue = options.clear ? text : el.value + text
          nativeSetter.call(el, newValue)
        } else {
          el.value = options.clear ? text : el.value + text
        }
        el.dispatchEvent(new InputEvent('input', { bubbles: true, data: text, inputType: 'insertText' }))
        el.dispatchEvent(new Event('change', { bubbles: true }))
        return { success: true, action, selector, value: el.value }
      })
    }

    case 'select': {
      return withMutationTracking(() => {
        if (!(el instanceof HTMLSelectElement)) {
          return {
            success: false,
            action,
            selector,
            error: 'not_select',
            message: `Element is not a <select>: ${el.tagName}` // nosemgrep: html-in-template-string
          }
        }
        const nativeSelectSetter = Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, 'value')?.set
        if (nativeSelectSetter) {
          nativeSelectSetter.call(el, options.value || '')
        } else {
          el.value = options.value || ''
        }
        el.dispatchEvent(new Event('change', { bubbles: true }))
        return { success: true, action, selector, value: el.value }
      })
    }

    case 'check': {
      return withMutationTracking(() => {
        if (!(el instanceof HTMLInputElement) || (el.type !== 'checkbox' && el.type !== 'radio')) {
          return {
            success: false,
            action,
            selector,
            error: 'not_checkable',
            message: `Element is not a checkbox or radio: ${el.tagName} type=${(el as HTMLInputElement).type || 'N/A'}`
          }
        }
        const desired = options.checked !== undefined ? options.checked : true
        if (el.checked !== desired) {
          el.click()
        }
        return { success: true, action, selector, value: el.checked }
      })
    }

    case 'get_text': {
      return { success: true, action, selector, value: el.textContent }
    }

    case 'get_value': {
      if (!('value' in el)) {
        return {
          success: false,
          action,
          selector,
          error: 'no_value_property',
          message: `Element has no value property: ${el.tagName}`
        }
      }
      return { success: true, action, selector, value: (el as HTMLInputElement).value }
    }

    case 'get_attribute': {
      return { success: true, action, selector, value: el.getAttribute(options.name || '') }
    }

    case 'set_attribute': {
      return withMutationTracking(() => {
        el.setAttribute(options.name || '', options.value || '')
        return { success: true, action, selector, value: el.getAttribute(options.name || '') }
      })
    }

    case 'focus': {
      if (!(el instanceof HTMLElement)) {
        return {
          success: false,
          action,
          selector,
          error: 'not_focusable',
          message: `Element is not an HTMLElement: ${el.tagName}`
        }
      }
      el.focus()
      return { success: true, action, selector }
    }

    case 'scroll_to': {
      el.scrollIntoView({ behavior: 'smooth', block: 'center' })
      return { success: true, action, selector }
    }

    case 'wait_for': {
      // Already found — return immediately
      return { success: true, action, selector, value: el.tagName.toLowerCase() }
    }

    case 'key_press': {
      return withMutationTracking(() => {
        if (!(el instanceof HTMLElement)) {
          return {
            success: false,
            action,
            selector,
            error: 'not_interactive',
            message: `Element is not an HTMLElement: ${el.tagName}`
          }
        }
        const key = options.text || 'Enter'

        // Tab/Shift+Tab: manually move focus (dispatchEvent can't trigger native tab traversal)
        if (key === 'Tab' || key === 'Shift+Tab') {
          const focusable = Array.from(
            el.ownerDocument.querySelectorAll(
              'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
            )
          ).filter((e) => (e as HTMLElement).offsetParent !== null) as HTMLElement[]
          const idx = focusable.indexOf(el)
          const next = key === 'Shift+Tab' ? focusable[idx - 1] : focusable[idx + 1]
          if (next) {
            next.focus()
            return { success: true, action, selector, value: key }
          }
          return { success: true, action, selector, value: key, message: 'No next focusable element' }
        }

        const keyMap: Record<string, { key: string; code: string; keyCode: number }> = {
          Enter: { key: 'Enter', code: 'Enter', keyCode: 13 },
          Tab: { key: 'Tab', code: 'Tab', keyCode: 9 },
          Escape: { key: 'Escape', code: 'Escape', keyCode: 27 },
          Backspace: { key: 'Backspace', code: 'Backspace', keyCode: 8 },
          ArrowDown: { key: 'ArrowDown', code: 'ArrowDown', keyCode: 40 },
          ArrowUp: { key: 'ArrowUp', code: 'ArrowUp', keyCode: 38 },
          Space: { key: ' ', code: 'Space', keyCode: 32 }
        }
        const mapped = keyMap[key] || { key, code: key, keyCode: 0 }
        el.dispatchEvent(
          new KeyboardEvent('keydown', { key: mapped.key, code: mapped.code, keyCode: mapped.keyCode, bubbles: true })
        )
        el.dispatchEvent(
          new KeyboardEvent('keypress', { key: mapped.key, code: mapped.code, keyCode: mapped.keyCode, bubbles: true })
        )
        el.dispatchEvent(
          new KeyboardEvent('keyup', { key: mapped.key, code: mapped.code, keyCode: mapped.keyCode, bubbles: true })
        )
        return { success: true, action, selector, value: key }
      })
    }

    default:
      return { success: false, action, selector, error: 'unknown_action', message: `Unknown DOM action: ${action}` }
  }
}

/**
 * wait_for variant that polls with MutationObserver (used when element not found initially).
 * Separate function because it returns a Promise.
 */
// #lizard forgives
export function domWaitFor(selector: string, timeoutMs: number): Promise<DOMResult> {
  // ---------------------------------------------------------------
  // Inline selector resolver (must be self-contained for chrome.scripting)
  // ---------------------------------------------------------------
  // #lizard forgives
  function resolveByTextSimple(searchText: string): Element | null {
    const walker = document.createTreeWalker(document.body || document.documentElement, NodeFilter.SHOW_TEXT)
    while (walker.nextNode()) {
      const node = walker.currentNode
      if (node.textContent && node.textContent.trim().includes(searchText)) {
        const parent = node.parentElement
        if (!parent) continue
        return parent.closest('a, button, [role="button"], [role="link"], label, summary') || parent
      }
    }
    return null
  }

  function resolveByLabelSimple(labelText: string): Element | null {
    for (const label of document.querySelectorAll('label')) {
      if (label.textContent && label.textContent.trim().includes(labelText)) {
        const forAttr = label.getAttribute('for')
        if (forAttr) {
          const t = document.getElementById(forAttr)
          if (t) return t
        }
        return label.querySelector('input, select, textarea') || label
      }
    }
    return null
  }

  const waitResolvers: [string, (value: string) => Element | null][] = [
    ['text=', (v) => resolveByTextSimple(v)],
    ['role=', (v) => document.querySelector(`[role="${CSS.escape(v)}"]`)],
    ['placeholder=', (v) => document.querySelector(`[placeholder="${CSS.escape(v)}"]`)],
    ['aria-label=', (v) => document.querySelector(`[aria-label="${CSS.escape(v)}"]`)],
    ['label=', (v) => resolveByLabelSimple(v)]
  ]

  function resolveElement(sel: string): Element | null {
    if (!sel) return null
    for (const [prefix, resolver] of waitResolvers) {
      if (sel.startsWith(prefix)) return resolver(sel.slice(prefix.length))
    }
    return document.querySelector(sel)
  }

  return new Promise((resolve) => {
    // Check immediately
    const existing = resolveElement(selector)
    if (existing) {
      resolve({ success: true, action: 'wait_for', selector, value: existing.tagName.toLowerCase() })
      return
    }

    let resolved = false
    const timer = setTimeout(() => {
      if (!resolved) {
        resolved = true
        observer.disconnect()
        resolve({
          success: false,
          action: 'wait_for',
          selector,
          error: 'timeout',
          message: `Element not found within ${timeoutMs}ms: ${selector}`
        })
      }
    }, timeoutMs)

    const observer = new MutationObserver(() => {
      const el = resolveElement(selector)
      if (el && !resolved) {
        resolved = true
        clearTimeout(timer)
        observer.disconnect()
        resolve({ success: true, action: 'wait_for', selector, value: el.tagName.toLowerCase() })
      }
    })

    observer.observe(document.documentElement, { childList: true, subtree: true })
  })
}

// =============================================================================
// Dispatcher: routes dom_action queries to pre-compiled functions
// =============================================================================

interface DOMActionParams {
  action?: string
  selector?: string
  text?: string
  value?: string
  clear?: boolean
  checked?: boolean
  name?: string
  timeout_ms?: number
  reason?: string
  analyze?: boolean
}

function parseDOMParams(query: PendingQuery): DOMActionParams | null {
  try {
    return typeof query.params === 'string' ? JSON.parse(query.params) : (query.params as DOMActionParams)
  } catch {
    return null
  }
}

function isReadOnlyAction(action: string): boolean {
  return action === 'list_interactive' || action.startsWith('get_')
}

async function executeWaitFor(
  tabId: number,
  params: DOMActionParams
): Promise<chrome.scripting.InjectionResult[] | DOMResult> {
  const selector = params.selector || ''
  const quickCheck = await chrome.scripting.executeScript({
    target: { tabId }, world: 'MAIN', func: domPrimitive,
    args: [params.action!, selector, { timeout_ms: params.timeout_ms }]
  })
  const quickResult = quickCheck?.[0]?.result as DOMResult | undefined
  if (quickResult?.success) return quickResult

  return chrome.scripting.executeScript({
    target: { tabId }, world: 'MAIN', func: domWaitFor,
    args: [selector, params.timeout_ms || 5000]
  })
}

async function executeStandardAction(tabId: number, params: DOMActionParams): Promise<chrome.scripting.InjectionResult[]> {
  return chrome.scripting.executeScript({
    target: { tabId }, world: 'MAIN', func: domPrimitive,
    args: [
      params.action!, params.selector || '',
      { text: params.text, value: params.value, clear: params.clear, checked: params.checked, name: params.name, timeout_ms: params.timeout_ms, analyze: params.analyze }
    ]
  })
}

function sendToastForResult(
  tabId: number, readOnly: boolean, result: { success?: boolean; error?: string },
  actionToast: ActionToast, toastLabel: string, toastDetail: string | undefined
): void {
  if (readOnly) return
  if (result.success) { actionToast(tabId, toastLabel, toastDetail, 'success') }
  else { actionToast(tabId, toastLabel, result.error || 'failed', 'error') }
}

// #lizard forgives
export async function executeDOMAction(
  query: PendingQuery, tabId: number, syncClient: SyncClient,
  sendAsyncResult: SendAsyncResult, actionToast: ActionToast
): Promise<void> {
  const params = parseDOMParams(query)
  if (!params) { sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, 'invalid_params'); return }

  const { action, selector, reason } = params
  if (!action) { sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, 'missing_action'); return }
  if (action === 'wait_for' && !selector) { sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, 'missing_selector'); return }

  const toastLabel = reason || action
  const toastDetail = reason ? undefined : selector || 'page'
  const readOnly = isReadOnlyAction(action)

  try {
    const tryingShownAt = Date.now()
    if (!readOnly) actionToast(tabId, toastLabel, toastDetail, 'trying', 10000)

    const rawResult = action === 'wait_for'
      ? await executeWaitFor(tabId, params)
      : await executeStandardAction(tabId, params)

    // wait_for quick-check can return a DOMResult directly
    if (!Array.isArray(rawResult)) {
      if (!readOnly) actionToast(tabId, toastLabel, toastDetail, 'success')
      sendAsyncResult(syncClient, query.id, query.correlation_id!, 'complete', rawResult)
      return
    }

    // Ensure "trying" toast is visible for at least 500ms
    const MIN_TOAST_MS = 500
    const elapsed = Date.now() - tryingShownAt
    if (!readOnly && elapsed < MIN_TOAST_MS) await new Promise((r) => setTimeout(r, MIN_TOAST_MS - elapsed))

    const firstResult = rawResult?.[0]?.result
    if (firstResult && typeof firstResult === 'object') {
      sendToastForResult(tabId, readOnly, firstResult as { success?: boolean; error?: string }, actionToast, toastLabel, toastDetail)
      sendAsyncResult(syncClient, query.id, query.correlation_id!, 'complete', firstResult)
    } else {
      if (!readOnly) actionToast(tabId, toastLabel, 'no result', 'error')
      sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, 'no_result')
    }
  } catch (err) {
    actionToast(tabId, action, (err as Error).message, 'error')
    sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, (err as Error).message)
  }
}
