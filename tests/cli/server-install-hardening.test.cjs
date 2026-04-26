const test = require('node:test')
const assert = require('node:assert')
const fs = require('node:fs')
const path = require('node:path')

const REPO_ROOT = path.resolve(__dirname, '../..')
const INSTALL_SCRIPT = path.join(REPO_ROOT, 'server', 'scripts', 'install.js')

function readInstallScript() {
  return fs.readFileSync(INSTALL_SCRIPT, 'utf8')
}

test('server postinstall verifies binary checksum from release checksums manifest', () => {
  const script = readInstallScript()

  assert.match(
    script,
    /releases\/download\/v\$\{VERSION\}\/checksums\.txt/,
    'install.js should fetch release checksums.txt for verification'
  )
  assert.match(
    script,
    /createHash\('sha256'\)/,
    'install.js should compute SHA-256 checksum for downloaded binary'
  )
  assert.match(
    script,
    /verifyDownloadedBinary\(/,
    'install.js should run explicit downloaded-binary verification'
  )
})

test('server postinstall validates existing daemon identity/version when port is already in use', () => {
  const script = readInstallScript()

  assert.match(
    script,
    /EXPECTED_SERVICE_NAME = 'kaboom-browser-devtools'/,
    'install.js should enforce expected health service identity'
  )
  assert.match(
    script,
    /checkServerIdentity\(port, VERSION\)/,
    'install.js should validate service identity and version before accepting in-use port'
  )
  assert.match(
    script,
    /non-matching service\/version/,
    'install.js should surface mismatch warning when port owner is not the expected daemon'
  )
})

test('server postinstall skips daemon auto-start in privileged contexts', () => {
  const script = readInstallScript()

  assert.match(
    script,
    /function isPrivilegedInstallContext\(/,
    'install.js should define an explicit elevated-privilege guard'
  )
  assert.match(
    script,
    /process\.getuid\?\.\(\) === 0|process\.env\.SUDO_USER/,
    'install.js should detect root or sudo execution contexts'
  )
  assert.match(
    script,
    /Skipping background auto-start because install is running with elevated privileges/,
    'install.js should warn instead of auto-starting a privileged daemon'
  )
})
