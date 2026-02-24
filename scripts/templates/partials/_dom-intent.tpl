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
      const scopeRoot = requestedScope ? activeScope : pickTopDialog(collectDialogs())
      if (!scopeRoot) {
        return {
          error: domError('overlay_not_found', 'No visible dialog/overlay found to dismiss.')
        }
      }
      const candidates = querySelectorAllDeep('button, [role="button"], [aria-label], [data-testid], [title]', scopeRoot)
      const ranked = uniqueElements(candidates).map((candidate) => {
        const label = extractElementLabel(candidate)
        let score = 0
        if (dismissVerb.test(label)) score += 800
        if (submitVerb.test(label)) score -= 550
        score += areaScore(candidate, 30)
        score += elementZIndexScore(candidate)
        return { element: candidate, score }
      })
      const best = pickBestIntentTarget(
        ranked,
        'intent_dismiss_top_overlay',
        'dismiss_action_not_found',
        'No dismiss control found in the top overlay.'
      )
      return {
        ...best,
        scope_selector_used: requestedScope || 'intent:auto_top_dialog'
      }
    }

    return { error: domError('unknown_action', `Unknown DOM action: ${action}`) }
  }
