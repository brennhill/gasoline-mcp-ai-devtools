/**
 * Doctor diagnostics for Gasoline MCP CLI
 * Checks config validity, binary availability, and provides repair suggestions
 */

const fs = require('fs');
const { execSync } = require('child_process');
const { getConfigCandidates, getToolNameFromPath, readConfigFile } = require('./config');

/**
 * Test if gasoline binary is available and working
 * @returns {Object} {ok: bool, path?: string, version?: string, error?: string}
 */
function testBinary() {
  try {
    // Try to find the binary from node_modules
    const path = require('path');
    const os = require('os');
    const platform = os.platform();
    const arch = os.arch();

    const platformMap = {
      'darwin-arm64': '@brennhill/gasoline-darwin-arm64',
      'darwin-x64': '@brennhill/gasoline-darwin-x64',
      'linux-arm64': '@brennhill/gasoline-linux-arm64',
      'linux-x64': '@brennhill/gasoline-linux-x64',
      'win32-x64': '@brennhill/gasoline-win32-x64',
    };

    const key = `${platform}-${arch}`;
    const pkg = platformMap[key];

    if (!pkg) {
      return {
        ok: false,
        error: `Unsupported platform: ${platform}-${arch}`,
      };
    }

    // Try to find binary
    const binaryName = platform === 'win32' ? 'gasoline.exe' : 'gasoline';
    const homeDir = os.homedir();

    // Check several locations
    const candidates = [
      path.join(homeDir, '.npm', '_npx', pkg, 'bin', binaryName),
      path.join(homeDir, 'node_modules', pkg, 'bin', binaryName),
      path.join(__dirname, '..', 'node_modules', pkg, 'bin', binaryName),
      path.join(__dirname, '..', '..', pkg, 'bin', binaryName),
      path.join(__dirname, '..', '..', '..', pkg, 'bin', binaryName),
    ];

    let binaryPath = null;
    for (const candidate of candidates) {
      if (fs.existsSync(candidate)) {
        binaryPath = candidate;
        break;
      }
    }

    if (!binaryPath) {
      return {
        ok: false,
        error: `Gasoline binary not found for platform ${key}`,
      };
    }

    // Test binary with --version
    try {
      const version = execSync(`${binaryPath} --version`, {
        encoding: 'utf8',
        stdio: ['pipe', 'pipe', 'pipe'],
      }).trim();

      return {
        ok: true,
        path: binaryPath,
        version: version || 'unknown',
      };
    } catch (e) {
      return {
        ok: false,
        path: binaryPath,
        error: 'Binary found but failed to execute',
      };
    }
  } catch (err) {
    return {
      ok: false,
      error: `Error testing binary: ${err.message}`,
    };
  }
}

/**
 * Run full diagnostics on all config locations
 * @param {boolean} verbose If true, log debug info
 * @returns {Object} Diagnostic report with tools array and summary
 */
function runDiagnostics(verbose = false) {
  const candidates = getConfigCandidates();
  const tools = [];

  // Check each config location
  for (const candidatePath of candidates) {
    const toolName = getToolNameFromPath(candidatePath);
    const tool = {
      name: toolName,
      path: candidatePath,
      status: 'error',
      issues: [],
      suggestions: [],
    };

    if (verbose) {
      console.log(`[DEBUG] Checking ${toolName} at ${candidatePath}`);
    }

    // Check if file exists
    if (!fs.existsSync(candidatePath)) {
      tool.status = 'info';
      tool.issues.push('Config file not found');
      tool.suggestions.push('Run: gasoline-mcp --install --for-all');
      tools.push(tool);
      continue;
    }

    // Try to read and validate
    const readResult = readConfigFile(candidatePath);
    if (!readResult.valid) {
      tool.status = 'error';
      tool.issues.push('Invalid JSON');
      tool.suggestions.push('Fix the JSON syntax or run: gasoline-mcp --install');
      tools.push(tool);
      continue;
    }

    // Check if gasoline entry exists
    if (!readResult.data.mcpServers || !readResult.data.mcpServers.gasoline) {
      tool.status = 'error';
      tool.issues.push('gasoline entry missing from mcpServers');
      tool.suggestions.push('Run: gasoline-mcp --install --for-all');
      tools.push(tool);
      continue;
    }

    // All checks passed
    tool.status = 'ok';
    tools.push(tool);
  }

  // Check binary availability
  const binary = testBinary();

  // Generate summary
  const okCount = tools.filter(t => t.status === 'ok').length;
  const errorCount = tools.filter(t => t.status === 'error').length;
  const infoCount = tools.filter(t => t.status === 'info').length;

  let summary = `Summary: ${okCount} tool${okCount === 1 ? '' : 's'} ready`;
  if (errorCount > 0) {
    summary += `, ${errorCount} need${errorCount === 1 ? 's' : ''} repair`;
  }
  if (infoCount > 0) {
    summary += `, ${infoCount} not configured`;
  }

  return {
    tools,
    binary,
    summary,
  };
}

module.exports = {
  testBinary,
  runDiagnostics,
};
