"""Tests for install module."""

import json
import os
import tempfile
import shutil
import unittest
from gasoline_mcp.install import (
    generate_default_config,
    build_mcp_entry,
    install_to_client,
    execute_install,
)


class TestGenerateDefaultConfig(unittest.TestCase):
    def test_returns_valid_config(self):
        cfg = generate_default_config()
        self.assertIn("mcpServers", cfg)
        self.assertIn("gasoline", cfg["mcpServers"])
        self.assertEqual(cfg["mcpServers"]["gasoline"]["command"], "gasoline-mcp")


class TestBuildMcpEntry(unittest.TestCase):
    def test_returns_json_string(self):
        entry = build_mcp_entry()
        parsed = json.loads(entry)
        self.assertEqual(parsed["command"], "gasoline-mcp")

    def test_includes_env_vars(self):
        entry = build_mcp_entry({"DEBUG": "1"})
        parsed = json.loads(entry)
        self.assertEqual(parsed["env"]["DEBUG"], "1")


class TestInstallToClient(unittest.TestCase):
    def test_creates_new_file_config(self):
        tmp = tempfile.mkdtemp()
        try:
            cfg_path = os.path.join(tmp, "mcp.json")
            d = {
                "id": "test", "name": "Test", "type": "file",
                "configPath": {"all": cfg_path},
                "detectDir": {"all": tmp},
            }
            result = install_to_client(d, {"dryRun": False, "envVars": {}})
            self.assertTrue(result["success"])
            self.assertTrue(result["isNew"])

            with open(cfg_path) as f:
                written = json.load(f)
            self.assertIn("gasoline", written["mcpServers"])
        finally:
            shutil.rmtree(tmp)

    def test_merges_existing_config(self):
        tmp = tempfile.mkdtemp()
        try:
            cfg_path = os.path.join(tmp, "mcp.json")
            with open(cfg_path, "w") as f:
                json.dump({"mcpServers": {"other": {"command": "other"}}}, f)

            d = {
                "id": "test", "name": "Test", "type": "file",
                "configPath": {"all": cfg_path},
                "detectDir": {"all": tmp},
            }
            result = install_to_client(d, {"dryRun": False, "envVars": {}})
            self.assertTrue(result["success"])
            self.assertFalse(result["isNew"])

            with open(cfg_path) as f:
                written = json.load(f)
            self.assertIn("gasoline", written["mcpServers"])
            self.assertIn("other", written["mcpServers"])
        finally:
            shutil.rmtree(tmp)

    def test_dry_run_no_write(self):
        tmp = tempfile.mkdtemp()
        try:
            cfg_path = os.path.join(tmp, "mcp.json")
            d = {
                "id": "test", "name": "Test", "type": "file",
                "configPath": {"all": cfg_path},
                "detectDir": {"all": tmp},
            }
            result = install_to_client(d, {"dryRun": True, "envVars": {}})
            self.assertTrue(result["success"])
            self.assertFalse(os.path.exists(cfg_path))
        finally:
            shutil.rmtree(tmp)

    def test_cli_dry_run(self):
        d = {
            "id": "claude-code", "name": "Claude Code", "type": "cli",
            "detectCommand": "claude",
            "installArgs": ["mcp", "add-json", "--scope", "user", "gasoline"],
        }
        result = install_to_client(d, {"dryRun": True, "envVars": {}})
        self.assertTrue(result["success"])
        self.assertEqual(result["method"], "cli")


class TestExecuteInstall(unittest.TestCase):
    def test_installs_to_file_clients(self):
        tmp = tempfile.mkdtemp()
        try:
            d = {
                "id": "test", "name": "Test", "type": "file",
                "configPath": {"all": os.path.join(tmp, "mcp.json")},
                "detectDir": {"all": tmp},
            }
            result = execute_install({"dryRun": False, "envVars": {}, "_clientOverrides": [d]})
            self.assertTrue(result["success"])
            self.assertEqual(len(result["installed"]), 1)
        finally:
            shutil.rmtree(tmp)

    def test_no_clients_detected(self):
        result = execute_install({"dryRun": False, "envVars": {}, "_clientOverrides": []})
        self.assertFalse(result["success"])


if __name__ == "__main__":
    unittest.main()
