// @ts-nocheck
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
  AI_CONTEXT_PIPELINE_TIMEOUT_MS,
} from './constants.js'

// AI Context state
let aiContextEnabled = true
let aiContextStateSnapshotEnabled = false
const aiSourceMapCache = new Map()

/**
 * Parse stack trace into structured frames
 * Supports Chrome and Firefox formats
 * @param {string} stack - The stack trace string
 * @returns {Array} Array of frame objects { functionName, filename, lineno, colno }
 */
export function parseStackFrames(stack) {
  if (!stack) return []

  const frames = []
  const lines = stack.split('\n')

  for (const line of lines) {
    const trimmed = line.trim()

    // Chrome format: "    at functionName (url:line:col)"
    // or "    at url:line:col"
    const chromeMatch = trimmed.match(/^at\s+(?:(.+?)\s+\()?(.+?):(\d+):(\d+)\)?$/)
    if (chromeMatch) {
      const filename = chromeMatch[2]
      if (filename.includes('<anonymous>')) continue
      frames.push({
        functionName: chromeMatch[1] || null,
        filename,
        lineno: parseInt(chromeMatch[3], 10),
        colno: parseInt(chromeMatch[4], 10),
      })
      continue
    }

    // Firefox format: "functionName@url:line:col"
    const firefoxMatch = trimmed.match(/^(.+?)@(.+?):(\d+):(\d+)$/)
    if (firefoxMatch) {
      const filename = firefoxMatch[2]
      if (filename.includes('<anonymous>')) continue
      frames.push({
        functionName: firefoxMatch[1] || null,
        filename,
        lineno: parseInt(firefoxMatch[3], 10),
        colno: parseInt(firefoxMatch[4], 10),
      })
      continue
    }
  }

  return frames
}

/**
 * Parse an inline base64 source map data URL
 * @param {string} dataUrl - The data: URL containing the source map
 * @returns {Object|null} Parsed source map or null
 */
export function parseSourceMap(dataUrl) {
  if (!dataUrl || typeof dataUrl !== 'string') return null
  if (!dataUrl.startsWith('data:')) return null

  try {
    // Extract base64 content after the last comma
    const base64Match = dataUrl.match(/;base64,(.+)$/)
    if (!base64Match) return null

    const decoded = atob(base64Match[1])
    const parsed = JSON.parse(decoded)

    // Only useful if it has sourcesContent
    if (!parsed.sourcesContent || parsed.sourcesContent.length === 0) return null

    return parsed
  } catch {
    return null
  }
}

/**
 * Extract a code snippet around a given line number
 * @param {string} sourceContent - The full source file content
 * @param {number} line - The 1-based line number of the error
 * @returns {Array|null} Array of { line, text, isError? } or null
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
 * @param {Array} frames - Parsed stack frames
 * @param {Object} mockSourceMaps - Map of filename to parsed source map
 * @returns {Promise<Array>} Array of snippet objects
 */
export async function extractSourceSnippets(frames, mockSourceMaps) {
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

/**
 * Detect which UI framework an element belongs to
 * @param {Object} element - The DOM element (or element-like object)
 * @returns {Object|null} { framework, key? } or null
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

/**
 * Walk a React fiber tree to extract component ancestry
 * @param {Object} fiber - The React fiber node
 * @returns {Array|null} Array of { name, propKeys?, hasState?, stateKeys? } in root-first order
 */
export function getReactComponentAncestry(fiber) {
  if (!fiber) return null

  const ancestry = []
  let current = fiber
  let depth = 0

  while (current && depth < AI_CONTEXT_MAX_ANCESTRY_DEPTH) {
    depth++

    // Only include component fibers (type is function/object), skip host elements (type is string)
    if (current.type && typeof current.type !== 'string') {
      const name = current.type.displayName || current.type.name || 'Anonymous'
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

/**
 * Capture application state snapshot from known store patterns
 * @param {string} errorMessage - The error message for keyword matching
 * @returns {Object|null} State snapshot or null
 */
export function captureStateSnapshot(errorMessage) {
  if (typeof window === 'undefined') return null

  try {
    // Try Redux store
    const store = window.__REDUX_STORE__
    if (!store || typeof store.getState !== 'function') return null

    const state = store.getState()
    if (!state || typeof state !== 'object') return null

    // Build keys with types
    const keys = {}
    for (const [key, value] of Object.entries(state)) {
      if (Array.isArray(value)) {
        keys[key] = { type: 'array' }
      } else if (value === null) {
        keys[key] = { type: 'null' }
      } else {
        keys[key] = { type: typeof value }
      }
    }

    // Build relevant slice
    const relevantSlice = {}
    let sliceCount = 0

    const errorWords = (errorMessage || '')
      .toLowerCase()
      .split(/\W+/)
      .filter((w) => w.length > 2)

    for (const [key, value] of Object.entries(state)) {
      if (sliceCount >= AI_CONTEXT_MAX_RELEVANT_SLICE) break

      if (typeof value === 'object' && value !== null && !Array.isArray(value)) {
        for (const [subKey, subValue] of Object.entries(value)) {
          if (sliceCount >= AI_CONTEXT_MAX_RELEVANT_SLICE) break

          const isRelevantKey = ['error', 'loading', 'status', 'failed'].some((k) => subKey.toLowerCase().includes(k))
          const isKeywordMatch = errorWords.some((w) => key.toLowerCase().includes(w))

          if (isRelevantKey || isKeywordMatch) {
            let val = subValue
            if (typeof val === 'string' && val.length > AI_CONTEXT_MAX_VALUE_LENGTH) {
              val = val.slice(0, AI_CONTEXT_MAX_VALUE_LENGTH)
            }
            relevantSlice[`${key}.${subKey}`] = val
            sliceCount++
          }
        }
      }
    }

    return {
      source: 'redux',
      keys,
      relevantSlice,
    }
  } catch {
    return null
  }
}

/**
 * Generate a template-based AI summary from enrichment data
 * @param {Object} data - { errorType, message, file, line, componentAncestry, stateSnapshot }
 * @returns {string} Summary string
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
      const stateInfo = sliceKeys.map((k) => `${k}=${JSON.stringify(data.stateSnapshot.relevantSlice[k])}`).join(', ')
      parts.push(`State: ${stateInfo}.`)
    }
  }

  return parts.join(' ')
}

/**
 * Full error enrichment pipeline
 * @param {Object} error - The error entry to enrich
 * @returns {Promise<Object>} The enriched error entry
 */
export async function enrichErrorWithAiContext(error) {
  if (!aiContextEnabled) return error

  const enriched = { ...error }

  try {
    // Race the entire pipeline against a timeout
    const context = await Promise.race([
      (async () => {
        const result = {}

        // Parse stack frames
        const frames = parseStackFrames(error.stack)
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
        if (typeof document !== 'undefined' && document.activeElement) {
          const framework = detectFramework(document.activeElement)
          if (framework && framework.framework === 'react' && framework.key) {
            const fiber = document.activeElement[framework.key]
            const components = getReactComponentAncestry(fiber)
            if (components && components.length > 0) {
              result.componentAncestry = { framework: 'react', components }
            }
          }
        }

        // State snapshot (if enabled)
        if (aiContextStateSnapshotEnabled) {
          const snapshot = captureStateSnapshot(error.message || '')
          if (snapshot) result.stateSnapshot = snapshot
        }

        // Generate summary
        result.summary = generateAiSummary({
          errorType: error.message?.split(':')[0] || 'Error',
          message: error.message || '',
          file: topFrame?.filename || null,
          line: topFrame?.lineno || null,
          componentAncestry: result.componentAncestry || null,
          stateSnapshot: result.stateSnapshot || null,
        })

        return result
      })(),
      new Promise((resolve) => {
        setTimeout(() => resolve({ summary: `${error.message || 'Error'}` }), AI_CONTEXT_PIPELINE_TIMEOUT_MS)
      }),
    ])

    enriched._aiContext = context
    if (!enriched._enrichments) enriched._enrichments = []
    enriched._enrichments.push('aiContext')
  } catch {
    // Pipeline failed, add minimal context
    enriched._aiContext = { summary: error.message || 'Unknown error' }
    if (!enriched._enrichments) enriched._enrichments = []
    enriched._enrichments.push('aiContext')
  }

  return enriched
}

/**
 * Enable or disable AI context enrichment
 * @param {boolean} enabled
 */
export function setAiContextEnabled(enabled) {
  aiContextEnabled = enabled
}

/**
 * Enable or disable state snapshot in AI context
 * @param {boolean} enabled
 */
export function setAiContextStateSnapshot(enabled) {
  aiContextStateSnapshotEnabled = enabled
}

/**
 * Cache a parsed source map for a URL
 * @param {string} url - The script URL
 * @param {Object} map - The parsed source map
 */
export function setSourceMapCache(url, map) {
  // Evict oldest if at capacity
  if (!aiSourceMapCache.has(url) && aiSourceMapCache.size >= AI_CONTEXT_SOURCE_MAP_CACHE_SIZE) {
    const firstKey = aiSourceMapCache.keys().next().value
    aiSourceMapCache.delete(firstKey)
  }
  aiSourceMapCache.set(url, map)
}

/**
 * Get a cached source map
 * @param {string} url - The script URL
 * @returns {Object|null} The cached source map or null
 */
export function getSourceMapCache(url) {
  return aiSourceMapCache.get(url) || null
}

/**
 * Get the number of cached source maps
 * @returns {number}
 */
export function getSourceMapCacheSize() {
  return aiSourceMapCache.size
}
