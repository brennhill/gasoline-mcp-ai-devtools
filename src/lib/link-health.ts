/**
 * @fileoverview Link Health Checker
 * Extracts all links from the current page and checks their health.
 * Categorizes issues as: ok (2xx), redirect (3xx), requires_auth (401/403),
 * broken (4xx/5xx), or timeout.
 */

export interface LinkHealthParams {
  readonly timeout_ms?: number
  readonly max_workers?: number
}

export interface LinkCheckResult {
  readonly url: string
  readonly status: number | null
  readonly code: 'ok' | 'redirect' | 'requires_auth' | 'broken' | 'timeout' | 'cors_blocked'
  readonly timeMs: number
  readonly isExternal: boolean
  readonly redirectTo?: string
  readonly error?: string
  readonly needsServerVerification?: boolean
}

export interface LinkHealthCheckResult {
  readonly summary: {
    readonly totalLinks: number
    readonly ok: number
    readonly redirect: number
    readonly requiresAuth: number
    readonly broken: number
    readonly timeout: number
    readonly corsBlocked: number
    readonly needsServerVerification: number
  }
  readonly results: LinkCheckResult[]
}

/**
 * Check all links on the current page for health issues.
 * Extracts links, deduplicates, and checks max 20 concurrently.
 */
function extractUniqueLinks(): string[] {
  const linkElements = document.querySelectorAll('a[href]')
  const urls = new Set<string>()
  for (const elem of linkElements) {
    const href = (elem as HTMLAnchorElement).href
    if (href && !isIgnoredLink(href)) urls.add(href)
  }
  return Array.from(urls)
}

function aggregateResults(results: LinkCheckResult[]): LinkHealthCheckResult['summary'] {
  const summary = { totalLinks: results.length, ok: 0, redirect: 0, requiresAuth: 0, broken: 0, timeout: 0, corsBlocked: 0, needsServerVerification: 0 }
  const codeToField: Record<string, keyof typeof summary> = {
    ok: 'ok', redirect: 'redirect', requires_auth: 'requiresAuth',
    broken: 'broken', timeout: 'timeout', cors_blocked: 'corsBlocked'
  }
  for (const result of results) {
    const field = codeToField[result.code]
    if (field) summary[field]++
    if (result.code === 'cors_blocked' && result.needsServerVerification) summary.needsServerVerification++
  }
  return summary
}

export async function checkLinkHealth(params: LinkHealthParams): Promise<LinkHealthCheckResult> {
  const timeout_ms = params.timeout_ms || 15000
  const max_workers = params.max_workers || 20
  const uniqueLinks = extractUniqueLinks()

  const results: LinkCheckResult[] = []
  const chunks = chunkArray(uniqueLinks, max_workers)
  for (const chunk of chunks) {
    const batchResults = await Promise.allSettled(chunk.map((url) => checkLink(url, timeout_ms)))
    for (const result of batchResults) {
      if (result.status === 'fulfilled' && result.value) results.push(result.value)
    }
  }

  return { summary: aggregateResults(results), results }
}

/**
 * Check a single link for health issues.
 */
async function checkLink(url: string, timeout_ms: number): Promise<LinkCheckResult> {
  const startTime = performance.now()
  const isExternal = new URL(url).origin !== window.location.origin

  try {
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), timeout_ms)

    try {
      const response = await fetch(url, {
        method: 'HEAD',
        signal: controller.signal,
        redirect: 'follow'
      })

      clearTimeout(timeoutId)
      const timeMs = Math.round(performance.now() - startTime)

      // Check if response is CORS-blocked (opaque response with status 0)
      // CORS-blocked responses have status 0 and unreadable headers
      if (response.status === 0) {
        return {
          url,
          status: null,
          code: 'cors_blocked',
          timeMs,
          isExternal,
          error: 'CORS policy blocked the request',
          needsServerVerification: isExternal // Only external links need server verification
        }
      }

      // Categorize by status
      let code: LinkCheckResult['code']
      if (response.status >= 200 && response.status < 300) {
        code = 'ok'
      } else if (response.status >= 300 && response.status < 400) {
        code = 'redirect'
      } else if (response.status === 401 || response.status === 403) {
        code = 'requires_auth'
      } else if (response.status >= 400) {
        code = 'broken'
      } else {
        code = 'broken'
      }

      return {
        url,
        status: response.status,
        code,
        timeMs,
        isExternal,
        redirectTo: response.redirected ? response.url : undefined
      }
    } finally {
      clearTimeout(timeoutId)
    }
  } catch (error) {
    const timeMs = Math.round(performance.now() - startTime)
    const isTimeout = (error as Error).name === 'AbortError'

    return {
      url,
      status: null,
      code: isTimeout ? 'timeout' : 'broken',
      timeMs,
      isExternal,
      error: isTimeout ? 'timeout' : (error as Error).message
    }
  }
}

/**
 * Determine if a link should be skipped (javascript:, mailto:, #anchor).
 */
function isIgnoredLink(href: string): boolean {
  if (href.startsWith('javascript:')) return true
  if (href.startsWith('mailto:')) return true
  if (href.startsWith('tel:')) return true
  if (href.startsWith('#')) return true
  if (href === '') return true
  return false
}

/**
 * Split array into chunks of specified size.
 */
function chunkArray<T>(arr: T[], chunkSize: number): T[][] {
  const chunks: T[][] = []
  for (let i = 0; i < arr.length; i += chunkSize) {
    chunks.push(arr.slice(i, i + chunkSize))
  }
  return chunks
}
