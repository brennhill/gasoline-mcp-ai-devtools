/**
 * @fileoverview Draw Mode — Full-viewport annotation overlay.
 * Lets users draw rectangles and attach text feedback on web pages.
 * Captures DOM elements under each rectangle for LLM consumption.
 * Activated by LLM (interact draw_mode_start) or user (keyboard shortcut / popup).
 */

// ============================================================================
// STATE
// ============================================================================

let active = false
let startedBy = 'user' // 'llm' | 'user'
let sessionName = '' // Named session for multi-page review
let sessionCorrelationId = '' // Correlation ID from MCP server for result retrieval
let overlay = null
let canvas = null
let ctx = null
let textInput = null
let annotations = []
let elementDetails = new Map() // correlationId → full detail
let drawing = false
let startX = 0
let startY = 0
let currentX = 0
let currentY = 0
let rafId = null
let saveTimeout = null
let isDeactivating = false // Re-entry guard for deactivateAndSendResults

const MIN_RECT_SIZE = 5
const OVERLAY_Z_INDEX = 2147483644
const ANNOTATION_COLOR = '#ef4444'
const ANNOTATION_FILL = 'rgba(239, 68, 68, 0.15)'
const ANNOTATION_STROKE_WIDTH = 2

// ============================================================================
// PUBLIC API
// ============================================================================

/**
 * Activate draw mode overlay.
 * @param {string} source - 'llm' or 'user'
 * @param {string} session - Optional named session for multi-page review
 * @returns {{ status: string, annotation_count?: number }}
 */
export function activateDrawMode(source = 'user', session = '', correlationId = '') {
  if (active) {
    return { status: 'already_active', annotation_count: annotations.length }
  }
  startedBy = source
  sessionName = session
  sessionCorrelationId = correlationId
  active = true
  createOverlay()
  loadAnnotations()
  return { status: 'active', started_by: source }
}

/**
 * Deactivate draw mode and return results.
 * @returns {{ annotations: Array, elementDetails: Object }}
 */
export function deactivateDrawMode() {
  if (!active) {
    return { annotations: [], elementDetails: {} }
  }
  cancelTextInput()
  const result = {
    annotations: annotations.map((a) => ({ ...a })),
    elementDetails: Object.fromEntries(elementDetails)
  }
  active = false
  // Clear state to prevent leaks across activate/deactivate cycles
  annotations = []
  elementDetails.clear()
  sessionName = ''
  sessionCorrelationId = ''
  destroyOverlay()
  return result
}

/**
 * Get current annotations.
 * @returns {Array}
 */
export function getAnnotations() {
  return annotations.map((a) => ({ ...a }))
}

/**
 * Get full DOM/style detail for a specific annotation.
 * @param {string} correlationId
 * @returns {Object|null}
 */
export function getElementDetail(correlationId) {
  return elementDetails.get(correlationId) || null
}

/**
 * Clear all annotations.
 */
export function clearAnnotations() {
  annotations = []
  elementDetails.clear()
  if (ctx && canvas) {
    renderAnnotations()
  }
  persistAnnotations()
}

/**
 * Check if draw mode is currently active.
 * @returns {boolean}
 */
export function isDrawModeActive() {
  return active
}

// ============================================================================
// OVERLAY CREATION / DESTRUCTION
// ============================================================================

function createOverlay() {
  overlay = document.createElement('div')
  overlay.id = 'gasoline-draw-overlay'
  Object.assign(overlay.style, {
    position: 'fixed',
    top: '0',
    left: '0',
    width: '100vw',
    height: '100vh',
    zIndex: String(OVERLAY_Z_INDEX),
    cursor: 'crosshair',
    boxShadow: 'inset 0 0 30px rgba(239, 68, 68, 0.3)',
    transition: 'opacity 0.3s ease-out, box-shadow 0.3s ease-in'
  })

  // Canvas for drawing
  canvas = document.createElement('canvas')
  canvas.width = window.innerWidth
  canvas.height = window.innerHeight
  Object.assign(canvas.style, {
    position: 'absolute',
    top: '0',
    left: '0',
    width: '100%',
    height: '100%'
  })
  overlay.appendChild(canvas)
  ctx = canvas.getContext('2d')

  // Mode badge (top-right)
  const badge = document.createElement('div')
  badge.id = 'gasoline-draw-badge'
  Object.assign(badge.style, {
    position: 'absolute',
    top: '12px',
    right: '12px',
    display: 'flex',
    alignItems: 'center',
    gap: '6px',
    padding: '6px 12px',
    background: 'rgba(0, 0, 0, 0.8)',
    color: '#ef4444',
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
    fontSize: '12px',
    fontWeight: '600',
    borderRadius: '6px',
    pointerEvents: 'none',
    zIndex: String(OVERLAY_Z_INDEX + 1)
  })

  // Pulsing dot
  const dot = document.createElement('span')
  Object.assign(dot.style, {
    width: '8px',
    height: '8px',
    borderRadius: '50%',
    background: '#ef4444',
    display: 'inline-block',
    animation: 'gasoline-draw-pulse 1.5s ease-in-out infinite'
  })
  badge.appendChild(dot)
  badge.appendChild(document.createTextNode('Draw Mode'))

  // ESC hint
  const hint = document.createElement('span')
  hint.textContent = '(ESC to finish)'
  Object.assign(hint.style, {
    color: '#888',
    fontWeight: '400',
    marginLeft: '4px'
  })
  badge.appendChild(hint)
  overlay.appendChild(badge)

  // Center instruction toast — fades out after 2.5s
  const instruction = document.createElement('div')
  instruction.id = 'gasoline-draw-instruction'
  Object.assign(instruction.style, {
    position: 'absolute',
    top: '50%',
    left: '50%',
    transform: 'translate(-50%, -50%)',
    padding: '16px 28px',
    background: 'rgba(0, 0, 0, 0.85)',
    color: '#fff',
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
    fontSize: '16px',
    fontWeight: '500',
    borderRadius: '10px',
    border: '1px solid rgba(239, 68, 68, 0.4)',
    pointerEvents: 'none',
    zIndex: String(OVERLAY_Z_INDEX + 1),
    transition: 'opacity 0.5s ease-out',
    textAlign: 'center',
    lineHeight: '1.5'
  })
  instruction.innerHTML =
    'Draw a box around what you want to change<br><span style="font-size:13px;color:#aaa">Then type your instruction. Press ESC when done.</span>'
  overlay.appendChild(instruction)
  setTimeout(() => {
    instruction.style.opacity = '0'
  }, 2500)
  setTimeout(() => {
    instruction.remove()
  }, 3000)

  // Inject animation keyframes
  injectStyles()

  // Event listeners
  overlay.addEventListener('mousedown', onMouseDown)
  overlay.addEventListener('mousemove', onMouseMove)
  overlay.addEventListener('mouseup', onMouseUp)
  document.addEventListener('keydown', onKeyDown)

  // Resize observer
  window.addEventListener('resize', onResize)

  // Warn before navigating away with unsaved annotations
  window.addEventListener('beforeunload', onBeforeUnload)

  const target = document.body || document.documentElement
  if (target) {
    target.appendChild(overlay)
  }
}

function destroyOverlay() {
  if (rafId) {
    cancelAnimationFrame(rafId)
    rafId = null
  }
  if (saveTimeout) {
    clearTimeout(saveTimeout)
    saveTimeout = null
  }
  if (overlay) {
    overlay.removeEventListener('mousedown', onMouseDown)
    overlay.removeEventListener('mousemove', onMouseMove)
    overlay.removeEventListener('mouseup', onMouseUp)
    overlay.remove()
    overlay = null
  }
  document.removeEventListener('keydown', onKeyDown)
  window.removeEventListener('resize', onResize)
  window.removeEventListener('beforeunload', onBeforeUnload)
  canvas = null
  ctx = null
  textInput = null
  drawing = false
  removeStyles()
}

function injectStyles() {
  if (document.getElementById('gasoline-draw-styles')) return
  const style = document.createElement('style')
  style.id = 'gasoline-draw-styles'
  style.textContent = `
        @keyframes gasoline-draw-pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.3; }
        }
    `
  document.head.appendChild(style)
}

function removeStyles() {
  const style = document.getElementById('gasoline-draw-styles')
  if (style) style.remove()
}

// ============================================================================
// EVENT HANDLERS
// ============================================================================

function onMouseDown(e) {
  if (textInput) return // Don't start new rect while typing
  if (e.button !== 0) return // Left click only
  drawing = true
  startX = e.clientX
  startY = e.clientY
  currentX = startX
  currentY = startY
}

function onMouseMove(e) {
  if (!drawing) return
  currentX = e.clientX
  currentY = e.clientY
  if (rafId) cancelAnimationFrame(rafId)
  rafId = requestAnimationFrame(renderFrame)
}

function onMouseUp(e) {
  if (!drawing) return
  drawing = false
  if (rafId) {
    cancelAnimationFrame(rafId)
    rafId = null
  }

  const rect = normalizeRect(startX, startY, e.clientX, e.clientY)

  // Ignore tiny rectangles (accidental clicks)
  if (rect.width < MIN_RECT_SIZE || rect.height < MIN_RECT_SIZE) {
    renderAnnotations()
    return
  }

  // Capture DOM elements under the rectangle
  const elementData = captureElementsUnderRect(rect)

  // Show text input
  showTextInput(rect, elementData)
}

function onKeyDown(e) {
  if (e.key === 'Escape') {
    if (textInput) {
      cancelTextInput()
      renderAnnotations()
    } else {
      // Exit draw mode entirely
      deactivateAndSendResults()
    }
    e.preventDefault()
    e.stopPropagation()
  }
}

function onResize() {
  if (!canvas) return
  canvas.width = window.innerWidth
  canvas.height = window.innerHeight
  renderAnnotations()
}

function onBeforeUnload(e) {
  if (active && annotations.length > 0) {
    e.preventDefault()
    // Returning a string is required by some browsers to trigger the dialog
    e.returnValue = 'You have unsaved annotations. Are you sure you want to leave?'
    return e.returnValue
  }
}

// ============================================================================
// RENDERING
// ============================================================================

function renderFrame() {
  if (!ctx || !canvas) return
  // Clear and re-render existing annotations
  ctx.clearRect(0, 0, canvas.width, canvas.height)
  drawExistingAnnotations()

  // Draw current rubber-band rectangle
  const rect = normalizeRect(startX, startY, currentX, currentY)
  ctx.setLineDash([6, 4])
  ctx.strokeStyle = ANNOTATION_COLOR
  ctx.lineWidth = ANNOTATION_STROKE_WIDTH
  ctx.strokeRect(rect.x, rect.y, rect.width, rect.height)
  ctx.setLineDash([])
}

function renderAnnotations() {
  if (!ctx || !canvas) return
  ctx.clearRect(0, 0, canvas.width, canvas.height)
  drawExistingAnnotations()
}

function drawExistingAnnotations() {
  if (!ctx) return
  for (let i = 0; i < annotations.length; i++) {
    const ann = annotations[i]
    const r = ann.rect

    // Semi-transparent fill
    ctx.fillStyle = ANNOTATION_FILL
    ctx.fillRect(r.x, r.y, r.width, r.height)

    // Solid stroke
    ctx.strokeStyle = ANNOTATION_COLOR
    ctx.lineWidth = ANNOTATION_STROKE_WIDTH
    ctx.setLineDash([])
    ctx.strokeRect(r.x, r.y, r.width, r.height)

    // Number badge (top-left corner)
    const badgeSize = 20
    ctx.fillStyle = ANNOTATION_COLOR
    ctx.beginPath()
    ctx.arc(r.x, r.y, badgeSize / 2, 0, Math.PI * 2)
    ctx.fill()
    ctx.fillStyle = '#fff'
    ctx.font = 'bold 11px -apple-system, sans-serif'
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'
    ctx.fillText(String(i + 1), r.x, r.y)

    // Text label (below rectangle)
    if (ann.text) {
      const labelY = r.y + r.height + 16
      ctx.fillStyle = 'rgba(0, 0, 0, 0.8)'
      const textWidth = ctx.measureText(ann.text).width
      const padding = 6
      ctx.fillRect(r.x - padding, labelY - 12, textWidth + padding * 2, 18)
      ctx.fillStyle = '#fff'
      ctx.font = '12px -apple-system, sans-serif'
      ctx.textAlign = 'left'
      ctx.textBaseline = 'middle'
      ctx.fillText(ann.text, r.x, labelY)
    }
  }
}

// ============================================================================
// TEXT INPUT
// ============================================================================

function showTextInput(rect, elementData) {
  if (textInput) cancelTextInput()

  const input = document.createElement('input')
  input.type = 'text'
  input.placeholder = 'What should the AI change here?'
  input.dataset.rectJson = JSON.stringify(rect)
  input.dataset.elementJson = JSON.stringify(elementData)

  // Clamp position so the input stays within the viewport
  const inputHeight = 36 // approximate height (padding + font + border)
  const inputGap = 8
  let inputTop = rect.y + rect.height + inputGap
  let inputLeft = rect.x
  if (inputTop + inputHeight > window.innerHeight) {
    // Place above the rectangle instead
    inputTop = Math.max(0, rect.y - inputHeight - inputGap)
  }
  if (inputLeft + 200 > window.innerWidth) {
    inputLeft = Math.max(0, window.innerWidth - 200)
  }

  Object.assign(input.style, {
    position: 'absolute',
    left: `${inputLeft}px`,
    top: `${inputTop}px`,
    minWidth: '200px',
    maxWidth: '400px',
    padding: '8px 12px',
    background: '#1a1a1a',
    color: '#e0e0e0',
    border: '2px solid ' + ANNOTATION_COLOR,
    borderRadius: '6px',
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
    fontSize: '13px',
    outline: 'none',
    zIndex: String(OVERLAY_Z_INDEX + 2),
    boxShadow: '0 4px 12px rgba(0, 0, 0, 0.5)'
  })

  input.addEventListener('keydown', onTextInputKeyDown)
  input.addEventListener('blur', onTextInputBlur)

  overlay.appendChild(input)
  textInput = input
  input.focus()
}

function onTextInputKeyDown(e) {
  e.stopPropagation()
  if (e.key === 'Enter') {
    e.preventDefault()
    confirmTextInput()
  } else if (e.key === 'Escape') {
    e.preventDefault()
    cancelTextInput()
    renderAnnotations()
  }
}

function onTextInputBlur() {
  // Auto-confirm on blur
  if (textInput) {
    confirmTextInput()
  }
}

function confirmTextInput() {
  if (!textInput) return
  // Capture and null immediately to prevent re-entry from blur during remove()
  const input = textInput
  textInput = null

  const text = input.value.trim()
  const rect = JSON.parse(input.dataset.rectJson)
  const elementData = JSON.parse(input.dataset.elementJson)

  // Remove input element
  input.removeEventListener('keydown', onTextInputKeyDown)
  input.removeEventListener('blur', onTextInputBlur)
  input.remove()

  // Empty text → discard annotation
  if (!text) {
    renderAnnotations()
    return
  }

  // Create annotation
  const id = `ann_${Date.now()}_${Math.random().toString(36).slice(2, 5)}`
  const correlationId = `ann_detail_${Math.random().toString(36).slice(2, 8)}`

  const annotation = {
    id,
    rect,
    text,
    timestamp: Date.now(),
    page_url: window.location.href,
    element_summary: elementData.summary || '',
    correlation_id: correlationId
  }
  annotations.push(annotation)

  // Store full detail for lazy retrieval
  elementDetails.set(correlationId, elementData.detail)

  renderAnnotations()
  persistAnnotations()
}

function cancelTextInput() {
  if (!textInput) return
  textInput.removeEventListener('keydown', onTextInputKeyDown)
  textInput.removeEventListener('blur', onTextInputBlur)
  textInput.remove()
  textInput = null
}

// ============================================================================
// DOM ELEMENT CAPTURE
// ============================================================================

/**
 * Capture DOM elements under the drawn rectangle.
 * Temporarily hides overlay to use document.elementsFromPoint().
 * Captures all elements (capped at MAX_CAPTURED_ELEMENTS), shadow DOM, iframes, and HTML.
 */
const MAX_CAPTURED_ELEMENTS = 15

function captureElementsUnderRect(rect) {
  if (!overlay) return { summary: '', detail: {} }

  // Temporarily hide overlay
  overlay.style.pointerEvents = 'none'
  overlay.style.visibility = 'hidden'

  try {
    // Sample points: corners + center + edge midpoints for better coverage
    const points = [
      { x: rect.x + rect.width / 2, y: rect.y + rect.height / 2 }, // center
      { x: rect.x + 2, y: rect.y + 2 }, // top-left
      { x: rect.x + rect.width - 2, y: rect.y + 2 }, // top-right
      { x: rect.x + 2, y: rect.y + rect.height - 2 }, // bottom-left
      { x: rect.x + rect.width - 2, y: rect.y + rect.height - 2 }, // bottom-right
      { x: rect.x + rect.width / 2, y: rect.y + 2 }, // top-center
      { x: rect.x + rect.width / 2, y: rect.y + rect.height - 2 }, // bottom-center
      { x: rect.x + 2, y: rect.y + rect.height / 2 }, // left-center
      { x: rect.x + rect.width - 2, y: rect.y + rect.height / 2 } // right-center
    ]

    const seenElements = new Set()
    const elements = []

    for (const pt of points) {
      try {
        const els = document.elementsFromPoint(pt.x, pt.y)
        for (const el of els) {
          if (seenElements.has(el)) continue
          if (el === document.body || el === document.documentElement) continue
          seenElements.add(el)
          elements.push(el)
          if (elements.length >= MAX_CAPTURED_ELEMENTS) break
        }
      } catch {
        // elementsFromPoint may fail on some edge cases
      }
      if (elements.length >= MAX_CAPTURED_ELEMENTS) break
    }

    // Fallback: if grid sampling found nothing, try single-point probe at rect center
    if (elements.length === 0) {
      try {
        const cx = rect.x + rect.width / 2
        const cy = rect.y + rect.height / 2
        const el = document.elementFromPoint(cx, cy)
        if (el && el !== document.body && el !== document.documentElement && !seenElements.has(el)) {
          seenElements.add(el)
          elements.push(el)
        }
      } catch {
        // elementFromPoint fallback failed
      }
    }

    // Fallback: walk DOM for elements whose bounding rect overlaps the drawn rectangle
    if (elements.length === 0) {
      try {
        const candidates = document.querySelectorAll('*')
        for (const el of candidates) {
          if (el === document.body || el === document.documentElement) continue
          if (seenElements.has(el)) continue
          const br = el.getBoundingClientRect()
          if (br.width === 0 && br.height === 0) continue
          const overlaps =
            br.left < rect.x + rect.width && br.right > rect.x && br.top < rect.y + rect.height && br.bottom > rect.y
          if (overlaps) {
            seenElements.add(el)
            elements.push(el)
            if (elements.length >= MAX_CAPTURED_ELEMENTS) break
          }
        }
      } catch {
        // DOM walk fallback failed
      }
    }

    // Also probe inside same-origin iframes
    const iframeElements = captureIframeElements(rect, seenElements)

    // Pick the most relevant element for the summary
    const target = pickBestElement(elements) || elements[0]

    if (!target && elements.length === 0 && iframeElements.length === 0) {
      return { summary: '', detail: {} }
    }

    const summary = target ? buildElementSummary(target) : ''

    // Build comprehensive detail: primary element + all elements + iframes
    const primaryDetail = target ? buildElementDetail(target) : {}
    const allElementDetails = elements.slice(0, MAX_CAPTURED_ELEMENTS).map((el) => ({
      tag: el.tagName.toLowerCase(),
      selector: buildCSSSelector(el),
      text: (el.textContent || '').trim().slice(0, 100),
      classes: Array.from(el.classList).slice(0, 5)
    }))

    const detail = {
      ...primaryDetail,
      all_elements: allElementDetails,
      element_count: elements.length
    }

    if (iframeElements.length > 0) {
      detail.iframe_content = iframeElements
    }

    return { summary, detail }
  } finally {
    // Always restore overlay, even if an exception occurs
    if (overlay) {
      overlay.style.pointerEvents = ''
      overlay.style.visibility = ''
    }
  }
}

/**
 * Capture elements inside same-origin iframes that overlap the drawn rectangle.
 * Cross-origin iframes are noted but their DOM is inaccessible.
 */
function captureIframeElements(rect, seenElements) {
  const results = []
  try {
    const iframes = document.querySelectorAll('iframe')
    for (const iframe of iframes) {
      const iframeRect = iframe.getBoundingClientRect()
      // Check if iframe overlaps with drawn rectangle
      if (
        iframeRect.right < rect.x ||
        iframeRect.left > rect.x + rect.width ||
        iframeRect.bottom < rect.y ||
        iframeRect.top > rect.y + rect.height
      ) {
        continue
      }
      try {
        const iframeDoc = iframe.contentDocument
        if (!iframeDoc) {
          results.push({ src: iframe.src, access: 'cross_origin', note: 'Cannot access cross-origin iframe DOM' })
          continue
        }
        // Adjust coordinates relative to iframe
        const adjustedX = rect.x - iframeRect.left + rect.width / 2
        const adjustedY = rect.y - iframeRect.top + rect.height / 2
        const els = iframeDoc.elementsFromPoint(adjustedX, adjustedY)
        const iframeEls = []
        for (const el of els) {
          if (seenElements.has(el)) continue
          if (el === iframeDoc.body || el === iframeDoc.documentElement) continue
          seenElements.add(el)
          iframeEls.push({
            tag: el.tagName.toLowerCase(),
            selector: buildCSSSelector(el),
            text: (el.textContent || '').trim().slice(0, 100),
            outer_html: el.outerHTML.slice(0, 500)
          })
          if (iframeEls.length >= 5) break
        }
        if (iframeEls.length > 0) {
          results.push({ src: iframe.src, access: 'same_origin', elements: iframeEls })
        }
      } catch {
        results.push({ src: iframe.src, access: 'blocked', note: 'SecurityError accessing iframe' })
      }
    }
  } catch {
    // Ignore iframe enumeration errors
  }
  return results
}

/**
 * Re-capture DOM element details for all existing annotations.
 * Called right before screenshot to ensure DOM data matches the visual state.
 */
function refreshElementDetails() {
  if (!overlay) return
  for (const ann of annotations) {
    if (!ann.rect || !ann.correlation_id) continue
    try {
      const freshData = captureElementsUnderRect(ann.rect)
      if (freshData.detail && Object.keys(freshData.detail).length > 0) {
        elementDetails.set(ann.correlation_id, freshData.detail)
        ann.element_summary = freshData.summary || ann.element_summary
      }
    } catch {
      // Keep existing data if re-capture fails
    }
  }
}

/**
 * Pick the most semantically relevant element from candidates.
 * Prefers interactive elements (button, a, input) over containers (div, span).
 */
function pickBestElement(elements) {
  const interactiveTags = new Set(['BUTTON', 'A', 'INPUT', 'SELECT', 'TEXTAREA', 'LABEL'])
  for (const el of elements) {
    if (interactiveTags.has(el.tagName)) return el
  }
  // Fall back to first element with meaningful text content
  for (const el of elements) {
    const text = el.textContent?.trim()
    if (text && text.length < 200 && text.length > 0) return el
  }
  return null
}

/**
 * Build compact element summary: "tag.class1.class2 'text'"
 */
function buildElementSummary(el) {
  const tag = el.tagName.toLowerCase()
  const classes = Array.from(el.classList).slice(0, 3).join('.')
  const text = (el.textContent || '').trim().slice(0, 40)
  let summary = tag
  if (classes) summary += '.' + classes
  if (text) summary += ` '${text}'`
  return summary
}

/**
 * Build full element detail for lazy retrieval.
 */
function buildElementDetail(el) {
  const computed = window.getComputedStyle(el)
  const styleProps = [
    'background-color',
    'color',
    'font-size',
    'font-weight',
    'font-family',
    'padding',
    'margin',
    'border',
    'border-radius',
    'display',
    'position',
    'z-index',
    'width',
    'height',
    'opacity',
    'flex-direction',
    'flex-wrap',
    'align-items',
    'justify-content',
    'gap',
    'grid-template-columns',
    'grid-template-rows',
    'overflow',
    'text-align',
    'text-decoration',
    'line-height',
    'letter-spacing',
    'box-shadow',
    'transform',
    'transition',
    'cursor',
    'visibility',
    'white-space',
    'max-width',
    'min-width',
    'max-height',
    'min-height'
  ]
  const computedStyles = {}
  for (const prop of styleProps) {
    computedStyles[prop] = computed.getPropertyValue(prop)
  }

  const boundingRect = el.getBoundingClientRect()

  // Build parent selector
  let parentSelector = ''
  try {
    const parent = el.parentElement
    if (parent && parent !== document.body && parent !== document.documentElement) {
      const pTag = parent.tagName.toLowerCase()
      const pClasses = Array.from(parent.classList).slice(0, 2).join('.')
      parentSelector = pTag
      if (parent.id) parentSelector += '#' + parent.id
      else if (pClasses) parentSelector += '.' + pClasses
      parentSelector += ' > '

      const childTag = el.tagName.toLowerCase()
      const childClasses = Array.from(el.classList).slice(0, 2).join('.')
      parentSelector += childTag
      if (el.id) parentSelector += '#' + el.id
      else if (childClasses) parentSelector += '.' + childClasses
    }
  } catch {
    // Ignore selector build errors
  }

  // Capture outer HTML (truncated for large elements)
  let outerHtml = ''
  try {
    outerHtml = el.outerHTML.slice(0, 2000)
  } catch {
    // outerHTML may fail on some special elements
  }

  // Shadow DOM detection
  let shadowInfo = null
  try {
    if (el.shadowRoot) {
      // Open shadow DOM — capture inner HTML
      shadowInfo = {
        mode: 'open',
        html: el.shadowRoot.innerHTML.slice(0, 2000),
        child_count: el.shadowRoot.childElementCount
      }
    } else if (el.attachShadow) {
      // Element supports shadow DOM but may have closed shadow root
      // We can detect this heuristically: if the element has no children but renders content
      const hasVisibleContent = el.getBoundingClientRect().height > 0
      const hasLightDOMChildren = el.childElementCount > 0
      if (hasVisibleContent && !hasLightDOMChildren && el.tagName.includes('-')) {
        shadowInfo = { mode: 'closed', note: 'Element likely has closed shadow DOM (content not accessible)' }
      }
    }
  } catch {
    // Shadow DOM access may fail
  }

  const detail = {
    selector: buildCSSSelector(el),
    tag: el.tagName.toLowerCase(),
    text_content: (el.textContent || '').trim().slice(0, 200),
    outer_html: outerHtml,
    classes: Array.from(el.classList).slice(0, 20),
    id: el.id || '',
    computed_styles: computedStyles,
    parent_selector: parentSelector,
    bounding_rect: {
      x: Math.round(boundingRect.x),
      y: Math.round(boundingRect.y),
      width: Math.round(boundingRect.width),
      height: Math.round(boundingRect.height)
    },
    a11y_flags: runA11yChecks(el, computed)
  }

  if (shadowInfo) {
    detail.shadow_dom = shadowInfo
  }

  // CSS rule tracing — find source stylesheets and selectors
  const matchedRules = traceMatchedCSSRules(el)
  if (matchedRules.length > 0) {
    detail.matched_css_rules = matchedRules
  }

  // Framework component detection
  const componentInfo = detectComponentSource(el)
  if (componentInfo) {
    detail.component = componentInfo
  }

  return detail
}

/**
 * Run lightweight accessibility checks on an element.
 * Returns an array of flag strings describing potential issues.
 * @param {Element} el
 * @param {CSSStyleDeclaration} computed
 * @returns {string[]}
 */
function runA11yChecks(el, computed) {
  const flags = []
  if (!el || !el.tagName) return flags
  const tag = el.tagName.toLowerCase()
  const getAttribute = (name) => (typeof el.getAttribute === 'function' ? el.getAttribute(name) : null)

  // 1. Image without alt text
  if (tag === 'img' && !getAttribute('alt')) {
    flags.push('missing_alt_text')
  }

  // 2. Interactive element without accessible name
  const interactiveTags = ['button', 'a', 'input', 'select', 'textarea']
  if (interactiveTags.includes(tag)) {
    const hasLabel = getAttribute('aria-label') || getAttribute('aria-labelledby') || getAttribute('title')
    const hasText = (el.textContent || '').trim()
    const hasPlaceholder = getAttribute('placeholder')
    if (!hasLabel && !hasText && !hasPlaceholder) {
      flags.push('missing_accessible_name')
    }
  }

  // 3. Div/span with click handler but no role
  if ((tag === 'div' || tag === 'span') && !getAttribute('role')) {
    if (getAttribute('onclick') || getAttribute('tabindex')) {
      flags.push('interactive_without_role')
    }
  }

  // 4. Contrast ratio check (foreground vs background)
  try {
    if (computed && typeof computed.getPropertyValue === 'function') {
      const fg = parseRGBColor(computed.getPropertyValue('color'))
      const bg = parseRGBColor(computed.getPropertyValue('background-color'))
      if (fg && bg && bg.a > 0) {
        const ratio = contrastRatio(fg, bg)
        const fontSize = parseFloat(computed.getPropertyValue('font-size'))
        const isBold = parseInt(computed.getPropertyValue('font-weight'), 10) >= 700
        const isLargeText = fontSize >= 24 || (fontSize >= 18.66 && isBold)
        const minRatio = isLargeText ? 3 : 4.5
        if (ratio < minRatio) {
          flags.push(`low_contrast:${ratio.toFixed(1)}:1`)
        }
      }
    }
  } catch {
    // Ignore contrast parse errors
  }

  // 5. Focus indicator removed
  try {
    if (interactiveTags.includes(tag) && computed && typeof computed.getPropertyValue === 'function') {
      const outline = computed.getPropertyValue('outline')
      const outlineStyle = computed.getPropertyValue('outline-style')
      if (outlineStyle === 'none' || outline === '0' || outline === 'none') {
        const boxShadow = computed.getPropertyValue('box-shadow')
        if (!boxShadow || boxShadow === 'none') {
          flags.push('no_focus_indicator')
        }
      }
    }
  } catch {
    // Ignore focus indicator check errors
  }

  // 6. Missing form label
  try {
    if ((tag === 'input' || tag === 'select' || tag === 'textarea') && !getAttribute('aria-label')) {
      const id = el.id
      const hasLabelFor =
        id &&
        typeof document !== 'undefined' &&
        typeof document.querySelector === 'function' &&
        document.querySelector(`label[for="${CSS.escape(id)}"]`)
      if (!hasLabelFor) {
        const parent = typeof el.closest === 'function' ? el.closest('label') : null
        if (!parent) {
          flags.push('missing_form_label')
        }
      }
    }
  } catch {
    // Ignore form label check errors
  }

  // 7. Small touch target (< 44x44 CSS pixels per WCAG 2.5.8)
  try {
    if (interactiveTags.includes(tag) && typeof el.getBoundingClientRect === 'function') {
      const rect = el.getBoundingClientRect()
      if (rect.width > 0 && rect.height > 0 && (rect.width < 44 || rect.height < 44)) {
        flags.push(`small_touch_target:${Math.round(rect.width)}x${Math.round(rect.height)}`)
      }
    }
  } catch {
    // Ignore touch target check errors
  }

  return flags
}

/**
 * Parse an RGB/RGBA color string into {r, g, b, a}.
 * @param {string} str - e.g. "rgb(255, 0, 0)" or "rgba(0, 0, 0, 0.5)"
 * @returns {{r:number, g:number, b:number, a:number}|null}
 */
function parseRGBColor(str) {
  if (!str) return null
  const m = str.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*([\d.]+))?\)/)
  if (!m) return null
  return {
    r: parseInt(m[1], 10),
    g: parseInt(m[2], 10),
    b: parseInt(m[3], 10),
    a: m[4] !== undefined ? parseFloat(m[4]) : 1
  }
}

/**
 * Calculate relative luminance of an sRGB color per WCAG 2.x.
 * @param {{r:number, g:number, b:number}} c
 * @returns {number}
 */
function luminance(c) {
  const [rs, gs, bs] = [c.r / 255, c.g / 255, c.b / 255].map((v) =>
    v <= 0.04045 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4)
  )
  return 0.2126 * rs + 0.7152 * gs + 0.0722 * bs
}

/**
 * Calculate WCAG contrast ratio between two colors.
 * @param {{r:number, g:number, b:number}} fg
 * @param {{r:number, g:number, b:number}} bg
 * @returns {number}
 */
function contrastRatio(fg, bg) {
  const l1 = luminance(fg)
  const l2 = luminance(bg)
  const lighter = Math.max(l1, l2)
  const darker = Math.min(l1, l2)
  return (lighter + 0.05) / (darker + 0.05)
}

/**
 * Build a CSS selector for the element.
 */
function buildCSSSelector(el) {
  const tag = el.tagName.toLowerCase()
  if (el.id) return `${tag}#${CSS.escape(el.id)}`
  const classes = Array.from(el.classList).slice(0, 3)
  if (classes.length > 0) return `${tag}.${classes.map((c) => CSS.escape(c)).join('.')}`
  return tag
}

/**
 * Trace CSS rules that match an element using document.styleSheets.
 * Returns matched rules with selector, properties, and source stylesheet.
 * Capped at MAX_MATCHED_RULES to avoid huge payloads.
 */
const MAX_MATCHED_RULES = 20
const MAX_RULES_EXAMINED = 5000 // Safety cap to prevent excessive work on huge stylesheets

function traceMatchedCSSRules(el) {
  const rules = []
  let totalExamined = 0
  try {
    for (const sheet of document.styleSheets) {
      if (totalExamined >= MAX_RULES_EXAMINED) break
      let cssRules
      try {
        cssRules = sheet.cssRules || sheet.rules
      } catch {
        // CORS blocks access to cross-origin stylesheets
        rules.push({
          stylesheet: sheet.href || '(inline)',
          access: 'blocked',
          note: 'Cross-origin stylesheet — rules not accessible'
        })
        continue
      }
      if (!cssRules) continue

      const sheetHref = sheet.href || '(inline)'
      for (let i = 0; i < cssRules.length; i++) {
        if (rules.length >= MAX_MATCHED_RULES) break
        if (++totalExamined > MAX_RULES_EXAMINED) break
        const rule = cssRules[i]
        if (rule.type !== CSSRule.STYLE_RULE) continue
        try {
          if (!el.matches(rule.selectorText)) continue
        } catch {
          continue // Invalid selector
        }
        // Extract only properties that differ from defaults (non-empty)
        const properties = {}
        for (let j = 0; j < rule.style.length; j++) {
          const prop = rule.style[j]
          properties[prop] = rule.style.getPropertyValue(prop)
          const priority = rule.style.getPropertyPriority(prop)
          if (priority) properties[prop] += ' !' + priority
        }
        rules.push({
          selector: rule.selectorText,
          properties,
          stylesheet: sheetHref,
          rule_index: i
        })
      }
      if (rules.length >= MAX_MATCHED_RULES) break
    }
  } catch {
    // Stylesheet enumeration may fail in rare cases
  }
  return rules
}

/**
 * Detect framework component information for an element.
 * Supports React, Vue, Angular, and common data attributes.
 */
function detectComponentSource(el) {
  const info = {}

  // React: __reactFiber$ or __reactInternalInstance$
  try {
    for (const key of Object.keys(el)) {
      if (key.startsWith('__reactFiber$') || key.startsWith('__reactInternalInstance$')) {
        const fiber = el[key]
        if (fiber) {
          info.framework = 'react'
          // Walk up to find the named component
          let node = fiber
          for (let depth = 0; depth < 10 && node; depth++) {
            if (typeof node.type === 'function' || typeof node.type === 'object') {
              const name = node.type?.displayName || node.type?.name || node.type?.render?.name
              if (name) {
                info.component = name
                // Try to get source file from _source (dev mode only)
                if (node._debugSource) {
                  info.source_file = node._debugSource.fileName
                  info.source_line = node._debugSource.lineNumber
                }
                break
              }
            }
            node = node.return
          }
        }
        break
      }
    }
  } catch {
    // React internals may throw
  }

  // Vue 2: __vue__ / Vue 3: __vue_app__ or __vueParentComponent
  if (!info.framework) {
    try {
      const vue = el.__vue__ || el.__vueParentComponent
      if (vue) {
        info.framework = 'vue'
        info.component = vue.$options?.name || vue.type?.name || vue.type?.__name || ''
        if (vue.$options?.__file) info.source_file = vue.$options.__file
        if (vue.type?.__file) info.source_file = vue.type.__file
      }
    } catch {
      // Vue internals may throw
    }
  }

  // Angular: ng-* attributes
  if (!info.framework) {
    try {
      for (const attr of el.attributes) {
        if (attr.name.startsWith('_ngcontent') || attr.name.startsWith('_nghost')) {
          info.framework = 'angular'
          // Try to get component name from ng-reflect-* or constructor
          const ngComponent = el.__ngContext__
          if (ngComponent) {
            info.component = el.constructor?.name || ''
          }
          break
        }
      }
    } catch {
      // Angular detection may fail
    }
  }

  // Common data attributes
  try {
    const testId = el.getAttribute('data-testid') || el.getAttribute('data-test-id') || el.getAttribute('data-cy')
    if (testId) info.test_id = testId
    const component = el.getAttribute('data-component') || el.getAttribute('data-source')
    if (component) info.data_component = component
  } catch {
    // Attribute access may fail
  }

  return Object.keys(info).length > 0 ? info : null
}

// ============================================================================
// PERSISTENCE (chrome.storage.session)
// ============================================================================

const MAX_PERSISTED_ANNOTATIONS = 50

function persistAnnotations() {
  if (saveTimeout) clearTimeout(saveTimeout)
  saveTimeout = setTimeout(() => {
    if (typeof chrome !== 'undefined' && chrome.storage?.session) {
      try {
        const key = 'gasoline_draw_annotations'
        // Cap stored annotations to prevent quota overflow
        const toStore =
          annotations.length > MAX_PERSISTED_ANNOTATIONS ? annotations.slice(-MAX_PERSISTED_ANNOTATIONS) : annotations
        chrome.storage.session.set(
          {
            [key]: {
              annotations: toStore,
              page_url: window.location.href,
              timestamp: Date.now()
            }
          },
          () => {
            if (chrome.runtime.lastError) {
              console.warn('[gasoline] Draw mode storage error:', chrome.runtime.lastError.message)
            }
          }
        )
      } catch {
        // Storage may not be available in all contexts
      }
    }
  }, 500) // Debounce 500ms
}

function clearPersistedAnnotations() {
  if (typeof chrome === 'undefined' || !chrome.storage?.session) return
  try {
    chrome.storage.session.remove('gasoline_draw_annotations')
  } catch {
    // Storage may not be available
  }
}

function loadAnnotations() {
  if (typeof chrome === 'undefined' || !chrome.storage?.session) return
  try {
    const key = 'gasoline_draw_annotations'
    chrome.storage.session.get([key], (result) => {
      if (chrome.runtime.lastError) {
        console.warn('[gasoline] Draw mode load error:', chrome.runtime.lastError.message)
        return
      }
      const data = result?.[key]
      if (data?.annotations && data.page_url === window.location.href) {
        annotations = data.annotations
        renderAnnotations()
      }
    })
  } catch {
    // Ignore storage read errors
  }
}

// ============================================================================
// DEACTIVATION + RESULT DELIVERY
// ============================================================================

/**
 * Called when user presses ESC (no active text input), from popup toggle,
 * or from GASOLINE_DRAW_MODE_STOP message.
 * Captures screenshot WHILE overlay is still visible, then deactivates and sends results.
 * Protected by re-entry guard to prevent double-ESC races.
 */
export function deactivateAndSendResults() {
  if (!active || isDeactivating) return
  isDeactivating = true
  const pageUrl = window.location.href
  const currentSessionName = sessionName // capture before deactivate clears it
  const currentCorrelationId = sessionCorrelationId // capture before deactivate clears it

  /**
   * Complete the deactivation: fade out overlay, show toast, tear down,
   * send results to background, and clear persisted storage.
   */
  const finishDeactivation = (screenshotDataUrl) => {
    // Fade out the overlay before tearing it down
    if (overlay) {
      overlay.style.opacity = '0'
    }

    // Show success toast via extension messaging
    try {
      if (typeof chrome !== 'undefined' && chrome.runtime) {
        chrome.runtime.sendMessage({
          type: 'GASOLINE_ACTION_TOAST',
          text: 'Annotations submitted',
          state: 'success',
          duration_ms: 2000
        })
      }
    } catch {
      // Extension context may be invalidated
    }

    // Delay teardown to let fade complete
    setTimeout(() => {
      const result = deactivateDrawMode()
      isDeactivating = false
      // Clear persisted annotations from storage after successful deactivation
      clearPersistedAnnotations()
      try {
        if (typeof chrome !== 'undefined' && chrome.runtime) {
          const msg = {
            type: 'DRAW_MODE_COMPLETED',
            annotations: result.annotations,
            elementDetails: result.elementDetails,
            page_url: pageUrl,
            screenshot_data_url: screenshotDataUrl,
            correlation_id: currentCorrelationId
          }
          if (currentSessionName) {
            msg.session_name = currentSessionName
          }
          chrome.runtime.sendMessage(msg)
        }
      } catch {
        // Extension context may be invalidated
      }
    }, 300)
  }

  // Re-capture DOM data for all annotations so element details match the screenshot moment.
  // Annotations may have been drawn minutes ago; the DOM may have changed since then.
  refreshElementDetails()

  // Request screenshot capture from background BEFORE deactivating,
  // so the overlay with annotation drawings is included in the screenshot.
  if (typeof chrome !== 'undefined' && chrome.runtime) {
    let screenshotHandled = false
    // Timeout fallback: if screenshot callback never fires (extension context
    // invalidated, background unresponsive), proceed without screenshot after 3s.
    const fallbackTimer = setTimeout(() => {
      if (!screenshotHandled) {
        screenshotHandled = true
        finishDeactivation('')
      }
    }, 3000)

    try {
      chrome.runtime.sendMessage({ type: 'GASOLINE_CAPTURE_SCREENSHOT' }, (screenshotResponse) => {
        if (screenshotHandled) return // Timeout already fired
        screenshotHandled = true
        clearTimeout(fallbackTimer)
        finishDeactivation(screenshotResponse?.dataUrl || '')
      })
    } catch {
      // Fallback: deactivate without screenshot
      if (!screenshotHandled) {
        screenshotHandled = true
        clearTimeout(fallbackTimer)
        finishDeactivation('')
      }
    }
  } else {
    deactivateDrawMode()
    isDeactivating = false
  }
}

// ============================================================================
// UTILITY
// ============================================================================

function normalizeRect(x1, y1, x2, y2) {
  return {
    x: Math.min(x1, x2),
    y: Math.min(y1, y2),
    width: Math.abs(x2 - x1),
    height: Math.abs(y2 - y1)
  }
}
