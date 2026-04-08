/**
 * Purpose: Self-contained DOM primitives for read/wait actions (get_text, get_value, get_attribute, wait_for, wait_for_text, wait_for_absent).
 * Why: Extracted from dom-primitives.ts to keep file sizes under the 800 LOC limit.
 *      These actions are read-only and use simplified (non-ambiguity) element resolution.
 * Docs: docs/features/feature/interact-explore/index.md
 */

// dom-primitives-read.ts — Self-contained read/wait DOM primitives for chrome.scripting.executeScript.
// This function MUST remain self-contained — Chrome serializes the function source only (no closures).

import type { DOMPrimitiveOptions, DOMResult } from './dom-types.js'

/**
 * Self-contained function for read-only DOM primitives and wait_for actions.
 * Passed to chrome.scripting.executeScript({ func: domPrimitiveRead, args: [...] }).
 * MUST NOT reference any module-level variables — Chrome serializes the function source only.
 */
export function domPrimitiveRead(
  action: string,
  selector: string,
  options: DOMPrimitiveOptions
): DOMResult {
  // --- Shared selector infrastructure (duplicated for self-containment) ---
  function getShadowRoot(el: Element): ShadowRoot | null { return el.shadowRoot ?? null }

  function querySelectorDeep(sel: string, root: ParentNode = document): Element | null {
    const fast = root.querySelector(sel)
    if (fast && !isKaboomOwnedElement(fast)) return fast
    return querySelectorDeepWalk(sel, root)
  }

  function querySelectorDeepWalk(sel: string, root: ParentNode, depth = 0): Element | null {
    if (depth > 10) return null
    const children = 'children' in root ? (root as Element).children : (root as Document).body?.children || (root as Document).documentElement?.children
    if (!children) return null
    for (let i = 0; i < children.length; i++) {
      const child = children[i]!
      const shadow = getShadowRoot(child)
      if (shadow) {
        const match = shadow.querySelector(sel)
        if (match && !isKaboomOwnedElement(match)) return match
        const deep = querySelectorDeepWalk(sel, shadow, depth + 1)
        if (deep) return deep
      }
      if (child.children.length > 0) { const deep = querySelectorDeepWalk(sel, child, depth + 1); if (deep) return deep }
    }
    return null
  }

  function querySelectorAllDeep(sel: string, root: ParentNode = document, results: Element[] = [], depth = 0): Element[] {
    if (depth > 10) return results
    for (const match of Array.from(root.querySelectorAll(sel))) { if (!isKaboomOwnedElement(match)) results.push(match) }
    const children = 'children' in root ? (root as Element).children : (root as Document).body?.children || (root as Document).documentElement?.children
    if (!children) return results
    for (let i = 0; i < children.length; i++) { const shadow = getShadowRoot(children[i]!); if (shadow) querySelectorAllDeep(sel, shadow, results, depth + 1) }
    return results
  }

  function resolveDeepCombinator(sel: string, root: ParentNode = document): Element | null {
    const parts = sel.split(' >>> ')
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
      } else { return querySelectorDeep(part, current) }
    }
    return null
  }

  function isKaboomOwnedElement(element: Element | null): boolean {
    let node: Element | null = element
    while (node) {
      const id = (node as HTMLElement).id || ''
      if (id.startsWith('kaboom-')) return true
      const className = (node as HTMLElement).className
      if (typeof className === 'string' && className.includes('kaboom-')) return true
      if (node.getAttribute && node.getAttribute('data-kaboom-owned') === 'true') return true
      node = node.parentElement
    }
    return false
  }

  function isVisible(el: Element): boolean {
    if (isKaboomOwnedElement(el)) return false
    if (!(el instanceof HTMLElement)) return true
    const style = getComputedStyle(el)
    if (style.visibility === 'hidden' || style.display === 'none') return false
    if (el.offsetParent === null && style.position !== 'fixed' && style.position !== 'sticky') {
      const rect = el.getBoundingClientRect()
      if (rect.width === 0 && rect.height === 0) return false
    }
    return true
  }

  function firstVisible(els: NodeListOf<Element> | Element[]): Element | null {
    let fb: Element | null = null
    for (const el of els) { if (!fb) fb = el; if (isVisible(el)) return el }
    return fb
  }

  function isActionableVisible(el: Element): boolean {
    if (!(el instanceof HTMLElement)) return true
    const rect = typeof el.getBoundingClientRect === 'function' ? el.getBoundingClientRect() : ({ width: 0, height: 0 } as DOMRect)
    if (!(rect.width > 0 && rect.height > 0)) return false
    if (el.offsetParent === null) {
      const s = typeof getComputedStyle === 'function' ? getComputedStyle(el) : null
      if (s?.position !== 'fixed' && s?.position !== 'sticky') return false
    }
    const vh = typeof window !== 'undefined' ? window.innerHeight ?? 0 : 0
    const vw = typeof window !== 'undefined' ? window.innerWidth ?? 0 : 0
    const l = rect.left ?? rect.x ?? 0, t = rect.top ?? rect.y ?? 0
    const r = rect.right ?? l + rect.width, b = rect.bottom ?? t + rect.height
    return (vw <= 0 || (r > 0 && l < vw)) && (vh <= 0 || (b > 0 && t < vh))
  }

  function resolveScopeRoot(rawScope?: string): ParentNode | null {
    const scope = (rawScope || '').trim()
    if (!scope) return document
    try { const m = querySelectorAllDeep(scope); return m.length === 0 ? null : firstVisible(m) || m[0] || null } catch { return null }
  }

  const scopeRoot = resolveScopeRoot(options.scope_selector)
  type ScopeRect = { x: number; y: number; width: number; height: number }

  function parseScopeRect(raw: unknown): ScopeRect | null {
    if (!raw || typeof raw !== 'object') return null
    const r = raw as Record<string, unknown>
    const x = Number(r.x), y = Number(r.y), w = Number(r.width), h = Number(r.height)
    if (![x, y, w, h].every(Number.isFinite)) return null
    return w > 0 && h > 0 ? { x, y, width: w, height: h } : null
  }

  const scopeRect = parseScopeRect(options.scope_rect)
  if (options.scope_rect !== undefined && !scopeRect) {
    return { success: false, action, selector, error: 'invalid_scope_rect', message: 'scope_rect must include finite x, y, width, and height > 0' }
  }

  function intersectsScopeRect(el: Element): boolean {
    if (!scopeRect) return true
    const htmlEl = el as HTMLElement
    if (!htmlEl || typeof htmlEl.getBoundingClientRect !== 'function') return false
    const r = htmlEl.getBoundingClientRect()
    const l = r.left ?? r.x ?? 0, t = r.top ?? r.y ?? 0
    const ri = r.right ?? l + r.width, b = r.bottom ?? t + r.height
    return l < scopeRect.x + scopeRect.width && ri > scopeRect.x && t < scopeRect.y + scopeRect.height && b > scopeRect.y
  }

  function filterByScopeRect(elements: Element[]): Element[] {
    return scopeRect ? elements.filter(intersectsScopeRect) : elements
  }

  // --- Element handle store ---
  type ElementHandleStore = { byElement: WeakMap<Element, string>; byID: Map<string, Element>; selectorByID: Map<string, string>; nextID: number }

  function getElementHandleStore(): ElementHandleStore {
    const root = globalThis as typeof globalThis & { __kaboomElementHandles?: ElementHandleStore }
    if (root.__kaboomElementHandles) {
      if (!root.__kaboomElementHandles.selectorByID) root.__kaboomElementHandles.selectorByID = new Map()
      return root.__kaboomElementHandles
    }
    const created: ElementHandleStore = { byElement: new WeakMap(), byID: new Map(), selectorByID: new Map(), nextID: 1 }
    root.__kaboomElementHandles = created
    return created
  }

  function getOrCreateElementID(el: Element): string {
    const store = getElementHandleStore()
    const existing = store.byElement.get(el)
    if (existing) { store.byID.set(existing, el); return existing }
    const eid = `el_${(store.nextID++).toString(36)}`
    store.byElement.set(el, eid); store.byID.set(eid, el)
    return eid
  }

  function resolveElementByID(rawEID?: string): Element | null {
    const eid = (rawEID || '').trim()
    if (!eid) return null
    const store = getElementHandleStore()
    const node = store.byID.get(eid)
    if (node && (node as Node).isConnected !== false) return node
    const storedSel = store.selectorByID.get(eid)
    if (storedSel) {
      const reresolved = resolveElement(storedSel, document)
      if (reresolved && (reresolved as Node).isConnected !== false) {
        store.byElement.set(reresolved, eid)
        store.byID.set(eid, reresolved)
        return reresolved
      }
    }
    if (node) store.byID.delete(eid)
    store.selectorByID.delete(eid)
    return null
  }

  // --- Semantic selector resolution ---

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
          let interactiveChild: Element | null = null
          if (!interactive && typeof parent.querySelectorAll === 'function') {
            const childCandidates = parent.querySelectorAll('a[href], button, input:not([type="hidden"]), select, textarea, [role="button"], [role="link"]')
            for (let ci = 0; ci < childCandidates.length; ci++) {
              const child = childCandidates[ci]!
              if (isActionableVisible(child)) { interactiveChild = child; break }
            }
          }
          const target = interactive || interactiveChild || parent
          if (isKaboomOwnedElement(target) || !isVisible(target)) continue
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
          let interactiveChild: Element | null = null
          if (!interactive && typeof parent.querySelectorAll === 'function') {
            const childCandidates = parent.querySelectorAll('a[href], button, input:not([type="hidden"]), select, textarea, [role="button"], [role="link"]')
            for (let ci = 0; ci < childCandidates.length; ci++) {
              const child = childCandidates[ci]!
              if (isActionableVisible(child)) { interactiveChild = child; break }
            }
          }
          const target = interactive || interactiveChild || parent
          if (isKaboomOwnedElement(target)) continue
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

  function parseNthMatchSelector(sel: string): { base: string; n: number } | null {
    const nthMatch = sel.match(/^(.*):nth-match\((\d+)\)$/)
    if (!nthMatch) return null
    const base = nthMatch[1] || ''
    const n = Number.parseInt(nthMatch[2] || '0', 10)
    if (!base || Number.isNaN(n) || n < 1) return null
    return { base, n }
  }

  function resolveElements(sel: string, scope: ParentNode = document): Element[] {
    if (!sel) return []
    if (sel.includes(' >>> ')) {
      const deep = resolveDeepCombinator(sel, scope)
      return deep ? [deep] : []
    }
    const parsedNth = parseNthMatchSelector(sel)
    if (parsedNth) {
      const matches = resolveElements(parsedNth.base, scope)
      const target = matches[parsedNth.n - 1]
      return target ? [target] : []
    }
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
    const parsedNth = parseNthMatchSelector(sel)
    if (parsedNth) {
      const matches = resolveElements(parsedNth.base, scope)
      return matches[parsedNth.n - 1] || null
    }
    if (sel.startsWith('text=')) return resolveByText(sel.slice('text='.length), scope)
    if (sel.startsWith('role=')) return firstVisible(querySelectorAllDeep(`[role="${CSS.escape(sel.slice('role='.length))}"]`, scope))
    if (sel.startsWith('placeholder=')) return firstVisible(querySelectorAllDeep(`[placeholder="${CSS.escape(sel.slice('placeholder='.length))}"]`, scope))
    if (sel.startsWith('label=')) return resolveByLabel(sel.slice('label='.length), scope)
    if (sel.startsWith('aria-label=')) return resolveByAriaLabel(sel.slice('aria-label='.length), scope)
    return querySelectorDeep(sel, scope)
  }

  // --- Helpers for result building ---

  function uniqueElements(elements: Element[]): Element[] {
    const out: Element[] = []
    const seen = new Set<Element>()
    for (const element of elements) {
      if (seen.has(element)) continue
      seen.add(element)
      out.push(element)
    }
    return out
  }

  function buildUniqueSelector(el: Element, htmlEl: HTMLElement, fallbackSelector: string): string {
    if (el.id) return `#${CSS.escape(el.id)}`
    if (el instanceof HTMLInputElement && el.name) return `input[name="${CSS.escape(el.name)}"]`
    const ariaLabel = el.getAttribute('aria-label')
    if (ariaLabel) return `[aria-label="${CSS.escape(ariaLabel)}"]`
    const placeholder = el.getAttribute('placeholder')
    if (placeholder) return `[placeholder="${CSS.escape(placeholder)}"]`
    const text = (htmlEl.textContent || '').trim().slice(0, 40)
    if (text) return `text=${text}`
    return fallbackSelector
  }

  function extractBoundingBox(el: Element): { x: number; y: number; width: number; height: number } {
    if (!(el instanceof HTMLElement) || typeof el.getBoundingClientRect !== 'function') {
      return { x: 0, y: 0, width: 0, height: 0 }
    }
    const rect = el.getBoundingClientRect()
    const x = typeof rect.left === 'number' ? rect.left : (typeof rect.x === 'number' ? rect.x : 0)
    const y = typeof rect.top === 'number' ? rect.top : (typeof rect.y === 'number' ? rect.y : 0)
    const width = Number.isFinite(rect.width) ? rect.width : 0
    const height = Number.isFinite(rect.height) ? rect.height : 0
    return { x, y, width, height }
  }

  function domError(error: string, message: string): DOMResult {
    return { success: false, action, selector, error, message }
  }

  // --- Simplified element resolution (read actions are NOT ambiguity-sensitive) ---

  function resolveReadTarget(): {
    element?: Element
    error?: DOMResult
    match_count?: number
    match_strategy?: string
    scope_selector_used?: string
    ambiguous_matches?: { total_count: number; warning: string; candidates: { tag: string; element_id: string; text_preview?: string }[] }
  } {
    const requestedScope = (options.scope_selector || '').trim()
    if (requestedScope && !scopeRoot) {
      return {
        error: domError('scope_not_found', `No scope element matches selector: ${requestedScope}`)
      }
    }
    const activeScope = scopeRoot || document
    const scopeSelectorUsed = requestedScope || undefined

    // wait_for_text and wait_for_absent target document.body
    if (action === 'wait_for_text' || action === 'wait_for_absent') {
      return { element: document.body, match_count: 1, match_strategy: action }
    }

    const requestedElementID = (options.element_id || '').trim()
    if (requestedElementID) {
      const resolvedByID = resolveElementByID(requestedElementID)
      if (!resolvedByID) {
        return {
          error: domError(
            'stale_element_id',
            `Element handle is stale or unknown: ${requestedElementID}. Call list_interactive again.`
          )
        }
      }
      if (activeScope !== document && typeof (activeScope as Element).contains === 'function') {
        const contains = (activeScope as Element).contains(resolvedByID)
        if (!contains) {
          return {
            error: domError(
              'element_id_scope_mismatch',
              `Element handle does not belong to scope: ${requestedScope || '<none>'}`
            )
          }
        }
      }
      if (scopeRect && !intersectsScopeRect(resolvedByID)) {
        return {
          error: domError(
            'element_id_scope_mismatch',
            `Element handle does not intersect scope_rect (${scopeRect.x}, ${scopeRect.y}, ${scopeRect.width}, ${scopeRect.height}).`
          )
        }
      }
      return {
        element: resolvedByID,
        match_count: 1,
        match_strategy: 'element_id',
        scope_selector_used: scopeSelectorUsed
      }
    }

    // nth parameter for explicit disambiguation
    const nthParam = options.nth
    if (nthParam !== undefined && nthParam !== null) {
      const nth = Number(nthParam)
      if (!Number.isInteger(nth)) {
        return { error: domError('invalid_nth', `nth must be an integer, got: ${nthParam}`) }
      }
      const allMatches = resolveElements(selector, activeScope)
      const uniqueAll = uniqueElements(allMatches)
      const rectFiltered = filterByScopeRect(uniqueAll)
      const visibleFiltered = rectFiltered.filter(isActionableVisible)
      const candidates = visibleFiltered.length > 0 ? visibleFiltered : rectFiltered
      if (candidates.length === 0) {
        return { error: domError('element_not_found', `No element matches selector: ${selector}`) }
      }
      const resolvedIndex = nth < 0 ? candidates.length + nth : nth
      if (resolvedIndex < 0 || resolvedIndex >= candidates.length) {
        return {
          error: domError(
            'nth_out_of_range',
            `nth=${nth} is out of range — selector matched ${candidates.length} element(s). Use nth 0..${candidates.length - 1} or -1..-${candidates.length}.`
          )
        }
      }
      return {
        element: candidates[resolvedIndex]!,
        match_count: candidates.length,
        match_strategy: 'nth_param',
        scope_selector_used: scopeSelectorUsed
      }
    }

    // Read actions use non-ambiguity-sensitive resolution: first match wins
    const allMatches = selector.startsWith('text=') ? resolveElements(selector, activeScope) : null
    const ambiguousInfo = (() => {
      if (!allMatches || allMatches.length <= 1) return undefined
      const uniqueAll = uniqueElements(allMatches)
      if (uniqueAll.length <= 1) return undefined
      return {
        total_count: uniqueAll.length,
        warning: `Selector "${selector}" matched ${uniqueAll.length} elements. First match was used. Use nth, :nth-match(N), or scope_selector to disambiguate.`,
        candidates: uniqueAll.slice(0, 5).map((c) => ({
          tag: c.tagName.toLowerCase(),
          element_id: getOrCreateElementID(c),
          text_preview: ((c as HTMLElement).textContent || '').trim().slice(0, 60) || undefined
        }))
      }
    })()

    const direct = resolveElement(selector, activeScope)
    if (direct && intersectsScopeRect(direct)) {
      return {
        element: direct,
        match_count: 1,
        match_strategy: selector.includes(':nth-match(')
          ? 'nth_match_selector'
          : (scopeRect ? 'rect_selector' : (requestedScope ? 'scoped_selector' : 'selector')),
        scope_selector_used: scopeSelectorUsed,
        ...(ambiguousInfo ? { ambiguous_matches: ambiguousInfo } : {})
      }
    }
    const scopedMatches = filterByScopeRect(uniqueElements(resolveElements(selector, activeScope)))
    const found = (() => {
      if (scopedMatches.length === 0) return null
      const visible = scopedMatches.filter(isActionableVisible)
      return visible[0] || scopedMatches[0] || null
    })()
    if (!found) return { error: domError('element_not_found', `No element matches selector: ${selector}`) }
    return {
      element: found,
      match_count: 1,
      match_strategy: scopeRect ? 'rect_selector' : (requestedScope ? 'scoped_selector' : 'selector'),
      scope_selector_used: scopeSelectorUsed,
      ...(ambiguousInfo ? { ambiguous_matches: ambiguousInfo } : {})
    }
  }

  const resolved = resolveReadTarget()
  if (resolved.error) return resolved.error
  const el = resolved.element!
  const resolvedScopeSelector = resolved.scope_selector_used
  const resolvedAmbiguousMatches = resolved.ambiguous_matches

  function matchedTarget(node: Element): NonNullable<DOMResult['matched']> {
    const htmlEl = node as HTMLElement
    const textPreview = (htmlEl.textContent || '').trim().slice(0, 80)
    const classList = typeof htmlEl.className === 'string' && htmlEl.className
      ? htmlEl.className.split(/\s+/).filter(Boolean).slice(0, 5)
      : undefined
    return {
      tag: node.tagName.toLowerCase(),
      role: node.getAttribute('role') || undefined,
      aria_label: node.getAttribute('aria-label') || undefined,
      text_preview: textPreview || undefined,
      classes: classList && classList.length > 0 ? classList : undefined,
      selector,
      element_id: getOrCreateElementID(node),
      bbox: extractBoundingBox(node),
      scope_selector_used: resolvedScopeSelector,
      ...(scopeRect ? { scope_rect_used: scopeRect } : {})
    }
  }

  // --- Action handlers ---

  type ActionHandler = () => DOMResult

  const handlers: Record<string, ActionHandler> = {
    get_text: () => {
      if (options.structured && el instanceof HTMLElement) {
        const sections: Array<{header?: string; content: string; expanded?: boolean; tag: string}> = []
        const children = el.children
        for (let i = 0; i < children.length && sections.length < 50; i++) {
          const child = children[i] as HTMLElement
          if (!child.tagName) continue
          const tag = child.tagName.toLowerCase()
          const heading = child.querySelector('h1, h2, h3, h4, h5, h6, [role="heading"], summary, button[aria-expanded]')
          if (heading) {
            const headerText = (heading as HTMLElement).innerText?.trim() || ''
            const ariaExpanded = heading.getAttribute('aria-expanded')
            const expanded = ariaExpanded !== null ? ariaExpanded === 'true' : undefined
            const contentParts: string[] = []
            const contentNodes = child.querySelectorAll('p, li, span, div, td, pre, code')
            contentNodes.forEach((cn) => {
              if (cn !== heading && !heading.contains(cn)) {
                const t = (cn as HTMLElement).innerText?.trim()
                if (t && t.length > 0) contentParts.push(t)
              }
            })
            sections.push({
              header: headerText,
              content: contentParts.join('\n') || (child.innerText?.replace(headerText, '').trim() || ''),
              expanded,
              tag,
            })
          } else {
            const t = child.innerText?.trim()
            if (t && t.length > 0) {
              sections.push({ content: t, tag })
            }
          }
        }
        return { success: true, action, selector, sections, section_count: sections.length }
      }
      const text = el instanceof HTMLElement ? el.innerText : el.textContent
      if (text === null || text === undefined) {
        return {
          success: true,
          action,
          selector,
          value: text,
          reason: 'no_text_content',
          message: 'Resolved text content is null'
        }
      }
      return { success: true, action, selector, value: text }
    },

    get_value: () => {
      if (!('value' in el)) return domError('no_value_property', `Element has no value property: ${el.tagName}`)
      const value = (el as HTMLInputElement).value
      if (value === null || value === undefined) {
        return {
          success: true,
          action,
          selector,
          value,
          reason: 'no_value',
          message: 'Element value is null'
        }
      }
      return { success: true, action, selector, value }
    },

    get_attribute: () => {
      const attrName = options.name || ''
      const value = el.getAttribute(attrName)
      if (value === null) {
        return {
          success: true,
          action,
          selector,
          value,
          reason: 'attribute_not_found',
          message: `Attribute "${attrName}" not found`
        }
      }
      return { success: true, action, selector, value }
    },

    wait_for: () => ({ success: true, action, selector, value: el.tagName.toLowerCase() }),

    wait_for_text: () => {
      const searchText = options.text || ''
      if (!searchText) {
        return { success: false, action, selector: '', error: 'empty_text', message: 'text parameter is required for wait_for_text' } as DOMResult
      }
      const bodyText = document.body?.innerText ?? ''
      if (bodyText.includes(searchText)) {
        return { success: true, action, selector: '', matched_text: searchText } as DOMResult
      }
      return { success: false, action, selector: '', error: 'text_not_found' } as DOMResult
    },

    wait_for_absent: () => {
      if (!selector) {
        return { success: false, action, selector: '', error: 'missing_selector', message: 'selector is required for wait_for_absent' } as DOMResult
      }
      const found = resolveElement(selector)
      if (!found) {
        return { success: true, action, selector, absent: true } as DOMResult
      }
      return { success: false, action, selector, error: 'element_still_present' } as DOMResult
    },
  }

  const handler = handlers[action]
  if (!handler) {
    return domError('unknown_action', `Unknown read/wait action: ${action}`)
  }

  const rawResult = handler()
  if (!resolvedAmbiguousMatches) return rawResult
  if (rawResult && typeof rawResult === 'object' && (rawResult as DOMResult).success) {
    return { ...(rawResult as DOMResult), ambiguous_matches: resolvedAmbiguousMatches }
  }
  return rawResult
}
