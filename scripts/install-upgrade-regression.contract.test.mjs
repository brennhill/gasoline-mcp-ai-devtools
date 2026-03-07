import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const scriptPath = path.join(__dirname, 'install-upgrade-regression.mjs')

test('upgrade regression script validates health service identity', () => {
  const source = fs.readFileSync(scriptPath, 'utf8')
  assert.match(source, /service-name/, 'expected service-name validation in health checks')
  assert.match(
    source,
    /gasoline-browser-devtools/i,
    'expected service identity check to enforce gasoline-browser-devtools'
  )
})

test('shell installer keeps canonical binary and legacy aliases', () => {
  const source = fs.readFileSync(path.join(__dirname, 'install.sh'), 'utf8')
  assert.match(source, /gasoline-agentic-devtools/, 'expected canonical binary name in install.sh')
  assert.match(source, /gasoline-agentic-browser/, 'expected legacy browser alias in install.sh')
  assert.match(source, /sync_binary_compat_aliases/, 'expected install.sh alias compatibility helper')
})

test('powershell installer keeps canonical binary and legacy aliases', () => {
  const source = fs.readFileSync(path.join(__dirname, 'install.ps1'), 'utf8')
  assert.match(source, /gasoline-agentic-devtools\.exe/, 'expected canonical binary name in install.ps1')
  assert.match(source, /gasoline-agentic-browser\.exe/, 'expected legacy browser alias in install.ps1')
  assert.match(source, /Sync-BinaryCompatAliases/, 'expected install.ps1 alias compatibility helper')
})

test('shell installer supports --hooks-only mode', () => {
  const source = fs.readFileSync(path.join(__dirname, 'install.sh'), 'utf8')
  assert.match(source, /HOOKS_ONLY/, 'expected HOOKS_ONLY variable in install.sh')
  assert.match(source, /--hooks-only/, 'expected --hooks-only flag handling')
  assert.match(source, /gasoline-hooks/, 'expected gasoline-hooks binary name')
  assert.match(source, /download_and_verify/, 'expected download_and_verify helper')
})

test('shell installer downloads both binaries by default', () => {
  const source = fs.readFileSync(path.join(__dirname, 'install.sh'), 'utf8')
  // gasoline-agentic-devtools is downloaded when HOOKS_ONLY != 1
  assert.match(source, /gasoline-agentic-devtools-\$PLATFORM/, 'expected main binary download')
  // gasoline-hooks is always downloaded
  assert.match(source, /gasoline-hooks-\$PLATFORM/, 'expected hooks binary download')
})

test('shell installer skips extension and daemon for hooks-only', () => {
  const source = fs.readFileSync(path.join(__dirname, 'install.sh'), 'utf8')
  assert.match(source, /HOOKS_ONLY.*guard/, 'expected HOOKS_ONLY guard comment')
})

test('npm wrapper exposes gasoline-agentic-devtools command alias', () => {
  const pkg = JSON.parse(
    fs.readFileSync(path.join(__dirname, '..', 'npm', 'gasoline-agentic-browser', 'package.json'), 'utf8')
  )
  assert.equal(
    pkg.bin?.['gasoline-agentic-devtools'],
    'bin/gasoline-agentic-browser',
    'expected npm bin alias for gasoline-agentic-devtools'
  )
})
