/**
 * Purpose: Conversational side panel for in-page chat with AI.
 * Why: Enables bi-directional conversation — user messages, AI responses, and annotations visible in-browser.
 * Docs: docs/features/feature/chat-panel/index.md
 */

// chat-panel.ts — Right-side panel with message display, input, and draw mode integration.

import { connectChatStream, type ChatMessage, type SSEConnection } from './chat-panel-sse.js'

const PANEL_ID = 'gasoline-chat-panel'
const MESSAGES_ID = 'gasoline-chat-messages'
const INPUT_ID = 'gasoline-chat-panel-input'
const PANEL_WIDTH = '400px'

/** Active SSE connection */
let sseConnection: SSEConnection | null = null

/** Current conversation ID */
let conversationId = ''

/** Guard against rapid toggle race conditions */
let isPanelRemoving = false

/** Whether panel is currently visible */
let isPanelOpen = false

/** Generate a unique conversation ID */
function generateConversationId(): string {
  const ts = Date.now().toString(36)
  const rand = Math.random().toString(36).slice(2, 8)
  return `conv-${ts}-${rand}`
}

/**
 * Toggle the chat panel visibility.
 * If visible, closes it. If hidden, opens it.
 */
export function toggleChatPanel(serverUrl: string): void {
  const existing = document.getElementById(PANEL_ID)
  if (existing && !isPanelRemoving) {
    removeChatPanel()
  } else if (!existing && !isPanelRemoving) {
    showChatPanel(serverUrl)
  }
}

/** Show the chat panel on the right side of the page. */
function showChatPanel(serverUrl: string): void {
  if (document.getElementById(PANEL_ID)) return

  // Generate a new conversation ID for fresh sessions
  if (!conversationId) {
    conversationId = generateConversationId()
  }

  const panel = document.createElement('div')
  panel.id = PANEL_ID
  panel.setAttribute('role', 'dialog')
  panel.setAttribute('aria-label', 'Chat with AI')
  Object.assign(panel.style, {
    position: 'fixed',
    right: '0',
    top: '0',
    width: PANEL_WIDTH,
    height: '100vh',
    background: '#1a1a2e',
    borderLeft: '1px solid rgba(255, 255, 255, 0.08)',
    boxShadow: '-4px 0 24px rgba(0, 0, 0, 0.3)',
    zIndex: '2147483643',
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
    display: 'flex',
    flexDirection: 'column',
    transform: 'translateX(100%)',
    transition: 'transform 200ms ease',
    boxSizing: 'border-box'
  })

  // Stop keydown propagation to prevent page handlers
  panel.addEventListener('keydown', (e: KeyboardEvent) => {
    e.stopPropagation()
    if (e.key === 'Escape') {
      e.preventDefault()
      removeChatPanel()
    }
  })

  // Header
  const header = createHeader()
  panel.appendChild(header)

  // Messages area
  const messages = document.createElement('div')
  messages.id = MESSAGES_ID
  Object.assign(messages.style, {
    flex: '1',
    overflowY: 'auto',
    padding: '12px 16px',
    display: 'flex',
    flexDirection: 'column',
    gap: '8px'
  })
  panel.appendChild(messages)

  // Input area
  const inputArea = createInputArea(serverUrl)
  panel.appendChild(inputArea)

  const target = document.body || document.documentElement
  if (!target) return
  target.appendChild(panel)

  // Slide in
  requestAnimationFrame(() => {
    panel.style.transform = 'translateX(0)'
    const input = document.getElementById(INPUT_ID) as HTMLTextAreaElement | null
    if (input) input.focus()
  })

  isPanelOpen = true

  // Connect SSE — on history delivery (initial or reconnect), clear existing messages
  // to avoid duplicates, then re-render the full history.
  sseConnection = connectChatStream(
    serverUrl,
    conversationId,
    (msg) => renderMessage(msg),
    (msgs) => {
      const container = document.getElementById(MESSAGES_ID)
      if (container) container.innerHTML = ''
      for (const msg of msgs) renderMessage(msg)
    },
    (err) => renderSystemMessage(err)
  )
}

/** Create the header bar with title and close button. */
function createHeader(): HTMLDivElement {
  const header = document.createElement('div')
  Object.assign(header.style, {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '12px 16px',
    background: 'rgba(255, 255, 255, 0.04)',
    borderBottom: '1px solid rgba(255, 255, 255, 0.06)',
    flexShrink: '0'
  })

  const title = document.createElement('span')
  title.textContent = 'Chat with AI'
  Object.assign(title.style, {
    color: '#e0e0e0',
    fontSize: '13px',
    fontWeight: '600',
    letterSpacing: '0.3px'
  })
  header.appendChild(title)

  const controls = document.createElement('div')
  Object.assign(controls.style, { display: 'flex', alignItems: 'center', gap: '6px' })

  // Minimize button
  const minBtn = document.createElement('button')
  minBtn.textContent = '\u2013' // en-dash
  minBtn.title = 'Minimize'
  minBtn.setAttribute('aria-label', 'Minimize chat panel')
  Object.assign(minBtn.style, {
    background: 'transparent',
    border: 'none',
    color: '#999',
    fontSize: '16px',
    cursor: 'pointer',
    padding: '0 4px',
    lineHeight: '1'
  })
  minBtn.addEventListener('click', (e: MouseEvent) => {
    e.stopPropagation()
    removeChatPanel()
  })
  controls.appendChild(minBtn)

  // Close button
  const closeBtn = document.createElement('button')
  closeBtn.textContent = '\u00d7'
  closeBtn.title = 'Close'
  closeBtn.setAttribute('aria-label', 'Close chat panel')
  Object.assign(closeBtn.style, {
    background: 'transparent',
    border: 'none',
    color: '#999',
    fontSize: '16px',
    cursor: 'pointer',
    padding: '0 4px',
    lineHeight: '1'
  })
  closeBtn.addEventListener('click', (e: MouseEvent) => {
    e.stopPropagation()
    // Close resets conversation
    conversationId = ''
    removeChatPanel()
  })
  controls.appendChild(closeBtn)

  header.appendChild(controls)
  return header
}

/** Create the input area with draw button, textarea, and send button. */
function createInputArea(serverUrl: string): HTMLDivElement {
  const area = document.createElement('div')
  Object.assign(area.style, {
    padding: '12px 16px',
    borderTop: '1px solid rgba(255, 255, 255, 0.06)',
    background: 'rgba(255, 255, 255, 0.02)',
    flexShrink: '0'
  })

  // Action bar (draw button)
  const actionBar = document.createElement('div')
  Object.assign(actionBar.style, {
    display: 'flex',
    alignItems: 'center',
    gap: '8px',
    marginBottom: '8px'
  })

  const drawBtn = document.createElement('button')
  drawBtn.textContent = '\u270f Draw'
  drawBtn.title = 'Draw annotations on the page'
  drawBtn.setAttribute('aria-label', 'Start draw mode for annotations')
  Object.assign(drawBtn.style, {
    background: 'rgba(255, 255, 255, 0.06)',
    border: '1px solid rgba(255, 255, 255, 0.1)',
    borderRadius: '6px',
    color: '#e0e0e0',
    fontSize: '12px',
    cursor: 'pointer',
    padding: '4px 10px',
    transition: 'all 0.15s ease'
  })
  drawBtn.addEventListener('click', (e: MouseEvent) => {
    e.stopPropagation()
    // Trigger draw mode via background
    chrome.runtime.sendMessage({ type: 'GASOLINE_DRAW_MODE_START', started_by: 'chat_panel' })
  })
  actionBar.appendChild(drawBtn)
  area.appendChild(actionBar)

  // Input row
  const inputRow = document.createElement('div')
  Object.assign(inputRow.style, {
    display: 'flex',
    alignItems: 'flex-end',
    gap: '8px'
  })

  const input = document.createElement('textarea')
  input.id = INPUT_ID
  input.placeholder = 'Type a message...'
  input.rows = 1
  input.maxLength = 10000
  input.setAttribute('aria-label', 'Chat message')
  Object.assign(input.style, {
    flex: '1',
    background: 'rgba(255, 255, 255, 0.06)',
    border: '1px solid rgba(255, 255, 255, 0.1)',
    borderRadius: '8px',
    color: '#e0e0e0',
    fontSize: '13px',
    lineHeight: '1.5',
    padding: '8px 12px',
    resize: 'none',
    outline: 'none',
    fontFamily: 'inherit',
    boxSizing: 'border-box',
    minHeight: '36px',
    maxHeight: '120px',
    transition: 'border-color 0.15s ease'
  })

  input.addEventListener('focus', () => {
    input.style.borderColor = 'rgba(59, 130, 246, 0.5)'
  })
  input.addEventListener('blur', () => {
    input.style.borderColor = 'rgba(255, 255, 255, 0.1)'
  })

  // Auto-resize textarea
  input.addEventListener('input', () => {
    input.style.height = 'auto'
    input.style.height = Math.min(input.scrollHeight, 120) + 'px'
  })

  // Enter to send, Shift+Enter for newline
  input.addEventListener('keydown', (e: KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      sendMessage(serverUrl)
    }
  })

  inputRow.appendChild(input)

  const sendBtn = document.createElement('button')
  sendBtn.textContent = 'Send'
  sendBtn.setAttribute('aria-label', 'Send message')
  Object.assign(sendBtn.style, {
    background: 'rgba(59, 130, 246, 0.3)',
    border: '1px solid rgba(59, 130, 246, 0.5)',
    borderRadius: '8px',
    color: '#60a5fa',
    fontSize: '12px',
    fontWeight: '600',
    cursor: 'pointer',
    padding: '8px 14px',
    transition: 'all 0.15s ease',
    flexShrink: '0'
  })
  sendBtn.addEventListener('click', (e: MouseEvent) => {
    e.stopPropagation()
    sendMessage(serverUrl)
  })
  inputRow.appendChild(sendBtn)

  area.appendChild(inputRow)

  // Keyboard hint
  const hint = document.createElement('div')
  hint.textContent = 'Enter send \u00b7 Shift+Enter newline \u00b7 Esc close'
  Object.assign(hint.style, {
    fontSize: '10px',
    color: '#666',
    marginTop: '6px',
    textAlign: 'center'
  })
  area.appendChild(hint)

  return area
}

/** Remove the chat panel with slide-out animation. */
function removeChatPanel(): void {
  if (isPanelRemoving) return
  const panel = document.getElementById(PANEL_ID)
  if (!panel) return

  isPanelRemoving = true
  isPanelOpen = false
  panel.style.transform = 'translateX(100%)'

  // Disconnect SSE
  if (sseConnection) {
    sseConnection.close()
    sseConnection = null
  }

  setTimeout(() => {
    panel.remove()
    isPanelRemoving = false
  }, 200)
}

/** Send a message via the background push handler. */
function sendMessage(serverUrl: string): void {
  const input = document.getElementById(INPUT_ID) as HTMLTextAreaElement | null
  if (!input) return

  const message = input.value.trim()
  if (!message) {
    input.style.borderColor = 'rgba(239, 68, 68, 0.5)'
    setTimeout(() => {
      input.style.borderColor = 'rgba(59, 130, 246, 0.5)'
    }, 600)
    return
  }

  // Render the user's message immediately (optimistic)
  renderMessage({
    role: 'user',
    text: message,
    timestamp: Date.now(),
    conversation_id: conversationId
  })

  input.value = ''
  input.style.height = 'auto'
  input.disabled = true

  chrome.runtime.sendMessage(
    {
      type: 'GASOLINE_PUSH_CHAT',
      message,
      page_url: window.location.href,
      conversation_id: conversationId,
      server_url: serverUrl
    },
    (response: { success: boolean; error?: string } | undefined) => {
      input.disabled = false
      input.focus()

      if (chrome.runtime.lastError || !response?.success) {
        renderSystemMessage(response?.error || 'Failed to send message')
      }
    }
  )
}

/** Render a chat message bubble in the messages area. */
function renderMessage(msg: ChatMessage): void {
  const container = document.getElementById(MESSAGES_ID)
  if (!container) return

  const bubble = document.createElement('div')

  if (msg.role === 'annotation') {
    // Annotation card
    Object.assign(bubble.style, {
      background: 'rgba(168, 85, 247, 0.1)',
      border: '1px solid rgba(168, 85, 247, 0.2)',
      borderRadius: '8px',
      padding: '8px 12px',
      fontSize: '12px',
      color: '#c084fc',
      alignSelf: 'stretch'
    })
    const icon = document.createElement('span')
    icon.textContent = '\ud83d\udccc '
    bubble.appendChild(icon)
    const text = document.createElement('span')
    text.textContent = msg.text
    bubble.appendChild(text)
  } else if (msg.role === 'user') {
    // User message — right-aligned blue
    Object.assign(bubble.style, {
      background: 'rgba(59, 130, 246, 0.15)',
      border: '1px solid rgba(59, 130, 246, 0.2)',
      borderRadius: '12px 12px 4px 12px',
      padding: '8px 12px',
      fontSize: '13px',
      color: '#e0e0e0',
      alignSelf: 'flex-end',
      maxWidth: '85%',
      wordBreak: 'break-word',
      whiteSpace: 'pre-wrap'
    })
    bubble.textContent = msg.text
  } else {
    // Assistant message — left-aligned gray
    Object.assign(bubble.style, {
      background: 'rgba(255, 255, 255, 0.06)',
      border: '1px solid rgba(255, 255, 255, 0.08)',
      borderRadius: '12px 12px 12px 4px',
      padding: '8px 12px',
      fontSize: '13px',
      color: '#e0e0e0',
      alignSelf: 'flex-start',
      maxWidth: '85%',
      wordBreak: 'break-word',
      whiteSpace: 'pre-wrap'
    })
    bubble.textContent = msg.text
  }

  container.appendChild(bubble)
  container.scrollTop = container.scrollHeight
}

/** Render a system/error message. */
function renderSystemMessage(text: string): void {
  const container = document.getElementById(MESSAGES_ID)
  if (!container) return

  const el = document.createElement('div')
  Object.assign(el.style, {
    fontSize: '11px',
    color: '#f87171',
    textAlign: 'center',
    padding: '4px 0'
  })
  el.textContent = text
  container.appendChild(el)
  container.scrollTop = container.scrollHeight
}

/**
 * Inject annotation data into the chat panel as a message.
 * Called when draw mode completes while the panel is open.
 */
export function injectAnnotationMessage(data: { annotation_count: number; details?: string }): void {
  if (!isPanelOpen) return
  renderMessage({
    role: 'annotation',
    text: `${data.annotation_count} annotations from draw mode`,
    timestamp: Date.now(),
    conversation_id: conversationId
  })
}

/** Check if the chat panel is currently open. */
export function isChatPanelOpen(): boolean {
  return isPanelOpen
}
