  // --- PARTIAL: Intent Resolution & List Interactive ---
  function listInteractiveCompatibility(): {
    success: boolean
    elements: unknown[]
    candidate_count?: number
    scope_rect_used?: ScopeRect
    error?: string
    message?: string
  } {
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
    const rawEntries: {
      element: Element
      baseSelector: string
      tag: string
      inputType?: string
      elementType: string
      label: string
      role?: string
      placeholder?: string
      visible: boolean
    }[] = []

    const scope = (selector || '').trim()
    const scopeRoot = (() => {
      if (!scope) return document as ParentNode
      try {
        const matches = querySelectorAllDeep(scope)
        if (matches.length === 0) return null
        return chooseBestScopeMatch(matches) as ParentNode
      } catch {
        return null
      }
    })()
    if (!scopeRoot) {
      return {
        success: false,
        elements: [],
        error: 'scope_not_found',
        message: `No scope element matches selector: ${scope}`
      }
    }

    for (const cssSelector of interactiveSelectors) {
      const matches = querySelectorAllDeep(cssSelector, scopeRoot)
      for (const el of matches) {
        if (seen.has(el)) continue
        seen.add(el)

        const htmlEl = el as HTMLElement
        const rect = typeof htmlEl.getBoundingClientRect === 'function'
          ? htmlEl.getBoundingClientRect()
          : ({ width: 0, height: 0 } as DOMRect)
        if (!intersectsScopeRect(el)) continue
        const visible = rect.width > 0 && rect.height > 0 && htmlEl.offsetParent !== null
        const shadowSelector = buildShadowSelector(el)
        const baseSelector = shadowSelector || buildUniqueSelector(el, htmlEl, cssSelector)
        const label =
          el.getAttribute('aria-label') ||
          el.getAttribute('title') ||
          el.getAttribute('placeholder') ||
          (htmlEl.textContent || '').trim().slice(0, 60) ||
          el.tagName.toLowerCase()

        rawEntries.push({
          element: el,
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

    const selectorCount = new Map<string, number>()
    for (const entry of rawEntries) {
      selectorCount.set(entry.baseSelector, (selectorCount.get(entry.baseSelector) || 0) + 1)
    }
    const selectorIndex = new Map<string, number>()

    const elements = rawEntries.map((entry, index) => {
      let selector = entry.baseSelector
      const count = selectorCount.get(entry.baseSelector) || 1
      if (count > 1) {
        const nth = (selectorIndex.get(entry.baseSelector) || 0) + 1
        selectorIndex.set(entry.baseSelector, nth)
        selector = `${entry.baseSelector}:nth-match(${nth})`
      }
      return {
        index,
        tag: entry.tag,
        type: entry.inputType,
        element_type: entry.elementType,
        selector,
        element_id: getOrCreateElementID(entry.element),
        label: entry.label,
        role: entry.role,
        placeholder: entry.placeholder,
        bbox: extractBoundingBox(entry.element),
        visible: entry.visible
      }
    })

    return {
      success: true,
      elements,
      candidate_count: elements.length,
      ...(scopeRect ? { scope_rect_used: scopeRect } : {})
    }
  }

  if (action === 'list_interactive') {
    return listInteractiveCompatibility()
  }

  // — Resolve element for all other actions —
  function domError(error: string, message: string): DOMResult {
    return { success: false, action, selector, error, message }
  }

  function matchedTarget(node: Element): NonNullable<DOMResult['matched']> {
    const htmlEl = node as HTMLElement
    const textPreview = (htmlEl.textContent || '').trim().slice(0, 80)
    return {
      tag: node.tagName.toLowerCase(),
      role: node.getAttribute('role') || undefined,
      aria_label: node.getAttribute('aria-label') || undefined,
      text_preview: textPreview || undefined,
      selector,
      element_id: getOrCreateElementID(node),
      bbox: extractBoundingBox(node),
      scope_selector_used: resolvedScopeSelector,
      ...(scopeRect ? { scope_rect_used: scopeRect } : {})
    }
  }

  function isActionableVisible(el: Element): boolean {
    if (!(el instanceof HTMLElement)) return true
    const rect = typeof el.getBoundingClientRect === 'function'
      ? el.getBoundingClientRect()
      : ({ width: 0, height: 0 } as DOMRect)
    return rect.width > 0 && rect.height > 0 && el.offsetParent !== null
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

  function summarizeCandidates(matches: Element[]): NonNullable<DOMResult['candidates']> {
    return matches.slice(0, 8).map((candidate) => {
      const htmlEl = candidate as HTMLElement
      const fallback = candidate.tagName.toLowerCase()
      return {
        tag: fallback,
        role: candidate.getAttribute('role') || undefined,
        aria_label: candidate.getAttribute('aria-label') || undefined,
        text_preview: (htmlEl.textContent || '').trim().slice(0, 80) || undefined,
        selector: buildUniqueSelector(candidate, htmlEl, fallback),
        element_id: getOrCreateElementID(candidate),
        bbox: extractBoundingBox(candidate),
        visible: isActionableVisible(candidate)
      }
    })
  }

  const intentActions = new Set([
    'open_composer',
    'submit_active_composer',
    'confirm_top_dialog',
    'dismiss_top_overlay'
  ])

  type RankedIntentCandidate = { element: Element; score: number }

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

  function elementZIndexScore(el: Element): number {
    if (!(el instanceof HTMLElement)) return 0
    const style = getComputedStyle(el)
    const raw = style.zIndex || ''
    const parsed = Number.parseInt(raw, 10)
    if (Number.isNaN(parsed)) return 0
    return parsed
  }

  function areaScore(el: Element, max: number): number {
    if (!(el instanceof HTMLElement) || typeof el.getBoundingClientRect !== 'function') return 0
    const rect = el.getBoundingClientRect()
    if (rect.width <= 0 || rect.height <= 0) return 0
    return Math.min(max, Math.round((rect.width * rect.height) / 10000))
  }

  function pickBestIntentTarget(
    ranked: RankedIntentCandidate[],
    matchStrategy: string,
    notFoundError: string,
    notFoundMessage: string
  ): { element?: Element; error?: DOMResult; match_count?: number; match_strategy?: string } {
    const viable = ranked
      .filter((entry) => entry.score > 0 && isActionableVisible(entry.element) && intersectsScopeRect(entry.element))
      .sort((a, b) => b.score - a.score)

    if (viable.length === 0) {
      return { error: domError(notFoundError, notFoundMessage) }
    }

    const topScore = viable[0]!.score
    const tiedTop = viable.filter((entry) => entry.score === topScore)
    if (tiedTop.length > 1) {
      return {
        error: {
          success: false,
          action,
          selector,
          error: 'ambiguous_target',
          message: `Multiple candidates tie for ${action}. Use scope_selector/scope_rect or list_interactive element_id.`,
          match_count: tiedTop.length,
          match_strategy: matchStrategy,
          ...(scopeRect ? { scope_rect_used: scopeRect } : {}),
          candidates: summarizeCandidates(tiedTop.map((entry) => entry.element))
        }
      }
    }

    return {
      element: viable[0]!.element,
      match_count: 1,
      match_strategy: matchStrategy
    }
  }

  function collectDialogs(): Element[] {
    const selectors = ['[role="dialog"]', '[aria-modal="true"]', 'dialog[open]']
    const dialogs: Element[] = []
    for (const dialogSelector of selectors) {
      dialogs.push(...querySelectorAllDeep(dialogSelector))
    }
    return uniqueElements(dialogs).filter(isActionableVisible)
  }

  function pickTopDialog(dialogs: Element[]): Element | null {
    if (dialogs.length === 0) return null
    const ranked = dialogs
      .map((dialog, index) => ({
        element: dialog,
        score: elementZIndexScore(dialog) * 1000 + areaScore(dialog, 200) + index
      }))
      .sort((a, b) => b.score - a.score)
    return ranked[0]?.element || null
  }

  function resolveIntentTarget(
    requestedScope: string,
    activeScope: ParentNode
  ): { element?: Element; error?: DOMResult; match_count?: number; match_strategy?: string; scope_selector_used?: string } {
    const submitVerb = /(post|share|publish|send|submit|save|done|continue|next|create|apply|confirm|yes|allow|accept)/i
    const dismissVerb = /(close|dismiss|cancel|not now|no thanks|skip|x|×|hide|back)/i
    const composerVerb = /(start( a)? post|create post|write (a )?post|what'?s on your mind|share( an)? update|compose|new post)/i

    if (action === 'open_composer') {
      const selectors = [
        'button',
        '[role="button"]',
        'a[href]',
        '[role="link"]',
        '[contenteditable="true"]',
        '[role="textbox"]',
        'textarea',
        'input[type="text"]',
        'input:not([type])'
      ]
      const candidates: Element[] = []
      for (const candidateSelector of selectors) {
        candidates.push(...querySelectorAllDeep(candidateSelector, activeScope))
      }

      const ranked = uniqueElements(candidates).map((candidate) => {
        const label = extractElementLabel(candidate).toLowerCase()
        const tag = candidate.tagName.toLowerCase()
        const role = candidate.getAttribute('role') || ''
        const contentEditable = candidate.getAttribute('contenteditable') === 'true'
        let score = 0
        if (composerVerb.test(label)) score += 700
        if (/\b(post|share|publish|compose|write|update)\b/i.test(label)) score += 280
        if (contentEditable || role === 'textbox' || tag === 'textarea' || tag === 'input') score += 220
        if (tag === 'button' || role === 'button') score += 80
        score += areaScore(candidate, 50)
        score += elementZIndexScore(candidate)
        return { element: candidate, score }
      })

      const best = pickBestIntentTarget(
        ranked,
        'intent_open_composer',
        'composer_not_found',
        'No composer trigger was found. Try a tighter scope_selector.'
      )
      return { ...best, scope_selector_used: requestedScope || undefined }
    }

    if (action === 'submit_active_composer') {
      let scopeRoot: ParentNode = activeScope
      let scopeUsed: string | undefined = requestedScope || undefined
      if (!requestedScope) {
        const dialogs = collectDialogs()
        const rankedDialogs = dialogs
          .map((dialog) => {
            const textboxes = querySelectorAllDeep('[role="textbox"], textarea, [contenteditable="true"]', dialog).filter(isActionableVisible).length
            const buttons = querySelectorAllDeep('button, [role="button"], input[type="submit"]', dialog)
            const submitLikeButtons = buttons.filter((button) => isActionableVisible(button) && submitVerb.test(extractElementLabel(button))).length
            return {
              element: dialog,
              score: textboxes * 1200 + submitLikeButtons * 300 + elementZIndexScore(dialog) * 2 + areaScore(dialog, 80)
            }
          })
          .sort((a, b) => b.score - a.score)

        if ((rankedDialogs[0]?.score || 0) > 0) {
          scopeRoot = rankedDialogs[0]!.element
          scopeUsed = 'intent:auto_composer_scope'
        }
      }

      const candidates = querySelectorAllDeep('button, [role="button"], input[type="submit"]', scopeRoot)
      const ranked = uniqueElements(candidates).map((candidate) => {
        const label = extractElementLabel(candidate)
        let score = 0
        if (submitVerb.test(label)) score += 700
        if (dismissVerb.test(label)) score -= 500
        score += areaScore(candidate, 30)
        score += elementZIndexScore(candidate)
        return { element: candidate, score }
      })
      const best = pickBestIntentTarget(
        ranked,
        'intent_submit_active_composer',
        'composer_submit_not_found',
        'No submit control found in active composer scope.'
      )
      return { ...best, scope_selector_used: scopeUsed }
    }

    if (action === 'confirm_top_dialog') {
      const scopeRoot = requestedScope ? activeScope : pickTopDialog(collectDialogs())
      if (!scopeRoot) {
        return {
          error: domError('dialog_not_found', 'No visible dialog/overlay found to confirm.')
        }
      }
      const candidates = querySelectorAllDeep('button, [role="button"], input[type="submit"]', scopeRoot)
      const ranked = uniqueElements(candidates).map((candidate) => {
        const label = extractElementLabel(candidate)
        let score = 0
        if (submitVerb.test(label)) score += 700
        if (dismissVerb.test(label)) score -= 500
        score += areaScore(candidate, 30)
        score += elementZIndexScore(candidate)
        return { element: candidate, score }
      })
      const best = pickBestIntentTarget(
        ranked,
        'intent_confirm_top_dialog',
        'confirm_action_not_found',
        'No confirm control found in the top dialog.'
      )
      return {
        ...best,
        scope_selector_used: requestedScope || 'intent:auto_top_dialog'
      }
    }

    if (action === 'dismiss_top_overlay') {
      // Enhanced dismiss: find overlay using z-index analysis + role detection,
      // then try multiple dismissal strategies in sequence (#334)
      const overlayElement = requestedScope ? activeScope as Element : findTopmostOverlay()
      if (!overlayElement) {
        return {
          error: domError('overlay_not_found', 'No visible dialog/overlay/modal found to dismiss.')
        }
      }
      const overlayInfo = describeOverlay(overlayElement)

      // Strategy A: Try expanded close button selectors within the overlay
      const closeButtonSelectors = [
        'button.close', '.btn-close',
        '[aria-label="Close"]', '[aria-label="close"]', '[aria-label="Dismiss"]', '[aria-label="dismiss"]',
        '[data-dismiss="modal"]', '[data-bs-dismiss="modal"]', '[data-dismiss="dialog"]',
        '[data-dismiss="alert"]', '[data-bs-dismiss="alert"]',
        'button.modal-close', '.dialog-close', '.overlay-close', '.popup-close',
      ]
      for (const closeSelector of closeButtonSelectors) {
        const matches = querySelectorAllDeep(closeSelector, overlayElement as ParentNode)
        const visible = matches.filter(isActionableVisible)
        if (visible.length > 0) {
          return {
            element: visible[0],
            match_count: 1,
            match_strategy: 'dismiss_close_button_selector',
            scope_selector_used: requestedScope || 'intent:auto_top_overlay'
          }
        }
      }

      // Strategy B: Find buttons with dismiss-like text content (expanded patterns)
      const dismissTextPatterns = /^(close|dismiss|cancel|not now|no thanks|skip|hide|back|got it|maybe later|x|\u00d7|\u2715|\u2716|\u2573)$/i
      const allButtons = querySelectorAllDeep('button, [role="button"]', overlayElement as ParentNode)
      const dismissButtons: RankedIntentCandidate[] = []
      for (const btn of uniqueElements(allButtons)) {
        if (!isActionableVisible(btn)) continue
        const label = extractElementLabel(btn).trim()
        let score = 0
        if (dismissTextPatterns.test(label)) score += 900
        else if (dismissVerb.test(label)) score += 700
        if (submitVerb.test(label)) score -= 600
        // SVG close icons: button containing only an SVG (common close icon pattern)
        const hasSvgIcon = btn.querySelector('svg') !== null
        const textLen = (btn.textContent || '').trim().length
        if (hasSvgIcon && textLen <= 2) score += 500
        // Small buttons in header area are likely close buttons
        const rect = (btn as HTMLElement).getBoundingClientRect()
        if (rect.width > 0 && rect.width < 60 && rect.height > 0 && rect.height < 60) score += 100
        score += elementZIndexScore(btn)
        if (score > 0) dismissButtons.push({ element: btn, score })
      }
      if (dismissButtons.length > 0) {
        dismissButtons.sort((a, b) => b.score - a.score)
        return {
          element: dismissButtons[0]!.element,
          match_count: 1,
          match_strategy: 'dismiss_text_button',
          scope_selector_used: requestedScope || 'intent:auto_top_overlay'
        }
      }

      // Strategy C: Try elements with dismiss-related attributes (data-testid, title)
      const attrCandidates = querySelectorAllDeep('[data-testid], [title]', overlayElement as ParentNode)
      for (const candidate of uniqueElements(attrCandidates)) {
        if (!isActionableVisible(candidate)) continue
        const testId = candidate.getAttribute('data-testid') || ''
        const title = candidate.getAttribute('title') || ''
        if (dismissVerb.test(testId) || dismissVerb.test(title)) {
          return {
            element: candidate,
            match_count: 1,
            match_strategy: 'dismiss_attr_match',
            scope_selector_used: requestedScope || 'intent:auto_top_overlay'
          }
        }
      }

      // Strategy D: Press Escape key (most modals respond to Escape)
      // Return the overlay itself as the element; the action handler will dispatch Escape
      return {
        element: overlayElement,
        match_count: 1,
        match_strategy: 'dismiss_escape_fallback',
        scope_selector_used: requestedScope || 'intent:auto_top_overlay'
      }
    }

    return { error: domError('unknown_action', `Unknown DOM action: ${action}`) }
  }

  // --- Helper: Find topmost visible overlay using z-index analysis + role detection (#334) ---
  function findTopmostOverlay(): Element | null {
    // Collect all dialog/modal candidates
    const dialogSelectors = [
      '[role="dialog"]', '[role="alertdialog"]', '[aria-modal="true"]', 'dialog[open]',
      '.modal.show', '.modal.in', '.modal.is-active', '.modal[style*="display: block"]',
      '.overlay', '.popup', '.lightbox',
      '[data-modal]', '[data-overlay]', '[data-dialog]',
    ]
    const candidates: Element[] = []
    for (const dialogSelector of dialogSelectors) {
      candidates.push(...querySelectorAllDeep(dialogSelector))
    }

    // Also check for high z-index elements that look like overlays
    const allElements = document.querySelectorAll('*')
    for (let i = 0; i < allElements.length; i++) {
      const el = allElements[i]!
      if (!(el instanceof HTMLElement)) continue
      const style = getComputedStyle(el)
      const zIndex = Number.parseInt(style.zIndex || '', 10)
      if (Number.isNaN(zIndex) || zIndex < 1000) continue
      const position = style.position || ''
      if (position !== 'fixed' && position !== 'absolute') continue
      const rect = el.getBoundingClientRect()
      // Must be reasonably sized (not a tiny tooltip)
      if (rect.width < 100 || rect.height < 100) continue
      // Must be visible
      if (style.display === 'none' || style.visibility === 'hidden' || style.opacity === '0') continue
      candidates.push(el)
    }

    const unique = uniqueElements(candidates).filter(isActionableVisible)
    if (unique.length === 0) return null

    // Score and pick the topmost
    const ranked = unique.map((candidate, index) => ({
      element: candidate,
      score: elementZIndexScore(candidate) * 1000 + areaScore(candidate, 200) + index
    }))
    ranked.sort((a, b) => b.score - a.score)
    return ranked[0]?.element || null
  }

  function describeOverlay(el: Element): { overlay_type: string; overlay_selector: string; overlay_text_preview: string } {
    const tag = el.tagName.toLowerCase()
    const role = el.getAttribute('role') || ''
    const ariaModal = el.getAttribute('aria-modal') || ''
    let overlayType = 'unknown'
    if (tag === 'dialog') overlayType = 'dialog'
    else if (role === 'dialog' || role === 'alertdialog') overlayType = role
    else if (ariaModal === 'true') overlayType = 'modal'
    else overlayType = 'overlay'
    const overlaySelector = (() => {
      if (el.id) return `#${el.id}`
      if (role) return `${tag}[role="${role}"]`
      const className = (el as HTMLElement).className
      if (typeof className === 'string' && className.trim()) return `${tag}.${className.trim().split(/\s+/)[0]}`
      return tag
    })()
    const textPreview = ((el as HTMLElement).textContent || '').trim().slice(0, 120)
    return { overlay_type: overlayType, overlay_selector: overlaySelector, overlay_text_preview: textPreview }
  }
