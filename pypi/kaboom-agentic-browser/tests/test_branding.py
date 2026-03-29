# Purpose: Validate PyPI wrapper branding surfaces and prevent legacy copy regressions.
# Why: Ensures the Python distribution exposes Kaboom-first names and diagnostics.
# Docs: docs/features/feature/enhanced-cli-config/index.md

import unittest

from kaboom_agentic_browser.errors import BinaryNotFoundError, KaboomError
from kaboom_agentic_browser.output import diagnostic_report, uninstall_result


class BrandingTests(unittest.TestCase):
    def test_kaboom_error_is_base_class(self):
        err = KaboomError("Test message", "Try again")
        self.assertEqual(err.name, "KaboomError")
        self.assertIn("Test message", err.format())

    def test_binary_not_found_error_uses_kaboom_copy(self):
        err = BinaryNotFoundError("/tmp/missing-kaboom")
        self.assertIn("Kaboom", err.message)
        self.assertIn("kaboom-agentic-browser", err.recovery)

    def test_diagnostic_report_uses_kaboom_title(self):
        report = diagnostic_report({
            "tools": [],
            "binary": {"ok": False, "error": "missing"},
            "summary": "Summary: 0 tools configured",
        })
        self.assertIn("Kaboom Agentic Browser Diagnostic Report", report)
        self.assertNotIn("Kaboom", report)

    def test_uninstall_result_uses_kaboom_empty_state_copy(self):
        result = uninstall_result({"removed": [], "notConfigured": [], "errors": []})
        self.assertIn("Kaboom not configured", result)
        self.assertNotIn("Kaboom", result)


if __name__ == "__main__":
    unittest.main()
