# Purpose: Validate test_install.py behavior and guard against regressions.
# Why: Prevents silent regressions in critical behavior paths.
# Docs: docs/features/feature/enhanced-cli-config/index.md

"""Tests for install module."""

import json
import os
import tempfile
import shutil
import unittest
import sys
from pathlib import Path

PACKAGE_ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(PACKAGE_ROOT))

from kaboom_agentic_browser.install import (
    generate_default_config,
    build_mcp_entry,
    install_to_client,
    execute_install,
)
from kaboom_agentic_browser.config import MCP_SERVER_NAME


class TestPackageIdentity(unittest.TestCase):
    def test_metadata_uses_kaboom_identity(self):
        pyproject = (PACKAGE_ROOT / "pyproject.toml").read_text(encoding="utf-8")
        readme = (PACKAGE_ROOT / "README.md").read_text(encoding="utf-8")

        self.assertIn('name = "kaboom-agentic-browser"', pyproject)
        self.assertIn("kaboom-agentic-browser-darwin-arm64", pyproject)
        self.assertIn('Homepage = "https://gokaboom.dev"', pyproject)
        self.assertIn("pip install kaboom-agentic-browser", readme)
        self.assertTrue((PACKAGE_ROOT / "kaboom_agentic_browser" / "__main__.py").exists())


class TestGenerateDefaultConfig(unittest.TestCase):
    def test_returns_valid_config(self):
        cfg = generate_default_config()
        self.assertIn("mcpServers", cfg)
        self.assertIn(MCP_SERVER_NAME, cfg["mcpServers"])
        self.assertEqual(cfg["mcpServers"][MCP_SERVER_NAME]["command"], "gasoline-agentic-browser")

    def test_honors_binary_path_override(self):
        cfg = generate_default_config(binary_path="/tmp/gasoline-bin")
        self.assertEqual(cfg["mcpServers"][MCP_SERVER_NAME]["command"], "/tmp/gasoline-bin")


class TestBuildMcpEntry(unittest.TestCase):
    def test_returns_json_string(self):
        entry = build_mcp_entry()
        parsed = json.loads(entry)
        self.assertEqual(parsed["command"], "gasoline-agentic-browser")

    def test_includes_env_vars(self):
        entry = build_mcp_entry({"DEBUG": "1"})
        parsed = json.loads(entry)
        self.assertEqual(parsed["env"]["DEBUG"], "1")

    def test_honors_binary_path_override(self):
        entry = build_mcp_entry(binary_path="/tmp/gasoline-bin")
        parsed = json.loads(entry)
        self.assertEqual(parsed["command"], "/tmp/gasoline-bin")


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
            result = install_to_client(d, {"dryRun": False, "envVars": {}, "binaryPath": "/tmp/gasoline-bin"})
            self.assertTrue(result["success"])
            self.assertTrue(result["isNew"])

            with open(cfg_path) as f:
                written = json.load(f)
            self.assertIn(MCP_SERVER_NAME, written["mcpServers"])
            self.assertEqual(written["mcpServers"][MCP_SERVER_NAME]["command"], "/tmp/gasoline-bin")
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
            self.assertIn(MCP_SERVER_NAME, written["mcpServers"])
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
            "installArgs": ["mcp", "add-json", "--scope", "user", MCP_SERVER_NAME],
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
