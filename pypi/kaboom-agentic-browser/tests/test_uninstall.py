# Purpose: Validate test_uninstall.py behavior and guard against regressions.
# Why: Prevents silent regressions in critical behavior paths.
# Docs: docs/features/feature/enhanced-cli-config/index.md

"""Tests for uninstall module."""

import json
import os
import tempfile
import shutil
import unittest
import sys
from pathlib import Path

PACKAGE_ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(PACKAGE_ROOT))

from kaboom_agentic_browser.uninstall import uninstall_from_client, execute_uninstall
from kaboom_agentic_browser.config import MCP_SERVER_NAME


class TestPackageCommandText(unittest.TestCase):
    def test_entrypoint_and_doctor_use_kaboom_commands(self):
        main_source = (PACKAGE_ROOT / "kaboom_agentic_browser" / "__main__.py").read_text(
            encoding="utf-8"
        )
        doctor_source = (PACKAGE_ROOT / "kaboom_agentic_browser" / "doctor.py").read_text(
            encoding="utf-8"
        )

        self.assertIn("kaboom-agentic-browser", main_source)
        self.assertIn("Kaboom Agentic Browser", main_source)
        self.assertIn("Run: kaboom-agentic-browser --install", doctor_source)


class TestUninstallFromClient(unittest.TestCase):
    def test_removes_kaboom_gasoline_and_strum_from_config(self):
        tmp = tempfile.mkdtemp()
        try:
            cfg_path = os.path.join(tmp, "mcp.json")
            with open(cfg_path, "w") as f:
                json.dump({
                    "mcpServers": {
                        MCP_SERVER_NAME: {"command": "gasoline-mcp", "args": []},
                        "gasoline": {"command": "gasoline-mcp", "args": []},
                        "strum": {"command": "strum-agentic-browser", "args": []},
                        "strum-browser-devtools": {"command": "strum-agentic-browser", "args": []},
                        "other": {"command": "other", "args": []},
                    }
                }, f)

            d = {
                "id": "test", "name": "Test", "type": "file",
                "configPath": {"all": cfg_path},
                "detectDir": {"all": tmp},
            }
            result = uninstall_from_client(d, {"dryRun": False})
            self.assertEqual(result["status"], "removed")

            with open(cfg_path) as f:
                written = json.load(f)
            self.assertNotIn(MCP_SERVER_NAME, written["mcpServers"])
            self.assertNotIn("gasoline", written["mcpServers"])
            self.assertNotIn("strum", written["mcpServers"])
            self.assertNotIn("strum-browser-devtools", written["mcpServers"])
            self.assertIn("other", written["mcpServers"])
        finally:
            shutil.rmtree(tmp)

    def test_deletes_file_when_only_server(self):
        tmp = tempfile.mkdtemp()
        try:
            cfg_path = os.path.join(tmp, "mcp.json")
            with open(cfg_path, "w") as f:
                json.dump({"mcpServers": {MCP_SERVER_NAME: {"command": "gasoline-mcp"}}}, f)

            d = {
                "id": "test", "name": "Test", "type": "file",
                "configPath": {"all": cfg_path},
                "detectDir": {"all": tmp},
            }
            result = uninstall_from_client(d, {"dryRun": False})
            self.assertEqual(result["status"], "removed")
            self.assertFalse(os.path.exists(cfg_path))
        finally:
            shutil.rmtree(tmp)

    def test_not_configured_when_missing(self):
        d = {
            "id": "test", "name": "Test", "type": "file",
            "configPath": {"all": "/tmp/nonexistent-12345/mcp.json"},
            "detectDir": {"all": "/tmp/nonexistent-12345"},
        }
        result = uninstall_from_client(d, {"dryRun": False})
        self.assertEqual(result["status"], "notConfigured")

    def test_dry_run_no_modify(self):
        tmp = tempfile.mkdtemp()
        try:
            cfg_path = os.path.join(tmp, "mcp.json")
            with open(cfg_path, "w") as f:
                json.dump({"mcpServers": {MCP_SERVER_NAME: {"command": "gasoline-mcp"}}}, f)

            d = {
                "id": "test", "name": "Test", "type": "file",
                "configPath": {"all": cfg_path},
                "detectDir": {"all": tmp},
            }
            result = uninstall_from_client(d, {"dryRun": True})
            self.assertEqual(result["status"], "removed")

            with open(cfg_path) as f:
                written = json.load(f)
            self.assertIn(MCP_SERVER_NAME, written["mcpServers"])
        finally:
            shutil.rmtree(tmp)

    def test_cli_dry_run(self):
        d = {
            "id": "claude-code", "name": "Claude Code", "type": "cli",
            "detectCommand": "claude",
            "removeArgs": ["mcp", "remove", "--scope", "user", MCP_SERVER_NAME],
        }
        result = uninstall_from_client(d, {"dryRun": True})
        self.assertEqual(result["status"], "removed")
        self.assertEqual(result["method"], "cli")


class TestExecuteUninstall(unittest.TestCase):
    def test_removes_from_file_clients(self):
        tmp = tempfile.mkdtemp()
        try:
            cfg_path = os.path.join(tmp, "mcp.json")
            with open(cfg_path, "w") as f:
                json.dump({
                    "mcpServers": {
                        MCP_SERVER_NAME: {"command": "gasoline-mcp"},
                        "other": {"command": "other"},
                    }
                }, f)

            d = {
                "id": "test", "name": "Test", "type": "file",
                "configPath": {"all": cfg_path},
                "detectDir": {"all": tmp},
            }
            result = execute_uninstall({"dryRun": False, "_clientOverrides": [d]})
            self.assertTrue(result["success"])
            self.assertEqual(len(result["removed"]), 1)
        finally:
            shutil.rmtree(tmp)

    def test_removes_managed_skill_files_for_kaboom_gasoline_and_strum(self):
        tmp = tempfile.mkdtemp()
        original_env = {key: os.environ.get(key) for key in ["GASOLINE_CLAUDE_SKILLS_DIR", "GASOLINE_SKILL_TARGETS"]}
        try:
            claude_root = os.path.join(tmp, "claude-skills")
            os.makedirs(claude_root, exist_ok=True)
            with open(os.path.join(claude_root, "debug.md"), "w", encoding="utf-8") as f:
                f.write("<!-- kaboom-managed-skill id:debug version:2 -->\ncurrent kaboom skill\n")
            with open(os.path.join(claude_root, "gasoline-debug.md"), "w", encoding="utf-8") as f:
                f.write("<!-- gasoline-managed-skill id:debug version:1 -->\nold gasoline skill\n")
            with open(os.path.join(claude_root, "strum-debug.md"), "w", encoding="utf-8") as f:
                f.write("<!-- strum-managed-skill id:debug version:1 -->\nold strum skill\n")

            os.environ["GASOLINE_CLAUDE_SKILLS_DIR"] = claude_root
            os.environ["GASOLINE_SKILL_TARGETS"] = "claude"
            result = execute_uninstall({"dryRun": False, "_clientOverrides": []})

            self.assertTrue(result["success"])
            self.assertEqual(result["skillCleanup"]["removed"], 3)
            self.assertFalse(os.path.exists(os.path.join(claude_root, "debug.md")))
            self.assertFalse(os.path.exists(os.path.join(claude_root, "gasoline-debug.md")))
            self.assertFalse(os.path.exists(os.path.join(claude_root, "strum-debug.md")))
        finally:
            for key, value in original_env.items():
                if value is None:
                    os.environ.pop(key, None)
                else:
                    os.environ[key] = value
            shutil.rmtree(tmp)


if __name__ == "__main__":
    unittest.main()
