"""Tests for uninstall module."""

import json
import os
import tempfile
import shutil
import unittest
from gasoline_mcp.uninstall import uninstall_from_client, execute_uninstall


class TestUninstallFromClient(unittest.TestCase):
    def test_removes_gasoline_from_config(self):
        tmp = tempfile.mkdtemp()
        try:
            cfg_path = os.path.join(tmp, "mcp.json")
            with open(cfg_path, "w") as f:
                json.dump({
                    "mcpServers": {
                        "gasoline": {"command": "gasoline-mcp", "args": []},
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
            self.assertNotIn("gasoline", written["mcpServers"])
            self.assertIn("other", written["mcpServers"])
        finally:
            shutil.rmtree(tmp)

    def test_deletes_file_when_only_server(self):
        tmp = tempfile.mkdtemp()
        try:
            cfg_path = os.path.join(tmp, "mcp.json")
            with open(cfg_path, "w") as f:
                json.dump({"mcpServers": {"gasoline": {"command": "gasoline-mcp"}}}, f)

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
                json.dump({"mcpServers": {"gasoline": {"command": "gasoline-mcp"}}}, f)

            d = {
                "id": "test", "name": "Test", "type": "file",
                "configPath": {"all": cfg_path},
                "detectDir": {"all": tmp},
            }
            result = uninstall_from_client(d, {"dryRun": True})
            self.assertEqual(result["status"], "removed")

            with open(cfg_path) as f:
                written = json.load(f)
            self.assertIn("gasoline", written["mcpServers"])
        finally:
            shutil.rmtree(tmp)

    def test_cli_dry_run(self):
        d = {
            "id": "claude-code", "name": "Claude Code", "type": "cli",
            "detectCommand": "claude",
            "removeArgs": ["mcp", "remove", "--scope", "user", "gasoline"],
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
                        "gasoline": {"command": "gasoline-mcp"},
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


if __name__ == "__main__":
    unittest.main()
