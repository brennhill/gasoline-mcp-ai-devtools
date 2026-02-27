// interact-explore.ts — explore_page compound command handler (#338).
// Combines page metadata, interactive elements, readable text, and navigation
// links into a single extension response, reducing MCP round-trips for AI agents.

import { domPrimitiveListInteractive } from '../dom-primitives-list-interactive.js'
import { registerCommand } from './registry.js'

// =============================================================================
// READABLE CONTENT EXTRACTION (self-contained for chrome.scripting.executeScript)
// =============================================================================

/**
 * Self-contained script that extracts readable text content from the page.
 * Mirrors the logic of getReadableScript in tools_interact_content.go but
 * as a function reference suitable for chrome.scripting.executeScript.
 */
function readableContentScript(): Record<string, unknown> {
  function cleanText(el: Element): string {
    if (!el) return ''
    const clone = el.cloneNode(true) as Element
    const removeTags = [
      'nav', 'header', 'footer', 'aside', 'script', 'style', 'noscript', 'svg',
      '[role="navigation"]', '[role="banner"]', '[role="contentinfo"]', '[aria-hidden="true"]',
      '.ad,.ads,.advertisement,.social-share,.comments,.sidebar,.related-posts,.newsletter'
    ]
    for (const sel of removeTags) {
      const els = clone.querySelectorAll(sel)
      for (const child of Array.from(els)) child.remove()
    }
    return ((clone as HTMLElement).innerText || clone.textContent || '').replace(/\s+/g, ' ').trim()
  }

  function findMainContent(): { el: Element; text: string } {
    const candidates = [
      'article', 'main', '[role="main"]', '.post-content', '.entry-content',
      '.article-body', '.article-content', '.story-body', '#content', '.content'
    ]
    for (const sel of candidates) {
      const el = document.querySelector(sel)
      if (el) {
        const text = cleanText(el)
        if (text.length > 100) return { el, text }
      }
    }
    return { el: document.body, text: cleanText(document.body) }
  }

  function getByline(): string {
    const selectors = ['.author', '[rel="author"]', '.byline', '.post-author', 'meta[name="author"]']
    for (const sel of selectors) {
      const el = document.querySelector(sel)
      if (el) {
        const text = (el.getAttribute('content') || (el as HTMLElement).innerText || '').trim()
        if (text.length > 0 && text.length < 200) return text
      }
    }
    return ''
  }

  const found = findMainContent()
  const content = found.text
  const excerpt = content.slice(0, 300)
  const words = content.split(/\s+/).filter(Boolean)

  return {
    title: document.title || '',
    content,
    excerpt,
    byline: getByline(),
    word_count: words.length,
    url: window.location.href
  }
}

// =============================================================================
// NAVIGATION DISCOVERY (self-contained for chrome.scripting.executeScript)
// =============================================================================

/**
 * Self-contained navigation discovery script. Mirrors analyze-navigation.ts
 * navigationDiscoveryScript but as a separate reference for explore_page.
 */
function navigationDiscoveryScript(): Record<string, unknown> {
  const MAX_LINKS_PER_REGION = 50
  const MAX_REGIONS = 20

  function cleanText(value: string, maxLen: number): string {
    const text = (value || '').replace(/[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]/g, '').replace(/\s+/g, ' ').trim()
    if (maxLen > 0 && text.length > maxLen) return text.slice(0, maxLen)
    return text
  }

  function absoluteHref(value: string): string {
    try { return new URL(value || '', window.location.href).href }
    catch { return value || '' }
  }

  function isSameOrigin(href: string): boolean {
    try { return new URL(href).origin === window.location.origin }
    catch { return false }
  }

  function getRegionLabel(el: Element): string {
    const ariaLabel = el.getAttribute('aria-label')
    if (ariaLabel) return cleanText(ariaLabel, 80)
    const ariaLabelledBy = el.getAttribute('aria-labelledby')
    if (ariaLabelledBy) {
      const labelEl = document.getElementById(ariaLabelledBy)
      if (labelEl) {
        const text = cleanText(labelEl.textContent || '', 80)
        if (text) return text
      }
    }
    const heading = el.querySelector('h1, h2, h3, h4, h5, h6')
    if (heading) {
      const text = cleanText(heading.textContent || '', 80)
      if (text) return text
    }
    return ''
  }

  function getPositionLabel(el: Element): string {
    const rect = el.getBoundingClientRect()
    const viewH = window.innerHeight
    if (rect.top < viewH * 0.15) return 'top'
    if (rect.bottom > viewH * 0.85) return 'bottom'
    if (rect.left < window.innerWidth * 0.25 && rect.width < window.innerWidth * 0.35) return 'left_sidebar'
    if (rect.right > window.innerWidth * 0.75 && rect.width < window.innerWidth * 0.35) return 'right_sidebar'
    return 'main'
  }

  interface NavLink { text: string; href: string; is_internal: boolean }
  interface NavRegion { tag: string; role: string; label: string; position: string; links: NavLink[] }

  const regionSelectors = [
    'nav', '[role="navigation"]', 'header', 'footer', 'aside',
    '[role="banner"]', '[role="contentinfo"]', '[role="complementary"]'
  ]

  const seenRegions = new Set<Element>()
  const regions: NavRegion[] = []

  for (const sel of regionSelectors) {
    const elements = document.querySelectorAll(sel)
    for (const el of Array.from(elements)) {
      if (seenRegions.has(el) || regions.length >= MAX_REGIONS) continue
      seenRegions.add(el)
      const anchors = el.querySelectorAll('a[href]')
      if (anchors.length === 0) continue
      const links: NavLink[] = []
      const seenHrefs = new Set<string>()
      for (const a of Array.from(anchors)) {
        if (links.length >= MAX_LINKS_PER_REGION) break
        const rawHref = a.getAttribute('href') || ''
        if (!rawHref || rawHref === '#' || rawHref.startsWith('javascript:')) continue
        const href = absoluteHref(rawHref)
        if (seenHrefs.has(href)) continue
        seenHrefs.add(href)
        links.push({ text: cleanText(a.textContent || '', 100), href, is_internal: isSameOrigin(href) })
      }
      if (links.length === 0) continue
      const tag = el.tagName.toLowerCase()
      const role = el.getAttribute('role') || ''
      const label = getRegionLabel(el) || tag
      const position = getPositionLabel(el)
      regions.push({ tag, role, label, position, links })
    }
  }

  // Unregioned links
  const allAnchors = document.querySelectorAll('a[href]')
  const regionedAnchors = new Set<Element>()
  for (const region of seenRegions) {
    for (const a of Array.from(region.querySelectorAll('a[href]'))) regionedAnchors.add(a)
  }
  const unregionedLinks: NavLink[] = []
  const seenUnregioned = new Set<string>()
  for (const a of Array.from(allAnchors)) {
    if (unregionedLinks.length >= MAX_LINKS_PER_REGION) break
    if (regionedAnchors.has(a)) continue
    const rawHref = a.getAttribute('href') || ''
    if (!rawHref || rawHref === '#' || rawHref.startsWith('javascript:')) continue
    const href = absoluteHref(rawHref)
    if (seenUnregioned.has(href)) continue
    seenUnregioned.add(href)
    unregionedLinks.push({ text: cleanText(a.textContent || '', 100), href, is_internal: isSameOrigin(href) })
  }

  const totalLinks = regions.reduce((sum, r) => sum + r.links.length, 0) + unregionedLinks.length
  const internalLinks = regions.reduce((sum, r) => sum + r.links.filter((l) => l.is_internal).length, 0) +
    unregionedLinks.filter((l) => l.is_internal).length

  return {
    regions,
    unregioned_links: unregionedLinks,
    summary: { total_regions: regions.length, total_links: totalLinks, internal_links: internalLinks, external_links: totalLinks - internalLinks }
  }
}

// =============================================================================
// EXPLORE_PAGE COMMAND (#338)
// =============================================================================

registerCommand('explore_page', async (ctx) => {
  try {
    // If URL is provided, navigate first
    const targetUrl = typeof ctx.params.url === 'string' ? ctx.params.url : undefined
    if (targetUrl) {
      await chrome.tabs.update(ctx.tabId, { url: targetUrl })
      // Wait for page load to complete
      await new Promise<void>((resolve) => {
        const onUpdated = (tabId: number, changeInfo: { status?: string }): void => {
          if (tabId === ctx.tabId && changeInfo.status === 'complete') {
            chrome.tabs.onUpdated.removeListener(onUpdated)
            resolve()
          }
        }
        chrome.tabs.onUpdated.addListener(onUpdated)
        // Safety timeout: resolve after 15s even if load doesn't fire
        setTimeout(() => {
          chrome.tabs.onUpdated.removeListener(onUpdated)
          resolve()
        }, 15000)
      })
    }

    // 1. Get tab info (page metadata)
    const tab = await chrome.tabs.get(ctx.tabId)

    // 2. Run all data collection in parallel
    const [interactiveResults, readableResults, navResults] = await Promise.all([
      // Interactive elements
      chrome.scripting.executeScript({
        target: { tabId: ctx.tabId, allFrames: true },
        world: 'MAIN',
        func: domPrimitiveListInteractive,
        args: ['']
      }).catch(() => []),

      // Readable content
      chrome.scripting.executeScript({
        target: { tabId: ctx.tabId },
        world: 'ISOLATED',
        func: readableContentScript
      }).catch(() => []),

      // Navigation links
      chrome.scripting.executeScript({
        target: { tabId: ctx.tabId },
        world: 'ISOLATED',
        func: navigationDiscoveryScript
      }).catch(() => [])
    ])

    // Process interactive elements (capped at 100)
    const elements: unknown[] = []
    let interactiveError: string | undefined
    for (const r of interactiveResults) {
      const res = r.result as {
        success?: boolean
        elements?: unknown[]
        error?: string
        message?: string
      } | null
      if (res?.success === false) {
        if (!interactiveError) interactiveError = res.error || res.message
        continue
      }
      if (res?.elements) {
        elements.push(...res.elements)
        if (elements.length >= 100) break
      }
    }
    let cappedElements = elements.slice(0, 100)

    // Apply visible_only filter if requested
    if (ctx.params.visible_only === true) {
      cappedElements = cappedElements.filter((el) => {
        const elem = el as { visible?: boolean }
        return elem.visible !== false
      })
    }

    // Apply limit if specified
    const limit = typeof ctx.params.limit === 'number' && ctx.params.limit > 0
      ? ctx.params.limit
      : cappedElements.length
    const finalElements = cappedElements.slice(0, limit)

    // Process readable content
    const readableFirst = readableResults?.[0]?.result
    const readable = readableFirst && typeof readableFirst === 'object'
      ? readableFirst as Record<string, unknown>
      : null

    // Process navigation links
    const navFirst = navResults?.[0]?.result
    const navigation = navFirst && typeof navFirst === 'object'
      ? navFirst as Record<string, unknown>
      : null

    // Build composite payload
    const payload: Record<string, unknown> = {
      // Page metadata
      url: tab.url || '',
      title: tab.title || '',
      tab_status: tab.status || '',
      favicon: tab.favIconUrl || '',
      viewport: {
        width: tab.width,
        height: tab.height
      },

      // Interactive elements
      interactive_elements: finalElements,
      interactive_count: finalElements.length,

      // Readable text
      readable: readable || { error: 'extraction_failed' },

      // Navigation links
      navigation: navigation || { error: 'extraction_failed' }
    }

    if (interactiveError && finalElements.length === 0) {
      payload.interactive_error = interactiveError
    }

    if (ctx.query.correlation_id) {
      ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, 'complete', payload)
    } else {
      ctx.sendResult(payload)
    }
  } catch (err) {
    const message = (err as Error).message || 'Explore page failed'
    if (ctx.query.correlation_id) {
      ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, 'error', null, message)
    } else {
      ctx.sendResult({
        error: 'explore_page_failed',
        message
      })
    }
  }
})
