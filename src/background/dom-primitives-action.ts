/**
 * Purpose: Self-contained DOM primitives for mutating selector-based actions
 *   (click, type, select, check, set_attribute, paste, key_press, hover, focus, scroll_to).
 * Why: Extracted from dom-primitives.ts to keep file sizes under the 800 LOC limit.
 *      These are ambiguity-sensitive actions that need full ranking + overlay detection.
 * Docs: docs/features/feature/interact-explore/index.md
 */
// dom-primitives-action.ts — Self-contained mutating DOM primitives for chrome.scripting.executeScript.
// This function MUST remain self-contained — Chrome serializes the function source only (no closures).
// eslint-disable max-lines - Self-contained for chrome.scripting.executeScript; infra duplication is required by MV3.

import type { DOMMutationEntry, DOMPrimitiveOptions, DOMResult } from './dom-types.js'

/**
 * Self-contained function for mutating DOM actions.
 * Passed to chrome.scripting.executeScript({ func: domPrimitiveAction, args: [...] }).
 * MUST NOT reference any module-level variables — Chrome serializes the function source only.
 */
export function domPrimitiveAction(
  action: string,
  selector: string,
  options: DOMPrimitiveOptions
): DOMResult | Promise<DOMResult> {
  // --- Shadow DOM traversal ---
  function getShadowRoot(el: Element): ShadowRoot | null { return el.shadowRoot ?? null }

  function querySelectorDeep(sel: string, root: ParentNode = document): Element | null {
    const fast = root.querySelector(sel)
    if (fast && !isKaboomOwnedElement(fast)) return fast
    return querySelectorDeepWalk(sel, root)
  }

  function querySelectorDeepWalk(sel: string, root: ParentNode, depth = 0): Element | null {
    if (depth > 10) return null
    const children = 'children' in root
      ? (root as Element).children
      : (root as Document).body?.children || (root as Document).documentElement?.children
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
      if (child.children.length > 0) {
        const deep = querySelectorDeepWalk(sel, child, depth + 1)
        if (deep) return deep
      }
    }
    return null
  }

  function querySelectorAllDeep(sel: string, root: ParentNode = document, results: Element[] = [], depth = 0): Element[] {
    if (depth > 10) return results
    const matches = Array.from(root.querySelectorAll(sel))
    for (const match of matches) { if (!isKaboomOwnedElement(match)) results.push(match) }
    const children = 'children' in root
      ? (root as Element).children
      : (root as Document).body?.children || (root as Document).documentElement?.children
    if (!children) return results
    for (let i = 0; i < children.length; i++) {
      const child = children[i]!
      const shadow = getShadowRoot(child)
      if (shadow) querySelectorAllDeep(sel, shadow, results, depth + 1)
    }
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

  // --- Visibility / ownership ---
  function isKaboomOwnedElement(element: Element | null): boolean {
    let node: Element | null = element
    while (node) {
      const id = (node as HTMLElement).id || ''
      if (id.startsWith('kaboom-')) return true
      const cn = (node as HTMLElement).className
      if (typeof cn === 'string' && cn.includes('kaboom-')) return true
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

  // --- Scope resolution ---
  function resolveScopeRoot(rawScope?: string): ParentNode | null {
    const scope = (rawScope || '').trim()
    if (!scope) return document
    try {
      const m = querySelectorAllDeep(scope)
      if (m.length === 0) return null
      return firstVisible(m) || m[0] || null
    } catch { return null }
  }

  const scopeRoot = resolveScopeRoot(options.scope_selector)
  type ScopeRect = { x: number; y: number; width: number; height: number }

  function parseScopeRect(raw: unknown): ScopeRect | null {
    if (!raw || typeof raw !== 'object') return null
    const r = raw as Record<string, unknown>
    const x = Number(r.x), y = Number(r.y), w = Number(r.width), h = Number(r.height)
    if (![x, y, w, h].every(Number.isFinite)) return null
    if (w <= 0 || h <= 0) return null
    return { x, y, width: w, height: h }
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
    if (!scopeRect) return elements
    return elements.filter(intersectsScopeRect)
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
      const re = resolveElement(storedSel, document)
      if (re && (re as Node).isConnected !== false) {
        store.byElement.set(re, eid); store.byID.set(eid, re); return re
      }
    }
    if (node) store.byID.delete(eid)
    store.selectorByID.delete(eid)
    return null
  }

  // --- Semantic selector resolution ---
  function resolveByTextAll(searchText: string, scope: ParentNode = document): Element[] {
    const results: Element[] = [], seen = new Set<Element>()
    function walkScope(root: ParentNode): void {
      const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT)
      while (walker.nextNode()) {
        const n = walker.currentNode
        if (n.textContent && n.textContent.trim().includes(searchText)) {
          const parent = n.parentElement
          if (!parent) continue
          const interactive = parent.closest('a, button, [role="button"], [role="link"], label, summary')
          let interactiveChild: Element | null = null
          if (!interactive && typeof parent.querySelectorAll === 'function') {
            const cc = parent.querySelectorAll('a[href], button, input:not([type="hidden"]), select, textarea, [role="button"], [role="link"]')
            for (let ci = 0; ci < cc.length; ci++) { const c = cc[ci]!; if (isActionableVisible(c)) { interactiveChild = c; break } }
          }
          const target = interactive || interactiveChild || parent
          if (isKaboomOwnedElement(target) || !isVisible(target)) continue
          if (!seen.has(target)) { seen.add(target); results.push(target) }
        }
      }
      const ch = 'children' in root ? (root as Element).children : (root as Document).body?.children || (root as Document).documentElement?.children
      if (ch) { for (let i = 0; i < ch.length; i++) { const c = ch[i]!; const s = getShadowRoot(c); if (s) walkScope(s) } }
    }
    walkScope(scope)
    return results
  }

  function resolveByLabelAll(labelText: string, scope: ParentNode = document): Element[] {
    const labels = querySelectorAllDeep('label', scope), results: Element[] = [], seen = new Set<Element>()
    const allowGlobal = scope === document || scope === document.body || scope === document.documentElement
    for (const label of labels) {
      if (label.textContent && label.textContent.trim().includes(labelText)) {
        const forAttr = label.getAttribute('for')
        if (forAttr) {
          const local = querySelectorAllDeep(`#${CSS.escape(forAttr)}`, scope)[0]
          const target = local || (allowGlobal ? document.getElementById(forAttr) : null)
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
    const results: Element[] = [], seen = new Set<Element>()
    for (const el of querySelectorAllDeep(`[aria-label="${CSS.escape(al)}"]`, scope)) { if (!seen.has(el)) { seen.add(el); results.push(el) } }
    for (const el of querySelectorAllDeep('[aria-label]', scope)) {
      const lb = el.getAttribute('aria-label') || ''
      if (lb.startsWith(al) && !seen.has(el)) { seen.add(el); results.push(el) }
    }
    return results
  }

  function resolveByText(searchText: string, scope: ParentNode = document): Element | null {
    let fallback: Element | null = null
    function walkScope(root: ParentNode): Element | null {
      const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT)
      while (walker.nextNode()) {
        const n = walker.currentNode
        if (n.textContent && n.textContent.trim().includes(searchText)) {
          const parent = n.parentElement
          if (!parent) continue
          const interactive = parent.closest('a, button, [role="button"], [role="link"], label, summary')
          let interactiveChild: Element | null = null
          if (!interactive && typeof parent.querySelectorAll === 'function') {
            const cc = parent.querySelectorAll('a[href], button, input:not([type="hidden"]), select, textarea, [role="button"], [role="link"]')
            for (let ci = 0; ci < cc.length; ci++) { const c = cc[ci]!; if (isActionableVisible(c)) { interactiveChild = c; break } }
          }
          const target = interactive || interactiveChild || parent
          if (isKaboomOwnedElement(target)) continue
          if (!fallback) fallback = target
          if (isVisible(target)) return target
        }
      }
      const ch = 'children' in root ? (root as Element).children : (root as Document).body?.children || (root as Document).documentElement?.children
      if (ch) { for (let i = 0; i < ch.length; i++) { const c = ch[i]!; const s = getShadowRoot(c); if (s) { const r = walkScope(s); if (r) return r } } }
      return null
    }
    return walkScope(scope) || fallback
  }

  function resolveByLabel(labelText: string, scope: ParentNode = document): Element | null {
    const labels = querySelectorAllDeep('label', scope)
    const allowGlobal = scope === document || scope === document.body || scope === document.documentElement
    for (const label of labels) {
      if (label.textContent && label.textContent.trim().includes(labelText)) {
        const forAttr = label.getAttribute('for')
        if (forAttr) {
          const local = querySelectorAllDeep(`#${CSS.escape(forAttr)}`, scope)[0]
          if (local) return local
          const target = allowGlobal ? document.getElementById(forAttr) : null
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
    let fallback: Element | null = null
    for (const el of querySelectorAllDeep('[aria-label]', scope)) {
      const lb = el.getAttribute('aria-label') || ''
      if (lb.startsWith(al)) { if (!fallback) fallback = el; if (isVisible(el)) return el }
    }
    return fallback
  }

  function parseNthMatchSelector(sel: string): { base: string; n: number } | null {
    const m = sel.match(/^(.*):nth-match\((\d+)\)$/)
    if (!m) return null
    const base = m[1] || '', n = Number.parseInt(m[2] || '0', 10)
    if (!base || Number.isNaN(n) || n < 1) return null
    return { base, n }
  }

  function resolveElements(sel: string, scope: ParentNode = document): Element[] {
    if (!sel) return []
    if (sel.includes(' >>> ')) { const d = resolveDeepCombinator(sel, scope); return d ? [d] : [] }
    const p = parseNthMatchSelector(sel)
    if (p) { const m = resolveElements(p.base, scope); const t = m[p.n - 1]; return t ? [t] : [] }
    if (sel.startsWith('text=')) return resolveByTextAll(sel.slice(5), scope)
    if (sel.startsWith('role=')) return querySelectorAllDeep(`[role="${CSS.escape(sel.slice(5))}"]`, scope)
    if (sel.startsWith('placeholder=')) return querySelectorAllDeep(`[placeholder="${CSS.escape(sel.slice(12))}"]`, scope)
    if (sel.startsWith('label=')) return resolveByLabelAll(sel.slice(6), scope)
    if (sel.startsWith('aria-label=')) return resolveByAriaLabelAll(sel.slice(11), scope)
    try { return querySelectorAllDeep(sel, scope) } catch { return [] }
  }

  function resolveElement(sel: string, scope: ParentNode = document): Element | null {
    if (!sel) return null
    if (sel.includes(' >>> ')) return resolveDeepCombinator(sel, scope)
    const p = parseNthMatchSelector(sel)
    if (p) { const m = resolveElements(p.base, scope); return m[p.n - 1] || null }
    if (sel.startsWith('text=')) return resolveByText(sel.slice(5), scope)
    if (sel.startsWith('role=')) return firstVisible(querySelectorAllDeep(`[role="${CSS.escape(sel.slice(5))}"]`, scope))
    if (sel.startsWith('placeholder=')) return firstVisible(querySelectorAllDeep(`[placeholder="${CSS.escape(sel.slice(12))}"]`, scope))
    if (sel.startsWith('label=')) return resolveByLabel(sel.slice(6), scope)
    if (sel.startsWith('aria-label=')) return resolveByAriaLabel(sel.slice(11), scope)
    return querySelectorDeep(sel, scope)
  }

  // --- Utility helpers ---
  function uniqueElements(elements: Element[]): Element[] {
    const out: Element[] = [], seen = new Set<Element>()
    for (const e of elements) { if (!seen.has(e)) { seen.add(e); out.push(e) } }
    return out
  }

  function buildUniqueSelector(el: Element, htmlEl: HTMLElement, fb: string): string {
    if (el.id) return `#${CSS.escape(el.id)}`
    if (el instanceof HTMLInputElement && el.name) return `input[name="${CSS.escape(el.name)}"]`
    const al = el.getAttribute('aria-label')
    if (al) return `[aria-label="${CSS.escape(al)}"]`
    const ph = el.getAttribute('placeholder')
    if (ph) return `[placeholder="${CSS.escape(ph)}"]`
    const txt = (htmlEl.textContent || '').trim().slice(0, 40)
    if (txt) return `text=${txt}`
    return fb
  }

  function extractBoundingBox(el: Element): { x: number; y: number; width: number; height: number } {
    if (!(el instanceof HTMLElement) || typeof el.getBoundingClientRect !== 'function') return { x: 0, y: 0, width: 0, height: 0 }
    const r = el.getBoundingClientRect()
    return { x: r.left ?? r.x ?? 0, y: r.top ?? r.y ?? 0, width: Number.isFinite(r.width) ? r.width : 0, height: Number.isFinite(r.height) ? r.height : 0 }
  }

  function extractElementLabel(el: Element): string {
    const h = el as HTMLElement
    return el.getAttribute('aria-label') || el.getAttribute('title') || el.getAttribute('placeholder') || (h?.textContent || '').trim().slice(0, 80) || el.tagName.toLowerCase()
  }

  function elementZIndexScore(el: Element): number {
    if (!(el instanceof HTMLElement)) return 0
    const p = Number.parseInt(getComputedStyle(el).zIndex || '', 10)
    return Number.isNaN(p) ? 0 : p
  }

  function areaScore(el: Element, max: number): number {
    if (!(el instanceof HTMLElement) || typeof el.getBoundingClientRect !== 'function') return 0
    const r = el.getBoundingClientRect()
    return r.width > 0 && r.height > 0 ? Math.min(max, Math.round((r.width * r.height) / 10000)) : 0
  }

  function collectDialogs(): Element[] {
    const d: Element[] = []
    for (const s of ['[role="dialog"]', '[aria-modal="true"]', 'dialog[open]']) d.push(...querySelectorAllDeep(s))
    return uniqueElements(d).filter(isActionableVisible)
  }

  function pickTopDialog(dialogs: Element[]): Element | null {
    if (dialogs.length === 0) return null
    const ranked = dialogs.map((d, i) => ({ element: d, score: elementZIndexScore(d) * 1000 + areaScore(d, 200) + i })).sort((a, b) => b.score - a.score)
    return ranked[0]?.element || null
  }

  function domError(error: string, message: string): DOMResult { return { success: false, action, selector, error, message } }

  // --- Overlay detection ---
  function findTopmostOverlay(): Element | null {
    const sels = ['[role="dialog"]','[role="alertdialog"]','[aria-modal="true"]','dialog[open]','.modal.show','.modal.in','.modal.is-active','.modal[style*="display: block"]','.overlay','.popup','.lightbox','[data-modal]','[data-overlay]','[data-dialog]']
    const cands: Element[] = []
    for (const s of sels) cands.push(...querySelectorAllDeep(s))
    const all = document.querySelectorAll('*')
    for (let i = 0; i < all.length; i++) {
      const el = all[i]!
      if (!(el instanceof HTMLElement)) continue
      const st = getComputedStyle(el)
      const z = Number.parseInt(st.zIndex || '', 10)
      if (Number.isNaN(z) || z < 1000) continue
      if (st.position !== 'fixed' && st.position !== 'absolute') continue
      const r = el.getBoundingClientRect()
      if (r.width < 100 || r.height < 100) continue
      if (st.display === 'none' || st.visibility === 'hidden' || st.opacity === '0') continue
      cands.push(el)
    }
    const u = uniqueElements(cands).filter(isActionableVisible)
    if (u.length === 0) return null
    const ranked = u.map((c, i) => ({ element: c, score: elementZIndexScore(c) * 1000 + areaScore(c, 200) + i })).sort((a, b) => b.score - a.score)
    return ranked[0]?.element || null
  }

  function describeOverlay(el: Element): { overlay_type: string; overlay_selector: string } {
    const tag = el.tagName.toLowerCase(), role = el.getAttribute('role') || '', am = el.getAttribute('aria-modal') || ''
    const ot = tag === 'dialog' ? 'dialog' : (role === 'dialog' || role === 'alertdialog') ? role : am === 'true' ? 'modal' : 'overlay'
    const os = el.id ? `#${el.id}` : role ? `${tag}[role="${role}"]` : (() => { const cn = (el as HTMLElement).className; return typeof cn === 'string' && cn.trim() ? `${tag}.${cn.trim().split(/\s+/)[0]}` : tag })()
    return { overlay_type: ot, overlay_selector: os }
  }

  function detectOverlayWarning(targetEl: Element): { overlay_warning?: string; overlay_selector?: string } {
    const o = findTopmostOverlay()
    if (!o) return {}
    if (typeof (o as { contains?: unknown }).contains === 'function' && o.contains(targetEl)) return {}
    const info = describeOverlay(o)
    return { overlay_warning: `An overlay (${info.overlay_type}) is covering the page. The action targeted the intended element, but input may be intercepted. Use dismiss_top_overlay to close it first.`, overlay_selector: info.overlay_selector }
  }

  function blockedByOverlayError(target: Element): DOMResult | null {
    const dialogs = collectDialogs()
    if (dialogs.length === 0) return null
    const top = pickTopDialog(dialogs)
    if (!top) return null
    if (typeof top.contains === 'function' && top.contains(target)) return null
    const tag = top.tagName.toLowerCase(), role = top.getAttribute('role') || '', al = top.getAttribute('aria-label') || ''
    const desc = al ? `${tag}[aria-label="${al}"]` : role ? `${tag}[role="${role}"]` : tag
    return domError('blocked_by_overlay', `Element is behind a modal overlay (${desc}). Use interact({what:"dismiss_top_overlay"}) to close it first.`)
  }

  // --- Ambiguous target ranking ---
  function summarizeCandidates(matches: Element[]): NonNullable<DOMResult['candidates']> {
    return matches.slice(0, 8).map((c) => {
      const h = c as HTMLElement, fb = c.tagName.toLowerCase()
      return { tag: fb, role: c.getAttribute('role') || undefined, aria_label: c.getAttribute('aria-label') || undefined, text_preview: (h.textContent || '').trim().slice(0, 80) || undefined, selector: buildUniqueSelector(c, h, fb), element_id: getOrCreateElementID(c), bbox: extractBoundingBox(c), visible: isActionableVisible(c) }
    })
  }

  function rankAmbiguousCandidates(candidates: Element[], act: string, selectorText: string): { winner: Element | null; gap: number; ranked: { element: Element; score: number }[] } {
    const dialogs = collectDialogs(), topDialog = dialogs.length > 0 ? pickTopDialog(dialogs) : null
    const selectorLabel = selectorText.startsWith('text=') ? selectorText.slice(5) : selectorText.startsWith('aria-label=') ? selectorText.slice(11) : selectorText.startsWith('label=') ? selectorText.slice(6) : selectorText.startsWith('placeholder=') ? selectorText.slice(12) : ''
    const clickLike = new Set(['click','key_press','focus','scroll_to','set_attribute','paste']), typeLike = new Set(['type','select','check'])
    const scored = candidates.map((el) => {
      const tag = el.tagName.toLowerCase(), role = el.getAttribute('role') || ''
      let score = 0
      if (topDialog && typeof topDialog.contains === 'function' && topDialog.contains(el)) score += 200
      if (clickLike.has(act)) { if (tag === 'button' || role === 'button' || (tag === 'input' && ((el as HTMLInputElement).type === 'submit' || (el as HTMLInputElement).type === 'button'))) score += 100; else if (tag === 'a' || role === 'link') score += 40 }
      else if (typeLike.has(act)) { if (tag === 'input' || tag === 'textarea' || tag === 'select' || el.getAttribute('contenteditable') === 'true' || role === 'textbox') score += 100; else if (tag === 'button' || role === 'button') score += 10 }
      if (selectorLabel) { const lb = extractElementLabel(el).trim(); if (lb === selectorLabel) score += 80; else if (lb.startsWith(selectorLabel) && lb.length <= selectorLabel.length + 5) score += 60 }
      if (tag === 'button' || role === 'button') { const h = el as HTMLElement, cls = (typeof h.className === 'string' ? h.className : '').toLowerCase(), tp = el.getAttribute('type') || ''; if (tp === 'submit') score += 60; else if (/\bprimary\b|\bbtn-primary\b|\bcta\b/.test(cls)) score += 60; else { const st = typeof getComputedStyle === 'function' ? getComputedStyle(h) : null; if (st) { const bg = st.backgroundColor || ''; if (bg && !/transparent|rgba\(0,\s*0,\s*0,\s*0\)|rgb\(255,\s*255,\s*255\)|rgb\(2[45]\d,\s*2[45]\d,\s*2[45]\d\)/.test(bg)) score += 30 } } }
      score += Math.min(50, Math.max(0, elementZIndexScore(el))) + areaScore(el, 30)
      return { element: el, score }
    })
    scored.sort((a, b) => b.score - a.score)
    const gap = (scored[0]?.score ?? 0) - (scored[1]?.score ?? 0)
    return { winner: gap >= 50 ? (scored[0]?.element ?? null) : null, gap, ranked: scored }
  }

  // --- Target resolution ---
  function resolveActionTarget(): { element?: Element; error?: DOMResult; match_count?: number; match_strategy?: string; scope_selector_used?: string; ranked_candidates?: { element_id: string; tag: string; text_preview?: string; score: number }[] } {
    const reqScope = (options.scope_selector || '').trim()
    if (reqScope && !scopeRoot) return { error: domError('scope_not_found', `No scope element matches selector: ${reqScope}`) }
    const activeScope = scopeRoot || document, scopeUsed = reqScope || undefined
    if (action === 'key_press' && !selector && !options.element_id) {
      const target = document.activeElement || document.body
      if (target) return { element: target, match_count: 1, match_strategy: 'active_element_fallback' }
    }
    const reqEID = (options.element_id || '').trim()
    if (reqEID) {
      const resolved = resolveElementByID(reqEID)
      if (!resolved) return { error: domError('stale_element_id', `Element handle is stale or unknown: ${reqEID}. Call list_interactive again.`) }
      if (activeScope !== document && typeof (activeScope as Element).contains === 'function' && !(activeScope as Element).contains(resolved)) return { error: domError('element_id_scope_mismatch', `Element handle does not belong to scope: ${reqScope || '<none>'}`) }
      if (scopeRect && !intersectsScopeRect(resolved)) return { error: domError('element_id_scope_mismatch', `Element handle does not intersect scope_rect.`) }
      return { element: resolved, match_count: 1, match_strategy: 'element_id', scope_selector_used: scopeUsed }
    }
    const nthParam = options.nth
    if (nthParam !== undefined && nthParam !== null) {
      const nth = Number(nthParam)
      if (!Number.isInteger(nth)) return { error: domError('invalid_nth', `nth must be an integer, got: ${nthParam}`) }
      const all = uniqueElements(resolveElements(selector, activeScope)), rf = filterByScopeRect(all)
      const vf = rf.filter(isActionableVisible), cands = vf.length > 0 ? vf : rf
      if (cands.length === 0) return { error: domError('element_not_found', `No element matches selector: ${selector}`) }
      const ri = nth < 0 ? cands.length + nth : nth
      if (ri < 0 || ri >= cands.length) return { error: domError('nth_out_of_range', `nth=${nth} is out of range — selector matched ${cands.length} element(s). Use nth 0..${cands.length - 1} or -1..-${cands.length}.`) }
      return { element: cands[ri]!, match_count: cands.length, match_strategy: 'nth_param', scope_selector_used: scopeUsed }
    }
    const raw = resolveElements(selector, activeScope), um = uniqueElements(raw), rsm = filterByScopeRect(um)
    const vm = (() => { if (rsm.length === 0) return rsm; const v = rsm.filter(isActionableVisible); return v.length > 0 ? v : rsm })()
    if (vm.length > 1) {
      const ranking = rankAmbiguousCandidates(vm, action, selector)
      const tc = ranking.ranked.slice(0, 3).map((e) => ({ element_id: getOrCreateElementID(e.element), tag: e.element.tagName.toLowerCase(), text_preview: ((e.element as HTMLElement).textContent || '').trim().slice(0, 60) || undefined, score: e.score }))
      if (ranking.winner) return { element: ranking.winner, match_count: 1, match_strategy: 'ranked_resolution', ranked_candidates: tc }
      return { error: { success: false, action, selector, error: 'ambiguous_target', message: `Selector matches multiple viable elements: ${selector}. Add nth, scope/scope_rect, or use list_interactive element_id/index.`, match_count: vm.length, match_strategy: 'ambiguous_ranked', ...(scopeRect ? { scope_rect_used: scopeRect } : {}), candidates: summarizeCandidates(ranking.ranked.map((e) => e.element)), ranked_candidates: tc, suggested_element_id: getOrCreateElementID(ranking.ranked[0]!.element) } }
    }
    const found = vm[0] || null
    if (!found) return { error: domError('element_not_found', `No element matches selector: ${selector}`) }
    const strategy = selector.includes(':nth-match(') ? 'nth_match_selector' : scopeRect ? 'rect_selector' : reqScope ? 'scoped_selector' : 'selector'
    return { element: found, match_count: 1, match_strategy: strategy, scope_selector_used: scopeUsed }
  }

  const resolved = resolveActionTarget()
  if (resolved.error) return resolved.error
  const el = resolved.element!
  const resolvedMatchCount = resolved.match_count || 1
  const resolvedMatchStrategy = resolved.match_strategy || 'selector'
  const resolvedScopeSelector = resolved.scope_selector_used
  const resolvedRankedCandidates = resolved.ranked_candidates

  // --- Action result helpers ---
  function captureViewport() {
    const w = typeof window !== 'undefined' ? window : null, de = document?.documentElement, bd = document?.body
    return { scroll_x: Math.round(w?.scrollX ?? w?.pageXOffset ?? 0), scroll_y: Math.round(w?.scrollY ?? w?.pageYOffset ?? 0), viewport_width: w?.innerWidth ?? de?.clientWidth ?? 0, viewport_height: w?.innerHeight ?? de?.clientHeight ?? 0, page_height: Math.max(bd?.scrollHeight || 0, de?.scrollHeight || 0) }
  }

  function matchedTarget(node: Element): NonNullable<DOMResult['matched']> {
    const h = node as HTMLElement, tp = (h.textContent || '').trim().slice(0, 80)
    const cl = typeof h.className === 'string' && h.className ? h.className.split(/\s+/).filter(Boolean).slice(0, 5) : undefined
    return { tag: node.tagName.toLowerCase(), role: node.getAttribute('role') || undefined, aria_label: node.getAttribute('aria-label') || undefined, text_preview: tp || undefined, classes: cl && cl.length > 0 ? cl : undefined, selector, element_id: getOrCreateElementID(node), bbox: extractBoundingBox(node), scope_selector_used: resolvedScopeSelector, ...(scopeRect ? { scope_rect_used: scopeRect } : {}) }
  }

  function mutatingSuccess(node: Element, extra?: Omit<Partial<DOMResult>, 'success' | 'action' | 'selector' | 'matched' | 'match_count' | 'match_strategy'>): DOMResult {
    const oi = detectOverlayWarning(node)
    return { success: true, action, selector, ...(scopeRect ? { scope_rect_used: scopeRect } : {}), ...(extra || {}), ...(oi.overlay_warning ? oi : {}), matched: matchedTarget(node), match_count: resolvedMatchCount, match_strategy: resolvedMatchStrategy, ...(resolvedRankedCandidates ? { ranked_candidates: resolvedRankedCandidates } : {}), viewport: captureViewport() }
  }

  function withMutationTracking(fn: () => DOMResult): Promise<DOMResult> {
    const t0 = performance.now(), muts: MutationRecord[] = []
    const obs = new MutationObserver((recs) => { muts.push(...recs) })
    obs.observe(document.body || document.documentElement, { childList: true, subtree: true, attributes: true, attributeOldValue: !!options.observe_mutations })
    const result = fn()
    if (!result.success) { obs.disconnect(); return Promise.resolve(result) }
    return new Promise((resolve) => {
      let done = false
      function finish() {
        if (done) return; done = true; obs.disconnect()
        const totalMs = Math.round(performance.now() - t0)
        const added = muts.reduce((s, m) => s + m.addedNodes.length, 0), removed = muts.reduce((s, m) => s + m.removedNodes.length, 0), modified = muts.filter((m) => m.type === 'attributes').length
        const parts: string[] = []; if (added > 0) parts.push(`${added} added`); if (removed > 0) parts.push(`${removed} removed`); if (modified > 0) parts.push(`${modified} modified`)
        const summary = parts.length > 0 ? parts.join(', ') : 'no DOM changes'
        const enriched: DOMResult = { ...result, dom_summary: summary }
        if (options.analyze) { enriched.timing = { total_ms: totalMs }; enriched.dom_changes = { added, removed, modified, summary }; enriched.analysis = `${result.action} completed in ${totalMs}ms. ${summary}.` }
        if (options.observe_mutations) {
          const max = 50, entries: DOMMutationEntry[] = []
          for (const m of muts) {
            if (entries.length >= max) break
            if (m.type === 'childList') {
              for (let i = 0; i < m.addedNodes.length && entries.length < max; i++) { const n = m.addedNodes[i]; if (n && n.nodeType === 1) { const e = n as Element; entries.push({ type: 'added', tag: e.tagName?.toLowerCase(), id: e.id || undefined, class: e.className?.toString()?.slice(0, 80) || undefined, text_preview: e.textContent?.slice(0, 100) || undefined }) } }
              for (let i = 0; i < m.removedNodes.length && entries.length < max; i++) { const n = m.removedNodes[i]; if (n && n.nodeType === 1) { const e = n as Element; entries.push({ type: 'removed', tag: e.tagName?.toLowerCase(), id: e.id || undefined, class: e.className?.toString()?.slice(0, 80) || undefined, text_preview: e.textContent?.slice(0, 100) || undefined }) } }
            } else if (m.type === 'attributes' && m.target.nodeType === 1) { const e = m.target as Element; entries.push({ type: 'attribute', tag: e.tagName?.toLowerCase(), id: e.id || undefined, attribute: m.attributeName || undefined, old_value: m.oldValue?.slice(0, 100) || undefined, new_value: e.getAttribute(m.attributeName || '')?.slice(0, 100) || undefined }) }
          }
          enriched.dom_mutations = entries
        }
        resolve(enriched)
      }
      setTimeout(finish, 80)
      if (typeof requestAnimationFrame === 'function') requestAnimationFrame(() => setTimeout(finish, 50))
    })
  }

  // --- Rich editor & keyboard helpers ---
  function detectRichEditor(node: Node): { type: string; target: HTMLElement } | null {
    const h = node instanceof HTMLElement ? node : (node.parentElement || null)
    if (!h) return null
    for (const { selector: s, type: t } of [{ selector: '.ql-editor', type: 'quill' }, { selector: '.ProseMirror', type: 'prosemirror' }, { selector: '[data-contents="true"]', type: 'draftjs' }, { selector: '[data-editor]', type: 'draftjs' }, { selector: '.mce-content-body', type: 'tinymce' }, { selector: '#tinymce', type: 'tinymce' }, { selector: '.ck-editor__editable', type: 'ckeditor' }]) {
      if (typeof h.matches === 'function' && h.matches(s)) return { type: t, target: h }
      if (typeof h.closest === 'function') { const a = h.closest(s); if (a instanceof HTMLElement) return { type: t, target: a } }
    }
    return null
  }

  function insertViaRichEditor(_t: string, target: HTMLElement, text: string, clear: boolean) {
    const parts = text.split('\n').map((l) => l.length > 0 ? '<p>' + l.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;') + '</p>' : '<p><br></p>')
    if (clear) target.innerHTML = parts.join(''); else target.insertAdjacentHTML('beforeend', parts.join(''))
    target.dispatchEvent(new Event('input', { bubbles: true }))
  }

  function keyCodeForChar(ch: string): { key: string; code: string; keyCode: number } {
    if (ch === '\n') return { key: 'Enter', code: 'Enter', keyCode: 13 }
    if (ch === '\t') return { key: 'Tab', code: 'Tab', keyCode: 9 }
    if (ch === ' ') return { key: ' ', code: 'Space', keyCode: 32 }
    const u = ch.toUpperCase(), isL = u >= 'A' && u <= 'Z', isD = ch >= '0' && ch <= '9'
    return { key: ch, code: isL ? 'Key' + u : isD ? 'Digit' + ch : '', keyCode: isL ? u.charCodeAt(0) : ch.charCodeAt(0) }
  }

  function dispatchKeySequence(target: EventTarget, ch: string, isCE: boolean) {
    const { key, code, keyCode } = keyCodeForChar(ch)
    const shiftKey = ch !== ch.toLowerCase() && ch === ch.toUpperCase() && ch.toLowerCase() !== ch.toUpperCase()
    const opts: KeyboardEventInit & { keyCode?: number } = { key, code, keyCode, bubbles: true, cancelable: true, shiftKey }
    target.dispatchEvent(new KeyboardEvent('keydown', opts)); target.dispatchEvent(new KeyboardEvent('keypress', opts))
    if (isCE) {
      target.dispatchEvent(new InputEvent('beforeinput', { bubbles: true, cancelable: true, inputType: 'insertText', data: ch }))
      const sel = document.getSelection()
      if (sel && sel.rangeCount > 0) { const range = sel.getRangeAt(0); range.deleteContents(); range.insertNode(ch === '\n' ? document.createElement('br') : document.createTextNode(ch)); range.collapse(false); sel.removeAllRanges(); sel.addRange(range) }
      target.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText', data: ch }))
    }
    target.dispatchEvent(new KeyboardEvent('keyup', opts))
  }

  function insertViaKeyboardSim(node: HTMLElement, text: string) { for (const ch of text) dispatchKeySequence(node, ch, true) }

  // --- Scroll helpers ---
  function findScrollableContainer(el2: Element): HTMLElement | null {
    let cur: Element | null = el2
    while (cur && cur !== document.documentElement) {
      if (cur instanceof HTMLElement && cur.scrollHeight > cur.clientHeight + 10) {
        const s = typeof getComputedStyle === 'function' ? getComputedStyle(cur) : null
        if (s && (s.overflow === 'auto' || s.overflow === 'scroll' || s.overflowY === 'auto' || s.overflowY === 'scroll')) return cur
      }
      cur = cur.parentElement
    }
    return null
  }

  function autoScrollIfNeeded(el2: Element): boolean {
    if (!(el2 instanceof HTMLElement) || typeof el2.getBoundingClientRect !== 'function') return false
    const r = el2.getBoundingClientRect()
    const vh = typeof window !== 'undefined' ? window.innerHeight ?? 0 : 0, vw = typeof window !== 'undefined' ? window.innerWidth ?? 0 : 0
    if (vh === 0 && vw === 0) return false
    if (r.bottom >= 0 && r.top <= vh && r.right >= 0 && r.left <= vw) return false
    el2.scrollIntoView({ behavior: 'instant', block: 'center' }); return true
  }

  function findInteractiveAncestor(el2: Element): Element | null {
    const tag = el2.tagName.toLowerCase(), role = el2.getAttribute('role') || ''
    if (new Set(['a','button','input','select','textarea']).has(tag) || new Set(['button','link','menuitem','tab','option','switch']).has(role)) return null
    if (typeof el2.closest === 'function') { const a = el2.closest('a, button, [role="button"], [role="link"], [role="menuitem"], [role="tab"], input, select, textarea'); if (a && a !== el2) return a }
    return null
  }

  type ActionHandler = () => DOMResult | Promise<DOMResult>

  // --- Action handlers ---
  const handlers: Record<string, ActionHandler> = {
    click: () => withMutationTracking(() => {
      if (!(el instanceof HTMLElement)) return domError('not_interactive', `Element is not an HTMLElement: ${el.tagName}`)
      const ia = findInteractiveAncestor(el), ct = (ia instanceof HTMLElement ? ia : el) as HTMLElement
      const oe = blockedByOverlayError(el); if (oe) return oe
      if (options.new_tab) {
        const ln = (() => { const t = ct.tagName.toLowerCase(); if (t === 'a') return ct as Element; return typeof ct.closest === 'function' ? ct.closest('a[href]') : null })()
        const href = ln ? (ln.getAttribute('href') || (ln as HTMLAnchorElement).href || '') : ''
        if (!href) return domError('new_tab_requires_link', 'new_tab=true requires a link target with href')
        let opened = false
        try { if (typeof window !== 'undefined' && typeof window.open === 'function') { window.open(href, '_blank', 'noopener,noreferrer'); opened = true } } catch { /* fall through */ }
        if (!opened && ln instanceof Element) { const pt = ln.getAttribute('target'); ln.setAttribute('target', '_blank'); (ln as HTMLElement).click(); if (pt == null) ln.removeAttribute('target'); else ln.setAttribute('target', pt) }
        return mutatingSuccess(ct, { value: href, reason: 'opened_new_tab' })
      }
      const didScroll = autoScrollIfNeeded(ct); ct.click()
      return mutatingSuccess(ct, didScroll ? { auto_scrolled: true } : undefined)
    }),
    type: () => withMutationTracking(() => {
      const oe = blockedByOverlayError(el); if (oe) return oe
      const text = (options.text || '').replace(/\\n/g, '\n')
      if (el instanceof HTMLElement && el.isContentEditable) {
        el.focus()
        if (options.clear) { const s = document.getSelection(); if (s) { s.selectAllChildren(el); s.deleteFromDocument() } }
        const editor = detectRichEditor(el); let strategy: string
        if (editor) { insertViaRichEditor(editor.type, editor.target, text, !!options.clear); strategy = editor.type + '_native' }
        else { insertViaKeyboardSim(el, text); strategy = 'keyboard_simulation' }
        return mutatingSuccess(el, { value: el.innerText, insertion_strategy: strategy })
      }
      if (!(el instanceof HTMLInputElement) && !(el instanceof HTMLTextAreaElement)) return domError('not_typeable', `Element is not an input, textarea, or contenteditable: ${el.tagName}`)
      el.focus(); for (const ch of text) dispatchKeySequence(el, ch, false)
      const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement : HTMLInputElement
      const ns = Object.getOwnPropertyDescriptor(proto.prototype, 'value')?.set
      if (ns) { ns.call(el, options.clear ? text : el.value + text) } else { el.value = options.clear ? text : el.value + text }
      el.dispatchEvent(new InputEvent('input', { bubbles: true, data: text, inputType: 'insertText' }))
      el.dispatchEvent(new Event('change', { bubbles: true }))
      return mutatingSuccess(el, { value: el.value, insertion_strategy: 'native_setter' })
    }),
    select: () => withMutationTracking(() => {
      const oe = blockedByOverlayError(el); if (oe) return oe
      if (!(el instanceof HTMLSelectElement)) return domError('not_select', `Element is not a <select>: ${el.tagName}`)
      const ns = Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, 'value')?.set
      if (ns) ns.call(el, options.value || ''); else el.value = options.value || ''
      el.dispatchEvent(new Event('change', { bubbles: true }))
      return mutatingSuccess(el, { value: el.value })
    }),
    check: () => withMutationTracking(() => {
      const oe = blockedByOverlayError(el); if (oe) return oe
      if (!(el instanceof HTMLInputElement) || (el.type !== 'checkbox' && el.type !== 'radio')) return domError('not_checkable', `Element is not a checkbox or radio: ${el.tagName} type=${(el as HTMLInputElement).type || 'N/A'}`)
      const desired = options.checked !== undefined ? options.checked : true
      if (el.checked !== desired) el.click()
      return mutatingSuccess(el, { value: el.checked })
    }),
    set_attribute: () => withMutationTracking(() => {
      el.setAttribute(options.name || '', options.value || '')
      return mutatingSuccess(el, { value: el.getAttribute(options.name || '') })
    }),
    focus: () => {
      const oe = blockedByOverlayError(el); if (oe) return oe
      if (!(el instanceof HTMLElement)) return domError('not_focusable', `Element is not an HTMLElement: ${el.tagName}`)
      el.focus(); return mutatingSuccess(el)
    },
    hover: () => withMutationTracking(() => {
      if (!(el instanceof HTMLElement)) return domError('not_interactive', `Element is not an HTMLElement: ${el.tagName}`)
      const r = el.getBoundingClientRect(), cx = r.left + r.width / 2, cy = r.top + r.height / 2
      const ei = { bubbles: true, cancelable: true, clientX: cx, clientY: cy }
      el.dispatchEvent(new MouseEvent('mouseenter', { ...ei, bubbles: false }))
      el.dispatchEvent(new MouseEvent('mouseover', ei)); el.dispatchEvent(new MouseEvent('mousemove', ei))
      return mutatingSuccess(el)
    }),
    paste: () => withMutationTracking(() => {
      const oe = blockedByOverlayError(el); if (oe) return oe
      if (!(el instanceof HTMLElement)) return domError('not_interactive', `Element is not an HTMLElement: ${el.tagName}`)
      el.focus()
      if (options.clear) { const s = document.getSelection(); if (s) { s.selectAllChildren(el); s.deleteFromDocument() } }
      const pasteText = (options.text || '').replace(/\\n/g, '\n'); let strategy: string
      const editor = detectRichEditor(el)
      if (editor && el.isContentEditable) { insertViaRichEditor(editor.type, editor.target, pasteText, !!options.clear); strategy = editor.type + '_native' }
      else { const dt = new DataTransfer(); dt.setData('text/plain', pasteText); el.dispatchEvent(new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true })); strategy = 'clipboard_event' }
      return mutatingSuccess(el, { value: el.innerText, insertion_strategy: strategy })
    }),
    key_press: () => withMutationTracking(() => {
      const oe = blockedByOverlayError(el); if (oe) return oe
      if (!(el instanceof HTMLElement)) return domError('not_interactive', `Element is not an HTMLElement: ${el.tagName}`)
      const key = options.text || options.key || 'Enter'
      if (key === 'Tab' || key === 'Shift+Tab') {
        const focusable = Array.from(el.ownerDocument.querySelectorAll('a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])')).filter((e) => (e as HTMLElement).offsetParent !== null) as HTMLElement[]
        const idx = focusable.indexOf(el), next = key === 'Shift+Tab' ? focusable[idx - 1] : focusable[idx + 1]
        if (next) { next.focus(); return mutatingSuccess(el, { value: key }) }
        return mutatingSuccess(el, { value: key, message: 'No next focusable element' })
      }
      const km: Record<string, { key: string; code: string; keyCode: number }> = { Enter: { key: 'Enter', code: 'Enter', keyCode: 13 }, Tab: { key: 'Tab', code: 'Tab', keyCode: 9 }, Escape: { key: 'Escape', code: 'Escape', keyCode: 27 }, Backspace: { key: 'Backspace', code: 'Backspace', keyCode: 8 }, ArrowDown: { key: 'ArrowDown', code: 'ArrowDown', keyCode: 40 }, ArrowUp: { key: 'ArrowUp', code: 'ArrowUp', keyCode: 38 }, Space: { key: ' ', code: 'Space', keyCode: 32 } }
      const mapped = km[key] || { key, code: key, keyCode: 0 }
      el.dispatchEvent(new KeyboardEvent('keydown', { key: mapped.key, code: mapped.code, keyCode: mapped.keyCode, bubbles: true }))
      el.dispatchEvent(new KeyboardEvent('keypress', { key: mapped.key, code: mapped.code, keyCode: mapped.keyCode, bubbles: true }))
      el.dispatchEvent(new KeyboardEvent('keyup', { key: mapped.key, code: mapped.code, keyCode: mapped.keyCode, bubbles: true }))
      return mutatingSuccess(el, { value: key })
    }),
    scroll_to: () => {
      function scrollToY(c: HTMLElement, top: number) { if (typeof (c as { scrollTo?: unknown }).scrollTo === 'function') c.scrollTo({ top, behavior: 'smooth' }); else (c as { scrollTop?: number }).scrollTop = top }
      function scrollByY(c: HTMLElement, dy: number) { if (typeof (c as { scrollBy?: unknown }).scrollBy === 'function') c.scrollBy({ top: dy, behavior: 'smooth' }); else (c as { scrollTop?: number }).scrollTop = (Number((c as { scrollTop?: unknown }).scrollTop) || 0) + dy }
      const dir = (options.direction || options.value || '').toLowerCase(), tag = el.tagName.toLowerCase()
      const isContainer = el instanceof HTMLElement && el.scrollHeight > el.clientHeight + 10 && (() => { const s = typeof getComputedStyle === 'function' ? getComputedStyle(el) : null; return s ? (s.overflow === 'auto' || s.overflow === 'scroll' || s.overflowY === 'auto' || s.overflowY === 'scroll') : false })()
      const dc = (() => { if (isContainer) return el as HTMLElement; const a = findScrollableContainer(el); if (a) return a; if (document.scrollingElement instanceof HTMLElement) return document.scrollingElement; return document.documentElement as HTMLElement })()
      if (dir && dc) {
        switch (dir) {
          case 'top': scrollToY(dc, 0); return mutatingSuccess(el, { reason: 'scrolled_container_top' })
          case 'bottom': scrollToY(dc, dc.scrollHeight); return mutatingSuccess(el, { reason: 'scrolled_container_bottom' })
          case 'up': scrollByY(dc, -dc.clientHeight * 0.8); return mutatingSuccess(el, { reason: 'scrolled_container_up' })
          case 'down': scrollByY(dc, dc.clientHeight * 0.8); return mutatingSuccess(el, { reason: 'scrolled_container_down' })
        }
      }
      if (tag === 'body' || tag === 'html') { const sc = findScrollableContainer(document.body); if (sc) { sc.scrollIntoView({ behavior: 'smooth', block: 'center' }); return mutatingSuccess(el, { reason: 'scrolled_nested_container' }) } }
      const pc = findScrollableContainer(el)
      if (pc && pc !== document.documentElement) { el.scrollIntoView({ behavior: 'smooth', block: 'center' }); return mutatingSuccess(el, { reason: 'scrolled_within_container' }) }
      el.scrollIntoView({ behavior: 'smooth', block: 'center' }); return mutatingSuccess(el)
    },
  }

  const handler = handlers[action]
  if (!handler) return domError('unknown_action', `Unknown action: ${action}`)
  return handler()
}
