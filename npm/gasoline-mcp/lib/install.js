/**
 * Install logic for Gasoline MCP CLI
 * Handles installation to all detected AI assistant clients
 */

const { execFileSync } = require('child_process');
const {
  CLIENT_DEFINITIONS,
  getClientConfigPath,
  getDetectedClients,
  readConfigFile,
  writeConfigFile,
  mergeGassolineConfig,
} = require('./config');

/**
 * Generate default MCP config for gasoline
 * @returns {Object} Default gasoline MCP config
 */
function generateDefaultConfig() {
  return {
    mcpServers: {
      gasoline: {
        command: 'gasoline-mcp',
        args: [],
      },
    },
  };
}

/**
 * Build the MCP entry JSON string for CLI-based install
 * @param {Object} [envVars] Optional env vars
 * @returns {string} JSON string of the gasoline MCP entry
 */
function buildMcpEntry(envVars = {}) {
  const entry = { command: 'gasoline-mcp', args: [] };
  if (envVars && Object.keys(envVars).length > 0) {
    entry.env = envVars;
  }
  return JSON.stringify(entry);
}

/**
 * Install to a CLI-type client (e.g. Claude Code via `claude mcp add-json`)
 * @param {Object} def Client definition
 * @param {Object} options {dryRun, envVars}
 * @returns {Object} {success, name, method, message}
 */
function installViaCli(def, options) {
  const { dryRun = false, envVars = {} } = options;
  const entryJson = buildMcpEntry(envVars);
  const cmd = def.detectCommand;
  const args = [...def.installArgs];

  if (dryRun) {
    return {
      success: true,
      name: def.name,
      id: def.id,
      method: 'cli',
      message: `Would run: ${cmd} ${args.join(' ')} '<json>'`,
    };
  }

  try {
    // Must unset CLAUDECODE env var to avoid nested-session error
    const env = { ...process.env };
    delete env.CLAUDECODE;

    execFileSync(cmd, args, {
      input: entryJson,
      env,
      stdio: ['pipe', 'pipe', 'pipe'],
      timeout: 15000,
    });

    return {
      success: true,
      name: def.name,
      id: def.id,
      method: 'cli',
      message: `Installed via ${cmd} CLI`,
    };
  } catch (err) {
    return {
      success: false,
      name: def.name,
      id: def.id,
      method: 'cli',
      message: `CLI install failed: ${err.message}`,
      error: err.message,
    };
  }
}

/**
 * Install to a file-type client (config file write)
 * @param {Object} def Client definition
 * @param {Object} options {dryRun, envVars}
 * @returns {Object} {success, name, method, path, isNew, message}
 */
function installViaFile(def, options) {
  const { dryRun = false, envVars = {} } = options;
  const cfgPath = getClientConfigPath(def);

  if (!cfgPath) {
    return {
      success: false,
      name: def.name,
      id: def.id,
      method: 'file',
      message: `No config path for this platform`,
    };
  }

  const gassolineEntry = { command: 'gasoline-mcp', args: [] };
  let configData;
  let isNew = false;

  const readResult = readConfigFile(cfgPath);
  if (readResult.valid) {
    configData = readResult.data;
  } else {
    configData = generateDefaultConfig();
    isNew = true;
  }

  const merged = mergeGassolineConfig(configData, gassolineEntry, envVars);
  writeConfigFile(cfgPath, merged, dryRun);

  return {
    success: true,
    name: def.name,
    id: def.id,
    method: 'file',
    path: cfgPath,
    isNew,
    message: dryRun ? `Would write to ${cfgPath}` : `Wrote to ${cfgPath}`,
  };
}

/**
 * Install to a single client (dispatches by type)
 * @param {Object} def Client definition
 * @param {Object} options {dryRun, envVars}
 * @returns {Object} Result with success, name, method, etc.
 */
function installToClient(def, options) {
  if (def.type === 'cli') {
    return installViaCli(def, options);
  }
  return installViaFile(def, options);
}

/**
 * Execute install operation across all detected clients
 * @param {Object} options {dryRun, envVars, verbose, _clientOverrides}
 * @returns {Object} {success, installed, errors, total}
 */
function executeInstall(options = {}) {
  const { dryRun = false, envVars = {}, verbose = false } = options;

  // Allow test override of client list
  const clients = options._clientOverrides !== undefined
    ? options._clientOverrides
    : getDetectedClients();

  const result = {
    success: false,
    installed: [],
    errors: [],
    total: CLIENT_DEFINITIONS.length,
  };

  for (const def of clients) {
    try {
      const installResult = installToClient(def, { dryRun, envVars });

      if (installResult.success) {
        result.installed.push(installResult);
      } else {
        result.errors.push(installResult);
      }

      if (verbose) {
        const status = installResult.success ? 'OK' : 'FAIL';
        console.log(`[DEBUG] ${def.name}: ${status} - ${installResult.message}`);
      }
    } catch (err) {
      result.errors.push({
        name: def.name,
        id: def.id,
        message: err.message,
        recovery: err.recovery,
      });

      if (verbose) {
        console.log(`[DEBUG] Error on ${def.name}: ${err.message}`);
      }
    }
  }

  result.success = result.installed.length > 0;
  return result;
}

module.exports = {
  generateDefaultConfig,
  buildMcpEntry,
  installToClient,
  executeInstall,
};
