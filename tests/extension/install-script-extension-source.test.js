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
    /\[ -f "\$base_dir\/theme-bootstrap\.js" \]/,
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

test('bash installer uses staged extension promotion and supports strict checksum mode', () => {
  const script = fs.readFileSync(INSTALL_SH, 'utf8')

  assert.match(
    script,
    /STAGE_EXT_DIR="\$INSTALL_DIR\/\.extension-stage-\$\$"/,
    'install.sh should stage extension files in a temp directory before promotion'
  )
  assert.match(
    script,
    /promote_extension_stage\(\)/,
    'install.sh should promote validated staged extension atomically'
  )
  assert.match(
    script,
    /STRICT_CHECKSUM="\$\{GASOLINE_INSTALL_STRICT:-0\}"/,
    'install.sh should expose strict checksum mode via GASOLINE_INSTALL_STRICT'
  )
  assert.match(
    script,
    /Strict checksum mode enabled/,
    'install.sh should surface strict checksum mode in installer output'
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

test('powershell installer uses unique temp paths, staged promotion, and strict checksum mode', () => {
  const script = fs.readFileSync(INSTALL_PS1, 'utf8')

  assert.match(
    script,
    /\$TEMP_TOKEN = \[Guid\]::NewGuid\(\)\.ToString\("N"\)/,
    'install.ps1 should derive unique temp token per run'
  )
  assert.match(
    script,
    /gasoline-ext-\$TEMP_TOKEN\.zip/,
    'install.ps1 should use unique extension zip temp path'
  )
  assert.match(
    script,
    /function Promote-ExtensionStage/,
    'install.ps1 should promote validated staged extension atomically'
  )
  assert.match(
    script,
    /\$STRICT_CHECKSUM = \$env:GASOLINE_INSTALL_STRICT -eq "1"/,
    'install.ps1 should support strict checksum mode via GASOLINE_INSTALL_STRICT'
  )
})

test('powershell installer force-stops stale server and prints manual recovery warning', () => {
  const script = fs.readFileSync(INSTALL_PS1, 'utf8')

  assert.match(
    script,
    /Stop-GasolineServerProcesses/,
    'install.ps1 must include explicit server stop logic before binary replacement'
  )
  assert.match(
    script,
    /taskkill\s+\/F\s+\/PID/,
    'install.ps1 must escalate to taskkill for stubborn running processes'
  )
  assert.match(
    script,
    /INSTALL WARNING: MANUAL ACTION REQUIRED/,
    'install.ps1 must print a high-visibility warning block when server replacement fails'
  )
  assert.match(
    script,
    /Get-Process gasoline -ErrorAction SilentlyContinue \| Stop-Process -Force/,
    'install.ps1 must provide manual process kill instructions'
  )
})
