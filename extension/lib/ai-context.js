/**
 * @fileoverview AI-preprocessed error enrichment pipeline.
 * Parses stack traces, resolves source maps, extracts code snippets,
 * detects UI frameworks (React/Vue/Svelte), captures state snapshots,
 * and generates AI-friendly error summaries. All within a timeout guard.
 */
import {
  AI_CONTEXT_SNIPPET_LINES,
  AI_CONTEXT_MAX_LINE_LENGTH,
  AI_CONTEXT_MAX_SNIPPETS_SIZE,
  AI_CONTEXT_MAX_ANCESTRY_DEPTH,
  AI_CONTEXT_MAX_PROP_KEYS,
  AI_CONTEXT_MAX_STATE_KEYS,
  AI_CONTEXT_MAX_RELEVANT_SLICE,
  AI_CONTEXT_MAX_VALUE_LENGTH,
  AI_CONTEXT_SOURCE_MAP_CACHE_SIZE,
  AI_CONTEXT_PIPELINE_TIMEOUT_MS
} from './constants.js'
// =============================================================================
// MODULE STATE
// =============================================================================
// AI Context state
let aiContextEnabled = true
let aiContextStateSnapshotEnabled = false
const aiSourceMapCache = new Map()
const CHROME_FRAME_RE = /^at\s+(?:(.+?)\s+\()?(.+?):(\d+):(\d+)\)?$/
const FIREFOX_FRAME_RE = /^(.+?)@(.+?):(\d+):(\d+)$/
function parseChromeFrame(line) {
  const m = line.match(CHROME_FRAME_RE)
  if (!m) return null
  const filename = m[2]
  if (!filename || filename.includes('<anonymous>')) return null
  if (!m[3] || !m[4]) return null
  return { functionName: m[1] || null, filename, lineno: parseInt(m[3], 10), colno: parseInt(m[4], 10) }
}
function parseFirefoxFrame(line) {
  const m = line.match(FIREFOX_FRAME_RE)
  if (!m) return null
  const filename = m[2]
  if (!filename || filename.includes('<anonymous>')) return null
  if (!m[3] || !m[4]) return null
  return { functionName: m[1] || null, filename, lineno: parseInt(m[3], 10), colno: parseInt(m[4], 10) }
}
const FRAME_PARSERS = [parseChromeFrame, parseFirefoxFrame]
export function parseStackFrames(stack) {
  if (!stack) return []
  const frames = []
  for (const line of stack.split('\n')) {
    const trimmed = line.trim()
    for (const parser of FRAME_PARSERS) {
      const frame = parser(trimmed)
      if (frame) {
        frames.push(frame)
        break
      }
    }
  }
  return frames
}
// =============================================================================
// SOURCE MAP PARSING
// =============================================================================
/**
 * Parse an inline base64 source map data URL
 * @param dataUrl - The data: URL containing the source map
 * @returns Parsed source map or null
 */
export function parseSourceMap(dataUrl) {
  if (!dataUrl || typeof dataUrl !== 'string') return null
  if (!dataUrl.startsWith('data:')) return null
  try {
    // Extract base64 content after the last comma
    const base64Match = dataUrl.match(/;base64,(.+)$/)
    if (!base64Match || !base64Match[1]) return null
    const decoded = atob(base64Match[1])
    const parsed = JSON.parse(decoded)
    // Only useful if it has sourcesContent
    if (!parsed.sourcesContent || parsed.sourcesContent.length === 0) return null
    return parsed
  } catch {
    return null
  }
}
// =============================================================================
// CODE SNIPPET EXTRACTION
// =============================================================================
/**
 * Extract a code snippet around a given line number
 * @param sourceContent - The full source file content
 * @param line - The 1-based line number of the error
 * @returns Array of { line, text, isError? } or null
 */
export function extractSnippet(sourceContent, line) {
  if (!sourceContent || typeof sourceContent !== 'string') return null
  if (!line || line < 1) return null
  const lines = sourceContent.split('\n')
  if (line > lines.length) return null
  const start = Math.max(0, line - 1 - AI_CONTEXT_SNIPPET_LINES)
  const end = Math.min(lines.length, line + AI_CONTEXT_SNIPPET_LINES)
  const snippet = []
  for (let i = start; i < end; i++) {
    let text = lines[i]
    if (!text) continue
    if (text.length > AI_CONTEXT_MAX_LINE_LENGTH) {
      text = text.slice(0, AI_CONTEXT_MAX_LINE_LENGTH)
    }
    const entry = { line: i + 1, text }
    if (i + 1 === line) entry.isError = true
    snippet.push(entry)
  }
  return snippet
}
/**
 * Extract source snippets for multiple stack frames
 * @param frames - Parsed stack frames
 * @param mockSourceMaps - Map of filename to parsed source map
 * @returns Array of snippet objects
 */
export async function extractSourceSnippets(frames, mockSourceMaps) {
  // SOURCE MAP CACHING STRATEGY:
  // This function works with a mockSourceMaps lookup that is pre-populated by
  // resolveSourceMap(). The caching layer is managed separately via the module-level
  // aiSourceMapCache Map, which stores up to AI_CONTEXT_SOURCE_MAP_CACHE_SIZE entries
  // using LRU eviction. When a source map is needed here, it should already be cached
  // by the MCP observe handler that parsed the HTTP response headers.
  //
  // OPTIMIZATION: We only process the top 3 stack frames to limit computation and avoid
  // redundant snippets. Most stack traces have the root cause in the first 1-3 frames.
  //
  // PARSE ERROR HANDLING: If sourcesContent is missing, we skip the frame entirely
  // rather than erroring. This gracefully handles source maps generated without embedded
  // sources (which only contain mappings, not code). We never throw here.
  //
  // SIZE ENFORCEMENT: Total snippets are capped at AI_CONTEXT_MAX_SNIPPETS_SIZE to prevent
  // bloating the error entry. Each snippet's JSON serialized size is checked before adding.
  // This ensures the enriched error entry stays lightweight for AI processing.
  const snippets = []
  let totalSize = 0
  for (const frame of frames.slice(0, 3)) {
    if (totalSize >= AI_CONTEXT_MAX_SNIPPETS_SIZE) break
    const sourceMap = mockSourceMaps[frame.filename]
    if (!sourceMap || !sourceMap.sourcesContent || !sourceMap.sourcesContent[0]) continue
    const snippet = extractSnippet(sourceMap.sourcesContent[0], frame.lineno)
    if (!snippet) continue
    const snippetObj = { file: frame.filename, line: frame.lineno, snippet }
    const snippetSize = JSON.stringify(snippetObj).length
    if (totalSize + snippetSize > AI_CONTEXT_MAX_SNIPPETS_SIZE) break
    totalSize += snippetSize
    snippets.push(snippetObj)
  }
  return snippets
}
// =============================================================================
// FRAMEWORK DETECTION
// =============================================================================
/**
 * Detect which UI framework an element belongs to
 * @param element - The DOM element (or element-like object)
 * @returns { framework, key? } or null
 */
export function detectFramework(element) {
  if (!element || typeof element !== 'object') return null
  // React: __reactFiber$ or __reactInternalInstance$
  const keys = Object.keys(element)
  const reactKey = keys.find((k) => k.startsWith('__reactFiber$') || k.startsWith('__reactInternalInstance$'))
  if (reactKey) return { framework: 'react', key: reactKey }
  // Vue 3: __vueParentComponent or __vue_app__
  if (element.__vueParentComponent || element.__vue_app__) {
    return { framework: 'vue' }
  }
  // Svelte: __svelte_meta
  if (element.__svelte_meta) {
    return { framework: 'svelte' }
  }
  return null
}
// =============================================================================
// REACT COMPONENT ANCESTRY
// =============================================================================
/**
 * Walk a React fiber tree to extract component ancestry
 * @param fiber - The React fiber node
 * @returns Array of { name, propKeys?, hasState?, stateKeys? } in root-first order
 */
// #lizard forgives
export function getReactComponentAncestry(fiber) {
  if (!fiber) return null
  const ancestry = []
  let current = fiber
  let depth = 0
  while (current && depth < AI_CONTEXT_MAX_ANCESTRY_DEPTH) {
    depth++
    // Only include component fibers (type is function/object), skip host elements (type is string)
    if (current.type && typeof current.type !== 'string') {
      const typeObj = current.type
      const name = typeObj.displayName || typeObj.name || 'Anonymous'
      const entry = { name }
      // Extract prop keys (excluding children)
      if (current.memoizedProps && typeof current.memoizedProps === 'object') {
        entry.propKeys = Object.keys(current.memoizedProps)
          .filter((k) => k !== 'children')
          .slice(0, AI_CONTEXT_MAX_PROP_KEYS)
      }
      // Extract state keys
      if (current.memoizedState && typeof current.memoizedState === 'object' && !Array.isArray(current.memoizedState)) {
        entry.hasState = true
        entry.stateKeys = Object.keys(current.memoizedState).slice(0, AI_CONTEXT_MAX_STATE_KEYS)
      }
      ancestry.push(entry)
    }
    current = current.return
  }
  return ancestry.reverse() // Root-first order
}
// =============================================================================
// STATE SNAPSHOT
// =============================================================================
/**
 * Capture application state snapshot from known store patterns
 * @param errorMessage - The error message for keyword matching
 * @returns State snapshot or null
 */
function classifyValueType(value) {
  if (Array.isArray(value)) return 'array'
  if (value === null) return 'null'
  return typeof value
}
const RELEVANT_STATE_KEYS = ['error', 'loading', 'status', 'failed']
// #lizard forgives
function buildRelevantSlice(state, errorWords) {
  const relevantSlice = {}
  let sliceCount = 0
  for (const [key, value] of Object.entries(state)) {
    if (sliceCount >= AI_CONTEXT_MAX_RELEVANT_SLICE) break
    if (typeof value !== 'object' || value === null || Array.isArray(value)) continue
    for (const [subKey, subValue] of Object.entries(value)) {
      if (sliceCount >= AI_CONTEXT_MAX_RELEVANT_SLICE) break
      const isRelevantKey = RELEVANT_STATE_KEYS.some((k) => subKey.toLowerCase().includes(k))
      const isKeywordMatch = errorWords.some((w) => key.toLowerCase().includes(w))
      if (!isRelevantKey && !isKeywordMatch) continue
      let val = subValue
      if (typeof val === 'string' && val.length > AI_CONTEXT_MAX_VALUE_LENGTH) {
        val = val.slice(0, AI_CONTEXT_MAX_VALUE_LENGTH)
      }
      relevantSlice[`${key}.${subKey}`] = val
      sliceCount++
    }
  }
  return relevantSlice
}
/**
 * Capture application state snapshot from known store patterns.
 *
 * STATE RELEVANCE MATCHING STRATEGY:
 * 1. Extract error keywords from the error message (words > 2 chars).
 * 2. Build a "relevant slice" by matching nested state keys against common error state
 *    keys ('error', 'loading', 'status', 'failed') and error message keywords.
 * 3. Caps at AI_CONTEXT_MAX_RELEVANT_SLICE entries; values truncated at MAX_VALUE_LENGTH.
 *
 * NOTE: Only supports Redux. Other state management would need additional window.__* patterns.
 */
export function captureStateSnapshot(errorMessage) {
  if (typeof window === 'undefined') return null
  try {
    const store = window.__REDUX_STORE__
    if (!store || typeof store.getState !== 'function') return null
    const state = store.getState()
    if (!state || typeof state !== 'object') return null
    const keys = {}
    for (const [key, value] of Object.entries(state)) {
      keys[key] = { type: classifyValueType(value) }
    }
    const errorWords = (errorMessage || '')
      .toLowerCase()
      .split(/\W+/)
      .filter((w) => w.length > 2)
    const relevantSlice = buildRelevantSlice(state, errorWords)
    return { source: 'redux', keys, relevantSlice }
  } catch {
    return null
  }
}
// =============================================================================
// AI SUMMARY GENERATION
// =============================================================================
/**
 * Generate a template-based AI summary from enrichment data
 * @param data - { errorType, message, file, line, componentAncestry, stateSnapshot }
 * @returns Summary string
 */
export function generateAiSummary(data) {
  const parts = []
  // Error type and location
  if (data.file && data.line) {
    parts.push(`${data.errorType} in ${data.file}:${data.line} — ${data.message}`)
  } else {
    parts.push(`${data.errorType}: ${data.message}`)
  }
  // Component context
  if (data.componentAncestry && data.componentAncestry.components) {
    const path = data.componentAncestry.components.map((c) => c.name).join(' > ')
    parts.push(`Component tree: ${path}.`)
  }
  // State context
  if (data.stateSnapshot && data.stateSnapshot.relevantSlice) {
    const sliceKeys = Object.keys(data.stateSnapshot.relevantSlice)
    if (sliceKeys.length > 0) {
      const stateInfo = sliceKeys.map((k) => `${k}=${JSON.stringify(data.stateSnapshot.relevantSlice[k])}`).join(', ') // nosemgrep: no-stringify-keys
      parts.push(`State: ${stateInfo}.`)
    }
  }
  return parts.join(' ')
}
/**
 * Full error enrichment pipeline
 * @param error - The error entry to enrich
 * @returns The enriched error entry
 */
// #lizard forgives
async function buildAiContext(error) {
  const result = {}
  const frames = parseStackFrames(error.stack)
  if (frames.length === 0) return { summary: error.message || 'Unknown error' }
  const topFrame = frames[0]
  // Source snippets (from cache)
  if (topFrame) {
    const cached = getSourceMapCache(topFrame.filename)
    if (cached) {
      const snippets = await extractSourceSnippets(frames, { [topFrame.filename]: cached })
      if (snippets.length > 0) result.sourceSnippets = snippets
    }
  }
  // Component ancestry from activeElement
  result.componentAncestry = extractComponentAncestry() || undefined
  // State snapshot (if enabled)
  if (aiContextStateSnapshotEnabled) {
    const snapshot = captureStateSnapshot(error.message || '')
    if (snapshot) result.stateSnapshot = snapshot
  }
  result.summary = generateAiSummary({
    errorType: error.message?.split(':')[0] || 'Error',
    message: error.message || '',
    file: topFrame?.filename || null,
    line: topFrame?.lineno || null,
    componentAncestry: result.componentAncestry || null,
    stateSnapshot: result.stateSnapshot || null
  })
  return result
}
function extractComponentAncestry() {
  if (typeof document === 'undefined' || !document.activeElement) return null
  const framework = detectFramework(document.activeElement)
  if (!framework || framework.framework !== 'react' || !framework.key) return null
  const fiber = document.activeElement[framework.key]
  const components = getReactComponentAncestry(fiber)
  if (!components || components.length === 0) return null
  return { framework: 'react', components }
}
function applyAiContext(enriched, context) {
  enriched._aiContext = context
  if (!enriched._enrichments) enriched._enrichments = []
  enriched._enrichments.push('aiContext')
}
export async function enrichErrorWithAiContext(error) {
  if (!aiContextEnabled) return error
  const enriched = { ...error }
  try {
    const context = await Promise.race([
      buildAiContext(error),
      new Promise((resolve) => {
        setTimeout(() => resolve({ summary: `${error.message || 'Error'}` }), AI_CONTEXT_PIPELINE_TIMEOUT_MS)
      })
    ])
    applyAiContext(enriched, context)
  } catch {
    applyAiContext(enriched, { summary: error.message || 'Unknown error' })
  }
  return enriched
}
// =============================================================================
// CONFIGURATION
// =============================================================================
/**
 * Enable or disable AI context enrichment
 * @param enabled
 */
export function setAiContextEnabled(enabled) {
  aiContextEnabled = enabled
}
/**
 * Enable or disable state snapshot in AI context
 * @param enabled
 */
export function setAiContextStateSnapshot(enabled) {
  aiContextStateSnapshotEnabled = enabled
}
// =============================================================================
// SOURCE MAP CACHE
// =============================================================================
/**
 * Cache a parsed source map for a URL
 * @param url - The script URL
 * @param map - The parsed source map
 */
export function setSourceMapCache(url, map) {
  // Evict oldest if adding new entry and at capacity
  if (!aiSourceMapCache.has(url) && aiSourceMapCache.size >= AI_CONTEXT_SOURCE_MAP_CACHE_SIZE) {
    const firstKey = aiSourceMapCache.keys().next().value
    if (firstKey) {
      aiSourceMapCache.delete(firstKey)
    }
  }
  // Move to end (LRU): delete first if exists, then add
  // This ensures recently accessed/updated entries are kept longest
  aiSourceMapCache.delete(url)
  aiSourceMapCache.set(url, map)
}
/**
 * Get a cached source map
 * @param url - The script URL
 * @returns The cached source map or null
 */
export function getSourceMapCache(url) {
  return aiSourceMapCache.get(url) || null
}
/**
 * Get the number of cached source maps
 * @returns
 */
export function getSourceMapCacheSize() {
  return aiSourceMapCache.size
}
/**
 * Reset all module state for testing purposes
 * Clears source map cache and restores default settings.
 * Call this in beforeEach/afterEach test hooks to prevent test pollution.
 */
export function resetForTesting() {
  aiContextEnabled = true
  aiContextStateSnapshotEnabled = false
  aiSourceMapCache.clear()
}
//# sourceMappingURL=ai-context.js.map
