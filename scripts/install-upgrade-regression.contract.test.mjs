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
