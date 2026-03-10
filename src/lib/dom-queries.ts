/**
 * Purpose: Structured DOM querying and page info extraction for the inject context.
 * Docs: docs/features/feature/query-dom/index.md
 */

/**
 * @fileoverview On-demand DOM queries.
 * Provides structured DOM querying and page info extraction.
 * Accessibility auditing lives in a11y-audit.ts.
 */

import {
  DOM_QUERY_MAX_ELEMENTS,
  DOM_QUERY_MAX_TEXT,
  DOM_QUERY_MAX_DEPTH
} from './constants.js'

// Re-export accessibility audit functions for backward compatibility
export { runAxeAudit, runAxeAuditWithTimeout, formatAxeResults } from './a11y-audit.js'

// Re-export page info extraction (split into its own module for coherence)
export { getPageInfo } from './page-info.js'

// DOM query parameters
export interface DOMQueryParams {
  selector: string
  include_styles?: boolean
  properties?: string[]
  include_children?: boolean
  max_depth?: number
}

// Bounding box type
interface BoundingBox {
  x: number
  y: number
  width: number
  height: number
}

// Serialized DOM element entry
interface DOMElementEntry {
  tag: string
  text: string
  visible: boolean
  attributes?: Record<string, string>
  boundingBox?: BoundingBox
  styles?: Record<string, string>
  children?: DOMElementEntry[]
}

// DOM query result
interface DOMQueryResult {
  url: string
  title: string
  matchCount: number
  returnedCount: number
  matches: DOMElementEntry[]
}

/**
 * Execute a DOM query and return structured results
 */
export async function executeDOMQuery(params: DOMQueryParams): Promise<DOMQueryResult> {
  const { selector, include_styles, properties, include_children, max_depth } = params

  const elements = document.querySelectorAll(selector)
  const matchCount = elements.length
  const cappedDepth = Math.min(max_depth || 3, DOM_QUERY_MAX_DEPTH)

  const matches: DOMElementEntry[] = []
  for (let i = 0; i < Math.min(elements.length, DOM_QUERY_MAX_ELEMENTS); i++) {
    const el = elements[i]
    if (!el) continue
    const entry = serializeDOMElement(el, include_styles, properties, include_children, cappedDepth, 0)
    matches.push(entry)
  }

  return {
    url: window.location.href,
    title: document.title,
    matchCount,
    returnedCount: matches.length,
    matches
  }
}

/**
 * Collect all attributes from an element into a plain object.
 */
function collectAttributes(el: Element): Record<string, string> | undefined {
  if (!el.attributes || el.attributes.length === 0) return undefined
  const attrs: Record<string, string> = {}
  for (const attr of el.attributes) {
    attrs[attr.name] = attr.value
  }
  return attrs
}

/**
 * Get the bounding box of an element, or undefined if unavailable.
 */
function collectBoundingBox(el: Element): BoundingBox | undefined {
  if (!el.getBoundingClientRect) return undefined
  const rect = el.getBoundingClientRect()
  return { x: rect.x, y: rect.y, width: rect.width, height: rect.height }
}

/**
 * Get computed styles for an element, either specific properties or defaults.
 */
function collectStyles(
  el: Element,
  includeStyles: boolean | undefined,
  styleProps: string[] | undefined
): Record<string, string> | undefined {
  if (!includeStyles || typeof window.getComputedStyle !== 'function') return undefined
  const computed = window.getComputedStyle(el)
  if (styleProps && styleProps.length > 0) {
    const styles: Record<string, string> = {}
    for (const prop of styleProps) {
      styles[prop] = computed.getPropertyValue(prop)
    }
    return styles
  }
  return { display: computed.display, color: computed.color, position: computed.position }
}

/**
 * Serialize child elements recursively up to maxDepth.
 */
// #lizard forgives
function collectChildren(
  el: Element,
  includeChildren: boolean | undefined,
  maxDepth: number,
  currentDepth: number
): DOMElementEntry[] | undefined {
  if (!includeChildren || currentDepth >= maxDepth || !el.children || el.children.length === 0) return undefined
  const children: DOMElementEntry[] = []
  const maxChildren = Math.min(el.children.length, DOM_QUERY_MAX_ELEMENTS)
  for (let i = 0; i < maxChildren; i++) {
    const child = el.children[i]
    if (child) {
      children.push(serializeDOMElement(child, false, undefined, true, maxDepth, currentDepth + 1))
    }
  }
  return children
}

/**
 * Serialize a DOM element to a plain object
 */
function serializeDOMElement(
  el: Element,
  includeStyles: boolean | undefined,
  styleProps: string[] | undefined,
  includeChildren: boolean | undefined,
  maxDepth: number,
  currentDepth: number
): DOMElementEntry {
  const entry: DOMElementEntry = {
    tag: el.tagName ? el.tagName.toLowerCase() : '',
    text: (el.textContent || '').slice(0, DOM_QUERY_MAX_TEXT),
    visible:
      (el as HTMLElement).offsetParent !== null || (el.getBoundingClientRect && el.getBoundingClientRect().width > 0)
  }

  entry.attributes = collectAttributes(el)
  entry.boundingBox = collectBoundingBox(el)
  entry.styles = collectStyles(el, includeStyles, styleProps)
  entry.children = collectChildren(el, includeChildren, maxDepth, currentDepth)

  return entry
}


