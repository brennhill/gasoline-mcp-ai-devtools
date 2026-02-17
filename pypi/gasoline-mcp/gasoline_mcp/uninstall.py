# pylint: disable=duplicate-code
"""Uninstall logic for Gasoline MCP CLI."""

import json
import os
import subprocess
from . import config


def _uninstall_via_cli(definition, options):
    """Uninstall from a CLI-type client."""
    dry_run = options.get("dryRun", False)
    verbose = options.get("verbose", False)
    cmd = definition["detectCommand"]
    args = list(definition["removeArgs"])

    if dry_run:
        if verbose:
            print(f"[DEBUG] Would run: {cmd} {' '.join(args)}")
        return {
            "status": "removed",
            "name": definition["name"],
            "id": definition["id"],
            "method": "cli",
            "message": f"Would run: {cmd} {' '.join(args)}",
        }

    try:
        env = dict(os.environ)
        env.pop("CLAUDECODE", None)

        subprocess.run(
            [cmd] + args,
            env=env,
            capture_output=True,
            text=True,
            timeout=15,
            check=True,
        )

        return {
            "status": "removed",
            "name": definition["name"],
            "id": definition["id"],
            "method": "cli",
            "message": f"Removed via {cmd} CLI",
        }
    except subprocess.CalledProcessError as err:
        stderr = err.stderr or ""
        if "not found" in stderr or "does not exist" in stderr:
            return {
                "status": "notConfigured",
                "name": definition["name"],
                "id": definition["id"],
                "method": "cli",
            }
        return {
            "status": "error",
            "name": definition["name"],
            "id": definition["id"],
            "method": "cli",
            "message": f"CLI uninstall failed: {err}",
        }
    except (subprocess.SubprocessError, OSError) as err:
        return {
            "status": "error",
            "name": definition["name"],
            "id": definition["id"],
            "method": "cli",
            "message": f"CLI uninstall failed: {err}",
        }


def _uninstall_via_file(definition, options):
    """Uninstall from a file-type client."""
    dry_run = options.get("dryRun", False)
    verbose = options.get("verbose", False)
    cfg_path = config.get_client_config_path(definition)

    if not cfg_path:
        return {"status": "notConfigured", "name": definition["name"], "id": definition["id"]}

    if not os.path.exists(cfg_path):
        return {"status": "notConfigured", "name": definition["name"], "id": definition["id"]}

    read_result = config.read_config_file(cfg_path)
    if not read_result["valid"]:
        return {
            "status": "error",
            "name": definition["name"],
            "id": definition["id"],
            "message": f"{definition['name']}: Invalid JSON, cannot uninstall",
        }

    if not read_result["data"].get("mcpServers", {}).get("gasoline"):
        return {"status": "notConfigured", "name": definition["name"], "id": definition["id"]}

    if dry_run:
        if verbose:
            print(f"[DEBUG] Would remove gasoline from {cfg_path}")
        return {
            "status": "removed",
            "name": definition["name"],
            "id": definition["id"],
            "method": "file",
            "path": cfg_path,
        }

    modified = json.loads(json.dumps(read_result["data"]))
    del modified["mcpServers"]["gasoline"]

    if modified["mcpServers"]:
        config.write_config_file(cfg_path, modified, False)
    else:
        os.remove(cfg_path)

    if verbose:
        print(f"[DEBUG] Removed gasoline from {cfg_path}")

    return {
        "status": "removed",
        "name": definition["name"],
        "id": definition["id"],
        "method": "file",
        "path": cfg_path,
    }


def uninstall_from_client(definition, options):
    """Uninstall from a single client (dispatches by type)."""
    if definition["type"] == "cli":
        return _uninstall_via_cli(definition, options)
    return _uninstall_via_file(definition, options)


def execute_uninstall(options=None):
    """Execute uninstall across all detected clients.

    Args:
        options: dict with {dryRun, verbose, _clientOverrides}

    Returns:
        dict: {success, removed, notConfigured, errors}
    """
    options = options or {}
    dry_run = options.get("dryRun", False)
    verbose = options.get("verbose", False)

    clients = (
        options["_clientOverrides"]
        if "_clientOverrides" in options
        else config.get_detected_clients()
    )

    result = {"success": False, "removed": [], "notConfigured": [], "errors": []}

    for definition in clients:
        try:
            r = uninstall_from_client(definition, {"dryRun": dry_run, "verbose": verbose})

            if r["status"] == "removed":
                result["removed"].append(r)
            elif r["status"] == "notConfigured":
                result["notConfigured"].append(r["name"])
            elif r["status"] == "error":
                result["errors"].append(r.get("message", f"{r['name']}: unknown error"))
        except (OSError, KeyError, ValueError) as err:
            result["errors"].append(f"{definition['name']}: {err}")
            if verbose:
                print(f"[DEBUG] Error uninstalling from {definition['name']}: {err}")

    result["success"] = len(result["removed"]) > 0
    return result
