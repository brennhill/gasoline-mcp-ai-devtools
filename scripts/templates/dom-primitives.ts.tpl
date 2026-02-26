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

import type { DOMMutationEntry, DOMPrimitiveOptions, DOMResult } from './dom-types'

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

    const ambiguitySensitiveActions = new Set([
      'click', 'type', 'select', 'check', 'set_attribute',
      'paste', 'key_press', 'focus', 'scroll_to'
    ])

    if (!ambiguitySensitiveActions.has(action)) {
      const direct = resolveElement(selector, activeScope)
      if (direct && intersectsScopeRect(direct)) {
        return {
          element: direct,
          match_count: 1,
          match_strategy: selector.includes(':nth-match(')
            ? 'nth_match_selector'
            : (scopeRect ? 'rect_selector' : (requestedScope ? 'scoped_selector' : 'selector')),
          scope_selector_used: scopeSelectorUsed
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
        scope_selector_used: scopeSelectorUsed
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
          message: `Selector matches multiple viable elements: ${selector}. Add scope/scope_rect, or use list_interactive element_id/index.`,
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

  function mutatingSuccess(
    node: Element,
    extra?: Omit<Partial<DOMResult>, 'success' | 'action' | 'selector' | 'matched' | 'match_count' | 'match_strategy'>
  ): DOMResult {
    return {
      success: true,
      action,
      selector,
      ...(scopeRect ? { scope_rect_used: scopeRect } : {}),
      ...(extra || {}),
      matched: matchedTarget(node),
      match_count: resolvedMatchCount,
      match_strategy: resolvedMatchStrategy,
      ...(resolvedRankedCandidates ? { ranked_candidates: resolvedRankedCandidates } : {})
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

  type ActionHandler = () => DOMResult | Promise<DOMResult>

  function buildActionHandlers(node: Element): Record<string, ActionHandler> {
    return {
      click: () =>
        withMutationTracking(() => {
          if (!(node instanceof HTMLElement)) return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`)
          if (options.new_tab) {
            const linkNode = (() => {
              const tag = node.tagName.toLowerCase()
              if (tag === 'a') return node as Element
              if (typeof node.closest === 'function') {
                return node.closest('a[href]')
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

            return mutatingSuccess(node, { value: href, reason: 'opened_new_tab' })
          }
          node.click()
          return mutatingSuccess(node)
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
        node.scrollIntoView({ behavior: 'smooth', block: 'center' })
        return mutatingSuccess(node)
      },

      wait_for: () => ({ success: true, action, selector, value: node.tagName.toLowerCase() }),

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

      dismiss_top_overlay: () =>
        withMutationTracking(() => {
          if (!(node instanceof HTMLElement)) return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`)
          node.click()
          return mutatingSuccess(node)
        })
    }
  }

  const handlers = buildActionHandlers(el)
  const handler = handlers[action]
  if (!handler) {
    return domError('unknown_action', `Unknown DOM action: ${action}`)
  }
  return handler()
}

// Dispatcher utilities (parseDOMParams, executeDOMAction, etc.) moved to ./dom-dispatch.ts
