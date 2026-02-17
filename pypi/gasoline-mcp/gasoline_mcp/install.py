"""Install logic for Gasoline MCP CLI."""

import json
import subprocess
from . import config


def generate_default_config():
    """Generate default MCP config for gasoline."""
    return {
        "mcpServers": {
            "gasoline": {
                "command": "gasoline-mcp",
                "args": [],
            },
        },
    }


def build_mcp_entry(env_vars=None):
    """Build the MCP entry JSON string for CLI-based install."""
    entry = {"command": "gasoline-mcp", "args": []}
    if env_vars:
        entry["env"] = dict(env_vars)
    return json.dumps(entry)


def _install_via_cli(definition, options):
    """Install to a CLI-type client (e.g. Claude Code)."""
    dry_run = options.get("dryRun", False)
    env_vars = options.get("envVars", {})
    entry_json = build_mcp_entry(env_vars)
    cmd = definition["detectCommand"]
    args = list(definition["installArgs"])

    if dry_run:
        return {
            "success": True,
            "name": definition["name"],
            "id": definition["id"],
            "method": "cli",
            "message": f"Would run: {cmd} {' '.join(args)} '<json>'",
        }

    try:
        import os  # pylint: disable=import-outside-toplevel
        env = dict(os.environ)
        env.pop("CLAUDECODE", None)

        subprocess.run(
            [cmd] + args,
            input=entry_json,
            env=env,
            capture_output=True,
            text=True,
            timeout=15,
            check=True,
        )

        return {
            "success": True,
            "name": definition["name"],
            "id": definition["id"],
            "method": "cli",
            "message": f"Installed via {cmd} CLI",
        }
    except (subprocess.SubprocessError, OSError) as err:
        return {
            "success": False,
            "name": definition["name"],
            "id": definition["id"],
            "method": "cli",
            "message": f"CLI install failed: {err}",
            "error": str(err),
        }


def _install_via_file(definition, options):
    """Install to a file-type client (config file write)."""
    dry_run = options.get("dryRun", False)
    env_vars = options.get("envVars", {})
    cfg_path = config.get_client_config_path(definition)

    if not cfg_path:
        return {
            "success": False,
            "name": definition["name"],
            "id": definition["id"],
            "method": "file",
            "message": "No config path for this platform",
        }

    gasoline_entry = {"command": "gasoline-mcp", "args": []}

    read_result = config.read_config_file(cfg_path)
    if read_result["valid"]:
        config_data = read_result["data"]
        is_new = False
    else:
        config_data = generate_default_config()
        is_new = True

    merged = config.merge_gasoline_config(config_data, gasoline_entry, env_vars)
    config.write_config_file(cfg_path, merged, dry_run)

    return {
        "success": True,
        "name": definition["name"],
        "id": definition["id"],
        "method": "file",
        "path": cfg_path,
        "isNew": is_new,
        "message": f"Would write to {cfg_path}" if dry_run else f"Wrote to {cfg_path}",
    }


def install_to_client(definition, options):
    """Install to a single client (dispatches by type)."""
    if definition["type"] == "cli":
        return _install_via_cli(definition, options)
    return _install_via_file(definition, options)


def execute_install(options=None):
    """Execute install operation across all detected clients.

    Args:
        options: dict with {dryRun, envVars, verbose, _clientOverrides}

    Returns:
        dict: {success, installed, errors, total}
    """
    options = options or {}
    dry_run = options.get("dryRun", False)
    env_vars = options.get("envVars", {})
    verbose = options.get("verbose", False)

    clients = (
        options["_clientOverrides"]
        if "_clientOverrides" in options
        else config.get_detected_clients()
    )

    result = {
        "success": False,
        "installed": [],
        "errors": [],
        "total": len(config.CLIENT_DEFINITIONS),
    }

    for definition in clients:
        try:
            install_result = install_to_client(definition, {"dryRun": dry_run, "envVars": env_vars})

            if install_result["success"]:
                result["installed"].append(install_result)
            else:
                result["errors"].append(install_result)

            if verbose:
                status = "OK" if install_result["success"] else "FAIL"
                print(f"[DEBUG] {definition['name']}: {status} - {install_result['message']}")
        except (OSError, ValueError, KeyError) as err:
            result["errors"].append({
                "name": definition["name"],
                "id": definition["id"],
                "message": str(err),
                "recovery": getattr(err, "recovery", ""),
            })
            if verbose:
                print(f"[DEBUG] Error on {definition['name']}: {err}")

    result["success"] = len(result["installed"]) > 0
    return result
