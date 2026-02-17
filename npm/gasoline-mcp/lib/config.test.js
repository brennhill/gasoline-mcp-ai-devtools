const test = require('node:test');
const assert = require('node:assert/strict');
const os = require('node:os');
const path = require('node:path');
const fs = require('node:fs');
const {
  CLIENT_DEFINITIONS,
  getClientConfigPath,
  isClientInstalled,
  getDetectedClients,
  commandExistsOnPath,
  getConfigCandidates,
  getToolNameFromPath,
  getClientById,
} = require('./config');

// --- CLIENT_DEFINITIONS ---

test('CLIENT_DEFINITIONS contains all 5 clients', () => {
  const ids = CLIENT_DEFINITIONS.map(c => c.id);
  assert.deepEqual(ids, ['claude-code', 'claude-desktop', 'cursor', 'windsurf', 'vscode']);
});

test('each client definition has required fields', () => {
  for (const def of CLIENT_DEFINITIONS) {
    assert.ok(def.id, `missing id`);
    assert.ok(def.name, `missing name for ${def.id}`);
    assert.ok(['cli', 'file'].includes(def.type), `invalid type for ${def.id}`);
    if (def.type === 'cli') {
      assert.ok(def.detectCommand, `missing detectCommand for ${def.id}`);
      assert.ok(Array.isArray(def.installArgs), `missing installArgs for ${def.id}`);
      assert.ok(Array.isArray(def.removeArgs), `missing removeArgs for ${def.id}`);
    } else {
      assert.ok(def.configPath, `missing configPath for ${def.id}`);
      assert.ok(def.detectDir, `missing detectDir for ${def.id}`);
    }
  }
});

test('claude-code is CLI type with correct detect command', () => {
  const cc = CLIENT_DEFINITIONS.find(c => c.id === 'claude-code');
  assert.equal(cc.type, 'cli');
  assert.equal(cc.detectCommand, 'claude');
});

test('cursor uses correct config path', () => {
  const cursor = CLIENT_DEFINITIONS.find(c => c.id === 'cursor');
  assert.equal(cursor.type, 'file');
  assert.ok(cursor.configPath.all.includes('.cursor/mcp.json'));
});

test('windsurf uses correct config path (not .codeium/mcp.json)', () => {
  const ws = CLIENT_DEFINITIONS.find(c => c.id === 'windsurf');
  assert.ok(ws.configPath.all.includes('.codeium/windsurf/mcp_config.json'));
});

// --- getClientById ---

test('getClientById returns definition by id', () => {
  const cursor = getClientById('cursor');
  assert.equal(cursor.name, 'Cursor');
});

test('getClientById returns undefined for unknown id', () => {
  assert.equal(getClientById('nonexistent'), undefined);
});

// --- getClientConfigPath ---

test('getClientConfigPath returns platform-specific path for claude-desktop on darwin', () => {
  const def = CLIENT_DEFINITIONS.find(c => c.id === 'claude-desktop');
  const result = getClientConfigPath(def, 'darwin');
  assert.ok(result.includes('Library/Application Support/Claude/claude_desktop_config.json'));
});

test('getClientConfigPath returns platform-specific path for vscode on linux', () => {
  const def = CLIENT_DEFINITIONS.find(c => c.id === 'vscode');
  const result = getClientConfigPath(def, 'linux');
  assert.ok(result.includes('.config/Code/User/mcp.json'));
});

test('getClientConfigPath returns "all" path for cursor', () => {
  const def = CLIENT_DEFINITIONS.find(c => c.id === 'cursor');
  const result = getClientConfigPath(def);
  assert.ok(result.includes('.cursor/mcp.json'));
});

test('getClientConfigPath returns null for CLI type', () => {
  const def = CLIENT_DEFINITIONS.find(c => c.id === 'claude-code');
  const result = getClientConfigPath(def);
  assert.equal(result, null);
});

test('getClientConfigPath returns null for unsupported platform', () => {
  const def = CLIENT_DEFINITIONS.find(c => c.id === 'claude-desktop');
  // claude-desktop only has darwin + win32
  const result = getClientConfigPath(def, 'linux');
  assert.equal(result, null);
});

// --- isClientInstalled ---

test('isClientInstalled detects existing directory for file-type client', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-test-'));
  const cursorDir = path.join(tmp, '.cursor');
  fs.mkdirSync(cursorDir);

  const def = {
    id: 'test-cursor',
    type: 'file',
    detectDir: { all: path.join(tmp, '.cursor') },
    configPath: { all: path.join(tmp, '.cursor', 'mcp.json') },
  };

  assert.equal(isClientInstalled(def), true);
  fs.rmSync(tmp, { recursive: true });
});

test('isClientInstalled returns false when directory does not exist', () => {
  const def = {
    id: 'test-missing',
    type: 'file',
    detectDir: { all: '/tmp/nonexistent-gasoline-test-dir-12345' },
    configPath: { all: '/tmp/nonexistent-gasoline-test-dir-12345/mcp.json' },
  };

  assert.equal(isClientInstalled(def), false);
});

test('isClientInstalled checks detectCommand for CLI type', () => {
  // 'node' should be on PATH
  const def = {
    id: 'test-cli',
    type: 'cli',
    detectCommand: 'node',
  };
  assert.equal(isClientInstalled(def), true);
});

test('isClientInstalled returns false for missing CLI command', () => {
  const def = {
    id: 'test-cli-missing',
    type: 'cli',
    detectCommand: 'nonexistent-command-gasoline-test-12345',
  };
  assert.equal(isClientInstalled(def), false);
});

// --- commandExistsOnPath ---

test('commandExistsOnPath finds node', () => {
  assert.equal(commandExistsOnPath('node'), true);
});

test('commandExistsOnPath returns false for missing command', () => {
  assert.equal(commandExistsOnPath('nonexistent-command-gasoline-test-12345'), false);
});

// --- getDetectedClients ---

test('getDetectedClients returns only installed clients', () => {
  const detected = getDetectedClients();
  assert.ok(Array.isArray(detected));
  // Each should have isDetected = true implicitly (they passed the filter)
  for (const d of detected) {
    assert.ok(d.id, 'each detected client should have an id');
  }
});

// --- getConfigCandidates (backward compat) ---

test('getConfigCandidates returns file paths for detected file-type clients', () => {
  const candidates = getConfigCandidates();
  assert.ok(Array.isArray(candidates));
  // Should be strings (file paths), no CLI entries
  for (const c of candidates) {
    assert.equal(typeof c, 'string');
  }
});

// --- getToolNameFromPath (backward compat) ---

test('getToolNameFromPath maps cursor path correctly', () => {
  const homeDir = os.homedir();
  assert.equal(getToolNameFromPath(path.join(homeDir, '.cursor', 'mcp.json')), 'Cursor');
});

test('getToolNameFromPath maps windsurf path correctly', () => {
  const homeDir = os.homedir();
  assert.equal(getToolNameFromPath(path.join(homeDir, '.codeium', 'windsurf', 'mcp_config.json')), 'Windsurf');
});

test('getToolNameFromPath returns Unknown for unrecognized path', () => {
  assert.equal(getToolNameFromPath('/some/random/path'), 'Unknown');
});
