// @ts-nocheck
/**
 * @fileoverview logo-motion.test.js — Tests idle and hover STRUM logo motion contracts.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const TEST_DIR = path.dirname(fileURLToPath(import.meta.url))
const REPO_ROOT = path.resolve(TEST_DIR, '../..')

let logoEl
let logoListeners
let importCounter = 0

function resetPopupLogoDom() {
  logoListeners = {}
  logoEl = {
    src: '',
    addEventListener: mock.fn((type, handler) => {
      logoListeners[type] = handler
    })
  }

  globalThis.document = {
    querySelector: mock.fn((selector) => {
      if (selector === '.logo') return logoEl
      return null
    })
  }

  globalThis.chrome = {
    runtime: {
      getURL: mock.fn((assetPath) => `chrome-extension://test/${assetPath}`)
    }
  }
}

describe('popup logo motion', () => {
  beforeEach(() => {
    mock.reset()
    resetPopupLogoDom()
  })

  test('uses icon.svg at rest and logo-animated.svg on hover', async () => {
    const { initPopupLogoMotion } = await import(`../../extension/popup/logo-motion.js?v=${++importCounter}`)

    initPopupLogoMotion()

    assert.ok(String(logoEl.src).includes('icons/icon.svg'), 'popup logo should start on idle icon.svg')
    assert.equal(typeof logoListeners.mouseenter, 'function', 'mouseenter handler should be installed')
    assert.equal(typeof logoListeners.mouseleave, 'function', 'mouseleave handler should be installed')

    logoListeners.mouseenter()
    assert.ok(
      String(logoEl.src).includes('icons/logo-animated.svg'),
      'popup logo should switch to stronger hover animation'
    )

    logoListeners.mouseleave()
    assert.ok(String(logoEl.src).includes('icons/icon.svg'), 'popup logo should return to idle icon.svg')
  })
})

describe('logo asset contracts', () => {
  test('popup html uses icon.svg as the idle asset', () => {
    const popupHtml = fs.readFileSync(path.join(REPO_ROOT, 'extension/popup.html'), 'utf8')
    assert.match(popupHtml, /src="icons\/icon\.svg"/)
  })

  test('icon.svg includes the slow idle strum animation', () => {
    const iconSvg = fs.readFileSync(path.join(REPO_ROOT, 'extension/icons/icon.svg'), 'utf8')
    assert.match(iconSvg, /--energy-speed:\s*2s/)
    assert.match(iconSvg, /--energy-displacement:\s*4px/)
    assert.match(iconSvg, /class="strum-path/)
    assert.match(iconSvg, /@keyframes harmonic-osc/)
  })

  test('logo-animated.svg includes the stronger hover strum treatment', () => {
    const animatedSvg = fs.readFileSync(path.join(REPO_ROOT, 'extension/icons/logo-animated.svg'), 'utf8')
    assert.match(animatedSvg, /--energy-speed:\s*0\.22s/)
    assert.match(animatedSvg, /--energy-displacement:\s*22px/)
    assert.match(animatedSvg, /ghost-1/)
    assert.match(animatedSvg, /ghost-2/)
  })
})
