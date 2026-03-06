  // --- PARTIAL: Ambiguous Target Ranking ---

  function rankAmbiguousCandidates(
    candidates: Element[],
    action: string,
    selectorText: string
  ): { winner: Element | null; gap: number; ranked: { element: Element; score: number }[] } {
    const dialogs = collectDialogs()
    const topDialog = dialogs.length > 0 ? pickTopDialog(dialogs) : null

    // Extract the text portion from semantic selectors (text=Post → "Post")
    const selectorLabel = (() => {
      if (selectorText.startsWith('text=')) return selectorText.slice(5)
      if (selectorText.startsWith('aria-label=')) return selectorText.slice(11)
      if (selectorText.startsWith('label=')) return selectorText.slice(6)
      if (selectorText.startsWith('placeholder=')) return selectorText.slice(12)
      return ''
    })()

    const clickLikeActions = new Set(['click', 'key_press', 'focus', 'scroll_to', 'set_attribute', 'paste'])
    const typeLikeActions = new Set(['type', 'select', 'check'])

    const scored = candidates.map((el) => {
      const tag = el.tagName.toLowerCase()
      const role = el.getAttribute('role') || ''
      let score = 0

      // Modal scoping: element inside the top open dialog
      if (topDialog && typeof topDialog.contains === 'function' && topDialog.contains(el)) {
        score += 200
      }

      // Element type match
      if (clickLikeActions.has(action)) {
        if (tag === 'button' || role === 'button' || tag === 'input' && ((el as HTMLInputElement).type === 'submit' || (el as HTMLInputElement).type === 'button')) {
          score += 100
        } else if (tag === 'a' || role === 'link') {
          score += 40
        }
      } else if (typeLikeActions.has(action)) {
        if (tag === 'input' || tag === 'textarea' || tag === 'select' || el.getAttribute('contenteditable') === 'true' || role === 'textbox') {
          score += 100
        } else if (tag === 'button' || role === 'button') {
          score += 10
        }
      }

      // Text matching (only when selector provides text)
      if (selectorLabel) {
        const elLabel = extractElementLabel(el)
        const trimmedLabel = elLabel.trim()
        if (trimmedLabel === selectorLabel) {
          score += 80 // exact match
        } else if (trimmedLabel.startsWith(selectorLabel) && trimmedLabel.length <= selectorLabel.length + 5) {
          score += 60 // tight prefix
        }
      }

      // Primary button heuristic
      if (tag === 'button' || role === 'button') {
        const htmlEl = el as HTMLElement
        const cls = (typeof htmlEl.className === 'string' ? htmlEl.className : '').toLowerCase()
        const type = el.getAttribute('type') || ''
        if (type === 'submit') score += 60
        else if (/\bprimary\b|\bbtn-primary\b|\bcta\b/.test(cls)) score += 60
        else {
          const style = typeof getComputedStyle === 'function' ? getComputedStyle(htmlEl) : null
          if (style) {
            const bg = style.backgroundColor || ''
            // Colored background (not transparent, not white, not gray-ish)
            if (bg && !/transparent|rgba\(0,\s*0,\s*0,\s*0\)|rgb\(255,\s*255,\s*255\)|rgb\(2[45]\d,\s*2[45]\d,\s*2[45]\d\)/.test(bg)) {
              score += 30
            }
          }
        }
      }

      // z-index (0–50)
      score += Math.min(50, Math.max(0, elementZIndexScore(el)))

      // Area (0–30)
      score += areaScore(el, 30)

      return { element: el, score }
    })

    scored.sort((a, b) => b.score - a.score)

    const topScore = scored[0]?.score ?? 0
    const secondScore = scored[1]?.score ?? 0
    const gap = topScore - secondScore
    const winner = gap >= 50 ? (scored[0]?.element ?? null) : null

    return { winner, gap, ranked: scored }
  }
