// dom-primitives-list-interactive.ts — Self-contained list_interactive DOM primitive for chrome.scripting.executeScript.
// Extracted from dom-primitives.ts to keep file sizes under the 800 LOC limit.
// This function MUST remain self-contained — Chrome serializes the function source only (no closures).

/**
 * Self-contained function that scans a page for interactive elements.
 * Passed to chrome.scripting.executeScript({ func: domPrimitiveListInteractive }).
 * MUST NOT reference any module-level variables.
 */
export function domPrimitiveListInteractive(): { success: boolean; elements: unknown[] } {
  // — Shadow DOM: deep traversal utilities (duplicated from dom-primitives.ts, required for self-containment) —

  function getShadowRoot(el: Element): ShadowRoot | null {
    return el.shadowRoot ?? null
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

  // — Selector and classification helpers —

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

  // Build >>> selector for an element inside a shadow root
  function buildShadowSelector(el: Element): string | null {
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

  function classifyElement(el: Element): string {
    const tag = el.tagName.toLowerCase()
    if (tag === 'a') return 'link'
    if (tag === 'button' || el.getAttribute('role') === 'button') return 'button'
    if (tag === 'input') {
      const inputType = (el as HTMLInputElement).type || 'text'
      if (inputType === 'submit' || inputType === 'button' || inputType === 'reset') return 'button'
      if (inputType === 'checkbox' || inputType === 'radio') return 'checkbox'
      return 'input'
    }
    if (tag === 'select') return 'select'
    if (tag === 'textarea') return 'textarea'
    if (el.getAttribute('role') === 'link') return 'link'
    if (el.getAttribute('role') === 'tab') return 'tab'
    if (el.getAttribute('role') === 'menuitem') return 'menuitem'
    if (el.getAttribute('contenteditable') === 'true') return 'textarea'
    return 'interactive'
  }

  // — Main scan logic —

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
    index: number
    tag: string
    type?: string
    element_type: string
    selector: string
    label: string
    role?: string
    placeholder?: string
    visible: boolean
  }[] = []

  // First pass: collect raw entries with their base selectors
  const rawEntries: {
    el: Element
    htmlEl: HTMLElement
    baseSelector: string
    tag: string
    inputType?: string
    elementType: string
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
      const shadowSel = buildShadowSelector(el)
      const baseSelector = shadowSel || buildUniqueSelector(el, htmlEl, cssSelector)

      // Build human-readable label
      const label =
        el.getAttribute('aria-label') ||
        el.getAttribute('title') ||
        el.getAttribute('placeholder') ||
        (htmlEl.textContent || '').trim().slice(0, 60) ||
        el.tagName.toLowerCase()

      rawEntries.push({
        el,
        htmlEl,
        baseSelector,
        tag: el.tagName.toLowerCase(),
        inputType: el instanceof HTMLInputElement ? el.type : undefined,
        elementType: classifyElement(el),
        label,
        role: el.getAttribute('role') || undefined,
        placeholder: el.getAttribute('placeholder') || undefined,
        visible
      })

      if (rawEntries.length >= 100) break
    }
    if (rawEntries.length >= 100) break
  }

  // Second pass: deduplicate selectors by appending :nth-match(N)
  const selectorCount = new Map<string, number>()
  for (const entry of rawEntries) {
    selectorCount.set(entry.baseSelector, (selectorCount.get(entry.baseSelector) || 0) + 1)
  }
  const selectorIndex = new Map<string, number>()

  for (let i = 0; i < rawEntries.length; i++) {
    const entry = rawEntries[i]!
    let finalSelector = entry.baseSelector
    const count = selectorCount.get(entry.baseSelector) || 1
    if (count > 1) {
      const nth = (selectorIndex.get(entry.baseSelector) || 0) + 1
      selectorIndex.set(entry.baseSelector, nth)
      finalSelector = `${entry.baseSelector}:nth-match(${nth})`
    }

    elements.push({
      index: i,
      tag: entry.tag,
      type: entry.inputType,
      element_type: entry.elementType,
      selector: finalSelector,
      label: entry.label,
      role: entry.role,
      placeholder: entry.placeholder,
      visible: entry.visible
    })
  }

  return { success: true, elements }
}
