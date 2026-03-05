  // --- PARTIAL: Overlay & Stability Action Handlers (dismiss, wait_for_stable, action_diff) ---
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

          // #444: Stamp overlay with dismiss timestamp for loop detection
          if (overlayEl instanceof HTMLElement && typeof overlayEl.setAttribute === 'function') {
            overlayEl.setAttribute('data-gasoline-dismiss-ts', String(Date.now()))
          }

          // #445: Detect extension-sourced overlays
          const extSource = detectExtensionOverlay(overlayEl)
          const sourceInfo = extSource ? { overlay_source: 'extension' as const } : { overlay_source: 'page' as const }

          // Strategy: escape_fallback — dispatch Escape key instead of clicking
          if (resolvedMatchStrategy === 'dismiss_escape_fallback') {
            const escKb: KeyboardEventInit & { keyCode?: number } = {
              key: 'Escape', code: 'Escape', keyCode: 27,
              bubbles: true, cancelable: true
            }
            dispatchEventIfPossible(document, new KeyboardEvent('keydown', escKb))
            dispatchEventIfPossible(document, new KeyboardEvent('keyup', escKb))
            // Also try the overlay element directly
            dispatchEventIfPossible(node, new KeyboardEvent('keydown', escKb))
            dispatchEventIfPossible(node, new KeyboardEvent('keyup', escKb))
            // #449: Clear dismiss stamp on successful dismissal to prevent stale loop detection
            if (overlayEl instanceof HTMLElement) overlayEl.removeAttribute('data-gasoline-dismiss-ts')
            return mutatingSuccess(node, {
              strategy: 'escape_key',
              ...overlayInfo,
              ...sourceInfo
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
          // #449: Clear dismiss stamp on successful dismissal to prevent stale loop detection
          if (overlayEl instanceof HTMLElement) overlayEl.removeAttribute('data-gasoline-dismiss-ts')
          return mutatingSuccess(node, {
            strategy,
            selector_used: selector || resolvedMatchStrategy,
            ...overlayInfo,
            ...sourceInfo
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

          // #444: Stamp overlay with dismiss timestamp for loop detection
          if (overlayEl instanceof HTMLElement && typeof overlayEl.setAttribute === 'function') {
            overlayEl.setAttribute('data-gasoline-dismiss-ts', String(Date.now()))
          }

          // #445: Detect extension-sourced overlays
          const extSource = detectExtensionOverlay(overlayEl)
          const sourceInfo = extSource ? { overlay_source: 'extension' as const } : { overlay_source: 'page' as const }

          // Strategy: escape_fallback — dispatch Escape key instead of clicking
          if (resolvedMatchStrategy === 'dismiss_escape_fallback') {
            const escKb: KeyboardEventInit & { keyCode?: number } = {
              key: 'Escape', code: 'Escape', keyCode: 27,
              bubbles: true, cancelable: true
            }
            dispatchEventIfPossible(document, new KeyboardEvent('keydown', escKb))
            dispatchEventIfPossible(document, new KeyboardEvent('keyup', escKb))
            dispatchEventIfPossible(node, new KeyboardEvent('keydown', escKb))
            dispatchEventIfPossible(node, new KeyboardEvent('keyup', escKb))
            // #449: Clear dismiss stamp on successful dismissal to prevent stale loop detection
            if (overlayEl instanceof HTMLElement) overlayEl.removeAttribute('data-gasoline-dismiss-ts')
            return mutatingSuccess(node, {
              dismissed_count: 1,
              strategy: 'escape_key',
              ...overlayInfo,
              ...sourceInfo
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
          // #449: Clear dismiss stamp on successful dismissal to prevent stale loop detection
          if (overlayEl instanceof HTMLElement) overlayEl.removeAttribute('data-gasoline-dismiss-ts')
          return mutatingSuccess(node, {
            dismissed_count: 1,
            strategy,
            selector_used: selector || resolvedMatchStrategy,
            ...overlayInfo,
            ...sourceInfo
          })
        }),

      wait_for_stable: (): Promise<DOMResult> => { // Smart DOM stability wait (#344)
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
