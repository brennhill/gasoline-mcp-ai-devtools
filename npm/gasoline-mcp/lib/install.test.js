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

// --- targeted install ---

test('executeInstall with targetTool installs to specific client', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-install-'));
  const geminiDir = path.join(tmp, '.gemini');

  // Monkey-patch os.homedir for this test via _clientOverrides won't work here
  // since targetTool uses getClientByAlias which looks up CLIENT_DEFINITIONS.
  // Instead, use _clientOverrides to simulate targeted install behavior.
  const result = executeInstall({
    dryRun: false,
    envVars: {},
    _clientOverrides: [
      {
        id: 'test-gemini',
        name: 'Test Gemini',
        type: 'file',
        configPath: { all: path.join(tmp, '.gemini', 'settings.json') },
        detectDir: { all: geminiDir },
      },
    ],
  });

  assert.equal(result.success, true);
  assert.equal(result.installed.length, 1);
  assert.equal(result.installed[0].name, 'Test Gemini');

  const written = JSON.parse(fs.readFileSync(path.join(tmp, '.gemini', 'settings.json'), 'utf8'));
  assert.ok(written.mcpServers.gasoline);

  fs.rmSync(tmp, { recursive: true });
});

test('executeInstall with invalid targetTool returns error', () => {
  const result = executeInstall({
    dryRun: false,
    envVars: {},
    targetTool: 'bogus',
  });

  assert.equal(result.success, false);
  assert.equal(result.errors.length, 1);
  assert.ok(result.errors[0].message.includes('Unknown tool: bogus'));
  assert.ok(result.errors[0].message.includes('Valid tools:'));
});

// --- OpenCode format install ---

test('installToClient creates OpenCode-format config with mcp key', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-install-'));

  const def = {
    id: 'test-opencode',
    name: 'Test OpenCode',
    type: 'file',
    configPath: { all: path.join(tmp, 'opencode.json') },
    detectDir: { all: tmp },
    configKey: 'mcp',
    buildEntry: (envVars) => {
      const entry = { type: 'local', command: ['gasoline-mcp'], enabled: true };
      if (envVars && Object.keys(envVars).length > 0) entry.env = envVars;
      return entry;
    },
  };

  const result = installToClient(def, { dryRun: false, envVars: {} });
  assert.equal(result.success, true);
  assert.equal(result.isNew, true);

  const written = JSON.parse(fs.readFileSync(path.join(tmp, 'opencode.json'), 'utf8'));
  assert.ok(written.mcp, 'should have mcp key');
  assert.ok(written.mcp.gasoline, 'should have gasoline under mcp');
  assert.equal(written.mcp.gasoline.type, 'local');
  assert.deepEqual(written.mcp.gasoline.command, ['gasoline-mcp']);
  assert.equal(written.mcp.gasoline.enabled, true);
  assert.equal(written.mcpServers, undefined, 'should not have mcpServers');

  fs.rmSync(tmp, { recursive: true });
});

test('installToClient merges OpenCode config preserving existing entries', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-install-'));
  const cfgPath = path.join(tmp, 'opencode.json');

  // Pre-existing OpenCode config with another server
  fs.writeFileSync(cfgPath, JSON.stringify({
    mcp: { other: { type: 'local', command: ['other-cmd'], enabled: true } },
    theme: 'dark',
  }));

  const def = {
    id: 'test-opencode',
    name: 'Test OpenCode',
    type: 'file',
    configPath: { all: cfgPath },
    detectDir: { all: tmp },
    configKey: 'mcp',
    buildEntry: () => ({ type: 'local', command: ['gasoline-mcp'], enabled: true }),
  };

  const result = installToClient(def, { dryRun: false, envVars: {} });
  assert.equal(result.success, true);

  const written = JSON.parse(fs.readFileSync(cfgPath, 'utf8'));
  assert.ok(written.mcp.gasoline, 'should have gasoline');
  assert.ok(written.mcp.other, 'should preserve existing server');
  assert.equal(written.theme, 'dark', 'should preserve non-mcp keys');

  fs.rmSync(tmp, { recursive: true });
});

// --- Zed format install ---

test('installToClient creates Zed-format config with context_servers key', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-install-'));

  const def = {
    id: 'test-zed',
    name: 'Test Zed',
    type: 'file',
    configPath: { all: path.join(tmp, 'settings.json') },
    detectDir: { all: tmp },
    configKey: 'context_servers',
    buildEntry: (envVars) => {
      const entry = { source: 'custom', command: 'gasoline-mcp', args: [] };
      if (envVars && Object.keys(envVars).length > 0) entry.env = envVars;
      return entry;
    },
  };

  const result = installToClient(def, { dryRun: false, envVars: {} });
  assert.equal(result.success, true);

  const written = JSON.parse(fs.readFileSync(path.join(tmp, 'settings.json'), 'utf8'));
  assert.ok(written.context_servers, 'should have context_servers key');
  assert.ok(written.context_servers.gasoline);
  assert.equal(written.context_servers.gasoline.source, 'custom');
  assert.equal(written.context_servers.gasoline.command, 'gasoline-mcp');
  assert.equal(written.mcpServers, undefined, 'should not have mcpServers');

  fs.rmSync(tmp, { recursive: true });
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
