// @ts-nocheck
/**
 * @fileoverview kaboom-cli-docs-branding.test.js — Guards enhanced CLI docs branding.
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

describe('kaboom cli docs branding', () => {
  test('enhanced CLI feature docs use Kaboom naming', () => {
    const files = [
      'docs/features/feature/enhanced-cli-config/product-spec.md',
      'docs/features/feature/enhanced-cli-config/implementation-plan.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /Kaboom|kaboom|gokaboom/)
      assert.doesNotMatch(
        source,
        /Gasoline|STRUM|getstrum|cookwithgasoline|gasoline-mcp|gasoline-agentic-browser|gasoline-browser-devtools|\.gasoline/
      )
    }
  })
})
