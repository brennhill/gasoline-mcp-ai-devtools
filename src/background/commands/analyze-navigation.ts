// analyze-navigation.ts — Navigation/SPA route discovery command handler (#335).

import { registerCommand } from './registry'

// =============================================================================
// NAVIGATION / SPA ROUTE DISCOVERY (#335)
// =============================================================================

/**
 * Self-contained script injected via chrome.scripting.executeScript to
 * discover navigable links grouped by landmark region.
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
    try {
      return new URL(value || '', window.location.href).href
    } catch {
      return value || ''
    }
  }

  function isSameOrigin(href: string): boolean {
    try {
      return new URL(href).origin === window.location.origin
    } catch {
      return false
    }
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

    // Look for a heading child
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

  interface NavLink {
    text: string
    href: string
    is_internal: boolean
  }

  interface NavRegion {
    tag: string
    role: string
    label: string
    position: string
    links: NavLink[]
  }

  const regionSelectors = [
    'nav',
    '[role="navigation"]',
    'header',
    'footer',
    'aside',
    '[role="banner"]',
    '[role="contentinfo"]',
    '[role="complementary"]'
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

        const text = cleanText(a.textContent || '', 100)
        links.push({
          text,
          href,
          is_internal: isSameOrigin(href)
        })
      }

      if (links.length === 0) continue

      const tag = el.tagName.toLowerCase()
      const role = el.getAttribute('role') || ''
      const label = getRegionLabel(el) || tag
      const position = getPositionLabel(el)

      regions.push({ tag, role, label, position, links })
    }
  }

  // Also collect unregioned top-level links not in any landmark
  const allAnchors = document.querySelectorAll('a[href]')
  const regionedAnchors = new Set<Element>()
  for (const region of seenRegions) {
    for (const a of Array.from(region.querySelectorAll('a[href]'))) {
      regionedAnchors.add(a)
    }
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

    const text = cleanText(a.textContent || '', 100)
    unregionedLinks.push({ text, href, is_internal: isSameOrigin(href) })
  }

  const totalLinks = regions.reduce((sum, r) => sum + r.links.length, 0) + unregionedLinks.length
  const internalLinks = regions.reduce((sum, r) => sum + r.links.filter((l) => l.is_internal).length, 0) +
    unregionedLinks.filter((l) => l.is_internal).length

  return {
    url: window.location.href,
    title: document.title,
    regions,
    unregioned_links: unregionedLinks,
    summary: {
      total_regions: regions.length,
      total_links: totalLinks,
      internal_links: internalLinks,
      external_links: totalLinks - internalLinks
    }
  }
}

registerCommand('navigation', async (ctx) => {
  try {
    const results = await chrome.scripting.executeScript({
      target: { tabId: ctx.tabId },
      world: 'ISOLATED',
      func: navigationDiscoveryScript
    })

    const first = results?.[0]?.result
    const payload = first && typeof first === 'object' ? (first as Record<string, unknown>) : {}

    if (ctx.query.correlation_id) {
      ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, 'complete', payload)
    } else {
      ctx.sendResult(payload)
    }
  } catch (err) {
    const message = (err as Error).message || 'Navigation discovery failed'
    if (ctx.query.correlation_id) {
      ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, 'error', null, message)
    } else {
      ctx.sendResult({
        error: 'navigation_discovery_failed',
        message
      })
    }
  }
})
