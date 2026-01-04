// @ts-nocheck
/**
 * @fileoverview Shared constants for the Gasoline extension capture modules.
 */

// Serialization limits
export const MAX_STRING_LENGTH = 10240 // 10KB
export const MAX_RESPONSE_LENGTH = 5120 // 5KB
export const MAX_DEPTH = 10
export const MAX_CONTEXT_SIZE = 50 // Max number of context keys
export const MAX_CONTEXT_VALUE_SIZE = 4096 // Max size of serialized context value
export const SENSITIVE_HEADERS = ['authorization', 'cookie', 'set-cookie', 'x-auth-token']

// User action replay settings
export const MAX_ACTION_BUFFER_SIZE = 20 // Max number of recent actions to keep
export const SCROLL_THROTTLE_MS = 250 // Throttle scroll events
export const SENSITIVE_INPUT_TYPES = ['password', 'credit-card', 'cc-number', 'cc-exp', 'cc-csc']

// Network Waterfall settings
export const MAX_WATERFALL_ENTRIES = 50 // Max network entries to capture
export const WATERFALL_TIME_WINDOW_MS = 30000 // Only capture last 30 seconds

// Performance Marks settings
export const MAX_PERFORMANCE_ENTRIES = 50 // Max performance entries to capture
export const PERFORMANCE_TIME_WINDOW_MS = 60000 // Only capture last 60 seconds

// WebSocket capture settings
export const WS_MAX_BODY_SIZE = 4096 // 4KB truncation limit
export const WS_PREVIEW_LIMIT = 200 // Preview character limit

// Network body capture settings
export const REQUEST_BODY_MAX = 8192 // 8KB
export const RESPONSE_BODY_MAX = 16384 // 16KB
export const BODY_READ_TIMEOUT_MS = 5
export const SENSITIVE_HEADER_PATTERNS =
  /^(authorization|cookie|set-cookie|x-api-key|x-auth-token|x-secret|x-password|.*token.*|.*secret.*|.*key.*|.*password.*)$/i
export const BINARY_CONTENT_TYPES = /^(image|video|audio|font)\/|^application\/(wasm|octet-stream|zip|gzip|pdf)/

// DOM query settings
export const DOM_QUERY_MAX_ELEMENTS = 50
export const DOM_QUERY_MAX_TEXT = 500
export const DOM_QUERY_MAX_DEPTH = 5
export const DOM_QUERY_MAX_HTML = 200
export const A11Y_MAX_NODES_PER_VIOLATION = 10
export const A11Y_AUDIT_TIMEOUT_MS = 30000

// Memory pressure settings
export const MEMORY_SOFT_LIMIT_MB = 20
export const MEMORY_HARD_LIMIT_MB = 50

// AI Context settings
export const AI_CONTEXT_SNIPPET_LINES = 5 // Lines before and after error
export const AI_CONTEXT_MAX_LINE_LENGTH = 200 // Truncate lines
export const AI_CONTEXT_MAX_SNIPPETS_SIZE = 10240 // 10KB total snippets
export const AI_CONTEXT_MAX_ANCESTRY_DEPTH = 10
export const AI_CONTEXT_MAX_PROP_KEYS = 20
export const AI_CONTEXT_MAX_STATE_KEYS = 10
export const AI_CONTEXT_MAX_RELEVANT_SLICE = 10
export const AI_CONTEXT_MAX_VALUE_LENGTH = 200
export const AI_CONTEXT_SOURCE_MAP_CACHE_SIZE = 20
export const AI_CONTEXT_PIPELINE_TIMEOUT_MS = 3000

// Reproduction script settings
export const ENHANCED_ACTION_BUFFER_SIZE = 50
export const CSS_PATH_MAX_DEPTH = 5
export const SELECTOR_TEXT_MAX_LENGTH = 50
export const SCRIPT_MAX_SIZE = 51200 // 50KB
export const CLICKABLE_TAGS = new Set(['BUTTON', 'A', 'SUMMARY'])

// Actionable keys for recording
export const ACTIONABLE_KEYS = new Set([
  'Enter',
  'Escape',
  'Tab',
  'ArrowUp',
  'ArrowDown',
  'ArrowLeft',
  'ArrowRight',
  'Backspace',
  'Delete',
])

// Performance snapshot settings
export const MAX_LONG_TASKS = 50
export const MAX_SLOWEST_REQUESTS = 3
export const MAX_URL_LENGTH = 80
