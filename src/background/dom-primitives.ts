// dom-primitives.ts — Pre-compiled DOM interaction functions for chrome.scripting.executeScript.
// These bypass CSP restrictions because they use the `func` parameter (no eval/new Function).
// Each function MUST be self-contained — no closures over external variables.

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
  // Shadow DOM: deep traversal utilities
  // ---------------------------------------------------------------

  function getShadowRoot(el: Element): ShadowRoot | null {
    if (el.shadowRoot) return el.shadowRoot
    const closed = (globalThis as unknown as Window).__GASOLINE_CLOSED_SHADOWS__
    return closed?.get(el) ?? null
  }

  function querySelectorDeep(selector: string, root: ParentNode = document): Element | null {
    const fast = root.querySelector(selector)
    if (fast) return fast
    return querySelectorDeepWalk(selector, root)
  }

  function querySelectorDeepWalk(selector: string, root: ParentNode, depth: number = 0): Element | null {
    if (depth > 10) return null
    // Navigate to children: handle Document (has body/documentElement) and Element/ShadowRoot (has children)
    const children = 'children' in root
      ? (root as Element).children
      : (root as Document).body?.children || (root as Document).documentElement?.children
    if (!children) return null
    for (let i = 0; i < children.length; i++) {
      const child = children[i]!
      const shadow = getShadowRoot(child)
      if (shadow) {
        const match = shadow.querySelector(selector)
        if (match) return match
        const deep = querySelectorDeepWalk(selector, shadow, depth + 1)
        if (deep) return deep
      }
      if (child.children.length > 0) {
        const deep = querySelectorDeepWalk(selector, child, depth + 1)
        if (deep) return deep
      }
    }
    return null
  }

  function querySelectorAllDeep(
    selector: string,
    root: ParentNode = document,
    results: Element[] = [],
    depth: number = 0
  ): Element[] {
    if (depth > 10) return results
    results.push(...Array.from(root.querySelectorAll(selector)))
    const children = 'children' in root
      ? (root as Element).children
      : (root as Document).body?.children || (root as Document).documentElement?.children
    if (!children) return results
    for (let i = 0; i < children.length; i++) {
      const child = children[i]!
      const shadow = getShadowRoot(child)
      if (shadow) {
        querySelectorAllDeep(selector, shadow, results, depth + 1)
      }
    }
    return results
  }

  function resolveDeepCombinator(selector: string): Element | null {
    const parts = selector.split(' >>> ')
    if (parts.length <= 1) return null

    let current: ParentNode = document
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i]!.trim()
      if (i < parts.length - 1) {
        const host = querySelectorDeep(part, current)
        if (!host) return null
        const shadow = getShadowRoot(host)
        if (!shadow) return null
        current = shadow
      } else {
        return querySelectorDeep(part, current)
      }
    }
    return null
  }

  // Build >>> selector for an element inside a shadow root
  function buildShadowSelector(el: Element, htmlEl: HTMLElement, fallbackSelector: string): string | null {
    const rootNode = el.getRootNode()
    if (!(rootNode instanceof ShadowRoot)) return null

    const parts: string[] = []
    let node: Element = el
    let root: Node = rootNode
    while (root instanceof ShadowRoot) {
      const inner = buildUniqueSelector(node, node as HTMLElement, node.tagName.toLowerCase())
      parts.unshift(inner)
      node = root.host
      root = node.getRootNode()
    }
    // Add the outermost host selector
    const hostSelector = buildUniqueSelector(node, node as HTMLElement, node.tagName.toLowerCase())
    parts.unshift(hostSelector)
    return parts.join(' >>> ')
  }

  // ---------------------------------------------------------------
  // Selector resolver: CSS or semantic (text=, role=, placeholder=, label=, aria-label=)
  // All semantic selectors prefer visible elements over hidden ones.
  // ---------------------------------------------------------------

  // Visibility check: skip display:none, visibility:hidden, zero-size elements
  function isVisible(el: Element): boolean {
    if (!(el instanceof HTMLElement)) return true
    const style = getComputedStyle(el)
    if (style.visibility === 'hidden' || style.display === 'none') return false
    if (el.offsetParent === null && style.position !== 'fixed' && style.position !== 'sticky') {
      const rect = el.getBoundingClientRect()
      if (rect.width === 0 && rect.height === 0) return false
    }
    return true
  }

  // Return first visible match from a list, falling back to first match
  function firstVisible(els: NodeListOf<Element> | Element[]): Element | null {
    let fallback: Element | null = null
    for (const el of els) {
      if (!fallback) fallback = el
      if (isVisible(el)) return el
    }
    return fallback
  }

  function resolveByText(searchText: string): Element | null {
    let fallback: Element | null = null

    function walkScope(root: ParentNode): Element | null {
      const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT)
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
      // Recurse into shadow roots
      const children = 'children' in root ? (root as Element).children : undefined
      if (children) {
        for (let i = 0; i < children.length; i++) {
          const child = children[i]!
          const shadow = getShadowRoot(child)
          if (shadow) {
            const result = walkScope(shadow)
            if (result) return result
          }
        }
      }
      return null
    }

    return walkScope(document.body || document.documentElement) || fallback
  }

  function resolveByLabel(labelText: string): Element | null {
    const labels = querySelectorAllDeep('label')
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
    const exact = querySelectorAllDeep(`[aria-label="${CSS.escape(al)}"]`)
    if (exact.length > 0) return firstVisible(exact)
    const all = querySelectorAllDeep('[aria-label]')
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
    ['role=', (v) => firstVisible(querySelectorAllDeep(`[role="${CSS.escape(v)}"]`))],
    ['placeholder=', (v) => firstVisible(querySelectorAllDeep(`[placeholder="${CSS.escape(v)}"]`))],
    ['label=', (v) => resolveByLabel(v)],
    ['aria-label=', (v) => resolveByAriaLabel(v)]
  ]

  function resolveElement(sel: string): Element | null {
    if (!sel) return null

    // Deep combinator: host >>> inner
    if (sel.includes(' >>> ')) return resolveDeepCombinator(sel)

    for (const [prefix, resolver] of selectorResolvers) {
      if (sel.startsWith(prefix)) return resolver(sel.slice(prefix.length))
    }

    // Fast path, then deep fallback
    return querySelectorDeep(sel)
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
      const matches = querySelectorAllDeep(cssSelector)
      for (const el of matches) {
        if (seen.has(el)) continue
        seen.add(el)

        const htmlEl = el as HTMLElement
        const rect = htmlEl.getBoundingClientRect()
        const visible = rect.width > 0 && rect.height > 0 && htmlEl.offsetParent !== null

        // Use >>> selector for shadow DOM elements, regular selector otherwise
        const shadowSel = buildShadowSelector(el, htmlEl, cssSelector)
        const uniqueSelector = shadowSel || buildUniqueSelector(el, htmlEl, cssSelector)

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
          // Split on newlines — each \n becomes an insertParagraph command
          const lines = text.split('\n')
          for (let i = 0; i < lines.length; i++) {
            const line = lines[i]!
            if (line.length > 0) {
              document.execCommand('insertText', false, line)
            }
            if (i < lines.length - 1) {
              document.execCommand('insertParagraph', false)
            }
          }
          return { success: true, action, selector, value: el.innerText }
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
      const text = el instanceof HTMLElement ? el.innerText : el.textContent
      return { success: true, action, selector, value: text }
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

    case 'paste': {
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
        el.focus()
        if (options.clear) {
          const selection = document.getSelection()
          if (selection) {
            selection.selectAllChildren(el)
            selection.deleteFromDocument()
          }
        }
        const pasteText = options.text || ''
        const dt = new DataTransfer()
        dt.setData('text/plain', pasteText)
        const event = new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true })
        el.dispatchEvent(event)
        return { success: true, action, selector, value: el.innerText }
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
  // Inline shadow DOM helpers (duplicated from domPrimitive — required
  // because chrome.scripting.executeScript serializes each function
  // independently, no shared closures)
  // ---------------------------------------------------------------
  // #lizard forgives

  function getShadowRoot(el: Element): ShadowRoot | null {
    if (el.shadowRoot) return el.shadowRoot
    const closed = (globalThis as unknown as Window).__GASOLINE_CLOSED_SHADOWS__
    return closed?.get(el) ?? null
  }

  function querySelectorDeepWalk(sel: string, root: ParentNode, depth = 0): Element | null {
    if (depth > 10) return null
    const ch = 'children' in root
      ? (root as Element).children
      : (root as Document).body?.children || (root as Document).documentElement?.children
    if (!ch) return null
    for (let i = 0; i < ch.length; i++) {
      const child = ch[i]!
      const shadow = getShadowRoot(child)
      if (shadow) {
        const match = shadow.querySelector(sel)
        if (match) return match
        const deep = querySelectorDeepWalk(sel, shadow, depth + 1)
        if (deep) return deep
      }
      if (child.children.length > 0) {
        const deep = querySelectorDeepWalk(sel, child, depth + 1)
        if (deep) return deep
      }
    }
    return null
  }

  function querySelectorDeep(sel: string, root: ParentNode = document): Element | null {
    const fast = root.querySelector(sel)
    if (fast) return fast
    return querySelectorDeepWalk(sel, root)
  }

  function querySelectorAllDeep(
    sel: string,
    root: ParentNode = document,
    results: Element[] = [],
    depth = 0
  ): Element[] {
    if (depth > 10) return results
    results.push(...Array.from(root.querySelectorAll(sel)))
    const ch = 'children' in root
      ? (root as Element).children
      : (root as Document).body?.children || (root as Document).documentElement?.children
    if (!ch) return results
    for (let i = 0; i < ch.length; i++) {
      const child = ch[i]!
      const shadow = getShadowRoot(child)
      if (shadow) {
        querySelectorAllDeep(sel, shadow, results, depth + 1)
      }
    }
    return results
  }

  function resolveDeepCombinator(sel: string): Element | null {
    const parts = sel.split(' >>> ')
    if (parts.length <= 1) return null
    let current: ParentNode = document
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i]!.trim()
      if (i < parts.length - 1) {
        const host = querySelectorDeep(part, current)
        if (!host) return null
        const shadow = getShadowRoot(host)
        if (!shadow) return null
        current = shadow
      } else {
        return querySelectorDeep(part, current)
      }
    }
    return null
  }

  // ---------------------------------------------------------------
  // Inline selector resolvers (shadow-aware)
  // ---------------------------------------------------------------

  function resolveByTextSimple(searchText: string): Element | null {
    function walkScope(root: ParentNode): Element | null {
      const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT)
      while (walker.nextNode()) {
        const node = walker.currentNode
        if (node.textContent && node.textContent.trim().includes(searchText)) {
          const parent = node.parentElement
          if (!parent) continue
          return parent.closest('a, button, [role="button"], [role="link"], label, summary') || parent
        }
      }
      const ch = 'children' in root ? (root as Element).children : undefined
      if (ch) {
        for (let i = 0; i < ch.length; i++) {
          const child = ch[i]!
          const shadow = getShadowRoot(child)
          if (shadow) {
            const result = walkScope(shadow)
            if (result) return result
          }
        }
      }
      return null
    }
    return walkScope(document.body || document.documentElement)
  }

  function resolveByLabelSimple(labelText: string): Element | null {
    for (const label of querySelectorAllDeep('label')) {
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
    ['role=', (v) => querySelectorDeep(`[role="${CSS.escape(v)}"]`)],
    ['placeholder=', (v) => querySelectorDeep(`[placeholder="${CSS.escape(v)}"]`)],
    ['aria-label=', (v) => querySelectorDeep(`[aria-label="${CSS.escape(v)}"]`)],
    ['label=', (v) => resolveByLabelSimple(v)]
  ]

  function resolveElement(sel: string): Element | null {
    if (!sel) return null
    if (sel.includes(' >>> ')) return resolveDeepCombinator(sel)
    for (const [prefix, resolver] of waitResolvers) {
      if (sel.startsWith(prefix)) return resolver(sel.slice(prefix.length))
    }
    return querySelectorDeep(sel)
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

// Dispatcher utilities (parseDOMParams, executeDOMAction, etc.) moved to ./dom-dispatch.ts

/**
 * Frame-matching probe executed in page context.
 * Must stay self-contained for chrome.scripting.executeScript({ func }).
 */
export function domFrameProbe(frameTarget: string | number): { matches: boolean } {
  const isTop = window === window.top

  const getParentFrameIndex = (): number => {
    if (isTop) return -1
    try {
      const parentFrames = window.parent?.frames
      if (!parentFrames) return -1
      for (let i = 0; i < parentFrames.length; i++) {
        if (parentFrames[i] === window) return i
      }
    } catch {
      return -1
    }
    return -1
  }

  if (typeof frameTarget === 'number') {
    return { matches: getParentFrameIndex() === frameTarget }
  }

  if (frameTarget === 'all') {
    return { matches: true }
  }

  if (isTop) {
    return { matches: false }
  }

  try {
    const frameEl = window.frameElement
    if (!frameEl || typeof frameEl.matches !== 'function') {
      return { matches: false }
    }
    return { matches: frameEl.matches(frameTarget) }
  } catch {
    return { matches: false }
  }
}
