"""Configuration file utilities for Gasoline MCP CLI."""

import json
import os
from pathlib import Path


class GasolineError(Exception):
    """Base class for Gasoline errors."""

    def __init__(self, message, recovery=""):
        self.message = message
        self.recovery = recovery
        super().__init__(message)

    def format(self):
        """Format error message with recovery suggestion."""
        output = f"❌ {self.message}"
        if self.recovery:
            output += f"\n   {self.recovery}"
        return output


class InvalidJSONError(GasolineError):
    """Raised when JSON parsing fails."""

    def __init__(self, path, line_number=None, error_message=""):
        msg = f"Invalid JSON in {path}"
        if line_number:
            msg += f" at line {line_number}"
        if error_message:
            msg += f"\n   {error_message}"
        recovery = f"Fix options:\n   1. Manually edit: code {path}\n   2. Restore from backup and try --install again\n   3. Run: gasoline-mcp --doctor (for more info)"
        super().__init__(msg, recovery)
        self.name = "InvalidJSONError"


class FileSizeError(GasolineError):
    """Raised when file exceeds size limit."""

    def __init__(self, path, size):
        msg = f"File {path} is too large ({size} bytes, max 1MB)"
        recovery = "The config file is too large. Please reduce its size or delete it and reinstall."
        super().__init__(msg, recovery)
        self.name = "FileSizeError"


class ConfigValidationError(GasolineError):
    """Raised when config validation fails."""

    def __init__(self, errors):
        msg = f"Config validation failed: {', '.join(errors)}"
        recovery = "Ensure config has mcpServers object with valid structure"
        super().__init__(msg, recovery)
        self.name = "ConfigValidationError"


def get_config_candidates():
    """Get list of potential config file paths for different tools."""
    home = str(Path.home())
    return [
        os.path.join(home, ".claude", "claude.mcp.json"),
        os.path.join(home, ".vscode", "claude.mcp.json"),
        os.path.join(home, ".cursor", "mcp.json"),
        os.path.join(home, ".codeium", "mcp.json"),
    ]


def get_tool_name_from_path(path):
    """Map config file path to tool name."""
    if ".claude" in path:
        return "Claude Desktop"
    elif ".vscode" in path:
        return "VSCode"
    elif ".cursor" in path:
        return "Cursor"
    elif ".codeium" in path:
        return "Codeium"
    return "Unknown"


def read_config_file(path):
    """Read and parse config file.

    Returns:
        dict: {valid: bool, data: dict, error: str, stats: dict}
    """
    try:
        if not os.path.exists(path):
            return {
                "valid": False,
                "data": None,
                "error": f"File not found: {path}",
                "stats": None,
            }

        # Check file size (1MB limit)
        size = os.path.getsize(path)
        if size > 1048576:  # 1MB
            raise FileSizeError(path, size)

        with open(path, "r", encoding="utf-8") as f:
            content = f.read()

        data = json.loads(content)

        return {
            "valid": True,
            "data": data,
            "error": None,
            "stats": {"size": size, "path": path},
        }

    except json.JSONDecodeError as e:
        raise InvalidJSONError(path, None, str(e))
    except Exception as e:
        return {
            "valid": False,
            "data": None,
            "error": str(e),
            "stats": None,
        }


def write_config_file(path, data, dry_run=False):
    """Write config file atomically.

    Args:
        path: File path
        data: Config data to write
        dry_run: If True, don't actually write

    Returns:
        dict: {success: bool, path: str, before: any}
    """
    if dry_run:
        return {"success": True, "path": path, "before": True}

    try:
        # Create directory if needed
        os.makedirs(os.path.dirname(path), exist_ok=True)

        # Write to temp file first
        temp_path = f"{path}.tmp"
        with open(temp_path, "w", encoding="utf-8") as f:
            json.dump(data, f, indent=2)
            f.write("\n")  # Add trailing newline

        # Atomic rename
        if os.path.exists(path):
            os.remove(path)
        os.rename(temp_path, path)

        return {"success": True, "path": path}

    except Exception as e:
        raise GasolineError(f"Failed to write {path}: {e}")


def validate_mcp_config(data):
    """Validate MCP configuration structure.

    Returns:
        list: List of validation errors (empty if valid)
    """
    errors = []

    if not isinstance(data, dict):
        errors.append("Config must be an object")
        return errors

    if "mcpServers" not in data:
        errors.append("Missing required field: mcpServers")

    elif not isinstance(data.get("mcpServers"), dict):
        errors.append("mcpServers must be an object")

    return errors


def merge_gasoline_config(existing, gasoline_entry, env_vars):
    """Merge gasoline config into existing config.

    Args:
        existing: Existing config
        gasoline_entry: Gasoline server entry
        env_vars: Environment variables dict

    Returns:
        dict: Merged config
    """
    import copy

    merged = copy.deepcopy(existing)

    if "mcpServers" not in merged:
        merged["mcpServers"] = {}

    merged["mcpServers"]["gasoline"] = copy.deepcopy(gasoline_entry)

    if env_vars:
        merged["mcpServers"]["gasoline"]["env"] = copy.deepcopy(env_vars)

    return merged


def parse_env_var(env_str):
    """Parse KEY=VALUE environment variable string.

    Returns:
        dict: {key: str, value: str}

    Raises:
        GasolineError: If format is invalid
    """
    if "=" not in env_str:
        raise GasolineError(
            f'Invalid env format "{env_str}". Expected: KEY=VALUE',
            'Examples:\n   - --env DEBUG=1\n   - --env API_KEY=secret'
        )

    key, value = env_str.split("=", 1)
    key = key.strip()
    value = value.strip()

    if not key:
        raise GasolineError(
            f'Invalid env format "{env_str}". Missing key',
            "Format: KEY=VALUE (key cannot be empty)"
        )

    if not value:
        raise GasolineError(
            f'Invalid env format "{env_str}". Missing value',
            "Format: KEY=VALUE (value cannot be empty)"
        )

    return {"key": key, "value": value}
