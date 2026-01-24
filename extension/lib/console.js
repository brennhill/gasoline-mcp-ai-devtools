// @ts-nocheck
/**
 * @fileoverview Console method capture.
 * Monkey-patches console.log/warn/error/info/debug to capture messages
 * and forward them via postLog, while preserving original behavior.
 */

import { safeSerialize } from './serialize.js'
import { postLog } from './bridge.js'

// Store original methods
let originalConsole = {}

/**
 * Install console capture hooks
 */
export function installConsoleCapture() {
  const methods = ['log', 'warn', 'error', 'info', 'debug']

  methods.forEach((method) => {
    originalConsole[method] = console[method]

    console[method] = function (...args) {
      // Post to extension
      postLog({
        level: method,
        type: 'console',
        args: args.map((arg) => safeSerialize(arg)),
      })

      // Call original
      originalConsole[method].apply(console, args)
    }
  })
}

/**
 * Uninstall console capture hooks
 */
export function uninstallConsoleCapture() {
  Object.keys(originalConsole).forEach((method) => {
    console[method] = originalConsole[method]
  })
  originalConsole = {}
}
