import os
import sys
import tempfile
import unittest
from unittest.mock import patch


ROOT = os.path.dirname(os.path.dirname(__file__))
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)

from gasoline_mcp import platform  # pylint: disable=wrong-import-position


class PlatformCleanupTests(unittest.TestCase):
    @patch("gasoline_mcp.platform.subprocess.run")
    def test_cleanup_removes_modern_and_legacy_pid_files(self, mock_run):
        with tempfile.TemporaryDirectory() as home:
            modern_pid = os.path.join(home, ".gasoline", "run", "gasoline-7890.pid")
            legacy_pid = os.path.join(home, ".gasoline-7890.pid")
            os.makedirs(os.path.dirname(modern_pid), exist_ok=True)
            with open(modern_pid, "w", encoding="utf-8") as f:
                f.write("111")
            with open(legacy_pid, "w", encoding="utf-8") as f:
                f.write("222")

            with patch.dict(os.environ, {"HOME": home}, clear=False):
                mock_run.return_value.stdout = ""
                platform.cleanup_old_processes()

            self.assertFalse(os.path.exists(modern_pid), f"expected pid removed: {modern_pid}")
            self.assertFalse(os.path.exists(legacy_pid), f"expected pid removed: {legacy_pid}")


if __name__ == "__main__":
    unittest.main()
