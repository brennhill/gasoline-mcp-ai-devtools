/**
 * Shared MCP helpers for E2E tests.
 *
 * Provides mcpCall, mcpToolText, and entryContains used across multiple spec files.
 */

/**
 * Call an MCP method via the HTTP endpoint
 * @param {string} serverUrl - The server base URL
 * @param {string} method - MCP method name
 * @param {Object} params - Method parameters
 * @returns {Promise<Object>} Parsed JSON response
 */
export async function mcpCall(serverUrl, method, params = {}) {
  const response = await fetch(`${serverUrl}/mcp`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      jsonrpc: '2.0',
      id: Date.now(),
      method,
      params,
    }),
  })
  return response.json()
}

/**
 * Call an MCP tool and return the raw text content
 * @param {string} serverUrl - The server base URL
 * @param {string} toolName - Tool name to call
 * @param {Object} args - Tool arguments
 * @returns {Promise<string>} Text content from the response
 */
export async function mcpToolText(serverUrl, toolName, args = {}) {
  const resp = await mcpCall(serverUrl, 'tools/call', { name: toolName, arguments: args })
  if (resp.error) throw new Error(`MCP error: ${JSON.stringify(resp.error)}`)
  const content = resp.result.content
  if (!content || content.length === 0) return ''
  return content[0].text
}

/**
 * Check if a log entry contains a string in its args or message
 * @param {Object} entry - Log entry to check
 * @param {string} text - Text to search for
 * @returns {boolean} True if the entry contains the text
 */
export function entryContains(entry, text) {
  // Exception entries have a message field
  if (entry.message && entry.message.includes(text)) return true
  // Console entries have an args array
  if (entry.args) {
    return entry.args.some((arg) => {
      if (typeof arg === 'string') return arg.includes(text)
      if (typeof arg === 'object' && arg !== null) {
        return JSON.stringify(arg).includes(text)
      }
      return false
    })
  }
  return false
}
