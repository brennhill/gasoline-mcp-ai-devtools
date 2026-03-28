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
const SITE_ROOT = 'gokaboom.dev'

function read(relativePath) {
  return fs.readFileSync(path.join(REPO_ROOT, relativePath), 'utf8')
}

function readSite(relativePath) {
  const absolutePath = path.join(REPO_ROOT, SITE_ROOT, relativePath)
  assert.ok(fs.existsSync(absolutePath), `missing site file: ${absolutePath}`)
  return fs.readFileSync(absolutePath, 'utf8')
}

describe('kaboom site brand shell', () => {
  test('site source lives under gokaboom.dev and root tooling references that path', () => {
    assert.ok(fs.existsSync(path.join(REPO_ROOT, SITE_ROOT)))
    assert.ok(!fs.existsSync(path.join(REPO_ROOT, 'getstrum.dev')))

    const makefile = read('Makefile')
    const eslintConfig = read('eslint.config.js')
    const packageJson = read('package.json')

    assert.match(makefile, /gokaboom\.dev/)
    assert.doesNotMatch(makefile, /cookwithgasoline\.com/)
    assert.match(eslintConfig, /gokaboom\.dev/)
    assert.doesNotMatch(eslintConfig, /cookwithgasoline\.com/)
    assert.match(packageJson, /https:\/\/gokaboom\.dev/)
  })

  test('homepage entry copy uses Kaboom install branding', () => {
    const indexMdx = readSite('src/content/docs/index.mdx')
    assert.match(indexMdx, /title:\s*"Kaboom"/)
    assert.match(indexMdx, /text:\s*Install Kaboom/)
  })

  test('head defaults to gokaboom.dev for markdown and shell metadata', () => {
    const head = readSite('src/components/Head.astro')
    assert.match(head, /https:\/\/gokaboom\.dev/)
    assert.doesNotMatch(head, /cookwithgasoline\.com/)
  })

  test('footer and landing shell remove legacy branding', () => {
    const footer = readSite('src/components/Footer.astro')
    const landing = readSite('src/components/Landing.astro')
    assert.match(footer, /Kaboom/)
    assert.doesNotMatch(footer, /STRUM Agentic Devtools|Gasoline/)
    assert.match(landing, /Kaboom/)
    assert.doesNotMatch(landing, /STRUM Agentic Devtools|STRUM mascot|Install STRUM/)
  })

  test('rotating hero publishes Kaboom page-title branding', () => {
    const rotatingHero = readSite('src/components/RotatingHero.astro')
    assert.match(rotatingHero, /Kaboom MCP:/)
    assert.doesNotMatch(rotatingHero, /STRUM MCP:/)
  })

  test('site flame logo assets use the restored idle and strum variants', () => {
    const idleLogo = readSite('src/assets/logo.svg')
    const animatedLogo = readSite('src/assets/logo-animated.svg')
    const publicAnimatedLogo = readSite('public/images/logo-animated.svg')

    assert.match(idleLogo, /--energy-speed/)
    assert.match(animatedLogo, /ghost-1/)
    assert.match(publicAnimatedLogo, /ghost-1/)
  })
})
