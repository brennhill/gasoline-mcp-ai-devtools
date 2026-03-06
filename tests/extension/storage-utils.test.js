// @ts-nocheck
/**
 * @fileoverview storage-utils.test.js -- Tests for storage-utils module.
 * Covers session storage CRUD, local storage CRUD, facade functions,
 * graceful degradation when APIs are unavailable, diagnostics, and
 * service worker restart detection.
 *
 * Chrome storage mocks include remove() for local, sync, and session --
 * both callback and Promise patterns (known project requirement).
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

// =============================================================================
// MOCK FACTORY: builds a mock chrome with configurable storage behavior
// =============================================================================

/**
 * Create a fresh chrome mock with working storage simulation.
 * Each storage area (local, session) maintains an in-memory store that
 * correctly dispatches callbacks, including the remove() method.
 */
function createStorageMock() {
  function makeArea() {
    let store = {}
    return {
      _store: store,
      get: mock.fn((keys, callback) => {
        const result = {}
        const keyArr = Array.isArray(keys) ? keys : [keys]
        for (const k of keyArr) {
          if (store[k] !== undefined) result[k] = store[k]
        }
        if (typeof callback === 'function') callback(result)
        else return Promise.resolve(result)
      }),
      set: mock.fn((data, callback) => {
        Object.assign(store, data)
        if (typeof callback === 'function') callback()
        else return Promise.resolve()
      }),
      remove: mock.fn((keys, callback) => {
        const keyArr = Array.isArray(keys) ? keys : [keys]
        for (const k of keyArr) {
          delete store[k]
        }
        if (typeof callback === 'function') callback()
        else return Promise.resolve()
      }),
      clear: mock.fn((callback) => {
        store = {}
        // Re-assign _store so external references pick up the new empty store.
        // Internal closures still reference the old `store`, but that's fine
        // because this mock is recreated per test.
        if (typeof callback === 'function') callback()
        else return Promise.resolve()
      })
    }
  }

  const local = makeArea()
  const session = makeArea()

  return {
    runtime: {
      lastError: null,
      onMessage: { addListener: mock.fn() },
      sendMessage: mock.fn(() => Promise.resolve()),
      getManifest: () => ({ version: '6.0.3' })
    },
    action: { setBadgeText: mock.fn(), setBadgeBackgroundColor: mock.fn() },
    storage: {
      local,
      sync: {
        get: mock.fn((k, cb) => cb && cb({})),
        set: mock.fn((d, cb) => cb && cb()),
        remove: mock.fn((k, cb) => {
          if (typeof cb === 'function') cb()
          else return Promise.resolve()
        })
      },
      session,
      onChanged: { addListener: mock.fn() }
    },
    tabs: {
      get: mock.fn(),
      query: mock.fn(),
      onRemoved: { addListener: mock.fn() }
    },
    alarms: { create: mock.fn(), onAlarm: { addListener: mock.fn() } }
  }
}

// We need to set up the chrome mock BEFORE importing the module because
// storage-utils checks chrome availability at import time.
let chromeMock = createStorageMock()
globalThis.chrome = chromeMock

// Mock navigator for getStorageDiagnostics
// navigator is a read-only getter in modern Node.js, so use defineProperty
if (!globalThis.navigator || !globalThis.navigator.userAgent) {
  Object.defineProperty(globalThis, 'navigator', {
    value: { userAgent: 'TestAgent/1.0' },
    writable: true,
    configurable: true
  })
}

// Dynamic import to allow re-importing with different chrome mocks
let storageUtils
async function loadModule() {
  // We can't truly re-import ESM easily, so we import once and test against
  // the live chrome global that the module references.
  if (!storageUtils) {
    storageUtils = await import('../../extension/background/storage-utils.js')
  }
  return storageUtils
}

// =============================================================================
// SESSION STORAGE
// =============================================================================

describe('Session Storage', () => {
  beforeEach(() => {
    chromeMock = createStorageMock()
    globalThis.chrome = chromeMock
  })

  test('setSessionValue should store value in session storage', async () => {
    const mod = await loadModule()
    await new Promise((resolve) => {
      mod.setSessionValue('testKey', 'testValue', () => {
        resolve()
      })
    })
    assert.strictEqual(chromeMock.storage.session.set.mock.calls.length, 1)
    const setArg = chromeMock.storage.session.set.mock.calls[0].arguments[0]
    assert.deepStrictEqual(setArg, { testKey: 'testValue' })
  })

  test('setSessionValue should work without callback', async () => {
    const mod = await loadModule()
    // Should not throw even without callback
    assert.doesNotThrow(() => {
      mod.setSessionValue('key', 'val')
    })
  })

  test('getSessionValue should retrieve value from session storage', async () => {
    const mod = await loadModule()
    // Pre-populate mock
    chromeMock.storage.session.get = mock.fn((keys, callback) => {
      callback({ myKey: 42 })
    })
    await new Promise((resolve) => {
      mod.getSessionValue('myKey', (value) => {
        assert.strictEqual(value, 42)
        resolve()
      })
    })
  })

  test('getSessionValue should return undefined for missing key', async () => {
    const mod = await loadModule()
    chromeMock.storage.session.get = mock.fn((keys, callback) => {
      callback({})
    })
    await new Promise((resolve) => {
      mod.getSessionValue('missing', (value) => {
        assert.strictEqual(value, undefined)
        resolve()
      })
    })
  })

  test('removeSessionValue should call session.remove', async () => {
    const mod = await loadModule()
    await new Promise((resolve) => {
      mod.removeSessionValue('deleteMe', () => {
        resolve()
      })
    })
    assert.strictEqual(chromeMock.storage.session.remove.mock.calls.length, 1)
    const removeArg = chromeMock.storage.session.remove.mock.calls[0].arguments[0]
    assert.deepStrictEqual(removeArg, ['deleteMe'])
  })

  test('removeSessionValue should work without callback', async () => {
    const mod = await loadModule()
    assert.doesNotThrow(() => {
      mod.removeSessionValue('noCallback')
    })
  })

  test('clearSessionStorage should call session.clear', async () => {
    const mod = await loadModule()
    await new Promise((resolve) => {
      mod.clearSessionStorage(() => {
        resolve()
      })
    })
    assert.strictEqual(chromeMock.storage.session.clear.mock.calls.length, 1)
  })

  test('clearSessionStorage should work without callback', async () => {
    const mod = await loadModule()
    assert.doesNotThrow(() => {
      mod.clearSessionStorage()
    })
  })
})

// =============================================================================
// SESSION STORAGE GRACEFUL DEGRADATION
// =============================================================================

describe('Session Storage Graceful Degradation', () => {
  beforeEach(() => {
    chromeMock = createStorageMock()
    globalThis.chrome = chromeMock
  })

  test('setSessionValue should call callback when session storage unavailable', async () => {
    const mod = await loadModule()
    // Remove session storage
    delete globalThis.chrome.storage.session
    let called = false
    mod.setSessionValue('key', 'val', () => {
      called = true
    })
    assert.strictEqual(called, true)
  })

  test('getSessionValue should return undefined when session storage unavailable', async () => {
    const mod = await loadModule()
    delete globalThis.chrome.storage.session
    await new Promise((resolve) => {
      mod.getSessionValue('key', (value) => {
        assert.strictEqual(value, undefined)
        resolve()
      })
    })
  })

  test('removeSessionValue should call callback when session storage unavailable', async () => {
    const mod = await loadModule()
    delete globalThis.chrome.storage.session
    let called = false
    mod.removeSessionValue('key', () => {
      called = true
    })
    assert.strictEqual(called, true)
  })

  test('clearSessionStorage should call callback when session storage unavailable', async () => {
    const mod = await loadModule()
    delete globalThis.chrome.storage.session
    let called = false
    mod.clearSessionStorage(() => {
      called = true
    })
    assert.strictEqual(called, true)
  })

  test('setSessionValue should call callback when chrome is undefined', async () => {
    const mod = await loadModule()
    const savedChrome = globalThis.chrome
    delete globalThis.chrome
    let called = false
    mod.setSessionValue('key', 'val', () => {
      called = true
    })
    assert.strictEqual(called, true)
    globalThis.chrome = savedChrome
  })

  test('getSessionValue should return undefined when chrome is undefined', async () => {
    const mod = await loadModule()
    const savedChrome = globalThis.chrome
    delete globalThis.chrome
    await new Promise((resolve) => {
      mod.getSessionValue('key', (value) => {
        assert.strictEqual(value, undefined)
        resolve()
      })
    })
    globalThis.chrome = savedChrome
  })
})

// =============================================================================
// LOCAL STORAGE
// =============================================================================

describe('Local Storage', () => {
  beforeEach(() => {
    chromeMock = createStorageMock()
    globalThis.chrome = chromeMock
  })

  test('setLocalValue should store value in local storage', async () => {
    const mod = await loadModule()
    await new Promise((resolve) => {
      mod.setLocalValue('persistKey', { data: 'hello' }, () => {
        resolve()
      })
    })
    assert.strictEqual(chromeMock.storage.local.set.mock.calls.length, 1)
    const setArg = chromeMock.storage.local.set.mock.calls[0].arguments[0]
    assert.deepStrictEqual(setArg, { persistKey: { data: 'hello' } })
  })

  test('setLocalValue should work without callback', async () => {
    const mod = await loadModule()
    assert.doesNotThrow(() => {
      mod.setLocalValue('key', 'val')
    })
  })

  test('getLocalValue should retrieve value from local storage', async () => {
    const mod = await loadModule()
    chromeMock.storage.local.get = mock.fn((keys, callback) => {
      callback({ serverUrl: 'http://localhost:3333' })
    })
    await new Promise((resolve) => {
      mod.getLocalValue('serverUrl', (value) => {
        assert.strictEqual(value, 'http://localhost:3333')
        resolve()
      })
    })
  })

  test('getLocalValue should return undefined for missing key', async () => {
    const mod = await loadModule()
    chromeMock.storage.local.get = mock.fn((keys, callback) => {
      callback({})
    })
    await new Promise((resolve) => {
      mod.getLocalValue('nope', (value) => {
        assert.strictEqual(value, undefined)
        resolve()
      })
    })
  })

  test('getLocalValue should return undefined on lastError', async () => {
    const mod = await loadModule()
    chromeMock.runtime.lastError = { message: 'QUOTA_BYTES exceeded' }
    chromeMock.storage.local.get = mock.fn((keys, callback) => {
      callback({ someKey: 'data' })
    })
    await new Promise((resolve) => {
      mod.getLocalValue('someKey', (value) => {
        assert.strictEqual(value, undefined)
        resolve()
      })
    })
    chromeMock.runtime.lastError = null
  })

  test('removeLocalValue should call local.remove', async () => {
    const mod = await loadModule()
    await new Promise((resolve) => {
      mod.removeLocalValue('deadKey', () => {
        resolve()
      })
    })
    assert.strictEqual(chromeMock.storage.local.remove.mock.calls.length, 1)
    const removeArg = chromeMock.storage.local.remove.mock.calls[0].arguments[0]
    assert.deepStrictEqual(removeArg, ['deadKey'])
  })

  test('removeLocalValue should work without callback', async () => {
    const mod = await loadModule()
    assert.doesNotThrow(() => {
      mod.removeLocalValue('noCallback')
    })
  })
})

// =============================================================================
// LOCAL STORAGE FALLBACK
// =============================================================================

describe('Local Storage Fallback', () => {
  beforeEach(() => {
    chromeMock = createStorageMock()
    globalThis.chrome = chromeMock
  })

  test('setLocalValue should call callback when chrome is undefined', async () => {
    const mod = await loadModule()
    const savedChrome = globalThis.chrome
    delete globalThis.chrome
    let called = false
    mod.setLocalValue('key', 'val', () => {
      called = true
    })
    assert.strictEqual(called, true)
    globalThis.chrome = savedChrome
  })

  test('getLocalValue should return undefined when chrome is undefined', async () => {
    const mod = await loadModule()
    const savedChrome = globalThis.chrome
    delete globalThis.chrome
    await new Promise((resolve) => {
      mod.getLocalValue('key', (value) => {
        assert.strictEqual(value, undefined)
        resolve()
      })
    })
    globalThis.chrome = savedChrome
  })

  test('removeLocalValue should call callback when chrome is undefined', async () => {
    const mod = await loadModule()
    const savedChrome = globalThis.chrome
    delete globalThis.chrome
    let called = false
    mod.removeLocalValue('key', () => {
      called = true
    })
    assert.strictEqual(called, true)
    globalThis.chrome = savedChrome
  })

  test('setLocalValue should call callback when chrome.storage is missing', async () => {
    const mod = await loadModule()
    delete globalThis.chrome.storage
    let called = false
    mod.setLocalValue('key', 'val', () => {
      called = true
    })
    assert.strictEqual(called, true)
  })

  test('getLocalValue should return undefined when chrome.storage is missing', async () => {
    const mod = await loadModule()
    delete globalThis.chrome.storage
    await new Promise((resolve) => {
      mod.getLocalValue('key', (value) => {
        assert.strictEqual(value, undefined)
        resolve()
      })
    })
  })
})

// =============================================================================
// FACADE FUNCTIONS (setValue, getValue, removeValue)
// =============================================================================

describe('Facade Functions', () => {
  beforeEach(() => {
    chromeMock = createStorageMock()
    globalThis.chrome = chromeMock
  })

  test('setValue should default to session storage', async () => {
    const mod = await loadModule()
    await new Promise((resolve) => {
      mod.setValue('key', 'val', undefined, () => {
        resolve()
      })
    })
    assert.strictEqual(chromeMock.storage.session.set.mock.calls.length, 1)
  })

  test('setValue with "local" area should use local storage', async () => {
    const mod = await loadModule()
    await new Promise((resolve) => {
      mod.setValue('key', 'val', 'local', () => {
        resolve()
      })
    })
    assert.strictEqual(chromeMock.storage.local.set.mock.calls.length, 1)
  })

  test('setValue with "session" area should use session storage', async () => {
    const mod = await loadModule()
    await new Promise((resolve) => {
      mod.setValue('key', 'val', 'session', () => {
        resolve()
      })
    })
    assert.strictEqual(chromeMock.storage.session.set.mock.calls.length, 1)
  })

  test('setValue with unknown area should just call callback', async () => {
    const mod = await loadModule()
    let called = false
    mod.setValue('key', 'val', 'unknown_area', () => {
      called = true
    })
    assert.strictEqual(called, true)
    assert.strictEqual(chromeMock.storage.session.set.mock.calls.length, 0)
    assert.strictEqual(chromeMock.storage.local.set.mock.calls.length, 0)
  })

  test('getValue should default to session storage', async () => {
    const mod = await loadModule()
    chromeMock.storage.session.get = mock.fn((keys, cb) => cb({ key: 'fromSession' }))
    await new Promise((resolve) => {
      mod.getValue('key', undefined, (value) => {
        assert.strictEqual(value, 'fromSession')
        resolve()
      })
    })
  })

  test('getValue with "local" area should use local storage', async () => {
    const mod = await loadModule()
    chromeMock.storage.local.get = mock.fn((keys, cb) => cb({ key: 'fromLocal' }))
    await new Promise((resolve) => {
      mod.getValue('key', 'local', (value) => {
        assert.strictEqual(value, 'fromLocal')
        resolve()
      })
    })
  })

  test('getValue with unknown area should return undefined', async () => {
    const mod = await loadModule()
    await new Promise((resolve) => {
      mod.getValue('key', 'unknown_area', (value) => {
        assert.strictEqual(value, undefined)
        resolve()
      })
    })
  })

  test('removeValue should default to session storage', async () => {
    const mod = await loadModule()
    await new Promise((resolve) => {
      mod.removeValue('key', undefined, () => {
        resolve()
      })
    })
    assert.strictEqual(chromeMock.storage.session.remove.mock.calls.length, 1)
  })

  test('removeValue with "local" area should use local storage', async () => {
    const mod = await loadModule()
    await new Promise((resolve) => {
      mod.removeValue('key', 'local', () => {
        resolve()
      })
    })
    assert.strictEqual(chromeMock.storage.local.remove.mock.calls.length, 1)
  })

  test('removeValue with unknown area should just call callback', async () => {
    const mod = await loadModule()
    let called = false
    mod.removeValue('key', 'unknown_area', () => {
      called = true
    })
    assert.strictEqual(called, true)
  })
})

// =============================================================================
// DIAGNOSTICS
// =============================================================================

describe('Storage Diagnostics', () => {
  beforeEach(() => {
    chromeMock = createStorageMock()
    globalThis.chrome = chromeMock
  })

  test('should report session and local storage availability', async () => {
    const mod = await loadModule()
    const diag = mod.getStorageDiagnostics()
    assert.strictEqual(diag.sessionStorageAvailable, true)
    assert.strictEqual(diag.localStorageAvailable, true)
    assert.strictEqual(typeof diag.browserVersion, 'string')
  })

  test('should report session storage unavailable when missing', async () => {
    const mod = await loadModule()
    delete globalThis.chrome.storage.session
    const diag = mod.getStorageDiagnostics()
    assert.strictEqual(diag.sessionStorageAvailable, false)
    assert.strictEqual(diag.localStorageAvailable, true)
  })

  test('should report local storage unavailable when chrome is undefined', async () => {
    const mod = await loadModule()
    const savedChrome = globalThis.chrome
    delete globalThis.chrome
    const diag = mod.getStorageDiagnostics()
    assert.strictEqual(diag.sessionStorageAvailable, false)
    assert.strictEqual(diag.localStorageAvailable, false)
    globalThis.chrome = savedChrome
  })
})

// =============================================================================
// SERVICE WORKER RESTART DETECTION
// =============================================================================

describe('Service Worker Restart Detection', () => {
  beforeEach(() => {
    chromeMock = createStorageMock()
    globalThis.chrome = chromeMock
  })

  test('wasServiceWorkerRestarted should return true when version not set', async () => {
    const mod = await loadModule()
    chromeMock.storage.session.get = mock.fn((keys, callback) => {
      if (typeof callback === 'function') callback({})
      else return Promise.resolve({})
    })
    const wasRestarted = await mod.wasServiceWorkerRestarted()
    assert.strictEqual(wasRestarted, true)
  })

  test('wasServiceWorkerRestarted should return false after markStateVersion', async () => {
    const mod = await loadModule()

    // markStateVersion stores the version in session storage
    // Then wasServiceWorkerRestarted checks if it matches
    let storedVersion = null
    chromeMock.storage.session.set = mock.fn((data, cb) => {
      for (const [k, v] of Object.entries(data)) {
        storedVersion = { key: k, value: v }
      }
      if (typeof cb === 'function') cb()
      else return Promise.resolve()
    })
    chromeMock.storage.session.get = mock.fn((keys, callback) => {
      const result = storedVersion ? { [storedVersion.key]: storedVersion.value } : {}
      if (typeof callback === 'function') callback(result)
      else return Promise.resolve(result)
    })

    await mod.markStateVersion()
    const wasRestarted = await mod.wasServiceWorkerRestarted()
    assert.strictEqual(wasRestarted, false)
  })

  test('wasServiceWorkerRestarted should return false when session storage unavailable', async () => {
    const mod = await loadModule()
    delete globalThis.chrome.storage.session
    const wasRestarted = await mod.wasServiceWorkerRestarted()
    assert.strictEqual(wasRestarted, false)
  })

  test('markStateVersion should resolve without error', async () => {
    const mod = await loadModule()
    await assert.doesNotReject(async () => {
      await mod.markStateVersion()
    })
  })
})

// =============================================================================
// DATA SERIALIZATION
// =============================================================================

describe('Data Serialization', () => {
  beforeEach(() => {
    chromeMock = createStorageMock()
    globalThis.chrome = chromeMock
  })

  test('should store and retrieve complex objects', async () => {
    const mod = await loadModule()
    const complexData = {
      nested: { array: [1, 2, 3], obj: { a: true } },
      number: 42,
      string: 'hello',
      nullVal: null
    }

    chromeMock.storage.local.set = mock.fn((data, cb) => {
      chromeMock.storage.local._lastSet = data
      if (cb) cb()
    })
    chromeMock.storage.local.get = mock.fn((keys, callback) => {
      callback(chromeMock.storage.local._lastSet || {})
    })

    await new Promise((resolve) => {
      mod.setLocalValue('complex', complexData, () => {
        resolve()
      })
    })

    await new Promise((resolve) => {
      mod.getLocalValue('complex', (value) => {
        assert.deepStrictEqual(value, complexData)
        resolve()
      })
    })
  })

  test('should handle boolean values', async () => {
    const mod = await loadModule()
    chromeMock.storage.session.set = mock.fn((data, cb) => {
      chromeMock.storage.session._lastSet = data
      if (cb) cb()
    })
    chromeMock.storage.session.get = mock.fn((keys, callback) => {
      callback(chromeMock.storage.session._lastSet || {})
    })

    await new Promise((resolve) => {
      mod.setSessionValue('flag', true, () => {
        resolve()
      })
    })

    await new Promise((resolve) => {
      mod.getSessionValue('flag', (value) => {
        assert.strictEqual(value, true)
        resolve()
      })
    })
  })

  test('should handle numeric values including zero', async () => {
    const mod = await loadModule()
    chromeMock.storage.session.set = mock.fn((data, cb) => {
      chromeMock.storage.session._lastSet = data
      if (cb) cb()
    })
    chromeMock.storage.session.get = mock.fn((keys, callback) => {
      callback(chromeMock.storage.session._lastSet || {})
    })

    await new Promise((resolve) => {
      mod.setSessionValue('count', 0, () => {
        resolve()
      })
    })

    await new Promise((resolve) => {
      mod.getSessionValue('count', (value) => {
        assert.strictEqual(value, 0)
        resolve()
      })
    })
  })
})

// =============================================================================
// STORAGE QUOTA HANDLING (lastError simulation)
// =============================================================================

describe('Storage Quota Handling', () => {
  beforeEach(() => {
    chromeMock = createStorageMock()
    globalThis.chrome = chromeMock
  })

  test('setLocalValue should still call callback on lastError', async () => {
    const mod = await loadModule()
    chromeMock.runtime.lastError = { message: 'QUOTA_BYTES_PER_ITEM quota exceeded' }
    let called = false
    await new Promise((resolve) => {
      mod.setLocalValue('bigKey', 'x'.repeat(100000), () => {
        called = true
        resolve()
      })
    })
    assert.strictEqual(called, true)
    chromeMock.runtime.lastError = null
  })

  test('getLocalValue should return undefined and call callback on lastError', async () => {
    const mod = await loadModule()
    chromeMock.runtime.lastError = { message: 'QUOTA_BYTES exceeded' }
    chromeMock.storage.local.get = mock.fn((keys, callback) => {
      callback({ key: 'should-be-ignored' })
    })
    await new Promise((resolve) => {
      mod.getLocalValue('key', (value) => {
        assert.strictEqual(value, undefined)
        resolve()
      })
    })
    chromeMock.runtime.lastError = null
  })
})
