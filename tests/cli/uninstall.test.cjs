const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const REPO_ROOT = path.resolve(__dirname, '..', '..')

test('npm publish script targets kaboom package names', () => {
  const publishScript = fs.readFileSync(path.join(REPO_ROOT, 'npm/publish.sh'), 'utf8')

  assert.match(publishScript, /npm\/kaboom-agentic-browser\/bin\/kaboom-agentic-browser/)
  assert.match(publishScript, /npm\/kaboom-agentic-browser\/extension/)
  assert.match(publishScript, /@brennhill\/kaboom-agentic-browser-\$\{pkg\}/)
  assert.match(publishScript, /Publishing main package \(kaboom-agentic-browser\)/)
  assert.doesNotMatch(publishScript, /gasoline-mcp|@brennhill\/gasoline-/)
})
