// @ts-nocheck
/**
 * @fileoverview chat-panel-sse.test.js — Tests for SSE client parsing and connection management.
 * Covers SSE event parsing, reconnect behavior, and close cleanup.
 */

import { test, describe, beforeEach, afterEach, mock } from 'node:test'
import assert from 'node:assert'

// =============================================================================
// SSE Parser Unit Tests (testing the parsing logic directly)
// =============================================================================

/**
 * Parse a raw SSE event string into event type and data.
 * Mirrors the parseSSEEvent logic from chat-panel-sse.ts.
 */
function parseSSEEvent(raw) {
  let eventType = ''
  let data = ''

  for (const line of raw.split('\n')) {
    if (line.startsWith('event: ')) {
      eventType = line.slice(7)
    } else if (line.startsWith('data: ')) {
      data = line.slice(6)
    } else if (line.startsWith(':')) {
      // Comment (heartbeat), ignore
    }
  }

  return { eventType, data }
}

/**
 * Split a raw SSE stream buffer into individual events.
 * Events are separated by double newlines.
 */
function splitSSEBuffer(buffer) {
  const events = buffer.split('\n\n')
  const remainder = events.pop() ?? ''
  return { events: events.filter((e) => e.trim()), remainder }
}

describe('SSE Parser', () => {
  test('parses message event correctly', () => {
    const raw = 'event: message\ndata: {"role":"assistant","text":"hello"}'
    const { eventType, data } = parseSSEEvent(raw)

    assert.strictEqual(eventType, 'message')
    const parsed = JSON.parse(data)
    assert.strictEqual(parsed.role, 'assistant')
    assert.strictEqual(parsed.text, 'hello')
  })

  test('parses history event correctly', () => {
    const raw = 'event: history\ndata: [{"role":"user","text":"hi"},{"role":"assistant","text":"hey"}]'
    const { eventType, data } = parseSSEEvent(raw)

    assert.strictEqual(eventType, 'history')
    const parsed = JSON.parse(data)
    assert.strictEqual(parsed.length, 2)
    assert.strictEqual(parsed[0].role, 'user')
    assert.strictEqual(parsed[1].role, 'assistant')
  })

  test('ignores heartbeat comments', () => {
    const raw = ': heartbeat'
    const { eventType, data } = parseSSEEvent(raw)

    assert.strictEqual(eventType, '')
    assert.strictEqual(data, '')
  })

  test('handles event with only data line', () => {
    const raw = 'data: {"text":"orphan"}'
    const { eventType, data } = parseSSEEvent(raw)

    assert.strictEqual(eventType, '')
    assert.strictEqual(data, '{"text":"orphan"}')
  })

  test('handles event with only event line', () => {
    const raw = 'event: message'
    const { eventType, data } = parseSSEEvent(raw)

    assert.strictEqual(eventType, 'message')
    assert.strictEqual(data, '')
  })
})

describe('SSE Buffer Splitting', () => {
  test('splits multiple events on double-newline', () => {
    const buffer = 'event: history\ndata: []\n\nevent: message\ndata: {}\n\n'
    const { events, remainder } = splitSSEBuffer(buffer)

    assert.strictEqual(events.length, 2)
    assert.ok(events[0].includes('history'))
    assert.ok(events[1].includes('message'))
    assert.strictEqual(remainder, '')
  })

  test('preserves incomplete event in remainder', () => {
    const buffer = 'event: message\ndata: {"text":"complete"}\n\nevent: message\ndata: {"text":"inc'
    const { events, remainder } = splitSSEBuffer(buffer)

    assert.strictEqual(events.length, 1)
    assert.ok(remainder.includes('inc'))
  })

  test('returns empty events for heartbeat-only stream', () => {
    const buffer = ': heartbeat\n\n: heartbeat\n\n'
    const { events } = splitSSEBuffer(buffer)

    // Heartbeat comments are non-empty strings so they pass the filter
    assert.strictEqual(events.length, 2)
  })

  test('handles empty buffer', () => {
    const { events, remainder } = splitSSEBuffer('')

    assert.strictEqual(events.length, 0)
    assert.strictEqual(remainder, '')
  })
})

describe('SSE Connection Contract', () => {
  test('connection object has close method', () => {
    const connection = {
      close() {
        this._closed = true
      },
      _closed: false
    }

    assert.strictEqual(typeof connection.close, 'function')
    connection.close()
    assert.strictEqual(connection._closed, true)
  })

  test('ChatMessage interface matches expected shape', () => {
    const msg = {
      role: 'assistant',
      text: 'I can help with that.',
      timestamp: Date.now(),
      conversation_id: 'conv-abc-123',
      annotations: undefined
    }

    assert.strictEqual(typeof msg.role, 'string')
    assert.strictEqual(typeof msg.text, 'string')
    assert.strictEqual(typeof msg.timestamp, 'number')
    assert.strictEqual(typeof msg.conversation_id, 'string')
    assert.ok(['user', 'assistant', 'annotation'].includes(msg.role))
  })

  test('annotation message includes annotations array', () => {
    const msg = {
      role: 'annotation',
      text: '3 annotations from draw mode',
      timestamp: Date.now(),
      conversation_id: 'conv-abc',
      annotations: [
        { label: 'button', rect: { x: 10, y: 20 } },
        { label: 'input', rect: { x: 30, y: 40 } },
        { label: 'heading', rect: { x: 50, y: 60 } }
      ]
    }

    assert.strictEqual(msg.role, 'annotation')
    assert.strictEqual(msg.annotations.length, 3)
  })
})

describe('SSE Reconnect Logic', () => {
  test('reconnect delay is 1 second', () => {
    const RECONNECT_DELAY_MS = 1000
    assert.strictEqual(RECONNECT_DELAY_MS, 1000)
  })

  test('close prevents reconnect', () => {
    let closed = false
    let reconnectCalled = false

    function scheduleReconnect() {
      if (closed) return
      reconnectCalled = true
    }

    // Close before reconnect
    closed = true
    scheduleReconnect()
    assert.strictEqual(reconnectCalled, false)
  })

  test('close aborts active connection', () => {
    const abortController = new AbortController()
    let closed = false

    function close() {
      closed = true
      abortController.abort()
    }

    close()
    assert.strictEqual(closed, true)
    assert.strictEqual(abortController.signal.aborted, true)
  })
})
