/**
 * Error classes and message catalog for Gasoline MCP CLI
 */

/**
 * Base error class for all Gasoline MCP CLI errors
 */
class GasolineError extends Error {
  constructor(message, recovery) {
    super(message);
    this.name = 'GasolineError';
    this.recovery = recovery;
  }

  format() {
    let output = `❌ Error: ${this.message}`;
    if (this.recovery) {
      output += `\n   ${this.recovery}`;
    }
    return output;
  }
}

class PermissionError extends GasolineError {
  constructor(path) {
    super(
      `Permission denied writing ${path}`,
      `Try: sudo gasoline-mcp --install\nOr: Check permissions with: ls -la ${path.split('/').slice(0, -1).join('/')}`
    );
    this.name = 'PermissionError';
  }
}

class InvalidJSONError extends GasolineError {
  constructor(path, lineNumber, errorMessage) {
    const msg = `Invalid JSON in ${path}${lineNumber ? ` at line ${lineNumber}` : ''}\n   ${errorMessage}`;
    super(
      msg,
      `Fix options:\n   1. Manually edit: code ${path}\n   2. Restore from backup and try --install again\n   3. Run: gasoline-mcp --doctor (for more info)`
    );
    this.name = 'InvalidJSONError';
  }
}

class BinaryNotFoundError extends GasolineError {
  constructor(expectedPath) {
    super(
      `Gasoline binary not found at ${expectedPath}`,
      `Reinstall: npm install -g gasoline-mcp@latest\nOr build from source: go build ./cmd/dev-console`
    );
    this.name = 'BinaryNotFoundError';
  }
}

class InvalidEnvFormatError extends GasolineError {
  constructor(envStr) {
    super(
      `Invalid env format "${envStr}". Expected: KEY=VALUE`,
      `Examples of valid formats:\n   - --env DEBUG=1\n   - --env GASOLINE_SERVER=http://localhost:7890\n   - --env LOG_LEVEL=info`
    );
    this.name = 'InvalidEnvFormatError';
  }
}

class EnvWithoutInstallError extends GasolineError {
  constructor() {
    super(
      '--env only works with --install',
      'Usage: gasoline-mcp --install --env KEY=VALUE'
    );
    this.name = 'EnvWithoutInstallError';
  }
}

class ConfigValidationError extends GasolineError {
  constructor(errors) {
    super(
      'Config validation failed',
      `Issues:\n${errors.map(e => `   - ${e}`).join('\n')}`
    );
    this.name = 'ConfigValidationError';
  }
}

class FileSizeError extends GasolineError {
  constructor(path, size) {
    super(
      `Config file too large: ${(size / 1024 / 1024).toFixed(2)}MB`,
      `Max size: 1MB. File: ${path}`
    );
    this.name = 'FileSizeError';
  }
}

module.exports = {
  GasolineError,
  PermissionError,
  InvalidJSONError,
  BinaryNotFoundError,
  InvalidEnvFormatError,
  EnvWithoutInstallError,
  ConfigValidationError,
  FileSizeError,
};
