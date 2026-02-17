const test = require('node:test');
const assert = require('node:assert/strict');
const os = require('node:os');
const path = require('node:path');
const fs = require('node:fs');
const { uninstallFromClient, executeUninstall } = require('./uninstall');

// --- uninstallFromClient: file-type ---

test('uninstallFromClient removes gasoline from file-type config', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-uninstall-'));
  const cfgPath = path.join(tmp, 'mcp.json');

  fs.writeFileSync(cfgPath, JSON.stringify({
    mcpServers: {
      gasoline: { command: 'gasoline-mcp', args: [] },
      other: { command: 'other-cmd', args: [] },
    },
  }));

  const def = {
    id: 'test-cursor',
    name: 'Test Cursor',
    type: 'file',
    configPath: { all: cfgPath },
    detectDir: { all: tmp },
  };

  const result = uninstallFromClient(def, { dryRun: false });
  assert.equal(result.status, 'removed');

  const written = JSON.parse(fs.readFileSync(cfgPath, 'utf8'));
  assert.equal(written.mcpServers.gasoline, undefined);
  assert.ok(written.mcpServers.other, 'should preserve other servers');

  fs.rmSync(tmp, { recursive: true });
});

test('uninstallFromClient deletes file when gasoline is only server', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-uninstall-'));
  const cfgPath = path.join(tmp, 'mcp.json');

  fs.writeFileSync(cfgPath, JSON.stringify({
    mcpServers: { gasoline: { command: 'gasoline-mcp', args: [] } },
  }));

  const def = {
    id: 'test-cursor',
    name: 'Test Cursor',
    type: 'file',
    configPath: { all: cfgPath },
    detectDir: { all: tmp },
  };

  const result = uninstallFromClient(def, { dryRun: false });
  assert.equal(result.status, 'removed');
  assert.equal(fs.existsSync(cfgPath), false, 'should delete file');

  fs.rmSync(tmp, { recursive: true });
});

test('uninstallFromClient returns notConfigured when file does not exist', () => {
  const def = {
    id: 'test-cursor',
    name: 'Test Cursor',
    type: 'file',
    configPath: { all: '/tmp/nonexistent-gasoline-test-12345/mcp.json' },
    detectDir: { all: '/tmp/nonexistent-gasoline-test-12345' },
  };

  const result = uninstallFromClient(def, { dryRun: false });
  assert.equal(result.status, 'notConfigured');
});

test('uninstallFromClient returns notConfigured when gasoline not in config', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-uninstall-'));
  const cfgPath = path.join(tmp, 'mcp.json');

  fs.writeFileSync(cfgPath, JSON.stringify({
    mcpServers: { other: { command: 'other-cmd', args: [] } },
  }));

  const def = {
    id: 'test-cursor',
    name: 'Test Cursor',
    type: 'file',
    configPath: { all: cfgPath },
    detectDir: { all: tmp },
  };

  const result = uninstallFromClient(def, { dryRun: false });
  assert.equal(result.status, 'notConfigured');

  fs.rmSync(tmp, { recursive: true });
});

test('uninstallFromClient dry-run does not modify file', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-uninstall-'));
  const cfgPath = path.join(tmp, 'mcp.json');

  const original = {
    mcpServers: { gasoline: { command: 'gasoline-mcp', args: [] } },
  };
  fs.writeFileSync(cfgPath, JSON.stringify(original));

  const def = {
    id: 'test-cursor',
    name: 'Test Cursor',
    type: 'file',
    configPath: { all: cfgPath },
    detectDir: { all: tmp },
  };

  const result = uninstallFromClient(def, { dryRun: true });
  assert.equal(result.status, 'removed');

  const still = JSON.parse(fs.readFileSync(cfgPath, 'utf8'));
  assert.ok(still.mcpServers.gasoline, 'should not modify in dry-run');

  fs.rmSync(tmp, { recursive: true });
});

// --- uninstallFromClient: CLI-type ---

test('uninstallFromClient handles CLI type with dry-run', () => {
  const def = {
    id: 'claude-code',
    name: 'Claude Code',
    type: 'cli',
    detectCommand: 'claude',
    removeArgs: ['mcp', 'remove', '--scope', 'user', 'gasoline'],
  };

  const result = uninstallFromClient(def, { dryRun: true });
  assert.equal(result.status, 'removed');
  assert.equal(result.method, 'cli');
});

// --- executeUninstall ---

test('executeUninstall removes from detected file-type clients', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-uninstall-'));
  const cfgPath = path.join(tmp, 'mcp.json');

  fs.writeFileSync(cfgPath, JSON.stringify({
    mcpServers: {
      gasoline: { command: 'gasoline-mcp', args: [] },
      other: { command: 'other-cmd', args: [] },
    },
  }));

  const result = executeUninstall({
    dryRun: false,
    _clientOverrides: [
      {
        id: 'test-cursor',
        name: 'Test Cursor',
        type: 'file',
        configPath: { all: cfgPath },
        detectDir: { all: tmp },
      },
    ],
  });

  assert.equal(result.success, true);
  assert.equal(result.removed.length, 1);

  fs.rmSync(tmp, { recursive: true });
});
