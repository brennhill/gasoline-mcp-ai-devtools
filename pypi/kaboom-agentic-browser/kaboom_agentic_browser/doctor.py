# pylint: disable=duplicate-code
"""Doctor diagnostics for the PyPI wrapper.

Purpose: Evaluate MCP config health and binary readiness for supported clients.
Why: Provides actionable repair guidance when install/config drifts occur.
Docs: docs/features/feature/enhanced-cli-config/index.md
"""

import os
import subprocess
from . import config


def _known_server_names():
    return [config.MCP_SERVER_NAME] + [
        name for name in config.LEGACY_MCP_SERVER_NAMES if name != config.MCP_SERVER_NAME
    ]


def test_binary(get_binary_path=None):
    """Test if the Kaboom binary is available and working.

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
        tool["suggestions"].append("Run: kaboom-agentic-browser --install")
        return tool

    read_result = config.read_config_file(cfg_path)
    if not read_result["valid"]:
        tool["issues"].append("Invalid JSON")
        tool["suggestions"].append("Fix the JSON syntax or run: kaboom-agentic-browser --install")
        return tool

    mcp_servers = read_result["data"].get("mcpServers", {})
    matched_name = next((name for name in _known_server_names() if mcp_servers.get(name)), None)
    if not matched_name:
        tool["issues"].append(f"{config.MCP_SERVER_NAME} entry missing from mcpServers")
        tool["suggestions"].append("Run: kaboom-agentic-browser --install")
        return tool
    if matched_name != config.MCP_SERVER_NAME:
        tool["issues"].append(
            f"Legacy MCP server name detected ({matched_name}); migrate to {config.MCP_SERVER_NAME}"
        )
        tool["suggestions"].append("Run: kaboom-agentic-browser --install")

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

    # Try to check if Kaboom is configured via CLI
    env = dict(os.environ)
    env.pop("CLAUDECODE", None)

    found = False
    for server_name in _known_server_names():
        try:
            subprocess.run(
                [definition["detectCommand"], "mcp", "get", server_name],
                capture_output=True,
                text=True,
                timeout=10,
                check=True,
                env=env,
            )
            found = True
            break
        except (subprocess.SubprocessError, OSError):
            pass

    if not found:
        tool["status"] = "error"
        tool["issues"].append(f"{config.MCP_SERVER_NAME} not configured")
        tool["suggestions"].append("Run: kaboom-agentic-browser --install")
    else:
        tool["status"] = "ok"

    return tool


def _check_legacy_paths():
    """Check for legacy/orphaned config files at old paths."""
    warnings = []
    for legacy in config.LEGACY_PATHS:
        expanded = config.expand_path(legacy["path"])
        if os.path.exists(expanded):
            try:
                read_result = config.read_config_file(expanded)
                if read_result["valid"] and any(
                        read_result["data"].get("mcpServers", {}).get(name)
                        for name in _known_server_names()):
                    warnings.append({
                        "path": expanded,
                        "description": legacy["description"],
                        "message": f"Orphaned {config.MCP_SERVER_NAME} config at old path: {expanded}",
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
