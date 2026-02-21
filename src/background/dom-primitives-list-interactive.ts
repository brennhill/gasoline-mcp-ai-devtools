// dom-primitives-list-interactive.ts — Self-contained list_interactive DOM primitive for chrome.scripting.executeScript.
// Extracted from dom-primitives.ts to keep file sizes under the 800 LOC limit.
// This function MUST remain self-contained — Chrome serializes the function source only (no closures).

/**
 * Self-contained function that scans a page for interactive elements.
 * Passed to chrome.scripting.executeScript({ func: domPrimitiveListInteractive }).
 * MUST NOT reference any module-level variables.
 */
export function domPrimitiveListInteractive(
  scopeSelector?: string,
  options?: { scope_rect?: { x?: unknown; y?: unknown; width?: unknown; height?: unknown } }
): {
  success: boolean
  elements: unknown[]
  candidate_count?: number
  scope_rect_used?: { x: number; y: number; width: number; height: number }
  error?: string
  message?: string
} {
  type ScopeRect = { x: number; y: number; width: number; height: number }
  type ElementHandleStore = {
    byElement: WeakMap<Element, string>
    byID: Map<string, Element>
    nextID: number
  }

  function getElementHandleStore(): ElementHandleStore {
    const root = globalThis as typeof globalThis & { __gasolineElementHandles?: ElementHandleStore }
    if (root.__gasolineElementHandles) {
      return root.__gasolineElementHandles
    }
    const created: ElementHandleStore = {
      byElement: new WeakMap<Element, string>(),
      byID: new Map<string, Element>(),
      nextID: 1
    }
    root.__gasolineElementHandles = created
    return created
  }

  function getOrCreateElementID(el: Element): string {
    const store = getElementHandleStore()
    const existing = store.byElement.get(el)
    if (existing) {
      store.byID.set(existing, el)
      return existing
    }
    const elementID = `el_${(store.nextID++).toString(36)}`
    store.byElement.set(el, elementID)
    store.byID.set(elementID, el)
    return elementID
  }

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

  function isVisibleElement(el: Element): boolean {
    const htmlEl = el as HTMLElement
    if (!htmlEl || typeof htmlEl.getBoundingClientRect !== 'function') return true
    const rect = htmlEl.getBoundingClientRect()
    return rect.width > 0 && rect.height > 0 && htmlEl.offsetParent !== null
  }

  function extractLabel(el: Element): string {
    const htmlEl = el as HTMLElement
    return (
      el.getAttribute('aria-label') ||
      el.getAttribute('title') ||
      el.getAttribute('placeholder') ||
      (htmlEl?.textContent || '').trim().slice(0, 80) ||
      el.tagName.toLowerCase()
    )
  }

  function parseScopeRect(raw: unknown): ScopeRect | null {
    if (!raw || typeof raw !== 'object') return null
    const rect = raw as { x?: unknown; y?: unknown; width?: unknown; height?: unknown }
    const x = Number(rect.x)
    const y = Number(rect.y)
    const width = Number(rect.width)
    const height = Number(rect.height)
    if (![x, y, width, height].every((v) => Number.isFinite(v))) return null
    if (width <= 0 || height <= 0) return null
    return { x, y, width, height }
  }

  const scopeRect = parseScopeRect(options?.scope_rect)
  if (options?.scope_rect !== undefined && !scopeRect) {
    return {
      success: false,
      elements: [],
      error: 'invalid_scope_rect',
      message: 'scope_rect must include finite x, y, width, and height > 0'
    }
  }

  function intersectsScopeRect(el: Element): boolean {
    if (!scopeRect) return true
    const htmlEl = el as HTMLElement
    if (!htmlEl || typeof htmlEl.getBoundingClientRect !== 'function') return false
    const rect = htmlEl.getBoundingClientRect()
    const left = typeof rect.left === 'number' ? rect.left : (typeof rect.x === 'number' ? rect.x : 0)
    const top = typeof rect.top === 'number' ? rect.top : (typeof rect.y === 'number' ? rect.y : 0)
    const right = typeof rect.right === 'number' ? rect.right : left + rect.width
    const bottom = typeof rect.bottom === 'number' ? rect.bottom : top + rect.height
    const scopeRight = scopeRect.x + scopeRect.width
    const scopeBottom = scopeRect.y + scopeRect.height
    const overlapX = left < scopeRight && right > scopeRect.x
    const overlapY = top < scopeBottom && bottom > scopeRect.y
    return overlapX && overlapY
  }

  function chooseBestScopeMatch(matches: Element[]): Element {
    if (matches.length === 1) return matches[0]!

    const submitVerb = /(post|share|publish|send|submit|save|done|continue|next|create|apply)/i
    let best = matches[0]!
    let bestScore = -1

    for (const candidate of matches) {
      const textboxes = querySelectorAllDeep('[role="textbox"], textarea, [contenteditable="true"]', candidate)
      const visibleTextboxes = textboxes.filter(isVisibleElement).length

      const buttonCandidates = querySelectorAllDeep('button, [role="button"], input[type="submit"]', candidate)
      let visibleButtons = 0
      let submitLikeButtons = 0
      for (const btn of buttonCandidates) {
        if (!isVisibleElement(btn)) continue
        visibleButtons++
        if (submitVerb.test(extractLabel(btn))) {
          submitLikeButtons++
        }
      }

      const interactiveCandidates = querySelectorAllDeep(
        'a[href], button, input, select, textarea, [role="button"], [role="link"], [role="tab"], [role="menuitem"], [contenteditable="true"]',
        candidate
      )
      const visibleInteractive = interactiveCandidates.filter(isVisibleElement).length
      const hiddenInteractive = Math.max(0, interactiveCandidates.length - visibleInteractive)

      const rect = (candidate as HTMLElement).getBoundingClientRect?.()
      const areaScore = rect && rect.width > 0 && rect.height > 0
        ? Math.min(20, Math.round((rect.width * rect.height) / 50000))
        : 0

      // Heuristic weighting:
      // - Visible textbox strongly indicates active editor/dialog.
      // - Submit-like visible controls indicate actionable composer.
      // - Prefer visible-rich over hidden-heavy containers.
      const score =
        visibleTextboxes*1000 +
        submitLikeButtons*250 +
        visibleButtons*10 +
        visibleInteractive -
        hiddenInteractive +
        areaScore

      if (score > bestScore) {
        bestScore = score
        best = candidate
      }
    }

    return best
  }

  function resolveScopeRoot(rawScope?: string): ParentNode | null {
    const scope = (rawScope || '').trim()
    if (!scope) return document
    try {
      const matches = querySelectorAllDeep(scope)
      if (matches.length === 0) return null
      return chooseBestScopeMatch(matches)
    } catch {
      return null
    }
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
    element_id: string
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

  const scopeRoot = resolveScopeRoot(scopeSelector)
  if (!scopeRoot) {
    return {
      success: false,
      elements: [],
      error: 'scope_not_found',
      message: `No scope element matches selector: ${scopeSelector || ''}`
    }
  }

  for (const cssSelector of interactiveSelectors) {
    const matches = querySelectorAllDeep(cssSelector, scopeRoot)
    for (const el of matches) {
      if (seen.has(el)) continue
      seen.add(el)

      const htmlEl = el as HTMLElement
      const rect = htmlEl.getBoundingClientRect()
      const visible = rect.width > 0 && rect.height > 0 && htmlEl.offsetParent !== null
      if (!intersectsScopeRect(el)) continue

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
      element_id: getOrCreateElementID(entry.el),
      label: entry.label,
      role: entry.role,
      placeholder: entry.placeholder,
      visible: entry.visible
    })
  }

  return {
    success: true,
    elements,
    candidate_count: elements.length,
    ...(scopeRect ? { scope_rect_used: scopeRect } : {})
  }
}
