// Purpose: Validate uninstall behavior for npm wrapper-managed MCP config entries.
// Why: Ensures cleanup removes only gasoline entries while preserving user config state.
// Docs: docs/features/feature/enhanced-cli-config/index.md

const test = require('node:test');
const assert = require('node:assert/strict');
const os = require('node:os');
const path = require('node:path');
const fs = require('node:fs');
const { uninstallFromClient, executeUninstall } = require('./uninstall');

test('npm wrapper no longer exposes gasoline launcher aliases', () => {
  const packageJson = JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'package.json'), 'utf8'));
  const hooksLauncher = fs.readFileSync(path.join(__dirname, '..', 'bin', 'kaboom-hooks'), 'utf8');

  assert.equal(packageJson.bin['gasoline-agentic-browser'], undefined);
  assert.equal(packageJson.bin['gasoline-hooks'], undefined);
  assert.equal(packageJson.bin['kaboom-agentic-browser'], 'bin/kaboom-agentic-browser');
  assert.equal(packageJson.bin['kaboom-hooks'], 'bin/kaboom-hooks');
  assert.match(hooksLauncher, /kaboom-hooks binary not found/);
  assert.match(hooksLauncher, /npm install -g kaboom-agentic-browser@latest/);
});

// --- uninstallFromClient: file-type ---

test('uninstallFromClient removes kaboom, gasoline, and strum managed entries from file-type config', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-uninstall-'));
  const cfgPath = path.join(tmp, 'mcp.json');

  fs.writeFileSync(cfgPath, JSON.stringify({
    mcpServers: {
      'kaboom-browser-devtools': { command: 'kaboom-agentic-browser', args: [] },
      gasoline: { command: 'gasoline-mcp', args: [] },
      'strum-browser-devtools': { command: 'strum-agentic-browser', args: [] },
      strum: { command: 'strum-agentic-browser', args: [] },
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
  assert.equal(written.mcpServers['kaboom-browser-devtools'], undefined);
  assert.equal(written.mcpServers.gasoline, undefined);
  assert.equal(written.mcpServers['strum-browser-devtools'], undefined);
  assert.equal(written.mcpServers.strum, undefined);
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
    removeArgs: ['mcp', 'remove', '--scope', 'user', 'kaboom-browser-devtools'],
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

test('executeUninstall removes kaboom, gasoline, and strum managed skill files', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-uninstall-'));
  const claudeRoot = path.join(tmp, 'claude-skills');
  fs.mkdirSync(claudeRoot, { recursive: true });
  fs.writeFileSync(
    path.join(claudeRoot, 'debug.md'),
    '<!-- kaboom-managed-skill id:debug version:2 -->\ncurrent kaboom skill\n',
    'utf8'
  );
  fs.writeFileSync(
    path.join(claudeRoot, 'gasoline-debug.md'),
    '<!-- gasoline-managed-skill id:debug version:1 -->\nold gasoline skill\n',
    'utf8'
  );
  fs.writeFileSync(
    path.join(claudeRoot, 'strum-debug.md'),
    '<!-- strum-managed-skill id:debug version:1 -->\nold strum skill\n',
    'utf8'
  );

  const originalClaudeDir = process.env.GASOLINE_CLAUDE_SKILLS_DIR;
  try {
    process.env.GASOLINE_CLAUDE_SKILLS_DIR = claudeRoot;
    const result = executeUninstall({
      dryRun: false,
      _clientOverrides: [],
      skillAgents: ['claude'],
      skillScope: 'global',
    });

    assert.equal(result.success, true);
    assert.ok(result.skillCleanup);
    assert.equal(result.skillCleanup.removed, 3);
    assert.equal(fs.existsSync(path.join(claudeRoot, 'debug.md')), false);
    assert.equal(fs.existsSync(path.join(claudeRoot, 'gasoline-debug.md')), false);
    assert.equal(fs.existsSync(path.join(claudeRoot, 'strum-debug.md')), false);
  } finally {
    if (originalClaudeDir === undefined) {
      delete process.env.GASOLINE_CLAUDE_SKILLS_DIR;
    } else {
      process.env.GASOLINE_CLAUDE_SKILLS_DIR = originalClaudeDir;
    }
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});
