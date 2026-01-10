/**
 * Install logic for Gasoline MCP CLI
 * Handles installation to config files with support for --dry-run, --for-all, --env flags
 */

const fs = require('fs');
const {
  getConfigCandidates,
  getToolNameFromPath,
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
 * Execute install operation
 * @param {Object} options {dryRun: bool, forAll: bool, envVars: {}, verbose: bool}
 * @returns {Object} {success: bool, updated: [{name, path}], errors: [{name, message}], diffs: []}
 */
function executeInstall(options = {}) {
  const { dryRun = false, forAll = false, envVars = {}, verbose = false } = options;

  const result = {
    success: false,
    updated: [],
    errors: [],
    diffs: [],
    total: 4,
  };

  const candidates = getConfigCandidates();
  const gassolineEntry = {
    command: 'gasoline-mcp',
    args: [],
  };

  let foundExisting = false;

  for (const candidatePath of candidates) {
    const toolName = getToolNameFromPath(candidatePath);

    try {
      let configData;
      let isNew = false;

      // Try to read existing config
      const readResult = readConfigFile(candidatePath);
      if (readResult.valid) {
        configData = readResult.data;
        foundExisting = true;
      } else {
        // File doesn't exist, create new config
        configData = generateDefaultConfig();
        isNew = true;
      }

      // Merge gasoline config
      const before = JSON.parse(JSON.stringify(configData)); // Deep copy for diff
      const merged = mergeGassolineConfig(configData, gassolineEntry, envVars);

      // Write config
      const writeResult = writeConfigFile(candidatePath, merged, dryRun);

      result.updated.push({
        name: toolName,
        path: candidatePath,
        isNew,
      });

      if (dryRun && writeResult.before) {
        result.diffs.push({
          path: candidatePath,
          before,
          after: merged,
        });
      }

      if (verbose) {
        console.log(`[DEBUG] ${isNew ? 'Created' : 'Updated'}: ${candidatePath}`);
      }

      // Stop at first match if not --for-all
      if (!forAll && foundExisting) {
        break;
      }
    } catch (err) {
      // If file doesn't exist and we're looking for existing, continue
      if (!fs.existsSync(candidatePath) && !forAll) {
        continue;
      }

      result.errors.push({
        name: toolName,
        message: err.message,
        recovery: err.recovery,
      });

      if (verbose) {
        console.log(`[DEBUG] Error on ${candidatePath}: ${err.message}`);
      }

      // If --for-all, continue even on error
      if (!forAll) {
        break;
      }
    }
  }

  // If no existing config found and --for-all not set, we still create the default
  if (!foundExisting && !forAll && result.updated.length === 0) {
    // This would have been handled by the try-catch above
    // But just in case, try the default location
    try {
      const defaultPath = candidates[0]; // ~/.claude/claude.mcp.json
      const merged = mergeGassolineConfig(
        generateDefaultConfig(),
        gassolineEntry,
        envVars
      );
      writeConfigFile(defaultPath, merged, dryRun);
      result.updated.push({
        name: getToolNameFromPath(defaultPath),
        path: defaultPath,
        isNew: true,
      });
    } catch (err) {
      result.errors.push({
        name: 'Claude Desktop',
        message: err.message,
        recovery: err.recovery,
      });
    }
  }

  result.success = result.updated.length > 0;
  return result;
}

module.exports = {
  generateDefaultConfig,
  executeInstall,
};
