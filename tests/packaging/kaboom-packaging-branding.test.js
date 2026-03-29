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
  test('npm package metadata uses Kaboom names and repo slug', () => {
    const source = read('npm/kaboom-agentic-browser/README.md')
    assert.match(source, /Kaboom-Browser-AI-Devtools-MCP|kaboom-agentic-browser|Kaboom|kaboom/i)
    assert.match(source, /Kaboom-Browser-AI-Devtools-MCP/)
    assert.doesNotMatch(
      source,
      /gasoline-mcp|gasoline-browser-devtools|gasoline-agentic-browser-devtools-mcp|Gasoline|STRUM|getstrum|cookwithgasoline|\.gasoline/
    )
  })

  test('PyPI distribution payload is removed from the repo', () => {
    const removedPaths = [
      'pypi/PYPI_STRUCTURE.md',
      'pypi/create-platform-packages.sh',
      'pypi/kaboom-agentic-browser/README.md',
      'pypi/kaboom-agentic-browser/pyproject.toml',
      'pypi/kaboom-agentic-browser/kaboom_agentic_browser/__init__.py',
      'pypi/kaboom-agentic-browser/tests/test_install.py'
    ]

    for (const relativePath of removedPaths) {
      assert.strictEqual(fs.existsSync(path.join(REPO_ROOT, relativePath)), false, relativePath)
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
