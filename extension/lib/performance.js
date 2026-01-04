// @ts-nocheck
/**
 * @fileoverview Performance marks and measures capture.
 * Wraps performance.mark/measure to capture calls, uses PerformanceObserver
 * for additional entries, and provides error-time performance snapshots.
 */

import { MAX_PERFORMANCE_ENTRIES, PERFORMANCE_TIME_WINDOW_MS } from './constants.js'

// Performance Marks state
let performanceMarksEnabled = false
let capturedMarks = []
let capturedMeasures = []
let originalPerformanceMark = null
let originalPerformanceMeasure = null
let performanceObserver = null
let performanceCaptureActive = false

/**
 * Get performance marks
 * @param {Object} options - Options for filtering
 * @returns {Array} Array of mark entries
 */
export function getPerformanceMarks(options = {}) {
  if (typeof performance === 'undefined' || !performance) return []

  try {
    let marks = performance.getEntriesByType('mark') || []

    // Filter by time range
    if (options.since) {
      marks = marks.filter((m) => m.startTime >= options.since)
    }

    // Sort by start time
    marks.sort((a, b) => a.startTime - b.startTime)

    // Limit entries
    if (marks.length > MAX_PERFORMANCE_ENTRIES) {
      marks = marks.slice(-MAX_PERFORMANCE_ENTRIES)
    }

    return marks.map((m) => ({
      name: m.name,
      startTime: m.startTime,
      detail: m.detail || null,
    }))
  } catch {
    return []
  }
}

/**
 * Get performance measures
 * @param {Object} options - Options for filtering
 * @returns {Array} Array of measure entries
 */
export function getPerformanceMeasures(options = {}) {
  if (typeof performance === 'undefined' || !performance) return []

  try {
    let measures = performance.getEntriesByType('measure') || []

    // Filter by time range
    if (options.since) {
      measures = measures.filter((m) => m.startTime >= options.since)
    }

    // Sort by start time
    measures.sort((a, b) => a.startTime - b.startTime)

    // Limit entries
    if (measures.length > MAX_PERFORMANCE_ENTRIES) {
      measures = measures.slice(-MAX_PERFORMANCE_ENTRIES)
    }

    return measures.map((m) => ({
      name: m.name,
      startTime: m.startTime,
      duration: m.duration,
      detail: m.detail || null,
    }))
  } catch {
    return []
  }
}

/**
 * Get captured marks from wrapper
 * @returns {Array} Array of captured marks
 */
export function getCapturedMarks() {
  return [...capturedMarks]
}

/**
 * Get captured measures from wrapper
 * @returns {Array} Array of captured measures
 */
export function getCapturedMeasures() {
  return [...capturedMeasures]
}

/**
 * Install performance capture wrapper
 */
export function installPerformanceCapture() {
  if (typeof performance === 'undefined' || !performance) return

  // Clear previous captured data
  capturedMarks = []
  capturedMeasures = []

  // Store originals
  originalPerformanceMark = performance.mark
  originalPerformanceMeasure = performance.measure

  // Wrap performance.mark
  performance.mark = function (name, options) {
    const result = originalPerformanceMark.call(performance, name, options)

    capturedMarks.push({
      name,
      startTime: result?.startTime || performance.now(),
      detail: options?.detail || null,
      capturedAt: new Date().toISOString(),
    })

    // Limit captured marks
    if (capturedMarks.length > MAX_PERFORMANCE_ENTRIES) {
      capturedMarks.shift()
    }

    return result
  }

  // Wrap performance.measure
  performance.measure = function (name, startMark, endMark) {
    const result = originalPerformanceMeasure.call(performance, name, startMark, endMark)

    capturedMeasures.push({
      name,
      startTime: result?.startTime || 0,
      duration: result?.duration || 0,
      capturedAt: new Date().toISOString(),
    })

    // Limit captured measures
    if (capturedMeasures.length > MAX_PERFORMANCE_ENTRIES) {
      capturedMeasures.shift()
    }

    return result
  }

  performanceCaptureActive = true

  // Try to use PerformanceObserver for additional entries
  if (typeof window !== 'undefined' && window.PerformanceObserver) {
    try {
      performanceObserver = new window.PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
          if (entry.entryType === 'mark') {
            // Avoid duplicates from our wrapper
            if (!capturedMarks.some((m) => m.name === entry.name && m.startTime === entry.startTime)) {
              capturedMarks.push({
                name: entry.name,
                startTime: entry.startTime,
                detail: entry.detail || null,
                capturedAt: new Date().toISOString(),
              })
            }
          } else if (entry.entryType === 'measure') {
            if (!capturedMeasures.some((m) => m.name === entry.name && m.startTime === entry.startTime)) {
              capturedMeasures.push({
                name: entry.name,
                startTime: entry.startTime,
                duration: entry.duration,
                capturedAt: new Date().toISOString(),
              })
            }
          }
        }
      })
      performanceObserver.observe({ entryTypes: ['mark', 'measure'] })
    } catch {
      // PerformanceObserver not supported, continue without it
    }
  }
}

/**
 * Uninstall performance capture wrapper
 */
export function uninstallPerformanceCapture() {
  if (typeof performance === 'undefined' || !performance) return

  if (originalPerformanceMark) {
    performance.mark = originalPerformanceMark
    originalPerformanceMark = null
  }

  if (originalPerformanceMeasure) {
    performance.measure = originalPerformanceMeasure
    originalPerformanceMeasure = null
  }

  if (performanceObserver) {
    performanceObserver.disconnect()
    performanceObserver = null
  }

  capturedMarks = []
  capturedMeasures = []
  performanceCaptureActive = false
}

/**
 * Check if performance capture is active
 * @returns {boolean} Whether performance capture is active
 */
export function isPerformanceCaptureActive() {
  return performanceCaptureActive
}

/**
 * Get performance snapshot for an error
 * @param {Object} errorEntry - The error entry
 * @returns {Promise<Object|null>} The performance snapshot
 */
export async function getPerformanceSnapshotForError(errorEntry) {
  if (!performanceMarksEnabled) return null

  const now = typeof performance !== 'undefined' && performance?.now ? performance.now() : 0
  const since = Math.max(0, now - PERFORMANCE_TIME_WINDOW_MS)

  const marks = getPerformanceMarks({ since })
  const measures = getPerformanceMeasures({ since })

  // Include navigation timing if available
  let navigation = null
  if (typeof performance !== 'undefined' && performance) {
    try {
      const navEntries = performance.getEntriesByType('navigation')
      if (navEntries && navEntries.length > 0) {
        const nav = navEntries[0]
        navigation = {
          type: nav.type,
          startTime: nav.startTime,
          domContentLoadedEventEnd: nav.domContentLoadedEventEnd,
          loadEventEnd: nav.loadEventEnd,
        }
      }
    } catch {
      // Navigation timing not available
    }
  }

  return {
    type: 'performance',
    ts: new Date().toISOString(),
    _enrichments: ['performanceMarks'],
    _errorTs: errorEntry.ts,
    marks,
    measures,
    navigation,
  }
}

/**
 * Set whether performance marks are enabled
 * @param {boolean} enabled - Whether to enable performance marks
 */
export function setPerformanceMarksEnabled(enabled) {
  performanceMarksEnabled = enabled
}

/**
 * Check if performance marks are enabled
 * @returns {boolean} Whether performance marks are enabled
 */
export function isPerformanceMarksEnabled() {
  return performanceMarksEnabled
}
