# Purpose: Validate test_platform_cleanup.py behavior and guard against regressions.
# Why: Prevents silent regressions in critical behavior paths.
# Docs: docs/features/feature/enhanced-cli-config/index.md

import os
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


ROOT = os.path.dirname(os.path.dirname(__file__))
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)

from kaboom_agentic_browser import platform  # pylint: disable=wrong-import-position

PYPI_ROOT = Path(ROOT).parent
PLATFORM_PACKAGES = [
    ("darwin-arm64", "kaboom_agentic_browser_darwin_arm64"),
    ("darwin-x64", "kaboom_agentic_browser_darwin_x64"),
    ("linux-arm64", "kaboom_agentic_browser_linux_arm64"),
    ("linux-x64", "kaboom_agentic_browser_linux_x64"),
    ("win32-x64", "kaboom_agentic_browser_win32_x64"),
]


class PlatformMetadataTests(unittest.TestCase):
    def test_platform_packages_use_kaboom_identity(self):
        for platform_key, module_name in PLATFORM_PACKAGES:
            package_root = PYPI_ROOT / f"kaboom-agentic-browser-{platform_key}"
            self.assertTrue(package_root.exists(), f"missing package root: {package_root}")

            pyproject = (package_root / "pyproject.toml").read_text(encoding="utf-8")
            readme = (package_root / "README.md").read_text(encoding="utf-8")
            manifest = (package_root / "MANIFEST.in").read_text(encoding="utf-8")
            init_py = (package_root / module_name / "__init__.py").read_text(encoding="utf-8")
            pkg_info = next(package_root.glob("*.egg-info/PKG-INFO")).read_text(encoding="utf-8")
            sources = next(package_root.glob("*.egg-info/SOURCES.txt")).read_text(encoding="utf-8")
            top_level = next(package_root.glob("*.egg-info/top_level.txt")).read_text(encoding="utf-8")

            self.assertIn(f'name = "kaboom-agentic-browser-{platform_key}"', pyproject)
            self.assertIn(f'packages = ["{module_name}"]', pyproject)
            self.assertIn("Kaboom Agentic Browser binary", pyproject)
            self.assertIn("https://gokaboom.dev", pyproject)
            self.assertIn("Kaboom", readme)
            self.assertIn(f"include {module_name}/kaboom", manifest)
            self.assertIn("Platform-specific Kaboom binary", init_py)
            self.assertIn(f"Name: kaboom-agentic-browser-{platform_key}", pkg_info)
            self.assertIn("Project-URL: Homepage, https://gokaboom.dev", pkg_info)
            self.assertIn(module_name, sources)
            self.assertEqual(module_name, top_level.strip())

            for text in (pyproject, readme, manifest, init_py, pkg_info, sources, top_level):
                self.assertNotIn("gasoline", text.lower())
                self.assertNotIn("cookwithgasoline", text.lower())
                self.assertNotIn("strum", text.lower())
                self.assertNotIn("getstrum", text.lower())


class PlatformCleanupTests(unittest.TestCase):
    @patch("kaboom_agentic_browser.platform.subprocess.run")
    def test_cleanup_removes_modern_and_legacy_pid_files(self, mock_run):
        with tempfile.TemporaryDirectory() as home:
            modern_pid = os.path.join(home, ".gasoline", "run", "gasoline-7890.pid")
            kaboom_pid = os.path.join(home, ".kaboom", "run", "kaboom-7890.pid")
            strum_pid = os.path.join(home, ".strum", "run", "strum-7890.pid")
            random_pid = os.path.join(home, ".gasoline", "run", "gasoline-44539.pid")
            legacy_pid = os.path.join(home, ".gasoline-7890.pid")
            os.makedirs(os.path.dirname(modern_pid), exist_ok=True)
            os.makedirs(os.path.dirname(kaboom_pid), exist_ok=True)
            os.makedirs(os.path.dirname(strum_pid), exist_ok=True)
            with open(modern_pid, "w", encoding="utf-8") as f:
                f.write("111")
            with open(kaboom_pid, "w", encoding="utf-8") as f:
                f.write("444")
            with open(strum_pid, "w", encoding="utf-8") as f:
                f.write("555")
            with open(random_pid, "w", encoding="utf-8") as f:
                f.write("333")
            with open(legacy_pid, "w", encoding="utf-8") as f:
                f.write("222")

            with patch.dict(os.environ, {"HOME": home}, clear=False):
                mock_run.return_value.stdout = ""
                platform.cleanup_old_processes()

            self.assertFalse(os.path.exists(modern_pid), f"expected pid removed: {modern_pid}")
            self.assertFalse(os.path.exists(kaboom_pid), f"expected pid removed: {kaboom_pid}")
            self.assertFalse(os.path.exists(strum_pid), f"expected pid removed: {strum_pid}")
            self.assertFalse(os.path.exists(random_pid), f"expected pid removed: {random_pid}")
            self.assertFalse(os.path.exists(legacy_pid), f"expected pid removed: {legacy_pid}")

    @patch("kaboom_agentic_browser.platform.subprocess.run")
    def test_cleanup_unix_targets_legacy_and_current_names(self, mock_run):
        mock_run.return_value.stdout = ""

        with patch.object(platform.sys, "platform", "linux"):
            platform.cleanup_old_processes()

        commands = [call.args[0] for call in mock_run.call_args_list if call.args]
        self.assertIn(["pgrep", "-af", "kaboom-agentic-browser"], commands)
        self.assertIn(["pgrep", "-af", "kaboom-mcp"], commands)
        self.assertIn(["pgrep", "-af", "kaboom"], commands)
        self.assertIn(["pgrep", "-af", "strum-agentic-browser"], commands)
        self.assertIn(["pgrep", "-af", "strum"], commands)
        self.assertIn(["pgrep", "-af", "gasoline-agentic-browser"], commands)
        self.assertIn(["pgrep", "-af", "gasoline-mcp"], commands)
        self.assertIn(["pgrep", "-af", "browser-agent"], commands)
        self.assertIn(["pgrep", "-af", "gasoline"], commands)

    @patch("kaboom_agentic_browser.platform.subprocess.run")
    def test_cleanup_unix_attempts_port_kills_for_all_known_ports(self, mock_run):
        mock_run.return_value.stdout = ""

        with patch.object(platform.sys, "platform", "linux"):
            platform.cleanup_old_processes()

        commands = [call.args[0] for call in mock_run.call_args_list if call.args]
        lsof_targets = {cmd[2] for cmd in commands if len(cmd) == 3 and cmd[:2] == ["lsof", "-ti"]}
        expected = {f":{port}" for port in platform.KNOWN_PORTS}
        self.assertSetEqual(
            expected,
            lsof_targets,
            "expected lsof lookup on every known gasoline port",
        )

    @patch("kaboom_agentic_browser.platform.subprocess.run")
    def test_cleanup_windows_prefers_home_env_when_expanduser_differs(self, mock_run):
        with tempfile.TemporaryDirectory() as home, tempfile.TemporaryDirectory() as other_home:
            modern_pid = os.path.join(home, ".gasoline", "run", "gasoline-7890.pid")
            os.makedirs(os.path.dirname(modern_pid), exist_ok=True)
            with open(modern_pid, "w", encoding="utf-8") as f:
                f.write("111")

            with patch.object(platform.sys, "platform", "win32"), \
                 patch.dict(os.environ, {"HOME": home, "USERPROFILE": other_home}, clear=False), \
                 patch("kaboom_agentic_browser.platform.os.path.expanduser", return_value=other_home):
                mock_run.return_value.stdout = ""
                platform.cleanup_old_processes()

            self.assertFalse(os.path.exists(modern_pid), f"expected pid removed: {modern_pid}")

    @patch("kaboom_agentic_browser.platform.subprocess.run")
    def test_cleanup_windows_falls_back_to_userprofile_when_home_missing(self, mock_run):
        with tempfile.TemporaryDirectory() as userprofile, tempfile.TemporaryDirectory() as expanded_home:
            modern_pid = os.path.join(userprofile, ".gasoline", "run", "gasoline-7890.pid")
            legacy_pid = os.path.join(userprofile, ".browser-agent-7890.pid")
            os.makedirs(os.path.dirname(modern_pid), exist_ok=True)
            with open(modern_pid, "w", encoding="utf-8") as f:
                f.write("111")
            with open(legacy_pid, "w", encoding="utf-8") as f:
                f.write("222")

            env = {"HOME": "", "USERPROFILE": userprofile}
            with patch.object(platform.sys, "platform", "win32"), \
                 patch.dict(os.environ, env, clear=False), \
                 patch("kaboom_agentic_browser.platform.os.path.expanduser", return_value=expanded_home):
                mock_run.return_value.stdout = ""
                platform.cleanup_old_processes()

            self.assertFalse(os.path.exists(modern_pid), f"expected pid removed: {modern_pid}")
            self.assertFalse(os.path.exists(legacy_pid), f"expected pid removed: {legacy_pid}")


if __name__ == "__main__":
    unittest.main()
