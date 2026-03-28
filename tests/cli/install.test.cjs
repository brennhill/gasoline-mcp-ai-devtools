const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const REPO_ROOT = path.resolve(__dirname, '..', '..')
const PLATFORM_PACKAGES = [
  ['darwin-arm64', '@brennhill/kaboom-agentic-browser-darwin-arm64'],
  ['darwin-x64', '@brennhill/kaboom-agentic-browser-darwin-x64'],
  ['linux-arm64', '@brennhill/kaboom-agentic-browser-linux-arm64'],
  ['linux-x64', '@brennhill/kaboom-agentic-browser-linux-x64'],
  ['win32-x64', '@brennhill/kaboom-agentic-browser-win32-x64']
]

function readJson(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(REPO_ROOT, relativePath), 'utf8'))
}

test('platform npm packages use kaboom names and descriptions', () => {
  for (const [folder, packageName] of PLATFORM_PACKAGES) {
    const packageJson = readJson(`npm/${folder}/package.json`)
    assert.equal(packageJson.name, packageName)
    assert.match(packageJson.description, /Kaboom/)
    assert.doesNotMatch(packageJson.description, /Gasoline/)
  }
})

test('npm skill installer cleanup targets kaboom-managed output and strum legacy artifacts', () => {
  const skillsSource = fs.readFileSync(path.join(REPO_ROOT, 'npm/kaboom-agentic-browser/lib/skills.js'), 'utf8')
  const postinstallSource = fs.readFileSync(path.join(REPO_ROOT, 'npm/kaboom-agentic-browser/lib/postinstall-skills.js'), 'utf8')

  assert.match(skillsSource, /kaboom-managed-skill/)
  assert.match(skillsSource, /strum-/)
  assert.match(postinstallSource, /\[kaboom-mcp\]/)
})
