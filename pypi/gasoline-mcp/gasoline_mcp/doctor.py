# pylint: disable=duplicate-code
"""Doctor diagnostics for Gasoline MCP CLI."""

import os
import subprocess
from . import config


def test_binary(get_binary_path=None):
    """Test if gasoline binary is available and working.

    Args:
        get_binary_path: Callable that returns the binary path.
                         Injected by caller to avoid circular imports.

    Returns:
        dict: {ok: bool, path?: str, version?: str, error?: str}
    """
    if get_binary_path is None:
        return {
            "ok": False,
            "error": "No binary path resolver provided",
        }

    try:
        binary_path = get_binary_path()

        # Test binary with --version
        try:
            result = subprocess.run(
                [binary_path, "--version"],
                capture_output=True,
                text=True,
                timeout=5,
                check=False,
            )
            version = result.stdout.strip() or "unknown"

            return {
                "ok": True,
                "path": binary_path,
                "version": version,
            }
        except (OSError, subprocess.SubprocessError) as e:
            return {
                "ok": False,
                "path": binary_path,
                "error": f"Binary found but failed to execute: {e}",
            }

    except RuntimeError as e:
        return {
            "ok": False,
            "error": str(e),
        }


def _diagnose_file_client(definition, verbose):
    """Diagnose a single file-type client."""
    cfg_path = config.get_client_config_path(definition)
    detected = config.is_client_installed(definition)

    tool = {
        "name": definition["name"],
        "id": definition["id"],
        "type": "file",
        "path": cfg_path,
        "detected": detected,
        "status": "error",
        "issues": [],
        "suggestions": [],
    }

    if verbose:
        print(f"[DEBUG] Checking {definition['name']} at {cfg_path}")

    if not detected:
        tool["status"] = "info"
        tool["issues"].append("Not installed on this system")
        return tool

    if not cfg_path:
        tool["status"] = "info"
        tool["issues"].append("No config path for this platform")
        return tool

    if not os.path.exists(cfg_path):
        tool["status"] = "error"
        tool["issues"].append("Config file not found")
        tool["suggestions"].append("Run: gasoline-mcp --install")
        return tool

    read_result = config.read_config_file(cfg_path)
    if not read_result["valid"]:
        tool["issues"].append("Invalid JSON")
        tool["suggestions"].append("Fix the JSON syntax or run: gasoline-mcp --install")
        return tool

    if not read_result["data"].get("mcpServers", {}).get("gasoline"):
        tool["issues"].append("gasoline entry missing from mcpServers")
        tool["suggestions"].append("Run: gasoline-mcp --install")
        return tool

    tool["status"] = "ok"
    return tool


def _diagnose_cli_client(definition, verbose):
    """Diagnose a CLI-type client."""
    detected = config.is_client_installed(definition)

    tool = {
        "name": definition["name"],
        "id": definition["id"],
        "type": "cli",
        "detected": detected,
        "status": "error",
        "issues": [],
        "suggestions": [],
    }

    if verbose:
        print(f"[DEBUG] Checking {definition['name']} (CLI: {definition['detectCommand']})")

    if not detected:
        tool["status"] = "info"
        tool["issues"].append(f"{definition['detectCommand']} CLI not found on PATH")
        return tool

    # Try to check if gasoline is configured via CLI
    try:
        env = dict(os.environ)
        env.pop("CLAUDECODE", None)

        subprocess.run(
            [definition["detectCommand"], "mcp", "get", "gasoline"],
            capture_output=True,
            text=True,
            timeout=10,
            check=True,
            env=env,
        )
        tool["status"] = "ok"
    except (subprocess.SubprocessError, OSError):
        tool["status"] = "error"
        tool["issues"].append("gasoline not configured")
        tool["suggestions"].append("Run: gasoline-mcp --install")

    return tool


def _check_legacy_paths():
    """Check for legacy/orphaned config files at old paths."""
    warnings = []
    for legacy in config.LEGACY_PATHS:
        expanded = config.expand_path(legacy["path"])
        if os.path.exists(expanded):
            try:
                read_result = config.read_config_file(expanded)
                if (read_result["valid"]
                        and read_result["data"].get("mcpServers", {}).get("gasoline")):
                    warnings.append({
                        "path": expanded,
                        "description": legacy["description"],
                        "message": f"Orphaned gasoline config at old path: {expanded}",
                    })
            except Exception:  # pylint: disable=broad-except
                pass
    return warnings


def _build_summary(tools):
    """Build a human-readable summary string from tool diagnostics."""
    ok_count = sum(1 for t in tools if t["status"] == "ok")
    error_count = sum(1 for t in tools if t["status"] == "error")
    info_count = sum(1 for t in tools if t["status"] == "info")

    summary = f"Summary: {ok_count} client{'s' if ok_count != 1 else ''} ready"
    if error_count > 0:
        summary += f", {error_count} need{'s' if error_count == 1 else ''} repair"
    if info_count > 0:
        summary += f", {info_count} not detected"
    return summary


def run_diagnostics(verbose=False, get_binary_path=None):
    """Run full diagnostics on all client locations.

    Args:
        verbose: If True, log debug info
        get_binary_path: Callable that returns the binary path.
                         Injected by caller to avoid circular imports.

    Returns:
        dict: Diagnostic report with tools array and summary
    """
    tools = []
    for definition in config.CLIENT_DEFINITIONS:
        if definition["type"] == "cli":
            tools.append(_diagnose_cli_client(definition, verbose))
        else:
            tools.append(_diagnose_file_client(definition, verbose))

    legacy_warnings = _check_legacy_paths()

    return {
        "tools": tools,
        "binary": test_binary(get_binary_path),
        "legacyWarnings": legacy_warnings,
        "summary": _build_summary(tools),
    }
