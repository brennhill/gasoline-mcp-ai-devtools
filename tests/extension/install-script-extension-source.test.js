/**
 * @fileoverview Regression guard for installer extension staging behavior.
 * Ensures fallback pulls from STABLE branch and validates CSP-safe bootstrap files.
 */

import { test } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const REPO_ROOT = path.resolve(__dirname, '../..')
const INSTALL_SH = path.join(REPO_ROOT, 'scripts', 'install.sh')
const INSTALL_PS1 = path.join(REPO_ROOT, 'scripts', 'install.ps1')

test('bash installer validates theme bootstrap and falls back to STABLE source zip', () => {
  const script = fs.readFileSync(INSTALL_SH, 'utf8')

  assert.match(
    script,
    /\[ -f "\$EXT_DIR\/theme-bootstrap\.js" \]/,
    'install.sh must validate theme-bootstrap.js in staged extension'
  )
  assert.match(
    script,
    /archive\/refs\/heads\/STABLE\.zip/,
    'install.sh fallback must use STABLE branch source zip'
  )
  assert.doesNotMatch(
    script,
    /archive\/refs\/tags\/v\$VERSION\.zip/,
    'install.sh must not fall back to version tag source zip for extension staging'
  )
})

test('powershell installer validates theme bootstrap and falls back to STABLE source zip', () => {
  const script = fs.readFileSync(INSTALL_PS1, 'utf8')

  assert.match(
    script,
    /theme-bootstrap\.js/,
    'install.ps1 must validate theme-bootstrap.js in staged extension'
  )
  assert.match(
    script,
    /archive\/refs\/heads\/STABLE\.zip/,
    'install.ps1 fallback must use STABLE branch source zip'
  )
  assert.doesNotMatch(
    script,
    /archive\/refs\/tags\/v\$VERSION\.zip/,
    'install.ps1 must not fall back to version tag source zip for extension staging'
  )
})
