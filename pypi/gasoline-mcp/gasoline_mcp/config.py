# pylint: disable=duplicate-code
"""Configuration file utilities for Gasoline MCP CLI."""

import json
import os
import shutil
import subprocess
import sys
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
        recovery = (
            f"Fix options:\n   1. Manually edit: code {path}"
            "\n   2. Restore from backup and try --install again"
            "\n   3. Run: gasoline-mcp --doctor (for more info)"
        )
        super().__init__(msg, recovery)
        self.name = "InvalidJSONError"


class FileSizeError(GasolineError):
    """Raised when file exceeds size limit."""

    def __init__(self, path, size):
        msg = f"File {path} is too large ({size} bytes, max 1MB)"
        recovery = (
            "The config file is too large."
            " Please reduce its size or delete it and reinstall."
        )
        super().__init__(msg, recovery)
        self.name = "FileSizeError"


class ConfigValidationError(GasolineError):
    """Raised when config validation fails."""

    def __init__(self, errors):
        msg = f"Config validation failed: {', '.join(errors)}"
        recovery = "Ensure config has mcpServers object with valid structure"
        super().__init__(msg, recovery)
        self.name = "ConfigValidationError"


# Client definitions for all supported AI assistant clients
CLIENT_DEFINITIONS = [
    {
        "id": "claude-code",
        "name": "Claude Code",
        "type": "cli",
        "detectCommand": "claude",
        "installArgs": ["mcp", "add-json", "--scope", "user", "gasoline"],
        "removeArgs": ["mcp", "remove", "--scope", "user", "gasoline"],
    },
    {
        "id": "claude-desktop",
        "name": "Claude Desktop",
        "type": "file",
        "configPath": {
            "darwin": "~/Library/Application Support/Claude/claude_desktop_config.json",
            "win32": "%APPDATA%/Claude/claude_desktop_config.json",
        },
        "detectDir": {
            "darwin": "~/Library/Application Support/Claude",
            "win32": "%APPDATA%/Claude",
        },
    },
    {
        "id": "cursor",
        "name": "Cursor",
        "type": "file",
        "configPath": {"all": "~/.cursor/mcp.json"},
        "detectDir": {"all": "~/.cursor"},
    },
    {
        "id": "windsurf",
        "name": "Windsurf",
        "type": "file",
        "configPath": {"all": "~/.codeium/windsurf/mcp_config.json"},
        "detectDir": {"all": "~/.codeium/windsurf"},
    },
    {
        "id": "vscode",
        "name": "VS Code",
        "type": "file",
        "configPath": {
            "darwin": "~/Library/Application Support/Code/User/mcp.json",
            "win32": "%APPDATA%/Code/User/mcp.json",
            "linux": "~/.config/Code/User/mcp.json",
        },
        "detectDir": {
            "darwin": "~/Library/Application Support/Code",
            "win32": "%APPDATA%/Code",
            "linux": "~/.config/Code",
        },
    },
]

# Legacy paths that may contain orphaned configs from older versions
LEGACY_PATHS = [
    {"path": "~/.codeium/mcp.json", "description": "Old Windsurf/Codeium path"},
    {"path": "~/.vscode/claude.mcp.json", "description": "Old VS Code path"},
    {"path": "~/.claude.json", "description": "Old Claude Code path (now uses CLI)"},
]


def _get_platform():
    """Map sys.platform to our platform key."""
    if sys.platform == "darwin":
        return "darwin"
    if sys.platform.startswith("linux"):
        return "linux"
    if sys.platform == "win32":
        return "win32"
    return sys.platform


def expand_path(p):
    """Expand ~ and %APPDATA% in a path string."""
    if not p:
        return p
    expanded = os.path.expanduser(p)
    if sys.platform == "win32" and "%APPDATA%" in expanded:
        appdata = os.environ.get("APPDATA", "")
        expanded = expanded.replace("%APPDATA%", appdata)
    return os.path.normpath(expanded)


def get_client_config_path(definition, platform=None):
    """Get resolved config path for a file-type client definition."""
    if definition["type"] == "cli":
        return None
    plat = platform or _get_platform()
    config_path = definition["configPath"]
    raw = config_path.get(plat) or config_path.get("all")
    return expand_path(raw) if raw else None


def get_client_detect_dir(definition, platform=None):
    """Get resolved detect directory for a file-type client definition."""
    if definition["type"] == "cli":
        return None
    plat = platform or _get_platform()
    detect_dir = definition["detectDir"]
    raw = detect_dir.get(plat) or detect_dir.get("all")
    return expand_path(raw) if raw else None


def command_exists_on_path(cmd):
    """Check if a command exists on PATH."""
    return shutil.which(cmd) is not None


def is_client_installed(definition):
    """Check if a client is installed/detected on this system."""
    if definition["type"] == "cli":
        return command_exists_on_path(definition["detectCommand"])
    detect_dir = get_client_detect_dir(definition)
    if not detect_dir:
        return False
    return os.path.isdir(detect_dir)


def get_detected_clients():
    """Get all detected (installed) clients."""
    return [d for d in CLIENT_DEFINITIONS if is_client_installed(d)]


def get_client_by_id(client_id):
    """Find a client definition by ID."""
    for d in CLIENT_DEFINITIONS:
        if d["id"] == client_id:
            return d
    return None


def get_config_candidates():
    """Backward-compat: returns config file paths for file-type clients."""
    paths = []
    for d in CLIENT_DEFINITIONS:
        if d["type"] != "file":
            continue
        p = get_client_config_path(d)
        if p:
            paths.append(p)
    return paths


def get_tool_name_from_path(path):
    """Backward-compat: map config file path to tool name."""
    normalized = os.path.normpath(path)
    for d in CLIENT_DEFINITIONS:
        if d["type"] != "file":
            continue
        cfg_path = get_client_config_path(d)
        if cfg_path and normalized == os.path.normpath(cfg_path):
            return d["name"]
    # Fallback substring matching for legacy paths
    if ".cursor" in normalized:
        return "Cursor"
    if os.path.join(".codeium", "windsurf") in normalized:
        return "Windsurf"
    if ".codeium" in normalized:
        return "Windsurf"
    if "Claude" in normalized:
        return "Claude Desktop"
    if "Code" in normalized:
        return "VS Code"
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
        raise InvalidJSONError(path, None, str(e)) from e
    except OSError as e:
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

    except OSError as e:
        raise GasolineError(f"Failed to write {path}: {e}") from e


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
    import copy  # pylint: disable=import-outside-toplevel

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
