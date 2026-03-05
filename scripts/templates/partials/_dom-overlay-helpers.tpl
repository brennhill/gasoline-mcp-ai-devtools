  // --- PARTIAL: Overlay Detection Helpers ---
  // Purpose: Find topmost overlay, describe overlays, detect extension-injected overlays.
  // Why: Separated from _dom-intent.tpl to keep each partial under 500 LOC.

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

  // #445: Detect if an overlay was injected by a browser extension
  function detectExtensionOverlay(el: Element): boolean {
    // Check for chrome-extension:// URLs in iframes or resource links within the overlay
    const iframes = el instanceof HTMLElement ? el.querySelectorAll('iframe, img, script, link') : []
    for (let i = 0; i < iframes.length; i++) {
      const child = iframes[i]!
      const src = child.getAttribute('src') || child.getAttribute('href') || ''
      if (src.startsWith('chrome-extension://') || src.startsWith('moz-extension://')) return true
    }
    // #453: Check if the element is inside a shadow DOM hosted by an extension-injected custom element.
    // Generic web components also use hyphenated tag names, so a bare hyphen check causes
    // false positives. Restrict detection to extension-origin signals:
    //   1. Shadow host's own resources reference extension URLs
    //   2. Host carries known extension-injected markers (data-extension-id, __ext, grammarly-, etc.)
    //   3. Host tag starts with a known extension prefix
    const extensionTagPrefixes = ['grammarly-', 'lastpass-', 'bitwarden-', '1password-', 'dashlane-', 'honey-', 'loom-']
    const extensionAttrPatterns = ['data-extension-id', 'data-ext-', '__ext']
    let node: Node | null = el
    while (node) {
      const root: Node | null = typeof node.getRootNode === 'function' ? node.getRootNode() : null
      if (root && root !== document && root instanceof ShadowRoot) {
        const host: Element | undefined = (root as ShadowRoot & { host?: Element }).host
        if (host) {
          const hostTag = host.tagName?.toLowerCase() || ''
          // Check if the shadow host itself references extension resources
          const hostResources = host.querySelectorAll('iframe, img, script, link')
          for (let j = 0; j < hostResources.length; j++) {
            const res = hostResources[j]!
            const resSrc = res.getAttribute('src') || res.getAttribute('href') || ''
            if (resSrc.startsWith('chrome-extension://') || resSrc.startsWith('moz-extension://')) return true
          }
          // Check for known extension tag prefixes
          if (extensionTagPrefixes.some(prefix => hostTag.startsWith(prefix))) return true
          // Check for extension-injected marker attributes on the host
          if (host.hasAttributes()) {
            const attrs = host.attributes
            for (let k = 0; k < attrs.length; k++) {
              const attrName = attrs[k]!.name.toLowerCase()
              if (extensionAttrPatterns.some(pat => attrName.startsWith(pat) || attrName.includes(pat))) return true
            }
          }
          // Cross the shadow boundary to continue walking up the tree
          node = host
          continue
        }
      }
      node = (node as Element).parentElement || null
    }
    return false
  }
