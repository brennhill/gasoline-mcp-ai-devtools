"""Tests for config module client registry."""

import os
import tempfile
import shutil
import unittest
from gasoline_mcp.config import (
    CLIENT_DEFINITIONS,
    get_client_config_path,
    get_client_detect_dir,
    is_client_installed,
    get_detected_clients,
    command_exists_on_path,
    get_config_candidates,
    get_tool_name_from_path,
    get_client_by_id,
)


class TestClientDefinitions(unittest.TestCase):
    """Test CLIENT_DEFINITIONS registry."""

    def test_contains_all_5_clients(self):
        ids = [c["id"] for c in CLIENT_DEFINITIONS]
        self.assertEqual(ids, ["claude-code", "claude-desktop", "cursor", "windsurf", "vscode"])

    def test_each_has_required_fields(self):
        for d in CLIENT_DEFINITIONS:
            self.assertIn("id", d)
            self.assertIn("name", d)
            self.assertIn("type", d)
            self.assertIn(d["type"], ["cli", "file"])
            if d["type"] == "cli":
                self.assertIn("detectCommand", d)
                self.assertIsInstance(d["installArgs"], list)
                self.assertIsInstance(d["removeArgs"], list)
            else:
                self.assertIn("configPath", d)
                self.assertIn("detectDir", d)

    def test_claude_code_is_cli_type(self):
        cc = get_client_by_id("claude-code")
        self.assertEqual(cc["type"], "cli")
        self.assertEqual(cc["detectCommand"], "claude")

    def test_cursor_uses_correct_path(self):
        cursor = get_client_by_id("cursor")
        self.assertIn(".cursor/mcp.json", cursor["configPath"]["all"])

    def test_windsurf_uses_correct_path(self):
        ws = get_client_by_id("windsurf")
        self.assertIn(".codeium/windsurf/mcp_config.json", ws["configPath"]["all"])


class TestGetClientById(unittest.TestCase):
    def test_returns_definition(self):
        cursor = get_client_by_id("cursor")
        self.assertEqual(cursor["name"], "Cursor")

    def test_returns_none_for_unknown(self):
        self.assertIsNone(get_client_by_id("nonexistent"))


class TestGetClientConfigPath(unittest.TestCase):
    def test_darwin_claude_desktop(self):
        d = get_client_by_id("claude-desktop")
        result = get_client_config_path(d, "darwin")
        self.assertIn("Library/Application Support/Claude/claude_desktop_config.json", result)

    def test_linux_vscode(self):
        d = get_client_by_id("vscode")
        result = get_client_config_path(d, "linux")
        self.assertIn(".config/Code/User/mcp.json", result)

    def test_cursor_all_platform(self):
        d = get_client_by_id("cursor")
        result = get_client_config_path(d)
        self.assertIn(".cursor/mcp.json", result)

    def test_returns_none_for_cli(self):
        d = get_client_by_id("claude-code")
        self.assertIsNone(get_client_config_path(d))

    def test_returns_none_for_unsupported_platform(self):
        d = get_client_by_id("claude-desktop")
        self.assertIsNone(get_client_config_path(d, "linux"))


class TestIsClientInstalled(unittest.TestCase):
    def test_detects_existing_dir(self):
        tmp = tempfile.mkdtemp()
        try:
            d = {
                "id": "test", "type": "file",
                "detectDir": {"all": tmp},
                "configPath": {"all": os.path.join(tmp, "mcp.json")},
            }
            self.assertTrue(is_client_installed(d))
        finally:
            shutil.rmtree(tmp)

    def test_returns_false_for_missing_dir(self):
        d = {
            "id": "test", "type": "file",
            "detectDir": {"all": "/tmp/nonexistent-gasoline-12345"},
            "configPath": {"all": "/tmp/nonexistent-gasoline-12345/mcp.json"},
        }
        self.assertFalse(is_client_installed(d))

    def test_cli_with_existing_command(self):
        d = {"id": "test", "type": "cli", "detectCommand": "python3"}
        self.assertTrue(is_client_installed(d))

    def test_cli_with_missing_command(self):
        d = {"id": "test", "type": "cli", "detectCommand": "nonexistent-cmd-12345"}
        self.assertFalse(is_client_installed(d))


class TestCommandExistsOnPath(unittest.TestCase):
    def test_finds_python(self):
        self.assertTrue(command_exists_on_path("python3"))

    def test_missing_command(self):
        self.assertFalse(command_exists_on_path("nonexistent-cmd-12345"))


class TestGetDetectedClients(unittest.TestCase):
    def test_returns_list(self):
        result = get_detected_clients()
        self.assertIsInstance(result, list)


class TestGetConfigCandidates(unittest.TestCase):
    def test_returns_file_paths(self):
        result = get_config_candidates()
        self.assertIsInstance(result, list)
        for p in result:
            self.assertIsInstance(p, str)


class TestGetToolNameFromPath(unittest.TestCase):
    def test_cursor_path(self):
        home = os.path.expanduser("~")
        result = get_tool_name_from_path(os.path.join(home, ".cursor", "mcp.json"))
        self.assertEqual(result, "Cursor")

    def test_windsurf_path(self):
        home = os.path.expanduser("~")
        result = get_tool_name_from_path(os.path.join(home, ".codeium", "windsurf", "mcp_config.json"))
        self.assertEqual(result, "Windsurf")

    def test_unknown_path(self):
        self.assertEqual(get_tool_name_from_path("/some/random/path"), "Unknown")


if __name__ == "__main__":
    unittest.main()
