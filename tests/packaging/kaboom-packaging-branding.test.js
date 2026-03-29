// @ts-nocheck
/**
 * @fileoverview kaboom-packaging-branding.test.js — Guards package README and metadata branding.
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

describe('kaboom packaging branding', () => {
  test('npm and PyPI package metadata use Kaboom names and repo slug', () => {
    const repoFiles = [
      'npm/kaboom-agentic-browser/README.md',
      'pypi/kaboom-agentic-browser/README.md',
      'pypi/kaboom-agentic-browser/pyproject.toml',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser.egg-info/PKG-INFO',
      'pypi/kaboom-agentic-browser-darwin-arm64/pyproject.toml',
      'pypi/kaboom-agentic-browser-darwin-x64/pyproject.toml',
      'pypi/kaboom-agentic-browser-linux-arm64/pyproject.toml',
      'pypi/kaboom-agentic-browser-linux-x64/pyproject.toml',
      'pypi/kaboom-agentic-browser-win32-x64/pyproject.toml',
      'pypi/kaboom-agentic-browser-darwin-arm64/kaboom_agentic_browser_darwin_arm64.egg-info/PKG-INFO',
      'pypi/kaboom-agentic-browser-darwin-x64/kaboom_agentic_browser_darwin_x64.egg-info/PKG-INFO',
      'pypi/kaboom-agentic-browser-linux-arm64/kaboom_agentic_browser_linux_arm64.egg-info/PKG-INFO',
      'pypi/kaboom-agentic-browser-linux-x64/kaboom_agentic_browser_linux_x64.egg-info/PKG-INFO',
      'pypi/kaboom-agentic-browser-win32-x64/kaboom_agentic_browser_win32_x64.egg-info/PKG-INFO'
    ]
    const metadataFiles = [
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser.egg-info/entry_points.txt',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser.egg-info/requires.txt',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser.egg-info/top_level.txt',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser.egg-info/SOURCES.txt'
    ]

    for (const file of repoFiles) {
      const source = read(file)
      assert.match(source, /Kaboom-Browser-AI-Devtools-MCP|kaboom-agentic-browser|Kaboom|kaboom/i)
      assert.match(source, /Kaboom-Browser-AI-Devtools-MCP/)
      assert.doesNotMatch(
        source,
        /gasoline-mcp|gasoline-browser-devtools|gasoline-agentic-browser-devtools-mcp|Gasoline|STRUM|getstrum|cookwithgasoline|\.gasoline/
      )
    }

    for (const file of metadataFiles) {
      const source = read(file)
      assert.match(source, /kaboom-agentic-browser|kaboom_agentic_browser|Kaboom|kaboom/)
      assert.doesNotMatch(
        source,
        /gasoline-mcp|gasoline-browser-devtools|gasoline-agentic-browser-devtools-mcp|Gasoline|STRUM|getstrum|cookwithgasoline|\.gasoline/
      )
    }
  })

  test('PyPI wrapper runtime copy uses Kaboom names', () => {
    const files = [
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser/__init__.py',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser/skills/skills.json',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser/install.py',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser/uninstall.py',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser/doctor.py',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser/config.py'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /Kaboom|kaboom/)
      assert.doesNotMatch(source, /Gasoline MCP package metadata|Test if gasoline binary|Remove gasoline MCP config entries/)
      assert.doesNotMatch(source, /merge_gasoline_config|gasoline_entry/)
    }
  })

  test('bundled PyPI skills use Kaboom branding', () => {
    const files = [
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser/skills/api-validation/SKILL.md',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser/skills/automate/SKILL.md',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser/skills/config-doctor/SKILL.md',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser/skills/debug-triage/SKILL.md',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser/skills/debug/SKILL.md',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser/skills/demo/SKILL.md',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser/skills/performance/SKILL.md',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser/skills/regression-test/SKILL.md',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser/skills/release-readiness/SKILL.md',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser/skills/reliability/SKILL.md',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser/skills/security-redaction/SKILL.md',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser/skills/site-audit/SKILL.md',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser/skills/test-coverage/SKILL.md',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser/skills/ux-audit/SKILL.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /# Kaboom|Kaboom setup|Kaboom/)
      assert.doesNotMatch(source, /# Gasoline|their Gasoline setup|Gasoline setup/)
    }
  })

  test('server package and kaboom-ci runtime surface only Kaboom branding', () => {
    const files = [
      'server/package.json',
      'packages/kaboom-ci/package.json',
      'packages/kaboom-ci/kaboom-ci.js'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /Kaboom|KaBOOM|kaboom|gokaboom/)
      assert.doesNotMatch(source, /Gasoline|STRUM|getstrum|cookwithgasoline/)
      assert.doesNotMatch(source, /__GASOLINE_TEST_ID|__GASOLINE_CI_INITIALIZED|GASOLINE_HOST|GASOLINE_PORT/)
    }
  })
})
