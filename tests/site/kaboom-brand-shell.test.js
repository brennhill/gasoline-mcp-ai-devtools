// @ts-nocheck
/**
 * @fileoverview kaboom-brand-shell.test.js — Guards top-level site shell branding.
 */

import { describe, test } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const TEST_DIR = path.dirname(fileURLToPath(import.meta.url))
const REPO_ROOT = path.resolve(TEST_DIR, '../..')

function read(relativePath) {
  return fs.readFileSync(path.join(REPO_ROOT, relativePath), 'utf8')
}

describe('kaboom site brand shell', () => {
  test('homepage entry copy uses Kaboom install branding', () => {
    const indexMdx = read('getstrum.dev/src/content/docs/index.mdx')
    assert.match(indexMdx, /title:\s*"Kaboom"/)
    assert.match(indexMdx, /text:\s*Install Kaboom/)
  })

  test('head defaults to gokaboom.dev for markdown and shell metadata', () => {
    const head = read('getstrum.dev/src/components/Head.astro')
    assert.match(head, /https:\/\/gokaboom\.dev/)
    assert.doesNotMatch(head, /cookwithgasoline\.com/)
  })

  test('footer and landing shell remove legacy branding', () => {
    const footer = read('getstrum.dev/src/components/Footer.astro')
    const landing = read('getstrum.dev/src/components/Landing.astro')
    assert.match(footer, /Kaboom/)
    assert.doesNotMatch(footer, /STRUM Agentic Devtools|Gasoline/)
    assert.match(landing, /Kaboom/)
    assert.doesNotMatch(landing, /STRUM Agentic Devtools|STRUM mascot|Install STRUM/)
  })

  test('rotating hero publishes Kaboom page-title branding', () => {
    const rotatingHero = read('getstrum.dev/src/components/RotatingHero.astro')
    assert.match(rotatingHero, /Kaboom MCP:/)
    assert.doesNotMatch(rotatingHero, /STRUM MCP:/)
  })

  test('site flame logo assets use the restored idle and strum variants', () => {
    const idleLogo = read('getstrum.dev/src/assets/logo.svg')
    const animatedLogo = read('getstrum.dev/src/assets/logo-animated.svg')
    const publicAnimatedLogo = read('getstrum.dev/public/images/logo-animated.svg')

    assert.match(idleLogo, /--energy-speed/)
    assert.match(animatedLogo, /ghost-1/)
    assert.match(publicAnimatedLogo, /ghost-1/)
  })
})
