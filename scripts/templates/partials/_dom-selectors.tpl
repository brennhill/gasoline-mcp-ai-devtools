  // --- PARTIAL: DOM Selector Resolution ---
  // — Shadow DOM: deep traversal utilities —

  function getShadowRoot(el: Element): ShadowRoot | null {
    return el.shadowRoot ?? null
    // Closed root support: see feat/closed-shadow-capture branch
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

  function resolveDeepCombinator(selector: string, root: ParentNode = document): Element | null {
    const parts = selector.split(' >>> ')
    if (parts.length <= 1) return null

    let current: ParentNode = root
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

  // — Selector resolver: CSS or semantic (text=, role=, placeholder=, label=, aria-label=) —

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

  function resolveScopeRoot(rawScope?: string): ParentNode | null {
    const scope = (rawScope || '').trim()
    if (!scope) return document
    try {
      const matches = querySelectorAllDeep(scope)
      if (matches.length === 0) return null
      return firstVisible(matches) || matches[0] || null
    } catch {
      return null
    }
  }

  const scopeRoot = resolveScopeRoot(options.scope_selector)

  type ScopeRect = { x: number; y: number; width: number; height: number }

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

  const scopeRect = parseScopeRect(options.scope_rect)
  if (options.scope_rect !== undefined && !scopeRect) {
    return {
      success: false,
      action,
      selector,
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

  function filterByScopeRect(elements: Element[]): Element[] {
    if (!scopeRect) return elements
    return elements.filter((el) => intersectsScopeRect(el))
  }

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

  function resolveElementByID(rawElementID?: string): Element | null {
    const elementID = (rawElementID || '').trim()
    if (!elementID) return null
    const store = getElementHandleStore()
    const node = store.byID.get(elementID)
    if (!node) return null
    if ((node as Node).isConnected === false) {
      store.byID.delete(elementID)
      return null
    }
    return node
  }

  function resolveByTextAll(searchText: string, scope: ParentNode = document): Element[] {
    const results: Element[] = []
    const seen = new Set<Element>()

    function walkScope(root: ParentNode): void {
      const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT)
      while (walker.nextNode()) {
        const node = walker.currentNode
        if (node.textContent && node.textContent.trim().includes(searchText)) {
          const parent = node.parentElement
          if (!parent) continue
          const interactive = parent.closest('a, button, [role="button"], [role="link"], label, summary')
          const target = interactive || parent
          if (!seen.has(target)) {
            seen.add(target)
            results.push(target)
          }
        }
      }
      const children = 'children' in root
        ? (root as Element).children
        : (root as Document).body?.children || (root as Document).documentElement?.children
      if (children) {
        for (let i = 0; i < children.length; i++) {
          const child = children[i]!
          const shadow = getShadowRoot(child)
          if (shadow) walkScope(shadow)
        }
      }
    }

    walkScope(scope)
    return results
  }

  function resolveByLabelAll(labelText: string, scope: ParentNode = document): Element[] {
    const labels = querySelectorAllDeep('label', scope)
    const results: Element[] = []
    const seen = new Set<Element>()
    const allowGlobalIdLookup =
      scope === document || scope === document.body || scope === document.documentElement
    for (const label of labels) {
      if (label.textContent && label.textContent.trim().includes(labelText)) {
        const forAttr = label.getAttribute('for')
        if (forAttr) {
          const local = querySelectorAllDeep(`#${CSS.escape(forAttr)}`, scope)[0]
          const target = local || (allowGlobalIdLookup ? document.getElementById(forAttr) : null)
          if (target && !seen.has(target)) { seen.add(target); results.push(target) }
        }
        const nested = label.querySelector('input, select, textarea')
        if (nested && !seen.has(nested)) { seen.add(nested); results.push(nested) }
        if (!seen.has(label)) { seen.add(label); results.push(label) }
      }
    }
    return results
  }

  function resolveByAriaLabelAll(al: string, scope: ParentNode = document): Element[] {
    const results: Element[] = []
    const seen = new Set<Element>()
    const exact = querySelectorAllDeep(`[aria-label="${CSS.escape(al)}"]`, scope)
    for (const el of exact) {
      if (!seen.has(el)) { seen.add(el); results.push(el) }
    }
    const all = querySelectorAllDeep('[aria-label]', scope)
    for (const el of all) {
      const label = el.getAttribute('aria-label') || ''
      if (label.startsWith(al) && !seen.has(el)) { seen.add(el); results.push(el) }
    }
    return results
  }

  function resolveByText(searchText: string, scope: ParentNode = document): Element | null {
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
      const children = 'children' in root
        ? (root as Element).children
        : (root as Document).body?.children || (root as Document).documentElement?.children
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

    return walkScope(scope) || fallback
  }

  function resolveByLabel(labelText: string, scope: ParentNode = document): Element | null {
    const labels = querySelectorAllDeep('label', scope)
    const allowGlobalIdLookup =
      scope === document || scope === document.body || scope === document.documentElement
    for (const label of labels) {
      if (label.textContent && label.textContent.trim().includes(labelText)) {
        const forAttr = label.getAttribute('for')
        if (forAttr) {
          const local = querySelectorAllDeep(`#${CSS.escape(forAttr)}`, scope)[0]
          if (local) return local
          const target = allowGlobalIdLookup ? document.getElementById(forAttr) : null
          if (target) return target
        }
        const nested = label.querySelector('input, select, textarea')
        if (nested) return nested
        return label
      }
    }
    return null
  }

  function resolveByAriaLabel(al: string, scope: ParentNode = document): Element | null {
    const exact = querySelectorAllDeep(`[aria-label="${CSS.escape(al)}"]`, scope)
    if (exact.length > 0) return firstVisible(exact)
    const all = querySelectorAllDeep('[aria-label]', scope)
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

  function resolveElements(sel: string, scope: ParentNode = document): Element[] {
    if (!sel) return []
    if (sel.startsWith('text=')) return resolveByTextAll(sel.slice('text='.length), scope)
    if (sel.startsWith('role=')) return querySelectorAllDeep(`[role="${CSS.escape(sel.slice('role='.length))}"]`, scope)
    if (sel.startsWith('placeholder=')) return querySelectorAllDeep(`[placeholder="${CSS.escape(sel.slice('placeholder='.length))}"]`, scope)
    if (sel.startsWith('label=')) return resolveByLabelAll(sel.slice('label='.length), scope)
    if (sel.startsWith('aria-label=')) return resolveByAriaLabelAll(sel.slice('aria-label='.length), scope)
    try {
      return querySelectorAllDeep(sel, scope)
    } catch {
      return []
    }
  }

  function resolveElement(sel: string, scope: ParentNode = document): Element | null {
    if (!sel) return null
    if (sel.includes(' >>> ')) return resolveDeepCombinator(sel, scope)

    const nthMatch = sel.match(/^(.*):nth-match\((\d+)\)$/)
    if (nthMatch) {
      const base = nthMatch[1] || ''
      const n = Number.parseInt(nthMatch[2] || '0', 10)
      if (!base || Number.isNaN(n) || n < 1) return null
      const matches = resolveElements(base, scope)
      return matches[n - 1] || null
    }

    if (sel.startsWith('text=')) return resolveByText(sel.slice('text='.length), scope)
    if (sel.startsWith('role=')) return firstVisible(querySelectorAllDeep(`[role="${CSS.escape(sel.slice('role='.length))}"]`, scope))
    if (sel.startsWith('placeholder=')) return firstVisible(querySelectorAllDeep(`[placeholder="${CSS.escape(sel.slice('placeholder='.length))}"]`, scope))
    if (sel.startsWith('label=')) return resolveByLabel(sel.slice('label='.length), scope)
    if (sel.startsWith('aria-label=')) return resolveByAriaLabel(sel.slice('aria-label='.length), scope)

    return querySelectorDeep(sel, scope)
  }

  // list_interactive is handled by domPrimitiveListInteractive in production dispatch,
  // but remains available here for backward compatibility and direct tests.
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

  function extractElementLabel(el: Element): string {
    const htmlEl = el as HTMLElement
    return (
      el.getAttribute('aria-label') ||
      el.getAttribute('title') ||
      el.getAttribute('placeholder') ||
      (htmlEl?.textContent || '').trim().slice(0, 80) ||
      el.tagName.toLowerCase()
    )
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
        if (submitVerb.test(extractElementLabel(btn))) {
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
