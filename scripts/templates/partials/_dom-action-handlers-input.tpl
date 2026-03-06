  // --- PARTIAL: Input & Interaction Action Handlers ---
  // Purpose: paste, key_press, hover.
  // #502: open_composer, submit_active_composer, confirm_top_dialog extracted to dom-primitives-intent.ts.
  // Why: Separated from main template to keep each partial under 500 LOC.

      paste: () =>
        withMutationTracking(() => {
          const overlayErr = blockedByOverlayError(node)
          if (overlayErr) return overlayErr

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
          const overlayErr = blockedByOverlayError(node)
          if (overlayErr) return overlayErr

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

      // #502: open_composer, submit_active_composer, confirm_top_dialog removed —
      // now in dom-primitives-intent.ts (self-contained, dispatched by dom-dispatch.ts).

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
