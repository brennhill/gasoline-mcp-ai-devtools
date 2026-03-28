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
from kaboom_agentic_browser.skills import install_bundled_skills


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
        self.assertEqual(cfg["mcpServers"][MCP_SERVER_NAME]["command"], "kaboom-agentic-browser")

    def test_honors_binary_path_override(self):
        cfg = generate_default_config(binary_path="/tmp/gasoline-bin")
        self.assertEqual(cfg["mcpServers"][MCP_SERVER_NAME]["command"], "/tmp/gasoline-bin")


class TestBuildMcpEntry(unittest.TestCase):
    def test_returns_json_string(self):
        entry = build_mcp_entry()
        parsed = json.loads(entry)
        self.assertEqual(parsed["command"], "kaboom-agentic-browser")

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

    def test_removes_gasoline_and_strum_entries_before_writing(self):
        tmp = tempfile.mkdtemp()
        try:
            cfg_path = os.path.join(tmp, "mcp.json")
            with open(cfg_path, "w", encoding="utf-8") as f:
                json.dump(
                    {
                        "mcpServers": {
                            "other": {"command": "other"},
                            "gasoline": {"command": "gasoline-mcp"},
                            "gasoline-agentic-browser": {"command": "gasoline-agentic-browser"},
                            "strum": {"command": "strum-agentic-browser"},
                            "strum-browser-devtools": {"command": "strum-agentic-browser"},
                        }
                    },
                    f,
                )

            d = {
                "id": "test", "name": "Test", "type": "file",
                "configPath": {"all": cfg_path},
                "detectDir": {"all": tmp},
            }
            result = install_to_client(d, {"dryRun": False, "envVars": {}, "binaryPath": "/tmp/kaboom-bin"})
            self.assertTrue(result["success"])

            with open(cfg_path, encoding="utf-8") as f:
                written = json.load(f)
            self.assertIn(MCP_SERVER_NAME, written["mcpServers"])
            self.assertNotIn("gasoline", written["mcpServers"])
            self.assertNotIn("gasoline-agentic-browser", written["mcpServers"])
            self.assertNotIn("strum", written["mcpServers"])
            self.assertNotIn("strum-browser-devtools", written["mcpServers"])
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


class TestSkillCleanup(unittest.TestCase):
    def test_install_bundled_skills_removes_gasoline_and_strum_legacy_files(self):
        tmp = tempfile.mkdtemp()
        original_env = {key: os.environ.get(key) for key in ["GASOLINE_CLAUDE_SKILLS_DIR", "GASOLINE_SKILL_TARGETS"]}
        try:
            claude_root = os.path.join(tmp, "claude-skills")
            os.makedirs(claude_root, exist_ok=True)
            with open(os.path.join(claude_root, "gasoline-debug.md"), "w", encoding="utf-8") as f:
                f.write("<!-- gasoline-managed-skill id:debug version:1 -->\nold gasoline skill\n")
            with open(os.path.join(claude_root, "strum-debug.md"), "w", encoding="utf-8") as f:
                f.write("<!-- strum-managed-skill id:debug version:1 -->\nold strum skill\n")

            os.environ["GASOLINE_CLAUDE_SKILLS_DIR"] = claude_root
            os.environ["GASOLINE_SKILL_TARGETS"] = "claude"
            result = install_bundled_skills(verbose=False)

            self.assertFalse(result["skipped"])
            self.assertGreaterEqual(result["summary"]["legacy_removed"], 2)
            self.assertFalse(os.path.exists(os.path.join(claude_root, "gasoline-debug.md")))
            self.assertFalse(os.path.exists(os.path.join(claude_root, "strum-debug.md")))
            self.assertTrue(os.path.exists(os.path.join(claude_root, "debug.md")))
        finally:
            for key, value in original_env.items():
                if value is None:
                    os.environ.pop(key, None)
                else:
                    os.environ[key] = value
            shutil.rmtree(tmp)


if __name__ == "__main__":
    unittest.main()
