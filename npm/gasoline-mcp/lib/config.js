/**
 * Config file utilities for Gasoline MCP CLI
 * Handles reading, writing, validating, and merging MCP configurations
 */

const fs = require('fs');
const path = require('path');
const os = require('os');
const {
  InvalidJSONError,
  PermissionError,
  ConfigValidationError,
  FileSizeError,
} = require('./errors');

const MAX_CONFIG_SIZE = 1024 * 1024; // 1MB

/**
 * Get list of candidate config file paths (Claude, VSCode, Cursor, Codeium)
 * @returns {Array<string>} Array of config file paths
 */
function getConfigCandidates() {
  const homeDir = os.homedir();
  return [
    path.join(homeDir, '.claude.json'),
    path.join(homeDir, '.vscode', 'claude.mcp.json'),
    path.join(homeDir, '.cursor', 'mcp.json'),
    path.join(homeDir, '.codeium', 'mcp.json'),
  ];
}

/**
 * Get tool name from config path
 * @param {string} configPath Path to config file
 * @returns {string} Tool name
 */
function getToolNameFromPath(configPath) {
  const normalized = path.normalize(configPath);
  if (normalized.includes('.claude')) return 'Claude Desktop';
  if (normalized.includes('.vscode')) return 'VSCode';
  if (normalized.includes('.cursor')) return 'Cursor';
  if (normalized.includes('.codeium')) return 'Codeium';
  return 'Unknown';
}

/**
 * Read and parse a config file
 * @param {string} filePath Path to config file
 * @returns {Object} {valid: bool, data: obj, error: string, stats: {size, mtime}}
 */
function readConfigFile(filePath) {
  try {
    // Check file size
    const stats = fs.statSync(filePath);
    if (stats.size > MAX_CONFIG_SIZE) {
      throw new FileSizeError(filePath, stats.size);
    }

    // Read and parse
    const content = fs.readFileSync(filePath, 'utf8');
    let data;
    try {
      data = JSON.parse(content);
    } catch (parseErr) {
      // Try to find line number
      const lines = content.split('\n');
      let lineNumber = 1;
      for (let i = 0; i < lines.length; i++) {
        try {
          JSON.parse(lines.slice(0, i + 1).join('\n'));
        } catch (e) {
          lineNumber = i + 1;
          break;
        }
      }
      throw new InvalidJSONError(filePath, lineNumber, parseErr.message);
    }

    return {
      valid: true,
      data,
      error: null,
      stats: { size: stats.size, mtime: stats.mtime },
    };
  } catch (err) {
    if (err instanceof InvalidJSONError || err instanceof FileSizeError) {
      throw err;
    }
    // File doesn't exist or can't be read
    return {
      valid: false,
      data: null,
      error: err.message,
      stats: null,
    };
  }
}

/**
 * Write config file (with optional dry-run)
 * Atomic write: temp file + rename
 * @param {string} filePath Path to config file
 * @param {Object} data Config object to write
 * @param {boolean} dryRun If true, returns what would be written without writing
 * @returns {Object} {success: bool, message: string, path: string, before?: Object, after?: Object}
 */
function writeConfigFile(filePath, data, dryRun = false) {
  try {
    // Validate data
    const errors = validateMCPConfig(data);
    if (errors.length > 0) {
      throw new ConfigValidationError(errors);
    }

    const jsonStr = JSON.stringify(data, null, 2);

    if (dryRun) {
      return {
        success: true,
        message: `Would write to ${filePath}`,
        path: filePath,
        after: data,
      };
    }

    // Ensure directory exists
    const dir = path.dirname(filePath);
    fs.mkdirSync(dir, { recursive: true });

    // Atomic write: temp file + rename
    const tempPath = `${filePath}.tmp`;
    try {
      fs.writeFileSync(tempPath, jsonStr + '\n', 'utf8');
      fs.renameSync(tempPath, filePath);
    } catch (writeErr) {
      // Clean up temp file if it exists
      try {
        fs.unlinkSync(tempPath);
      } catch (e) {
        // Ignore cleanup errors
      }

      if (writeErr.code === 'EACCES') {
        throw new PermissionError(filePath);
      }
      throw writeErr;
    }

    return {
      success: true,
      message: `Wrote to ${filePath}`,
      path: filePath,
      after: data,
    };
  } catch (err) {
    if (err instanceof ConfigValidationError || err instanceof PermissionError) {
      throw err;
    }
    throw err;
  }
}

/**
 * Validate MCP config structure
 * @param {Object} data Config object to validate
 * @returns {Array<string>} Array of error messages (empty if valid)
 */
function validateMCPConfig(data) {
  const errors = [];

  if (!data || typeof data !== 'object') {
    errors.push('Config must be an object');
    return errors;
  }

  if (!data.mcpServers) {
    errors.push('Config must have "mcpServers" property');
  } else if (typeof data.mcpServers !== 'object' || Array.isArray(data.mcpServers)) {
    errors.push('"mcpServers" must be an object (not an array)');
  }

  return errors;
}

/**
 * Merge gasoline config into existing config
 * @param {Object} existing Existing config object
 * @param {Object} gassolineEntry New gasoline entry {command, args, env}
 * @param {Object} envVars Additional env vars to merge {KEY: VALUE}
 * @returns {Object} Merged config
 */
function mergeGassolineConfig(existing, gassolineEntry, envVars = {}) {
  const merged = JSON.parse(JSON.stringify(existing)); // Deep copy

  // Ensure mcpServers exists
  if (!merged.mcpServers) {
    merged.mcpServers = {};
  }

  // Merge gasoline entry
  merged.mcpServers.gasoline = {
    command: gassolineEntry.command,
    args: gassolineEntry.args || [],
  };

  // Add env vars if provided
  if (envVars && Object.keys(envVars).length > 0) {
    merged.mcpServers.gasoline.env = envVars;
  }

  return merged;
}

/**
 * Parse and validate env var string (KEY=VALUE)
 * @param {string} envStr Environment variable string
 * @returns {Object} {key: string, value: string} or throws InvalidEnvFormatError
 */
function parseEnvVar(envStr) {
  const { InvalidEnvFormatError } = require('./errors');
  const parts = envStr.split('=');
  if (parts.length !== 2 || !parts[0] || !parts[1]) {
    throw new InvalidEnvFormatError(envStr);
  }
  const [key, value] = parts;

  // Validate key (no null bytes or control chars)
  if (!/^[A-Z_][A-Z0-9_]*$/i.test(key)) {
    throw new InvalidEnvFormatError(envStr);
  }

  return { key, value };
}

module.exports = {
  getConfigCandidates,
  getToolNameFromPath,
  readConfigFile,
  writeConfigFile,
  validateMCPConfig,
  mergeGassolineConfig,
  parseEnvVar,
};
