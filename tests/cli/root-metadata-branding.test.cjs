const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const REPO_ROOT = path.resolve(__dirname, '..', '..')
const KABOOM_REPO = 'https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP'

function read(relativePath) {
  return fs.readFileSync(path.join(REPO_ROOT, relativePath), 'utf8')
}

test('root package metadata points at Kaboom and gokaboom.dev', () => {
  const packageJson = JSON.parse(read('package.json'))

  assert.equal(packageJson.name, 'kaboom-browser-ai-devtools-mcp')
  assert.equal(packageJson.homepage, 'https://gokaboom.dev')
  assert.equal(packageJson.repository.url, KABOOM_REPO)
  assert.equal(packageJson.bugs.url, `${KABOOM_REPO}/issues`)
})

test('root README uses Kaboom install and repo branding', () => {
  const readme = read('README.md')

  assert.match(readme, /Kaboom/)
  assert.match(readme, /gokaboom\.dev/)
  assert.match(readme, /window\.__kaboom\.annotate\(\)/)
  assert.match(readme, /~\/\.kaboom\/extension/)
  assert.match(readme, /github\.com\/brennhill\/Kaboom-Browser-AI-Devtools-MCP/)
  assert.doesNotMatch(
    readme,
    /STRUM|Gasoline|cookwithgasoline\.com|getstrum|Strum-AI-Devtools|window\.__strum\.annotate\(\)|~\/\.strum\/extension/
  )
})

test('root contributor and agent docs publish only Kaboom naming', () => {
  const contributing = read('CONTRIBUTING.md')
  const claude = read('CLAUDE.md')
  const codex = read('CODEX.md')

  for (const source of [contributing, claude, codex]) {
    assert.match(source, /Kaboom/)
    assert.doesNotMatch(source, /STRUM|Gasoline|cookwithgasoline\.com|getstrum/)
  }

  assert.match(contributing, /github\.com\/brennhill\/Kaboom-Browser-AI-Devtools-MCP/)
  assert.match(contributing, /https:\/\/gokaboom\.dev/)

  assert.match(claude, /`kaboom-mcp` from PATH/)
  assert.match(claude, /gitnexus:\/\/repo\/kaboom\//)

  assert.match(codex, /`kaboom-mcp` from PATH/)
  assert.match(codex, /gitnexus:\/\/repo\/kaboom\//)
})

test('extension shell pages publish only Kaboom branding', () => {
  const files = [
    'extension/sidepanel.html',
    'extension/options.html',
    'extension/mic-permission.html',
    'extension/offscreen.html'
  ]

  for (const file of files) {
    const source = read(file)
    assert.match(source, /Kaboom/)
    assert.doesNotMatch(source, /STRUM|Gasoline|Strum|getstrum|cookwithgasoline/)
  }
})
