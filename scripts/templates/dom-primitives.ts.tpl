// eslint-disable max-lines - Auto-generated from template + partials; must be a single self-contained function for chrome.scripting.executeScript.
/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
// dom-primitives.ts — Pre-compiled DOM interaction functions for chrome.scripting.executeScript.
// These bypass CSP restrictions because they use the `func` parameter (no eval/new Function).
// Each function MUST be self-contained — no closures over external variables.

import type { DOMMutationEntry, DOMPrimitiveOptions, DOMResult } from './dom-types.js'

// Re-export list_interactive primitive for backward compatibility
export { domPrimitiveListInteractive } from './dom-primitives-list-interactive.js'

/**
 * Single self-contained function for all DOM primitives.
 * Passed to chrome.scripting.executeScript({ func: domPrimitive, args: [...] }).
 * MUST NOT reference any module-level variables — Chrome serializes the function source only.
 */
export function domPrimitive(
  action: string,
  selector: string,
  options: DOMPrimitiveOptions
): DOMResult | Promise<DOMResult> | {
  success: boolean
  elements: unknown[]
  candidate_count?: number
  scope_rect_used?: { x: number; y: number; width: number; height: number }
  error?: string
  message?: string
} {
  // @include _dom-selectors.tpl

  // @include _dom-intent.tpl

  // @include _dom-ranking.tpl

  function resolveActionTarget(): {
    element?: Element
    error?: DOMResult
    match_count?: number
    match_strategy?: string
    scope_selector_used?: string
    ranked_candidates?: { element_id: string; tag: string; text_preview?: string; score: number }[]
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
    const scopeRectUsed = scopeRect || undefined

    if (intentActions.has(action)) {
      return resolveIntentTarget(requestedScope, activeScope)
    }

    // key_press without selector: dispatch on activeElement or body (#321)
    if (action === 'key_press' && !selector && !options.element_id) {
      const target = document.activeElement || document.body
      if (target) {
        return {
          element: target,
          match_count: 1,
          match_strategy: 'active_element_fallback'
        }
      }
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

    // #385: nth parameter for explicit disambiguation — works for all selector-based actions
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

    const ambiguitySensitiveActions = new Set([
      'click', 'type', 'select', 'check', 'set_attribute',
      'paste', 'key_press', 'focus', 'scroll_to', 'hover'
    ])

    if (!ambiguitySensitiveActions.has(action)) {
      // #316: For text= selectors, always check total match count to add disambiguation warning
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

    const rawMatches = resolveElements(selector, activeScope)
    const uniqueMatches: Element[] = []
    const seen = new Set<Element>()
    for (const match of rawMatches) {
      if (seen.has(match)) continue
      seen.add(match)
      uniqueMatches.push(match)
    }

    const rectScopedMatches = filterByScopeRect(uniqueMatches)

    const viableMatches = (() => {
      if (rectScopedMatches.length === 0) return rectScopedMatches
      const visible = rectScopedMatches.filter(isActionableVisible)
      return visible.length > 0 ? visible : rectScopedMatches
    })()

    if (viableMatches.length > 1) {
      const ranking = rankAmbiguousCandidates(viableMatches, action, selector)
      const topCandidates = ranking.ranked.slice(0, 3).map((entry) => ({
        element_id: getOrCreateElementID(entry.element),
        tag: entry.element.tagName.toLowerCase(),
        text_preview: ((entry.element as HTMLElement).textContent || '').trim().slice(0, 60) || undefined,
        score: entry.score
      }))

      if (ranking.winner) {
        return {
          element: ranking.winner,
          match_count: 1,
          match_strategy: 'ranked_resolution',
          ranked_candidates: topCandidates
        }
      }

      const sortedCandidates = ranking.ranked.map((entry) => entry.element)
      return {
        error: {
          success: false,
          action,
          selector,
          error: 'ambiguous_target',
          message: `Selector matches multiple viable elements: ${selector}. Add nth, scope/scope_rect, or use list_interactive element_id/index.`,
          match_count: viableMatches.length,
          match_strategy: 'ambiguous_ranked',
          ...(scopeRect ? { scope_rect_used: scopeRect } : {}),
          candidates: summarizeCandidates(sortedCandidates),
          ranked_candidates: topCandidates,
          suggested_element_id: getOrCreateElementID(ranking.ranked[0]!.element)
        }
      }
    }

    const found = viableMatches[0] || null
    if (!found) return { error: domError('element_not_found', `No element matches selector: ${selector}`) }
    const strategy = (() => {
      if (selector.includes(':nth-match(')) return 'nth_match_selector'
      if (scopeRectUsed) return 'rect_selector'
      if (requestedScope) return 'scoped_selector'
      return 'selector'
    })()
    return {
      element: found,
      match_count: 1,
      match_strategy: strategy,
      scope_selector_used: scopeSelectorUsed
    }
  }

  const resolved = resolveActionTarget()
  if (resolved.error) return resolved.error
  const el = resolved.element!
  const resolvedMatchCount = resolved.match_count || 1
  const resolvedMatchStrategy = resolved.match_strategy || 'selector'
  const resolvedScopeSelector = resolved.scope_selector_used
  const resolvedRankedCandidates = resolved.ranked_candidates
  const resolvedAmbiguousMatches = resolved.ambiguous_matches

  /** Capture current viewport/scroll position for action responses. */
  function captureViewport(): { scroll_x: number; scroll_y: number; viewport_width: number; viewport_height: number; page_height: number } {
    return {
      scroll_x: Math.round(window.scrollX || window.pageXOffset || 0),
      scroll_y: Math.round(window.scrollY || window.pageYOffset || 0),
      viewport_width: window.innerWidth || document.documentElement.clientWidth || 0,
      viewport_height: window.innerHeight || document.documentElement.clientHeight || 0,
      page_height: Math.max(
        document.body?.scrollHeight || 0,
        document.documentElement?.scrollHeight || 0
      )
    }
  }

  // #368: Check if an overlay might be obscuring the target element
  function detectOverlayWarning(targetEl: Element): { overlay_warning?: string; overlay_selector?: string } {
    const overlay = findTopmostOverlay()
    if (!overlay) return {}
    // If the target is inside the overlay, no warning needed — the action is targeting the overlay correctly
    if (overlay.contains(targetEl)) return {}
    const overlayInfo = describeOverlay(overlay)
    return {
      overlay_warning: `An overlay (${overlayInfo.overlay_type}) is covering the page. The action targeted the intended element, but input may be intercepted. Use dismiss_top_overlay to close it first.`,
      overlay_selector: overlayInfo.overlay_selector
    }
  }

  function mutatingSuccess(
    node: Element,
    extra?: Omit<Partial<DOMResult>, 'success' | 'action' | 'selector' | 'matched' | 'match_count' | 'match_strategy'>
  ): DOMResult {
    const overlayInfo = detectOverlayWarning(node)
    return {
      success: true,
      action,
      selector,
      ...(scopeRect ? { scope_rect_used: scopeRect } : {}),
      ...(extra || {}),
      ...(overlayInfo.overlay_warning ? overlayInfo : {}),
      matched: matchedTarget(node),
      match_count: resolvedMatchCount,
      match_strategy: resolvedMatchStrategy,
      ...(resolvedRankedCandidates ? { ranked_candidates: resolvedRankedCandidates } : {}),
      viewport: captureViewport()
    }
  }

  // — Mutation tracking: MutationObserver wrapper for DOM change capture —
  function withMutationTracking(fn: () => DOMResult): Promise<DOMResult> {
    const t0 = performance.now()
    const mutations: MutationRecord[] = []
    const observer = new MutationObserver((records) => {
      mutations.push(...records)
    })
    observer.observe(document.body || document.documentElement, {
      childList: true,
      subtree: true,
      attributes: true,
      attributeOldValue: !!options.observe_mutations
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

        if (options.observe_mutations) {
          const maxEntries = 50
          const entries: DOMMutationEntry[] = []
          for (const m of mutations) {
            if (entries.length >= maxEntries) break
            if (m.type === 'childList') {
              for (let i = 0; i < m.addedNodes.length && entries.length < maxEntries; i++) {
                const n = m.addedNodes[i] as Node | undefined
                if (n && n.nodeType === 1) {
                  const el = n as Element
                  entries.push({ type: 'added', tag: el.tagName?.toLowerCase(), id: el.id || undefined, class: el.className?.toString()?.slice(0, 80) || undefined, text_preview: el.textContent?.slice(0, 100) || undefined })
                }
              }
              for (let i = 0; i < m.removedNodes.length && entries.length < maxEntries; i++) {
                const n = m.removedNodes[i] as Node | undefined
                if (n && n.nodeType === 1) {
                  const el = n as Element
                  entries.push({ type: 'removed', tag: el.tagName?.toLowerCase(), id: el.id || undefined, class: el.className?.toString()?.slice(0, 80) || undefined, text_preview: el.textContent?.slice(0, 100) || undefined })
                }
              }
            } else if (m.type === 'attributes' && m.target.nodeType === 1) {
              const el = m.target as Element
              entries.push({ type: 'attribute', tag: el.tagName?.toLowerCase(), id: el.id || undefined, attribute: m.attributeName || undefined, old_value: m.oldValue?.slice(0, 100) || undefined, new_value: el.getAttribute(m.attributeName || '')?.slice(0, 100) || undefined })
            }
          }
          enriched.dom_mutations = entries
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

  // — Rich editor detection: walk up from target to find known editor containers —
  function detectRichEditor(node: Node): { type: string; target: HTMLElement } | null {
    const el = node instanceof HTMLElement ? node : (node.parentElement || null)
    if (!el) return null
    const checks: Array<{ selector: string; type: string }> = [
      { selector: '.ql-editor', type: 'quill' },
      { selector: '.ProseMirror', type: 'prosemirror' },
      { selector: '[data-contents="true"]', type: 'draftjs' },
      { selector: '[data-editor]', type: 'draftjs' },
      { selector: '.mce-content-body', type: 'tinymce' },
      { selector: '#tinymce', type: 'tinymce' },
      { selector: '.ck-editor__editable', type: 'ckeditor' },
    ]
    for (const check of checks) {
      if (typeof el.matches === 'function' && el.matches(check.selector)) {
        return { type: check.type, target: el }
      }
      if (typeof el.closest === 'function') {
        const ancestor = el.closest(check.selector)
        if (ancestor instanceof HTMLElement) {
          return { type: check.type, target: ancestor }
        }
      }
    }
    return null
  }

  // — Native DOM insertion for detected rich editors (Quill, ProseMirror, etc.) —
  function insertViaRichEditor(
    _editorType: string,
    target: HTMLElement,
    text: string,
    clear: boolean
  ): { success: boolean } {
    const lines = text.split('\n')
    const htmlParts: string[] = []
    for (const line of lines) {
      if (line.length > 0) {
        htmlParts.push('<p>' + line.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;') + '</p>')
      } else {
        htmlParts.push('<p><br></p>')
      }
    }
    const html = htmlParts.join('')
    if (clear) {
      target.innerHTML = html
    } else {
      target.insertAdjacentHTML('beforeend', html)
    }
    target.dispatchEvent(new Event('input', { bubbles: true }))
    return { success: true }
  }

  // — Keyboard event helpers —
  function keyCodeForChar(char: string): { key: string; code: string; keyCode: number } {
    if (char === '\n') return { key: 'Enter', code: 'Enter', keyCode: 13 }
    if (char === '\t') return { key: 'Tab', code: 'Tab', keyCode: 9 }
    if (char === ' ') return { key: ' ', code: 'Space', keyCode: 32 }

    const upper = char.toUpperCase()
    const isLetter = upper >= 'A' && upper <= 'Z'
    const isDigit = char >= '0' && char <= '9'

    let code: string
    let keyCode: number

    if (isLetter) {
      code = 'Key' + upper
      keyCode = upper.charCodeAt(0)
    } else if (isDigit) {
      code = 'Digit' + char
      keyCode = char.charCodeAt(0)
    } else {
      // Punctuation / symbols: use Unidentified code, charCode as keyCode
      code = ''
      keyCode = char.charCodeAt(0)
    }

    return { key: char, code, keyCode }
  }

  function dispatchKeySequence(target: EventTarget, char: string, isContentEditable: boolean): void {
    const { key, code, keyCode } = keyCodeForChar(char)
    const shiftKey = char !== char.toLowerCase() && char === char.toUpperCase() && char.toLowerCase() !== char.toUpperCase()

    const kbOpts: KeyboardEventInit & { keyCode?: number } = { key, code, keyCode, bubbles: true, cancelable: true, shiftKey }

    target.dispatchEvent(new KeyboardEvent('keydown', kbOpts))
    target.dispatchEvent(new KeyboardEvent('keypress', kbOpts))

    if (isContentEditable) {
      // Browsers fire beforeinput/input as InputEvents on contenteditable
      target.dispatchEvent(new InputEvent('beforeinput', {
        bubbles: true, cancelable: true, inputType: 'insertText', data: char,
      }))
      // Insert text at selection (replaces execCommand)
      const sel = document.getSelection()
      if (sel && sel.rangeCount > 0) {
        const range = sel.getRangeAt(0)
        range.deleteContents()
        if (char === '\n') {
          range.insertNode(document.createElement('br'))
        } else {
          range.insertNode(document.createTextNode(char))
        }
        range.collapse(false)
        sel.removeAllRanges()
        sel.addRange(range)
      }
      target.dispatchEvent(new InputEvent('input', {
        bubbles: true, inputType: 'insertText', data: char,
      }))
    }

    target.dispatchEvent(new KeyboardEvent('keyup', kbOpts))
  }

  // — Keyboard simulation for generic contenteditable (no framework detected) —
  function insertViaKeyboardSim(node: HTMLElement, text: string): { success: boolean } {
    for (const char of text) {
      dispatchKeySequence(node, char, true)
    }
    return { success: true }
  }

  // --- #336: Check if element is outside the viewport and auto-scroll into view ---
  function isElementOutsideViewport(el: Element): boolean {
    if (!(el instanceof HTMLElement) || typeof el.getBoundingClientRect !== 'function') return false
    const rect = el.getBoundingClientRect()
    const viewHeight = typeof window !== 'undefined' && typeof window.innerHeight === 'number'
      ? window.innerHeight
      : (typeof document !== 'undefined' && document.documentElement ? document.documentElement.clientHeight : 0)
    const viewWidth = typeof window !== 'undefined' && typeof window.innerWidth === 'number'
      ? window.innerWidth
      : (typeof document !== 'undefined' && document.documentElement ? document.documentElement.clientWidth : 0)
    if (viewHeight === 0 && viewWidth === 0) return false
    return rect.bottom < 0 || rect.top > viewHeight || rect.right < 0 || rect.left > viewWidth
  }

  function autoScrollIfNeeded(el: Element): boolean {
    if (isElementOutsideViewport(el)) {
      el.scrollIntoView({ behavior: 'instant', block: 'center' })
      return true
    }
    return false
  }

  // --- #332: Find nearest interactive ancestor for non-interactive wrapper elements ---
  function findInteractiveAncestor(el: Element): Element | null {
    const tag = el.tagName.toLowerCase()
    const role = el.getAttribute('role') || ''
    const interactiveTags = new Set(['a', 'button', 'input', 'select', 'textarea'])
    const interactiveRoles = new Set(['button', 'link', 'menuitem', 'tab', 'option', 'switch'])
    // Already interactive — no need to bubble up
    if (interactiveTags.has(tag) || interactiveRoles.has(role)) return null
    if (typeof el.closest === 'function') {
      const ancestor = el.closest('a, button, [role="button"], [role="link"], [role="menuitem"], [role="tab"], input, select, textarea')
      if (ancestor && ancestor !== el) return ancestor
    }
    return null
  }

  type ActionHandler = () => DOMResult | Promise<DOMResult>

  // Detect if an element is obscured by a modal/dialog overlay.
  // Returns the overlay element if blocking, null otherwise.
  function detectBlockingOverlay(el: Element): Element | null {
    const dialogs = collectDialogs()
    if (dialogs.length === 0) return null
    const topDialog = pickTopDialog(dialogs)
    if (!topDialog) return null
    // If the element is inside the top dialog, it's not blocked
    if (typeof topDialog.contains === 'function' && topDialog.contains(el)) return null
    // Element is outside the top dialog — it's blocked by the overlay
    return topDialog
  }

  function buildActionHandlers(node: Element): Record<string, ActionHandler> {
    return {
      click: () =>
        withMutationTracking(() => {
          if (!(node instanceof HTMLElement)) return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`)

          // #332: Bubble up to nearest interactive ancestor if the matched element is a wrapper
          const interactiveAncestor = findInteractiveAncestor(node)
          const clickTarget = (interactiveAncestor instanceof HTMLElement ? interactiveAncestor : node) as HTMLElement

          // Check if element is behind a modal overlay before clicking
          const blockingOverlay = detectBlockingOverlay(node)
          if (blockingOverlay) {
            const overlayTag = blockingOverlay.tagName.toLowerCase()
            const overlayRole = blockingOverlay.getAttribute('role') || ''
            const overlayLabel = blockingOverlay.getAttribute('aria-label') || ''
            const overlayDesc = overlayLabel ? `${overlayTag}[aria-label="${overlayLabel}"]` : overlayRole ? `${overlayTag}[role="${overlayRole}"]` : overlayTag
            return domError('blocked_by_overlay', `Element is behind a modal overlay (${overlayDesc}). Use interact({what:"dismiss_top_overlay"}) to close it first.`)
          }
          if (options.new_tab) {
            const linkNode = (() => {
              const tag = clickTarget.tagName.toLowerCase()
              if (tag === 'a') return clickTarget as Element
              if (typeof clickTarget.closest === 'function') {
                return clickTarget.closest('a[href]')
              }
              return null
            })()

            const href = linkNode
              ? (linkNode.getAttribute('href') || (linkNode as HTMLAnchorElement).href || '')
              : ''
            if (!href) {
              return domError('new_tab_requires_link', 'new_tab=true requires a link target with href')
            }

            let opened = false
            try {
              if (typeof window !== 'undefined' && typeof window.open === 'function') {
                window.open(href, '_blank', 'noopener,noreferrer')
                opened = true
              }
            } catch {
              // Fall through to target=_blank click fallback.
            }

            if (!opened && linkNode instanceof Element) {
              const previousTarget = linkNode.getAttribute('target')
              linkNode.setAttribute('target', '_blank')
              ;(linkNode as HTMLElement).click()
              if (previousTarget == null) {
                linkNode.removeAttribute('target')
              } else {
                linkNode.setAttribute('target', previousTarget)
              }
            }

            return mutatingSuccess(clickTarget, { value: href, reason: 'opened_new_tab' })
          }


          // #336: Auto-scroll off-screen elements into view before clicking
          const didScroll = autoScrollIfNeeded(clickTarget)
          clickTarget.click()
          return mutatingSuccess(clickTarget, didScroll ? { auto_scrolled: true } : undefined)
        }),

      type: () =>
        withMutationTracking(() => {
          // Normalize literal \n sequences to actual newlines (MCP parameter encoding)
          const text = (options.text || '').replace(/\\n/g, '\n')

          // Contenteditable elements (Gmail compose body, rich text editors)
          if (node instanceof HTMLElement && node.isContentEditable) {
            node.focus()
            if (options.clear) {
              const selection = document.getSelection()
              if (selection) {
                selection.selectAllChildren(node)
                selection.deleteFromDocument()
              }
            }

            // Detect rich editor framework
            const editor = detectRichEditor(node)
            let strategy: string

            if (editor) {
              // Native DOM insertion — bypasses CSP, works with Quill/ProseMirror/etc
              insertViaRichEditor(editor.type, editor.target, text, !!options.clear)
              strategy = editor.type + '_native'
            } else {
              // Per-character keyboard event simulation for all generic contenteditable
              insertViaKeyboardSim(node, text)
              strategy = 'keyboard_simulation'
            }

            return mutatingSuccess(node, { value: node.innerText, insertion_strategy: strategy })
          }

          if (!(node instanceof HTMLInputElement) && !(node instanceof HTMLTextAreaElement)) {
            return domError('not_typeable', `Element is not an input, textarea, or contenteditable: ${node.tagName}`)
          }

          // Dispatch per-character keyboard events so React/Vue onChange handlers fire
          node.focus()
          for (const char of text) {
            dispatchKeySequence(node, char, false)
          }

          // Set the value via native setter (needed to bypass React's synthetic event system)
          const proto = node instanceof HTMLTextAreaElement ? HTMLTextAreaElement : HTMLInputElement
          const nativeSetter = Object.getOwnPropertyDescriptor(proto.prototype, 'value')?.set
          if (nativeSetter) {
            const newValue = options.clear ? text : node.value + text
            nativeSetter.call(node, newValue)
          } else {
            node.value = options.clear ? text : node.value + text
          }
          node.dispatchEvent(new InputEvent('input', { bubbles: true, data: text, inputType: 'insertText' }))
          node.dispatchEvent(new Event('change', { bubbles: true }))
          return mutatingSuccess(node, { value: node.value, insertion_strategy: 'native_setter' })
        }),

      select: () =>
        withMutationTracking(() => {
          if (!(node instanceof HTMLSelectElement)) return domError('not_select', `Element is not a <select>: ${node.tagName}`) // nosemgrep: html-in-template-string
          const nativeSelectSetter = Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, 'value')?.set
          if (nativeSelectSetter) {
            nativeSelectSetter.call(node, options.value || '')
          } else {
            node.value = options.value || ''
          }
          node.dispatchEvent(new Event('change', { bubbles: true }))
          return mutatingSuccess(node, { value: node.value })
        }),

      check: () =>
        withMutationTracking(() => {
          if (!(node instanceof HTMLInputElement) || (node.type !== 'checkbox' && node.type !== 'radio')) {
            return domError('not_checkable', `Element is not a checkbox or radio: ${node.tagName} type=${(node as HTMLInputElement).type || 'N/A'}`)
          }
          const desired = options.checked !== undefined ? options.checked : true
          if (node.checked !== desired) {
            node.click()
          }
          return mutatingSuccess(node, { value: node.checked })
        }),

      get_text: () => {
        if (options.structured && node instanceof HTMLElement) {
          // Structured extraction: preserve hierarchy for accordions, lists, etc.
          const sections: Array<{header?: string; content: string; expanded?: boolean; tag: string}> = []
          const children = node.children
          for (let i = 0; i < children.length && sections.length < 50; i++) {
            const child = children[i] as HTMLElement
            if (!child.tagName) continue
            const tag = child.tagName.toLowerCase()
            // Detect accordion/collapsible patterns
            const heading = child.querySelector('h1, h2, h3, h4, h5, h6, [role="heading"], summary, button[aria-expanded]')
            if (heading) {
              const headerText = (heading as HTMLElement).innerText?.trim() || ''
              const ariaExpanded = heading.getAttribute('aria-expanded')
              const expanded = ariaExpanded !== null ? ariaExpanded === 'true' : undefined
              // Get content from sibling/next panel or remaining text
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
              // Non-accordion child: just capture its text
              const t = child.innerText?.trim()
              if (t && t.length > 0) {
                sections.push({ content: t, tag })
              }
            }
          }
          return { success: true, action, selector, sections, section_count: sections.length }
        }
        const text = node instanceof HTMLElement ? node.innerText : node.textContent
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
        if (!('value' in node)) return domError('no_value_property', `Element has no value property: ${node.tagName}`)
        const value = (node as HTMLInputElement).value
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
        const value = node.getAttribute(attrName)
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

      set_attribute: () =>
        withMutationTracking(() => {
          node.setAttribute(options.name || '', options.value || '')
          return mutatingSuccess(node, { value: node.getAttribute(options.name || '') })
        }),

      focus: () => {
        if (!(node instanceof HTMLElement)) return domError('not_focusable', `Element is not an HTMLElement: ${node.tagName}`)
        node.focus()
        return mutatingSuccess(node)
      },

      scroll_to: () => {
        // #387: Container-aware scroll_to — find scrollable ancestor and support directional scrolling
        function findScrollableContainer(el: Element): HTMLElement | null {
          let current: Element | null = el
          while (current && current !== document.documentElement) {
            if (current instanceof HTMLElement && current.scrollHeight > current.clientHeight + 10) {
              const style = typeof getComputedStyle === 'function' ? getComputedStyle(current) : null
              if (style) {
                const ov = style.overflow || ''
                const ovY = style.overflowY || ''
                if (ov === 'auto' || ov === 'scroll' || ovY === 'auto' || ovY === 'scroll') {
                  return current
                }
              }
            }
            current = current.parentElement
          }
          return null
        }

        const direction = (options.value || '').toLowerCase()
        const tag = node.tagName.toLowerCase()

        // Check if the target itself is a scrollable container
        const isContainer = node instanceof HTMLElement &&
          node.scrollHeight > node.clientHeight + 10 && (() => {
            const s = typeof getComputedStyle === 'function' ? getComputedStyle(node) : null
            if (!s) return false
            const ov = s.overflow || ''
            const ovY = s.overflowY || ''
            return ov === 'auto' || ov === 'scroll' || ovY === 'auto' || ovY === 'scroll'
          })()

        // Directional scrolling within a container
        if (direction && (isContainer || tag === 'body' || tag === 'html')) {
          const container = isContainer ? (node as HTMLElement) : (findScrollableContainer(node) || document.documentElement)
          switch (direction) {
            case 'top':
              container.scrollTo({ top: 0, behavior: 'smooth' })
              return mutatingSuccess(node, { reason: 'scrolled_container_top' })
            case 'bottom':
              container.scrollTo({ top: container.scrollHeight, behavior: 'smooth' })
              return mutatingSuccess(node, { reason: 'scrolled_container_bottom' })
            case 'up':
              container.scrollBy({ top: -container.clientHeight * 0.8, behavior: 'smooth' })
              return mutatingSuccess(node, { reason: 'scrolled_container_up' })
            case 'down':
              container.scrollBy({ top: container.clientHeight * 0.8, behavior: 'smooth' })
              return mutatingSuccess(node, { reason: 'scrolled_container_down' })
          }
        }

        // #333: For body/html targets, find the actual scrollable container in SPA layouts
        if (tag === 'body' || tag === 'html') {
          const scrollable = findScrollableContainer(document.body)
          if (scrollable) {
            scrollable.scrollIntoView({ behavior: 'smooth', block: 'center' })
            return mutatingSuccess(node, { reason: 'scrolled_nested_container' })
          }
        }

        // If element is inside a scrollable container, scroll it into view within that container
        const parentContainer = findScrollableContainer(node)
        if (parentContainer && parentContainer !== document.documentElement) {
          node.scrollIntoView({ behavior: 'smooth', block: 'center' })
          return mutatingSuccess(node, { reason: 'scrolled_within_container' })
        }

        node.scrollIntoView({ behavior: 'smooth', block: 'center' })
        return mutatingSuccess(node)
      },

      wait_for: () => ({ success: true, action, selector, value: node.tagName.toLowerCase() }),

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
        const el = resolveElement(selector)
        if (!el) {
          return { success: true, action, selector, absent: true } as DOMResult
        }
        return { success: false, action, selector, error: 'element_still_present' } as DOMResult
      },

      paste: () =>
        withMutationTracking(() => {
          if (!(node instanceof HTMLElement)) return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`)
          node.focus()
          if (options.clear) {
            const selection = document.getSelection()
            if (selection) {
              selection.selectAllChildren(node)
              selection.deleteFromDocument()
            }
          }
          // Normalize literal \n sequences to actual newlines (MCP parameter encoding)
          const pasteText = (options.text || '').replace(/\\n/g, '\n')
          let strategy: string

          // Try rich editor native insertion first
          const editor = detectRichEditor(node)
          if (editor && node.isContentEditable) {
            insertViaRichEditor(editor.type, editor.target, pasteText, !!options.clear)
            strategy = editor.type + '_native'
          } else {
            // Fallback: synthetic ClipboardEvent (existing behavior)
            const dt = new DataTransfer()
            dt.setData('text/plain', pasteText)
            const event = new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true })
            node.dispatchEvent(event)
            strategy = 'clipboard_event'
          }

          return mutatingSuccess(node, { value: node.innerText, insertion_strategy: strategy })
        }),

      key_press: () =>
        withMutationTracking(() => {
          if (!(node instanceof HTMLElement)) return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`)
          const key = options.text || options.key || 'Enter'

          // Tab/Shift+Tab: manually move focus (dispatchEvent can't trigger native tab traversal)
          if (key === 'Tab' || key === 'Shift+Tab') {
            const focusable = Array.from(
              node.ownerDocument.querySelectorAll(
                'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
              )
            ).filter((e) => (e as HTMLElement).offsetParent !== null) as HTMLElement[]
            const idx = focusable.indexOf(node)
            const next = key === 'Shift+Tab' ? focusable[idx - 1] : focusable[idx + 1]
            if (next) {
              next.focus()
              return mutatingSuccess(node, { value: key })
            }
            return mutatingSuccess(node, { value: key, message: 'No next focusable element' })
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
          node.dispatchEvent(
            new KeyboardEvent('keydown', { key: mapped.key, code: mapped.code, keyCode: mapped.keyCode, bubbles: true })
          )
          node.dispatchEvent(
            new KeyboardEvent('keypress', { key: mapped.key, code: mapped.code, keyCode: mapped.keyCode, bubbles: true })
          )
          node.dispatchEvent(
            new KeyboardEvent('keyup', { key: mapped.key, code: mapped.code, keyCode: mapped.keyCode, bubbles: true })
          )
          return mutatingSuccess(node, { value: key })
        }),

      open_composer: () =>
        withMutationTracking(() => {
          if (!(node instanceof HTMLElement)) return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`)
          const tag = node.tagName.toLowerCase()
          const isInputLike =
            node.isContentEditable ||
            node.getAttribute('role') === 'textbox' ||
            tag === 'textarea' ||
            tag === 'input'
          if (isInputLike) {
            node.focus()
            return mutatingSuccess(node, { reason: 'composer_ready' })
          }
          node.click()
          return mutatingSuccess(node)
        }),

      submit_active_composer: () =>
        withMutationTracking(() => {
          if (!(node instanceof HTMLElement)) return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`)
          node.click()
          return mutatingSuccess(node)
        }),

      confirm_top_dialog: () =>
        withMutationTracking(() => {
          if (!(node instanceof HTMLElement)) return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`)
          node.click()
          return mutatingSuccess(node)
        }),

      hover: () =>
        withMutationTracking(() => {
          if (!(node instanceof HTMLElement)) return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`)
          const rect = node.getBoundingClientRect()
          const centerX = rect.left + rect.width / 2
          const centerY = rect.top + rect.height / 2
          const eventInit = { bubbles: true, cancelable: true, clientX: centerX, clientY: centerY }
          node.dispatchEvent(new MouseEvent('mouseenter', { ...eventInit, bubbles: false }))
          node.dispatchEvent(new MouseEvent('mouseover', eventInit))
          node.dispatchEvent(new MouseEvent('mousemove', eventInit))
          return mutatingSuccess(node)
        }),

      dismiss_top_overlay: () =>
        withMutationTracking(() => {
          if (!(node instanceof HTMLElement)) return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`)

          // Resolve overlay info for response enrichment
          const overlayEl = (() => {
            const dialogs = collectDialogs()
            const top = pickTopDialog(dialogs)
            if (top) return top
            // Fallback: the resolved node may itself be the overlay
            return node
          })()
          const overlayInfo = describeOverlay(overlayEl)

          // Strategy: escape_fallback — dispatch Escape key instead of clicking
          if (resolvedMatchStrategy === 'dismiss_escape_fallback') {
            const escKb: KeyboardEventInit & { keyCode?: number } = {
              key: 'Escape', code: 'Escape', keyCode: 27,
              bubbles: true, cancelable: true
            }
            document.dispatchEvent(new KeyboardEvent('keydown', escKb))
            document.dispatchEvent(new KeyboardEvent('keyup', escKb))
            // Also try the overlay element directly
            node.dispatchEvent(new KeyboardEvent('keydown', escKb))
            node.dispatchEvent(new KeyboardEvent('keyup', escKb))
            return mutatingSuccess(node, {
              strategy: 'escape_key',
              ...overlayInfo
            })
          }

          // Strategy: click the resolved dismiss button
          const strategy = (() => {
            if (resolvedMatchStrategy === 'dismiss_close_button_selector') return 'close_button'
            if (resolvedMatchStrategy === 'dismiss_text_button') return 'text_button'
            if (resolvedMatchStrategy === 'dismiss_attr_match') return 'attribute_match'
            if (resolvedMatchStrategy === 'consent_framework_selector') return 'consent_framework'
            if (resolvedMatchStrategy === 'auto_dismiss_close_button') return 'close_button'
            if (resolvedMatchStrategy === 'auto_dismiss_text_button') return 'text_button'
            return 'close_button'
          })()

          node.click()
          return mutatingSuccess(node, {
            strategy,
            selector_used: selector || resolvedMatchStrategy,
            ...overlayInfo
          })
        }),

      auto_dismiss_overlays: () =>
        withMutationTracking(() => {
          if (!(node instanceof HTMLElement)) return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`)

          // Resolve overlay info for response enrichment
          const overlayEl = (() => {
            const dialogs = collectDialogs()
            const top = pickTopDialog(dialogs)
            if (top) return top
            return node
          })()
          const overlayInfo = describeOverlay(overlayEl)

          // Strategy: escape_fallback — dispatch Escape key instead of clicking
          if (resolvedMatchStrategy === 'dismiss_escape_fallback') {
            const escKb: KeyboardEventInit & { keyCode?: number } = {
              key: 'Escape', code: 'Escape', keyCode: 27,
              bubbles: true, cancelable: true
            }
            document.dispatchEvent(new KeyboardEvent('keydown', escKb))
            document.dispatchEvent(new KeyboardEvent('keyup', escKb))
            node.dispatchEvent(new KeyboardEvent('keydown', escKb))
            node.dispatchEvent(new KeyboardEvent('keyup', escKb))
            return mutatingSuccess(node, {
              dismissed_count: 1,
              strategy: 'escape_key',
              ...overlayInfo
            })
          }

          // Click the resolved dismiss/accept button
          const strategy = (() => {
            if (resolvedMatchStrategy === 'consent_framework_selector') return 'consent_framework'
            if (resolvedMatchStrategy === 'auto_dismiss_close_button') return 'close_button'
            if (resolvedMatchStrategy === 'auto_dismiss_text_button') return 'text_button'
            return resolvedMatchStrategy || 'close_button'
          })()

          node.click()
          return mutatingSuccess(node, {
            dismissed_count: 1,
            strategy,
            selector_used: selector || resolvedMatchStrategy,
            ...overlayInfo
          })
        }),

      wait_for_stable: (): Promise<DOMResult> => {
        // Smart DOM stability wait (#344)
        const stabilityMs = typeof options.stability_ms === 'number' && options.stability_ms > 0
          ? options.stability_ms : 500
        const maxTimeout = typeof options.timeout_ms === 'number' && options.timeout_ms > 0
          ? options.timeout_ms : 5000

        return new Promise<DOMResult>((resolve) => {
          let mutationCount = 0
          let lastMutationTime = performance.now()
          const startTime = performance.now()

          const observer = new MutationObserver(() => {
            mutationCount++
            lastMutationTime = performance.now()
          })

          observer.observe(document.body || document.documentElement, {
            childList: true,
            subtree: true,
            attributes: true,
            characterData: true
          })

          function checkStability() {
            const elapsed = performance.now() - startTime
            const sinceLastMutation = performance.now() - lastMutationTime

            if (sinceLastMutation >= stabilityMs) {
              // DOM is stable
              observer.disconnect()
              resolve({
                success: true,
                action: 'wait_for_stable',
                selector: '',
                stable: true,
                waited_ms: Math.round(elapsed),
                mutations_observed: mutationCount,
                stability_ms: stabilityMs
              } as DOMResult)
              return
            }

            if (elapsed >= maxTimeout) {
              // Timed out
              observer.disconnect()
              resolve({
                success: true,
                action: 'wait_for_stable',
                selector: '',
                stable: false,
                timed_out: true,
                waited_ms: Math.round(elapsed),
                mutations_observed: mutationCount,
                stability_ms: stabilityMs
              } as DOMResult)
              return
            }

            // Check again after a short interval
            setTimeout(checkStability, Math.min(100, stabilityMs / 2))
          }

          // Start checking after initial delay
          setTimeout(checkStability, Math.min(100, stabilityMs / 2))
        })
      },

      action_diff: (): Promise<DOMResult> => {
        // #343: Structured mutation summary — instruments a MutationObserver,
        // waits for DOM to settle, then classifies mutations into categories.
        const timeoutMs = typeof options.timeout_ms === 'number' && options.timeout_ms > 0
          ? options.timeout_ms : 3000
        const settleMs = 500 // wait for DOM to stop mutating

        return new Promise<DOMResult>((resolve) => {
          // Snapshot "before" state
          const beforeURL = location.href
          const beforeTitle = document.title

          // Track text content of elements we can observe
          const textSnapshots = new Map<Element, string>()
          const snapshotSelectors = ['.status', '[role="status"]', '[data-status]', 'h1', 'h2', '.title', '.heading']
          for (const snapSel of snapshotSelectors) {
            try {
              const matches = document.querySelectorAll(snapSel)
              for (let i = 0; i < matches.length && i < 20; i++) {
                const el = matches[i]!
                textSnapshots.set(el, (el.textContent || '').trim().slice(0, 200))
              }
            } catch { /* ignore invalid selectors */ }
          }

          // Track overlays that exist before the action
          const beforeOverlays = new Set<Element>()
          const overlaySelectors = [
            '[role="dialog"]', '[role="alertdialog"]', '[aria-modal="true"]', 'dialog[open]',
            '.modal.show', '.modal.in', '.modal.is-active'
          ]
          for (const oSel of overlaySelectors) {
            try {
              const matches = document.querySelectorAll(oSel)
              for (let i = 0; i < matches.length; i++) {
                beforeOverlays.add(matches[i]!)
              }
            } catch { /* ignore */ }
          }

          let elementsAdded = 0
          let elementsRemoved = 0
          let networkRequests = 0
          let lastMutationTime = performance.now()
          const addedNodes: Element[] = []
          const startTime = performance.now()

          // Count network requests triggered by the action using PerformanceObserver.
          // This avoids monkey-patching fetch/XHR and the associated type complexity.
          let perfObserver: PerformanceObserver | null = null
          if (typeof PerformanceObserver !== 'undefined') {
            try {
              perfObserver = new PerformanceObserver((list) => {
                networkRequests += list.getEntries().length
              })
              perfObserver.observe({ entryTypes: ['resource'] })
            } catch { /* PerformanceObserver not available */ }
          }

          const observer = new MutationObserver((records) => {
            lastMutationTime = performance.now()
            for (const record of records) {
              if (record.type === 'childList') {
                for (let i = 0; i < record.addedNodes.length; i++) {
                  const n = record.addedNodes[i]
                  if (n && n.nodeType === 1) {
                    elementsAdded++
                    if (addedNodes.length < 500) addedNodes.push(n as Element)
                  }
                }
                for (let i = 0; i < record.removedNodes.length; i++) {
                  const n = record.removedNodes[i]
                  if (n && n.nodeType === 1) {
                    elementsRemoved++
                  }
                }
              }
            }
          })

          observer.observe(document.body || document.documentElement, {
            childList: true,
            subtree: true,
            attributes: true,
            characterData: true
          })

          function classifyAndResolve() {
            observer.disconnect()
            // Disconnect PerformanceObserver
            if (perfObserver) {
              try { perfObserver.disconnect() } catch { /* ignore */ }
            }

            // Classify mutations
            const urlChanged = location.href !== beforeURL
            const titleChanged = document.title !== beforeTitle

            // Detect overlays opened/closed
            interface OverlayEntry { selector: string; text: string }
            const overlaysOpened: OverlayEntry[] = []
            const overlaysClosed: OverlayEntry[] = []

            const afterOverlays = new Set<Element>()
            for (const oSel of overlaySelectors) {
              try {
                const matches = document.querySelectorAll(oSel)
                for (let i = 0; i < matches.length; i++) {
                  afterOverlays.add(matches[i]!)
                }
              } catch { /* ignore */ }
            }
            // Also check added nodes for overlay-like elements
            for (const added of addedNodes) {
              if (isOverlayElement(added)) afterOverlays.add(added)
              // Check children
              try {
                for (const oSel of overlaySelectors) {
                  const children = added.querySelectorAll(oSel)
                  for (let i = 0; i < children.length; i++) {
                    afterOverlays.add(children[i]!)
                  }
                }
              } catch { /* ignore */ }
            }

            for (const el of afterOverlays) {
              if (!beforeOverlays.has(el)) {
                overlaysOpened.push({
                  selector: describeSelector(el),
                  text: (el.textContent || '').trim().slice(0, 120)
                })
              }
            }
            for (const el of beforeOverlays) {
              if (!afterOverlays.has(el) || !document.contains(el)) {
                overlaysClosed.push({
                  selector: describeSelector(el),
                  text: ''
                })
              }
            }

            // Detect toasts / notifications
            interface ToastEntry { text: string; type: string }
            const toasts: ToastEntry[] = []
            const toastSelectors = [
              '[role="alert"]', '[role="status"]', '[aria-live="polite"]', '[aria-live="assertive"]',
              '.toast', '.snackbar', '.notification', '.alert',
              '[class*="toast"]', '[class*="snackbar"]', '[class*="notification"]'
            ]
            for (const added of addedNodes) {
              if (matchesAnySelectorSafe(added, toastSelectors)) {
                const text = (added.textContent || '').trim().slice(0, 200)
                if (text) {
                  toasts.push({ text, type: classifyToastType(added) })
                }
              }
              // Check children for toast elements
              try {
                for (const tSel of toastSelectors) {
                  const children = added.querySelectorAll(tSel)
                  for (let i = 0; i < children.length; i++) {
                    const child = children[i]!
                    const text = (child.textContent || '').trim().slice(0, 200)
                    if (text) {
                      toasts.push({ text, type: classifyToastType(child) })
                    }
                  }
                }
              } catch { /* ignore */ }
            }

            // Detect form errors
            const formErrors: string[] = []
            const errorSelectors = [
              '.error', '.invalid', '.field-error', '.form-error', '.validation-error',
              '[aria-invalid="true"]', '.has-error', '.is-invalid'
            ]
            for (const added of addedNodes) {
              if (matchesAnySelectorSafe(added, errorSelectors)) {
                const text = (added.textContent || '').trim().slice(0, 200)
                if (text) formErrors.push(text)
              }
              try {
                for (const eSel of errorSelectors) {
                  const children = added.querySelectorAll(eSel)
                  for (let i = 0; i < children.length; i++) {
                    const text = (children[i]!.textContent || '').trim().slice(0, 200)
                    if (text && !formErrors.includes(text)) formErrors.push(text)
                  }
                }
              } catch { /* ignore */ }
            }

            // Detect loading indicators
            const loadingIndicators: string[] = []
            const loadingSelectors = [
              '.spinner', '.loading', '.skeleton', '[aria-busy="true"]',
              '[class*="spinner"]', '[class*="loading"]', '[class*="skeleton"]'
            ]
            for (const added of addedNodes) {
              if (matchesAnySelectorSafe(added, loadingSelectors)) {
                loadingIndicators.push(describeSelector(added))
              }
            }

            // Detect text changes
            interface TextChangeEntry { selector: string; from: string; to: string }
            const textChanges: TextChangeEntry[] = []
            for (const [el, oldText] of textSnapshots) {
              if (!document.contains(el)) continue
              const newText = (el.textContent || '').trim().slice(0, 200)
              if (newText !== oldText) {
                textChanges.push({
                  selector: describeSelector(el),
                  from: oldText.slice(0, 100),
                  to: newText.slice(0, 100)
                })
              }
            }

            resolve({
              success: true,
              action: 'action_diff',
              selector: '',
              action_diff: {
                url_changed: urlChanged,
                title_changed: titleChanged,
                overlays_opened: overlaysOpened.slice(0, 10),
                overlays_closed: overlaysClosed.slice(0, 10),
                toasts: toasts.slice(0, 10),
                form_errors: formErrors.slice(0, 20),
                loading_indicators: loadingIndicators.slice(0, 10),
                elements_added: elementsAdded,
                elements_removed: elementsRemoved,
                text_changes: textChanges.slice(0, 20),
                network_requests: networkRequests
              }
            } as DOMResult)
          }

          // Helper: check if element looks like an overlay
          function isOverlayElement(el: Element): boolean {
            if (!(el instanceof HTMLElement)) return false
            const role = el.getAttribute('role') || ''
            if (role === 'dialog' || role === 'alertdialog') return true
            if (el.getAttribute('aria-modal') === 'true') return true
            if (el.tagName.toLowerCase() === 'dialog') return true
            const style = getComputedStyle(el)
            const zIndex = Number.parseInt(style.zIndex || '', 10)
            if (!Number.isNaN(zIndex) && zIndex >= 1000) {
              const position = style.position || ''
              if (position === 'fixed' || position === 'absolute') {
                const rect = el.getBoundingClientRect()
                if (rect.width >= 100 && rect.height >= 100) return true
              }
            }
            return false
          }

          // Helper: check if element matches any selector from a list
          function matchesAnySelectorSafe(el: Element, sels: string[]): boolean {
            for (const sel of sels) {
              try { if (typeof el.matches === 'function' && el.matches(sel)) return true } catch {}
            }
            return false
          }

          // Helper: classify toast type from element classes/attributes
          function classifyToastType(el: Element): string {
            const cls = ((el as HTMLElement).className || '').toString().toLowerCase()
            const role = el.getAttribute('role') || ''
            if (cls.includes('success') || cls.includes('positive')) return 'success'
            if (cls.includes('error') || cls.includes('danger') || cls.includes('negative')) return 'error'
            if (cls.includes('warning') || cls.includes('caution')) return 'warning'
            if (cls.includes('info') || cls.includes('information')) return 'info'
            if (role === 'alert') return 'alert'
            if (role === 'status') return 'status'
            return 'info'
          }


          // Helper: generate a compact selector description for an element
          function describeSelector(el: Element): string {
            const tag = el.tagName.toLowerCase()
            if (el.id) return `#${el.id}`
            const role = el.getAttribute('role')
            if (role) return `${tag}[role="${role}"]`
            const cls = (el as HTMLElement).className
            if (typeof cls === 'string' && cls.trim()) {
              return `${tag}.${cls.trim().split(/\s+/)[0]}`
            }
            return tag
          }

          // Wait for mutations to settle, then classify
          function checkSettled() {
            const elapsed = performance.now() - startTime
            const sinceLastMutation = performance.now() - lastMutationTime

            if (sinceLastMutation >= settleMs || elapsed >= timeoutMs) {
              classifyAndResolve()
              return
            }
            setTimeout(checkSettled, Math.min(100, settleMs / 2))
          }

          // Start checking after a brief delay to capture initial mutations
          setTimeout(checkSettled, Math.min(100, settleMs / 2))
        })
      }
    }
  }

  const handlers = buildActionHandlers(el)
  const handler = handlers[action]
  if (!handler) {
    return domError('unknown_action', `Unknown DOM action: ${action}`)
  }

  // #316: Enrich result with ambiguous_matches warning if text= matched multiple elements
  const rawResult = handler()
  if (!resolvedAmbiguousMatches) return rawResult
  if (rawResult instanceof Promise) {
    return rawResult.then((r) => {
      if (r && typeof r === 'object' && r.success) {
        return { ...r, ambiguous_matches: resolvedAmbiguousMatches }
      }
      return r
    })
  }
  if (rawResult && typeof rawResult === 'object' && (rawResult as DOMResult).success) {
    return { ...(rawResult as DOMResult), ambiguous_matches: resolvedAmbiguousMatches }
  }
  return rawResult
}

// Dispatcher utilities (parseDOMParams, executeDOMAction, etc.) moved to ./dom-dispatch.ts
