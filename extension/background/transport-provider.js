/**
 * @fileoverview Extension transport provider abstraction.
 * Centralizes all extension->server HTTP I/O so transport can be swapped.
 */

function getExtensionVersion() {
  try {
    return chrome?.runtime?.getManifest?.().version || '0.0.0'
  } catch {
    return '0.0.0'
  }
}

/**
 * Build request headers with client version metadata.
 */
export function getRequestHeaders(
  additionalHeaders = {},
  extensionVersion = getExtensionVersion()
) {
  return {
    'Content-Type': 'application/json',
    'X-Gasoline-Client': `gasoline-extension/${extensionVersion}`,
    'X-Gasoline-Extension-Version': extensionVersion,
    ...additionalHeaders
  }
}

/**
 * HTTP implementation of extension transport provider.
 */
export class HTTPExtensionTransportProvider {
  constructor(endpoint, fetchImpl = (...args) => globalThis.fetch(...args)) {
    this.endpoint = endpoint
    this.fetchImpl = fetchImpl
  }

  id() {
    return 'http'
  }

  Endpoint() {
    return this.endpoint
  }

  setEndpoint(endpoint) {
    this.endpoint = endpoint
  }

  async sendSync(request, extensionVersion = getExtensionVersion()) {
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), 8000)
    try {
      const response = await this.fetchImpl(`${this.endpoint}/sync`, {
        method: 'POST',
        headers: getRequestHeaders({}, extensionVersion),
        body: JSON.stringify(request),
        signal: controller.signal
      })
      if (!response.ok) {
        throw new Error(
          `Sync request failed: HTTP ${response.status} ${response.statusText} from ${this.endpoint}/sync`
        )
      }
      return await response.json()
    } finally {
      clearTimeout(timeoutId)
    }
  }

  async postLogs(entries, extensionVersion = getExtensionVersion(), debugLogFn) {
    if (debugLogFn) debugLogFn('connection', `Sending ${entries.length} entries to server`)
    const response = await this.fetchImpl(`${this.endpoint}/logs`, {
      method: 'POST',
      headers: getRequestHeaders({}, extensionVersion),
      body: JSON.stringify({ entries })
    })
    if (!response.ok) {
      const error = `Server error: ${response.status} ${response.statusText}`
      if (debugLogFn) debugLogFn('error', error)
      throw new Error(error)
    }
    const result = await response.json()
    if (debugLogFn) debugLogFn('connection', `Server accepted entries, total: ${result.entries}`)
    return result
  }

  async postWebSocketEvents(events, extensionVersion = getExtensionVersion(), debugLogFn) {
    if (debugLogFn) debugLogFn('connection', `Sending ${events.length} WS events to server`)
    const response = await this.fetchImpl(`${this.endpoint}/websocket-events`, {
      method: 'POST',
      headers: getRequestHeaders({}, extensionVersion),
      body: JSON.stringify({ events })
    })
    if (!response.ok) {
      const error = `Server error (WS): ${response.status} ${response.statusText}`
      if (debugLogFn) debugLogFn('error', error)
      throw new Error(error)
    }
    if (debugLogFn) debugLogFn('connection', `Server accepted ${events.length} WS events`)
  }

  async postNetworkBodies(bodies, extensionVersion = getExtensionVersion(), debugLogFn) {
    if (debugLogFn) debugLogFn('connection', `Sending ${bodies.length} network bodies to server`)
    const response = await this.fetchImpl(`${this.endpoint}/network-bodies`, {
      method: 'POST',
      headers: getRequestHeaders({}, extensionVersion),
      body: JSON.stringify({ bodies })
    })
    if (!response.ok) {
      const error = `Server error (network bodies): ${response.status} ${response.statusText}`
      if (debugLogFn) debugLogFn('error', error)
      throw new Error(error)
    }
    if (debugLogFn) debugLogFn('connection', `Server accepted ${bodies.length} network bodies`)
  }

  async postNetworkWaterfall(payload, extensionVersion = getExtensionVersion(), debugLogFn) {
    if (debugLogFn) debugLogFn('connection', `Sending ${payload.entries.length} waterfall entries to server`)
    const response = await this.fetchImpl(`${this.endpoint}/network-waterfall`, {
      method: 'POST',
      headers: getRequestHeaders({}, extensionVersion),
      body: JSON.stringify(payload)
    })
    if (!response.ok) {
      const error = `Server error (network waterfall): ${response.status} ${response.statusText}`
      if (debugLogFn) debugLogFn('error', error)
      throw new Error(error)
    }
    if (debugLogFn) debugLogFn('connection', `Server accepted ${payload.entries.length} waterfall entries`)
  }

  async postEnhancedActions(actions, extensionVersion = getExtensionVersion(), debugLogFn) {
    if (debugLogFn) debugLogFn('connection', `Sending ${actions.length} enhanced actions to server`)
    const response = await this.fetchImpl(`${this.endpoint}/enhanced-actions`, {
      method: 'POST',
      headers: getRequestHeaders({}, extensionVersion),
      body: JSON.stringify({ actions })
    })
    if (!response.ok) {
      const error = `Server error (enhanced actions): ${response.status} ${response.statusText}`
      if (debugLogFn) debugLogFn('error', error)
      throw new Error(error)
    }
    if (debugLogFn) debugLogFn('connection', `Server accepted ${actions.length} enhanced actions`)
  }

  async postPerformanceSnapshots(snapshots, extensionVersion = getExtensionVersion(), debugLogFn) {
    if (debugLogFn) debugLogFn('connection', `Sending ${snapshots.length} performance snapshots to server`)
    const response = await this.fetchImpl(`${this.endpoint}/performance-snapshots`, {
      method: 'POST',
      headers: getRequestHeaders({}, extensionVersion),
      body: JSON.stringify({ snapshots })
    })
    if (!response.ok) {
      const error = `Server error (performance snapshots): ${response.status} ${response.statusText}`
      if (debugLogFn) debugLogFn('error', error)
      throw new Error(error)
    }
    if (debugLogFn) debugLogFn('connection', `Server accepted ${snapshots.length} performance snapshots`)
  }

  async checkHealth(extensionVersion = getExtensionVersion()) {
    try {
      const response = await this.fetchImpl(`${this.endpoint}/health`, {
        headers: getRequestHeaders({}, extensionVersion)
      })
      if (!response.ok) {
        return { connected: false, error: `HTTP ${response.status}` }
      }
      let data
      try {
        data = await response.json()
      } catch {
        return {
          connected: false,
          error: 'Server returned invalid response - check Server URL in options'
        }
      }
      return { ...data, connected: true }
    } catch (error) {
      return { connected: false, error: error.message }
    }
  }

  async postQueryResult(
    queryId,
    typeOrResult,
    maybeResult,
    extensionVersion = getExtensionVersion(),
    debugLogFn
  ) {
    const hasType = typeof maybeResult !== 'undefined'
    const type = hasType ? typeOrResult : 'unknown'
    const result = hasType ? maybeResult : typeOrResult
    const endpoint = '/query-result'

    const logData = { queryId, type, endpoint, resultSize: JSON.stringify(result).length }
    if (debugLogFn) debugLogFn('api', `POST ${endpoint}`, logData)
    console.log(`[Gasoline API] POST ${endpoint}`, logData) // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- internal debug output

    try {
      const response = await this.fetchImpl(`${this.endpoint}${endpoint}`, {
        method: 'POST',
        headers: getRequestHeaders({}, extensionVersion),
        body: JSON.stringify({ id: queryId, result })
      })
      if (!response.ok) {
        const errMsg = `Failed to post query result: HTTP ${response.status}`
        if (debugLogFn) debugLogFn('api', errMsg, { queryId, type, endpoint })
        console.error(`[Gasoline API] ${errMsg}`, { queryId, type, endpoint }) // nosemgrep
      } else {
        if (debugLogFn) debugLogFn('api', `POST ${endpoint} success`, { queryId })
        console.log(`[Gasoline API] POST ${endpoint} success`, { queryId }) // nosemgrep
      }
    } catch (err) {
      const errMsg = err.message
      if (debugLogFn) debugLogFn('api', `POST ${endpoint} error: ${errMsg}`, { queryId, type })
      console.error('[Gasoline API] Error posting query result:', { queryId, type, endpoint, error: errMsg })
    }
  }

  async postAsyncCommandResult(
    correlationId,
    status,
    result = null,
    error = null,
    extensionVersion = getExtensionVersion(),
    debugLogFn
  ) {
    const payload = { correlation_id: correlationId, status }
    if (result !== null) payload.result = result
    if (error !== null) payload.error = error

    try {
      const response = await this.fetchImpl(`${this.endpoint}/query-result`, {
        method: 'POST',
        headers: getRequestHeaders({}, extensionVersion),
        body: JSON.stringify(payload)
      })
      if (!response.ok) {
        console.error(`[Gasoline] Failed to post async command result: HTTP ${response.status}`, {
          correlationId,
          status
        }) // nosemgrep
      }
    } catch (err) {
      console.error('[Gasoline] Error posting async command result:', {
        correlationId,
        status,
        error: err.message
      })
      if (debugLogFn) {
        debugLogFn('connection', 'Failed to post async command result', {
          correlationId,
          status,
          error: err.message
        })
      }
    }
  }

  async postExtensionLogs(logs, extensionVersion = getExtensionVersion()) {
    if (logs.length === 0) return
    try {
      const response = await this.fetchImpl(`${this.endpoint}/extension-logs`, {
        method: 'POST',
        headers: getRequestHeaders({}, extensionVersion),
        body: JSON.stringify({ logs })
      })
      if (!response.ok) {
        console.error(`[Gasoline] Failed to post extension logs: HTTP ${response.status}`, { count: logs.length }) // nosemgrep
      }
    } catch (err) {
      console.error('[Gasoline] Error posting extension logs:', { count: logs.length, error: err.message })
    }
  }

  async postStatusPing(statusMessage, extensionVersion = getExtensionVersion(), diagnosticLogFn) {
    try {
      const response = await this.fetchImpl(`${this.endpoint}/api/extension-status`, {
        method: 'POST',
        headers: getRequestHeaders({}, extensionVersion),
        body: JSON.stringify(statusMessage)
      })
      if (!response.ok) {
        console.error(`[Gasoline] Failed to send status ping: HTTP ${response.status}`, { type: statusMessage.type }) // nosemgrep
      }
    } catch (err) {
      console.error('[Gasoline] Error sending status ping:', { type: statusMessage.type, error: err.message })
      if (diagnosticLogFn) diagnosticLogFn('[Gasoline] Status ping error: ' + err.message)
    }
  }

  async pollPendingQueries(
    extSessionId,
    pilotState,
    extensionVersion = getExtensionVersion(),
    diagnosticLogFn,
    debugLogFn
  ) {
    try {
      if (diagnosticLogFn) diagnosticLogFn(`[Diagnostic] Poll request: header=${pilotState}`)
      const response = await this.fetchImpl(`${this.endpoint}/pending-queries`, {
        headers: getRequestHeaders(
          { 'X-Gasoline-Ext-Session': extSessionId, 'X-Gasoline-Pilot': pilotState },
          extensionVersion
        )
      })
      if (!response.ok) {
        if (debugLogFn) debugLogFn('connection', 'Poll pending-queries failed', { status: response.status })
        return []
      }
      const data = await response.json()
      if (!data.queries || data.queries.length === 0) return []
      if (debugLogFn) debugLogFn('connection', 'Got pending queries', { count: data.queries.length })
      return data.queries
    } catch (err) {
      if (debugLogFn) debugLogFn('connection', 'Poll pending-queries error', { error: err.message })
      return []
    }
  }

  async postScreenshot(payload, extensionVersion = getExtensionVersion()) {
    const response = await this.fetchImpl(`${this.endpoint}/screenshots`, {
      method: 'POST',
      headers: getRequestHeaders({}, extensionVersion),
      body: JSON.stringify(payload)
    })
    if (!response.ok) {
      throw new Error(`Failed to upload screenshot: server returned HTTP ${response.status} ${response.statusText}`)
    }
    return await response.json()
  }

  async postDrawModeCompletion(payload, extensionVersion = getExtensionVersion()) {
    return this.fetchImpl(`${this.endpoint}/draw-mode/complete`, {
      method: 'POST',
      headers: getRequestHeaders({}, extensionVersion),
      body: JSON.stringify(payload)
    })
  }

  async readFile(payload, extensionVersion = getExtensionVersion(), timeoutMs = 0) {
    const controller = new AbortController()
    const timeoutId =
      timeoutMs > 0 ? setTimeout(() => controller.abort(), timeoutMs) : null

    try {
      const response = await this.fetchImpl(`${this.endpoint}/api/file/read`, {
        method: 'POST',
        headers: getRequestHeaders({}, extensionVersion),
        body: JSON.stringify(payload),
        signal: controller.signal
      })
      if (!response.ok) {
        const err = new Error(`HTTP ${response.status}`)
        err.status = response.status
        throw err
      }
      return await response.json()
    } finally {
      if (timeoutId) clearTimeout(timeoutId)
    }
  }

  async osAutomationInject(payload, extensionVersion = getExtensionVersion(), timeoutMs = 0) {
    const controller = new AbortController()
    const timeoutId =
      timeoutMs > 0 ? setTimeout(() => controller.abort(), timeoutMs) : null

    try {
      const response = await this.fetchImpl(`${this.endpoint}/api/os-automation/inject`, {
        method: 'POST',
        headers: getRequestHeaders({}, extensionVersion),
        body: JSON.stringify(payload),
        signal: controller.signal
      })
      let data = null
      try {
        data = await response.json()
      } catch {
        data = null
      }
      return { ok: response.ok, status: response.status, data }
    } finally {
      if (timeoutId) clearTimeout(timeoutId)
    }
  }

  async osAutomationDismiss(extensionVersion = getExtensionVersion(), timeoutMs = 0) {
    const controller = new AbortController()
    const timeoutId =
      timeoutMs > 0 ? setTimeout(() => controller.abort(), timeoutMs) : null
    try {
      return await this.fetchImpl(`${this.endpoint}/api/os-automation/dismiss`, {
        method: 'POST',
        headers: getRequestHeaders({}, extensionVersion),
        signal: controller.signal
      })
    } finally {
      if (timeoutId) clearTimeout(timeoutId)
    }
  }
}

export function createHTTPExtensionTransportProvider(
  endpoint,
  fetchImpl = (...args) => globalThis.fetch(...args)
) {
  return new HTTPExtensionTransportProvider(endpoint, fetchImpl)
}

let activeTransportProvider = null

export function setTransportProvider(provider) {
  activeTransportProvider = provider
}

export function resetTransportProvider() {
  activeTransportProvider = null
}

export function getOrCreateTransportProvider(endpoint) {
  if (activeTransportProvider) {
    if (endpoint && typeof activeTransportProvider.setEndpoint === 'function') {
      activeTransportProvider.setEndpoint(endpoint)
    }
    return activeTransportProvider
  }
  activeTransportProvider = createHTTPExtensionTransportProvider(endpoint)
  return activeTransportProvider
}
