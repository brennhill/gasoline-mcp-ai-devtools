const test = require('node:test');
const assert = require('node:assert/strict');
const os = require('node:os');
const path = require('node:path');
const fs = require('node:fs');
const {
  generateDefaultConfig,
  buildMcpEntry,
  installToClient,
  executeInstall,
} = require('./install');

// --- generateDefaultConfig ---

test('generateDefaultConfig returns valid MCP config', () => {
  const cfg = generateDefaultConfig();
  assert.ok(cfg.mcpServers);
  assert.ok(cfg.mcpServers.gasoline);
  assert.equal(cfg.mcpServers.gasoline.command, 'gasoline-mcp');
});

// --- buildMcpEntry ---

test('buildMcpEntry returns JSON string for MCP entry', () => {
  const entry = buildMcpEntry();
  const parsed = JSON.parse(entry);
  assert.equal(parsed.command, 'gasoline-mcp');
  assert.deepEqual(parsed.args, []);
});

test('buildMcpEntry includes env vars when provided', () => {
  const entry = buildMcpEntry({ DEBUG: '1' });
  const parsed = JSON.parse(entry);
  assert.equal(parsed.env.DEBUG, '1');
});

// --- installToClient: file-type ---

test('installToClient creates new config for file-type client', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-install-'));
  const cfgPath = path.join(tmp, 'mcp.json');

  const def = {
    id: 'test-cursor',
    name: 'Test Cursor',
    type: 'file',
    configPath: { all: cfgPath },
    detectDir: { all: tmp },
  };

  const result = installToClient(def, { dryRun: false, envVars: {} });
  assert.equal(result.success, true);
  assert.equal(result.method, 'file');
  assert.equal(result.isNew, true);

  const written = JSON.parse(fs.readFileSync(cfgPath, 'utf8'));
  assert.ok(written.mcpServers.gasoline);
  assert.equal(written.mcpServers.gasoline.command, 'gasoline-mcp');

  fs.rmSync(tmp, { recursive: true });
});

test('installToClient merges into existing file-type config', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-install-'));
  const cfgPath = path.join(tmp, 'mcp.json');

  // Pre-existing config with another server
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

  const result = installToClient(def, { dryRun: false, envVars: {} });
  assert.equal(result.success, true);
  assert.equal(result.isNew, false);

  const written = JSON.parse(fs.readFileSync(cfgPath, 'utf8'));
  assert.ok(written.mcpServers.gasoline);
  assert.ok(written.mcpServers.other, 'should preserve existing server');

  fs.rmSync(tmp, { recursive: true });
});

test('installToClient dry-run does not write file', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-install-'));
  const cfgPath = path.join(tmp, 'mcp.json');

  const def = {
    id: 'test-cursor',
    name: 'Test Cursor',
    type: 'file',
    configPath: { all: cfgPath },
    detectDir: { all: tmp },
  };

  const result = installToClient(def, { dryRun: true, envVars: {} });
  assert.equal(result.success, true);
  assert.equal(fs.existsSync(cfgPath), false, 'should not create file in dry-run');

  fs.rmSync(tmp, { recursive: true });
});

test('installToClient adds env vars to file-type config', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-install-'));
  const cfgPath = path.join(tmp, 'mcp.json');

  const def = {
    id: 'test-cursor',
    name: 'Test Cursor',
    type: 'file',
    configPath: { all: cfgPath },
    detectDir: { all: tmp },
  };

  installToClient(def, { dryRun: false, envVars: { DEBUG: '1' } });
  const written = JSON.parse(fs.readFileSync(cfgPath, 'utf8'));
  assert.equal(written.mcpServers.gasoline.env.DEBUG, '1');

  fs.rmSync(tmp, { recursive: true });
});

// --- installToClient: CLI-type ---

test('installToClient handles CLI type with dry-run', () => {
  const def = {
    id: 'claude-code',
    name: 'Claude Code',
    type: 'cli',
    detectCommand: 'claude',
    installArgs: ['mcp', 'add-json', '--scope', 'user', 'gasoline'],
  };

  const result = installToClient(def, { dryRun: true, envVars: {} });
  assert.equal(result.success, true);
  assert.equal(result.method, 'cli');
  assert.ok(result.message.includes('claude'));
});

// --- executeInstall ---

test('executeInstall installs to detected file-type clients', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-install-'));
  const cursorDir = path.join(tmp, '.cursor');
  fs.mkdirSync(cursorDir);

  // Override detected clients for test
  const result = executeInstall({
    dryRun: false,
    envVars: {},
    _clientOverrides: [
      {
        id: 'test-cursor',
        name: 'Test Cursor',
        type: 'file',
        configPath: { all: path.join(cursorDir, 'mcp.json') },
        detectDir: { all: cursorDir },
      },
    ],
  });

  assert.equal(result.success, true);
  assert.equal(result.installed.length, 1);
  assert.equal(result.installed[0].name, 'Test Cursor');

  fs.rmSync(tmp, { recursive: true });
});

test('executeInstall reports when no clients detected', () => {
  const result = executeInstall({
    dryRun: false,
    envVars: {},
    _clientOverrides: [],
  });

  assert.equal(result.success, false);
  assert.equal(result.installed.length, 0);
});

test('executeInstall dry-run reports all detected clients without writing', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-install-'));
  const cursorDir = path.join(tmp, '.cursor');
  fs.mkdirSync(cursorDir);

  const result = executeInstall({
    dryRun: true,
    envVars: {},
    _clientOverrides: [
      {
        id: 'test-cursor',
        name: 'Test Cursor',
        type: 'file',
        configPath: { all: path.join(cursorDir, 'mcp.json') },
        detectDir: { all: cursorDir },
      },
    ],
  });

  assert.equal(result.success, true);
  assert.equal(result.installed.length, 1);
  assert.equal(fs.existsSync(path.join(cursorDir, 'mcp.json')), false);

  fs.rmSync(tmp, { recursive: true });
});
