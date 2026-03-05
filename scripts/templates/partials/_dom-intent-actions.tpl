  // --- PARTIAL: Intent Action Resolution ---
  // Purpose: resolveIntentTarget for composer, dialog, overlay, dismiss, and auto-dismiss actions.
  // Why: Separated from _dom-intent.tpl to keep each partial under 500 LOC.

  function resolveIntentTarget(
    requestedScope: string,
    activeScope: ParentNode
  ): { element?: Element; error?: DOMResult; match_count?: number; match_strategy?: string; scope_selector_used?: string } {
    // #444: Shared TTL for dismiss loop detection across dismiss_top_overlay and auto_dismiss_overlays
    const dismissStampTTL = 30000 // 30 seconds
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

      // #444: Dismiss loop detection — check if this overlay was already attempted
      const priorStamp = overlayElement.getAttribute('data-gasoline-dismiss-ts')
      if (priorStamp) {
        const elapsed = Date.now() - Number(priorStamp)
        if (elapsed < dismissStampTTL) {
          const info = describeOverlay(overlayElement)
          const loopError = domError(
            'dismiss_loop_detected',
            `Overlay (${info.overlay_selector}) was already attempted ${Math.round(elapsed / 1000)}s ago and is still visible. ` +
            'It may be non-dismissable. Try a different approach: use a specific selector to target its close mechanism, ' +
            'navigate away, or ignore it if it does not block interaction.'
          )
          loopError.overlay_type = info.overlay_type
          loopError.overlay_selector = info.overlay_selector
          loopError.overlay_text_preview = info.overlay_text_preview
          loopError.overlay_source = detectExtensionOverlay(overlayElement) ? 'extension' : 'page'
          return { error: loopError }
        }
        // Stale stamp — clear it and proceed
        overlayElement.removeAttribute('data-gasoline-dismiss-ts')
      }

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
            match_strategy: 'intent_dismiss_top_overlay',
            scope_selector_used: requestedScope || 'intent:auto_top_overlay'
          }
        }
      }

      // Strategy B: Find buttons with dismiss-like text content (expanded patterns)
      const dismissTextPatterns = /^(close|dismiss|cancel|not now|no thanks|skip|hide|back|got it|maybe later|x|\u00d7|\u2715|\u2716|\u2573)$/i
      const allButtons = querySelectorAllDeep('button, [role="button"], [aria-label], [data-testid], [title]', overlayElement as ParentNode)
      const dismissButtons: RankedIntentCandidate[] = []
      for (const btn of uniqueElements(allButtons)) {
        if (!isActionableVisible(btn)) continue
        const label = extractElementLabel(btn).trim()
        let score = 0
        if (dismissTextPatterns.test(label)) score += 900
        else if (dismissVerb.test(label)) score += 700
        if (submitVerb.test(label)) score -= 600
        // SVG close icons: button containing only an SVG (common close icon pattern)
        const hasSvgIcon = typeof btn.querySelector === 'function' && btn.querySelector('svg') !== null
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
          match_strategy: 'intent_dismiss_top_overlay',
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
            match_strategy: 'intent_dismiss_top_overlay',
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

    if (action === 'wait_for_stable') {
      return { element: document.body, match_count: 1, match_strategy: 'wait_for_stable' }
    }

    if (action === 'wait_for_text') {
      return { element: document.body, match_count: 1, match_strategy: 'wait_for_text' }
    }

    if (action === 'wait_for_absent') {
      return { element: document.body, match_count: 1, match_strategy: 'wait_for_absent' }
    }

    if (action === 'action_diff') {
      return { element: document.body, match_count: 1, match_strategy: 'action_diff' }
    }

    if (action === 'auto_dismiss_overlays') {
      // Auto-dismiss cookie consent banners and overlays (#342)

      // #453: Dismiss loop detection must run BEFORE consent-selector short-circuit
      // to prevent infinite loops when a consent banner cannot be dismissed.
      const overlayElement = findTopmostOverlay()
      if (overlayElement) {
        const priorAutoStamp = overlayElement.getAttribute('data-gasoline-dismiss-ts')
        if (priorAutoStamp) {
          const elapsed = Date.now() - Number(priorAutoStamp)
          if (elapsed < dismissStampTTL) {
            const info = describeOverlay(overlayElement)
            const loopError = domError(
              'dismiss_loop_detected',
              `Overlay (${info.overlay_selector}) was already attempted ${Math.round(elapsed / 1000)}s ago and is still visible. ` +
              'It may be non-dismissable. Try a different approach: use a specific selector to target its close mechanism, ' +
              'navigate away, or ignore it if it does not block interaction.'
            )
            loopError.overlay_type = info.overlay_type
            loopError.overlay_selector = info.overlay_selector
            loopError.overlay_text_preview = info.overlay_text_preview
            loopError.overlay_source = detectExtensionOverlay(overlayElement) ? 'extension' : 'page'
            return { error: loopError }
          }
          overlayElement.removeAttribute('data-gasoline-dismiss-ts')
        }
      }

      // Strategy 1: Try known consent framework selectors (most specific)
      const consentSelectors = [
        // CookieBot
        '#CybotCookiebotDialogBodyLevelButtonLevelOptinAllowAll',
        '#CybotCookiebotDialogBodyButtonDecline',
        // OneTrust
        '#onetrust-accept-btn-handler',
        '.onetrust-close-btn-handler',
        // CookieYes
        '.cky-btn-accept',
        // Quantcast / GDPR generic
        '[data-cookieconsent="accept"]',
        '.cc-accept',
        '.cc-dismiss',
        // Generic patterns
        'button[id*="cookie" i][id*="accept" i]',
        'button[id*="consent" i][id*="accept" i]',
      ]
      for (const consentSelector of consentSelectors) {
        try {
          const matches = querySelectorAllDeep(consentSelector)
          const visible = matches.filter(isActionableVisible)
          if (visible.length > 0) {
            return {
              element: visible[0],
              match_count: 1,
              match_strategy: 'consent_framework_selector',
              scope_selector_used: 'intent:auto_dismiss_consent'
            }
          }
        } catch {
          // Ignore invalid selectors (e.g., :i flag not supported in some contexts)
          continue
        }
      }

      // Strategy 2: Fall back to dismiss_top_overlay multi-strategy approach
      if (overlayElement) {
        // Reuse the dismiss_top_overlay strategy chain
        const closeButtonSelectors = [
          'button.close', '.btn-close',
          '[aria-label="Close"]', '[aria-label="close"]', '[aria-label="Dismiss"]', '[aria-label="dismiss"]',
          '[data-dismiss="modal"]', '[data-bs-dismiss="modal"]',
        ]
        for (const closeSelector of closeButtonSelectors) {
          const matches = querySelectorAllDeep(closeSelector, overlayElement as ParentNode)
          const visible = matches.filter(isActionableVisible)
          if (visible.length > 0) {
            return {
              element: visible[0],
              match_count: 1,
              match_strategy: 'auto_dismiss_close_button',
              scope_selector_used: 'intent:auto_dismiss_overlay'
            }
          }
        }

        // Try dismiss-like text buttons
        const dismissTextPatterns = /^(close|dismiss|cancel|not now|no thanks|skip|hide|got it|maybe later|x|\u00d7|\u2715|\u2716|\u2573|accept|allow|agree|ok|okay)$/i
        const allButtons = querySelectorAllDeep('button, [role="button"]', overlayElement as ParentNode)
        const dismissCandidates: RankedIntentCandidate[] = []
        for (const btn of uniqueElements(allButtons)) {
          if (!isActionableVisible(btn)) continue
          const label = extractElementLabel(btn).trim()
          let score = 0
          if (dismissTextPatterns.test(label)) score += 900
          else if (dismissVerb.test(label)) score += 700
          const hasSvgIcon = btn.querySelector('svg') !== null
          const textLen = (btn.textContent || '').trim().length
          if (hasSvgIcon && textLen <= 2) score += 500
          score += elementZIndexScore(btn)
          if (score > 0) dismissCandidates.push({ element: btn, score })
        }
        if (dismissCandidates.length > 0) {
          dismissCandidates.sort((a, b) => b.score - a.score)
          return {
            element: dismissCandidates[0]!.element,
            match_count: 1,
            match_strategy: 'auto_dismiss_text_button',
            scope_selector_used: 'intent:auto_dismiss_overlay'
          }
        }

        // Escape fallback
        return {
          element: overlayElement,
          match_count: 1,
          match_strategy: 'dismiss_escape_fallback',
          scope_selector_used: 'intent:auto_dismiss_overlay'
        }
      }

      // No overlay found — return success with no element (nothing to dismiss)
      return {
        error: domError('no_overlays', 'No cookie consent banners or overlays found to dismiss.')
      }
    }

    return { error: domError('unknown_action', `Unknown DOM action: ${action}`) }
  }
