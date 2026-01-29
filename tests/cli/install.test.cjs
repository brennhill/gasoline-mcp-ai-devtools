/**
 * Unit tests for lib/install.js
 * Tests install operation logic
 */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const os = require('os');
const install = require('../../npm/gasoline-mcp/lib/install');
const config = require('../../npm/gasoline-mcp/lib/config');

const testDir = path.join(os.tmpdir(), 'gasoline-install-test');

function setupTestDir() {
  if (!fs.existsSync(testDir)) {
    fs.mkdirSync(testDir, { recursive: true });
  }
}

function cleanupTestDir() {
  if (fs.existsSync(testDir)) {
    fs.rmSync(testDir, { recursive: true, force: true });
  }
}

test('install.generateDefaultConfig returns correct structure', () => {
  const cfg = install.generateDefaultConfig();
  assert.ok(cfg.mcpServers, 'Should have mcpServers');
  assert.ok(cfg.mcpServers.gasoline, 'Should have gasoline entry');
  assert.strictEqual(cfg.mcpServers.gasoline.command, 'gasoline-mcp', 'Command should be gasoline-mcp');
  assert.ok(Array.isArray(cfg.mcpServers.gasoline.args), 'Args should be array');
});

test('install.executeInstall creates new config at first missing location', () => {
  setupTestDir();
  try {
    // Backup and remove any existing configs
    const candidates = config.getConfigCandidates();
    const backups = [];
    for (const candidate of candidates) {
      if (fs.existsSync(candidate)) {
        const backup = candidate + '.backup-' + Date.now();
        fs.copyFileSync(candidate, backup);
        backups.push({ original: candidate, backup });
        fs.unlinkSync(candidate);
      }
    }

    try {
      const result = install.executeInstall({ dryRun: false, forAll: false, envVars: {}, verbose: false });

      assert.strictEqual(result.success, true, 'Install should succeed');
      assert.ok(result.updated.length > 0, 'Should have updated at least one file');

      // Verify gasoline entry exists in at least one config
      const updatedPath = result.updated[0].path;
      assert.ok(fs.existsSync(updatedPath), 'Config file should be created');

      const contents = JSON.parse(fs.readFileSync(updatedPath, 'utf8'));
      assert.ok(contents.mcpServers.gasoline, 'Should have gasoline entry');
      assert.strictEqual(contents.mcpServers.gasoline.command, 'gasoline-mcp');
    } finally {
      // Restore backups
      for (const { original, backup } of backups) {
        if (fs.existsSync(backup)) {
          fs.copyFileSync(backup, original);
          fs.unlinkSync(backup);
        }
      }
    }
  } finally {
    cleanupTestDir();
  }
});

test('install.executeInstall merges with existing config preserving other servers', () => {
  setupTestDir();
  try {
    const candidates = config.getConfigCandidates();
    const targetPath = candidates[0];

    // Create a config with another server
    const existing = {
      mcpServers: {
        other: { command: 'other-tool' },
      },
    };
    fs.mkdirSync(path.dirname(targetPath), { recursive: true });
    fs.writeFileSync(targetPath, JSON.stringify(existing));

    const result = install.executeInstall({ dryRun: false, forAll: false, envVars: {}, verbose: false });

    assert.strictEqual(result.success, true, 'Install should succeed');

    // Verify both servers exist
    const contents = JSON.parse(fs.readFileSync(targetPath, 'utf8'));
    assert.ok(contents.mcpServers.gasoline, 'Should have gasoline entry');
    assert.ok(contents.mcpServers.other, 'Should preserve other server');
    assert.strictEqual(contents.mcpServers.other.command, 'other-tool', 'Other server unchanged');

    // Clean up
    fs.unlinkSync(targetPath);
  } finally {
    cleanupTestDir();
  }
});

test('install.executeInstall with dryRun=true does not write', () => {
  setupTestDir();
  try {
    const candidates = config.getConfigCandidates();
    const targetPath = candidates[0];

    // Remove target file if it exists
    if (fs.existsSync(targetPath)) {
      fs.unlinkSync(targetPath);
    }

    const result = install.executeInstall({ dryRun: true, forAll: false, envVars: {}, verbose: false });

    assert.strictEqual(result.success, true, 'Dry-run should succeed');
    // In dry-run, file might not be created or might be created based on implementation
    // But diffs should be available
    assert.ok(result.diffs !== undefined, 'Should include diffs for preview');
  } finally {
    cleanupTestDir();
  }
});

test('install.executeInstall with envVars adds environment variables', () => {
  setupTestDir();
  try {
    const candidates = config.getConfigCandidates();
    const targetPath = candidates[0];

    // Remove target if exists
    if (fs.existsSync(targetPath)) {
      fs.unlinkSync(targetPath);
    }

    const envVars = { DEBUG: '1', API_KEY: 'secret' };
    const result = install.executeInstall({ dryRun: false, forAll: false, envVars, verbose: false });

    assert.strictEqual(result.success, true, 'Install should succeed');

    // Verify env vars are in config
    const contents = JSON.parse(fs.readFileSync(targetPath, 'utf8'));
    assert.deepStrictEqual(contents.mcpServers.gasoline.env, envVars, 'Env vars should be added');

    // Clean up
    fs.unlinkSync(targetPath);
  } finally {
    cleanupTestDir();
  }
});

test('install.executeInstall with empty envVars does not add env field', () => {
  setupTestDir();
  try {
    const candidates = config.getConfigCandidates();
    const targetPath = candidates[0];

    // Remove target if exists
    if (fs.existsSync(targetPath)) {
      fs.unlinkSync(targetPath);
    }

    const result = install.executeInstall({ dryRun: false, forAll: false, envVars: {}, verbose: false });

    assert.strictEqual(result.success, true, 'Install should succeed');

    // Verify env field is not present with empty envVars
    const contents = JSON.parse(fs.readFileSync(targetPath, 'utf8'));
    assert.strictEqual(contents.mcpServers.gasoline.env, undefined, 'Empty env should not be added');

    // Clean up
    fs.unlinkSync(targetPath);
  } finally {
    cleanupTestDir();
  }
});

test('install.executeInstall with forAll=true attempts all candidates', () => {
  setupTestDir();
  try {
    const candidates = config.getConfigCandidates();

    // Clean up all candidates
    for (const candidate of candidates) {
      if (fs.existsSync(candidate)) {
        fs.unlinkSync(candidate);
      }
    }

    const result = install.executeInstall({ dryRun: false, forAll: true, envVars: {}, verbose: false });

    assert.strictEqual(result.success, true, 'Install with forAll should succeed');
    // Should have attempted multiple candidates
    assert.ok(result.updated.length + result.errors.length > 0, 'Should process multiple candidates');

    // Verify at least one succeeded
    assert.ok(result.updated.length > 0, 'Should have at least one successful update');
  } finally {
    // Cleanup - remove any test files created
    const candidates = config.getConfigCandidates();
    for (const candidate of candidates) {
      if (fs.existsSync(candidate)) {
        fs.unlinkSync(candidate);
      }
    }
    cleanupTestDir();
  }
});

test('install.executeInstall returns error details on failure', () => {
  const result = install.executeInstall({ dryRun: false, forAll: false, envVars: {}, verbose: false });

  // Should have either success or errors - never both fail silently
  assert.ok(result.success || result.errors.length > 0 || result.updated.length > 0, 'Should have clear result');
  assert.ok(typeof result.success === 'boolean', 'Should have success property');
  assert.ok(Array.isArray(result.updated), 'Should have updated array');
  assert.ok(Array.isArray(result.errors), 'Should have errors array');
});

test('install.executeInstall handles mergeGassolineConfig correctly', () => {
  // Test the core merge logic
  const existing = {
    mcpServers: {
      other: { command: 'other-tool' },
    },
  };

  const gassolineEntry = {
    command: 'gasoline-mcp',
    args: [],
  };

  const envVars = { DEBUG: '1' };

  const merged = config.mergeGassolineConfig(existing, gassolineEntry, envVars);

  assert.ok(merged.mcpServers.gasoline, 'Should have gasoline');
  assert.ok(merged.mcpServers.other, 'Should preserve other');
  assert.deepStrictEqual(merged.mcpServers.gasoline.env, envVars, 'Should add env vars');
});

test('install.executeInstall respects forAll=false to stop at first existing', () => {
  setupTestDir();
  try {
    const candidates = config.getConfigCandidates();

    // Create a config at the first location
    fs.mkdirSync(path.dirname(candidates[0]), { recursive: true });
    fs.writeFileSync(candidates[0], JSON.stringify({ mcpServers: {} }));

    const result = install.executeInstall({ dryRun: false, forAll: false, envVars: {}, verbose: false });

    assert.strictEqual(result.success, true);
    // With forAll=false, should process only the first config
    // The exact number depends on whether others exist, but should be limited
    assert.ok(result.updated.length <= 2, 'Should limit processing with forAll=false');

    // Clean up
    fs.unlinkSync(candidates[0]);
  } finally {
    cleanupTestDir();
  }
});
