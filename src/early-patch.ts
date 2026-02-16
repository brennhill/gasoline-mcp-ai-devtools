// early-patch.ts — Lightweight WebSocket + attachShadow patches.
// Runs in MAIN world at document_start before any page scripts.
// Saves original WebSocket and buffers connections for handoff to inject.bundled.js.
// Captures closed shadow roots in a WeakMap for dom-primitives deep traversal.
// Must be self-contained: no imports, no chrome.* APIs (MAIN world).

;(function () {
  'use strict'

  if (typeof window === 'undefined') return

  // Buffer early-patch diagnostics until inject script can forward them.
  const MAX_EARLY_LOGS = 50
  function emitEarlyLog(
    level: 'debug' | 'info' | 'warn' | 'error',
    message: string,
    category: string,
    data?: unknown
  ): void {
    const entry: EarlyPatchLogEntry = {
      ts: new Date().toISOString(),
      level,
      message,
      source: 'early-patch',
      category,
      ...(data !== undefined ? { data } : {})
    }

    if (window.__GASOLINE_INJECT_READY__) {
      // inject.js is active; emit directly into the standard log path.
      try {
        window.postMessage(
          {
            type: 'GASOLINE_LOG',
            payload: {
              ts: entry.ts,
              level: entry.level,
              message: entry.message,
              source: entry.source,
              category: entry.category,
              type: 'early_patch',
              ...(entry.data !== undefined ? { data: entry.data } : {})
            }
          },
          window.location.origin
        )
        return
      } catch {
        // Fall through to queue when postMessage fails.
      }
    }

    const queue = window.__GASOLINE_EARLY_LOGS__ || []
    queue.push(entry)
    if (queue.length > MAX_EARLY_LOGS) {
      queue.splice(0, queue.length - MAX_EARLY_LOGS)
    }
    window.__GASOLINE_EARLY_LOGS__ = queue
  }

  // --- Closed Shadow Root Capture ---
  const OriginalAttachShadow = Element.prototype.attachShadow
  if (OriginalAttachShadow && !window.__GASOLINE_CLOSED_SHADOWS__) {
    const closedRoots = new WeakMap<Element, ShadowRoot>()
    window.__GASOLINE_CLOSED_SHADOWS__ = closedRoots
    window.__GASOLINE_ORIGINAL_ATTACH_SHADOW__ = OriginalAttachShadow
    let overwriteCount = 0
    let delegate: typeof Element.prototype.attachShadow = OriginalAttachShadow

    // Always route attachShadow through this trampoline so closed-root capture survives page-level overrides.
    const trampoline: typeof Element.prototype.attachShadow = function (
      this: Element,
      init: ShadowRootInit
    ): ShadowRoot {
      const root = delegate.call(this, init)
      if (init.mode === 'closed') {
        closedRoots.set(this, root)
      }
      return root
    }

    const originalDescriptor = Object.getOwnPropertyDescriptor(Element.prototype, 'attachShadow')
    try {
      Object.defineProperty(Element.prototype, 'attachShadow', {
        configurable: true,
        enumerable: originalDescriptor?.enumerable ?? false,
        get(): typeof Element.prototype.attachShadow {
          return trampoline
        },
        set(next: unknown): void {
          if (typeof next !== 'function') {
            emitEarlyLog('warn', 'attachShadow overwrite ignored (non-function)', 'shadow_dom', {
              assigned_type: typeof next
            })
            return
          }
          if (next === trampoline) return

          const replacement = next as typeof Element.prototype.attachShadow
          delegate = function (this: Element, init: ShadowRootInit): ShadowRoot {
            return replacement.call(this, init)
          }

          overwriteCount += 1
          const marker = (replacement as { __gasolineMarker?: string }).__gasolineMarker
          emitEarlyLog('warn', 'attachShadow overwrite intercepted', 'shadow_dom', {
            overwrite_count: overwriteCount,
            replacement_name: replacement.name || 'anonymous',
            ...(marker ? { marker } : {})
          })
        }
      })
    } catch (error) {
      // Fallback to direct patch if descriptor hardening fails.
      emitEarlyLog('error', 'attachShadow hardening install failed; using fallback wrapper', 'shadow_dom', {
        error: error instanceof Error ? error.message : String(error)
      })
      Element.prototype.attachShadow = trampoline
    }
  }

  if (!window.WebSocket) return

  // Guard: only install once (extension reloads, multiple frames)
  if (window.__GASOLINE_ORIGINAL_WS__) return

  const OriginalWS = window.WebSocket

  // Store original for inject script to retrieve
  window.__GASOLINE_ORIGINAL_WS__ = OriginalWS

  // Buffer for early connections
  const earlyConnections: EarlyWsConnection[] = []
  window.__GASOLINE_EARLY_WS__ = earlyConnections

  // Thin wrapper: creates real WebSocket + buffers lifecycle events
  function EarlyWebSocket(this: unknown, url: string | URL, protocols?: string | string[]): WebSocket {
    const ws = protocols !== undefined ? new OriginalWS(url, protocols) : new OriginalWS(url)
    const conn: EarlyWsConnection = { ws, url: url.toString(), createdAt: Date.now(), events: [] }

    ws.addEventListener('open', () => {
      conn.events.push({ type: 'open', ts: Date.now() })
    })
    ws.addEventListener('close', (e: CloseEvent) => {
      conn.events.push({ type: 'close', ts: Date.now(), code: e.code, reason: e.reason })
    })
    ws.addEventListener('error', () => {
      conn.events.push({ type: 'error', ts: Date.now() })
    })

    earlyConnections.push(conn)

    // Cap buffer to bound memory
    if (earlyConnections.length > 50) {
      earlyConnections.shift()
    }

    return ws
  }

  // Preserve prototype chain: instanceof WebSocket still works
  EarlyWebSocket.prototype = OriginalWS.prototype

  // Preserve static constants
  Object.defineProperty(EarlyWebSocket, 'CONNECTING', { value: 0, writable: false })
  Object.defineProperty(EarlyWebSocket, 'OPEN', { value: 1, writable: false })
  Object.defineProperty(EarlyWebSocket, 'CLOSING', { value: 2, writable: false })
  Object.defineProperty(EarlyWebSocket, 'CLOSED', { value: 3, writable: false })

  window.WebSocket = EarlyWebSocket as unknown as typeof WebSocket
})()
