"""Tests for bundled skill installation in the PyPI wrapper.

Purpose: Validate managed write/update/skip behavior for packaged skills.
Why: Prevents channel drift between npm and PyPI skill-install semantics.
Docs: docs/features/feature/enhanced-cli-config/index.md
"""

import os
import tempfile
import unittest
from unittest.mock import patch

from gasoline_agentic_browser import skills


class SkillsInstallTests(unittest.TestCase):
    def test_install_bundled_skills_for_codex_global_root(self):
        with tempfile.TemporaryDirectory() as tmp:
            codex_root = os.path.join(tmp, "codex-skills")
            env = {
                "GASOLINE_SKILL_TARGETS": "codex",
                "GASOLINE_SKILL_SCOPE": "global",
                "GASOLINE_CODEX_SKILLS_DIR": codex_root,
            }

            with patch.dict(os.environ, env, clear=False):
                result = skills.install_bundled_skills(verbose=False)

            self.assertFalse(result["skipped"])
            self.assertGreater(result["summary"]["created"], 0)

            expected = os.path.join(codex_root, "debug-triage", "SKILL.md")
            self.assertTrue(os.path.exists(expected))

            with open(expected, "r", encoding="utf-8") as f:
                content = f.read()
            self.assertIn("<!-- gasoline-managed-skill", content)
            self.assertIn("name: debug-triage", content)

    def test_skip_respects_env_flag(self):
        with patch.dict(os.environ, {"GASOLINE_SKIP_SKILL_INSTALL": "1"}, clear=False):
            result = skills.install_bundled_skills(verbose=False)

        self.assertTrue(result["skipped"])
        self.assertEqual(result["reason"], "disabled_by_env")

    def test_user_owned_skill_is_not_overwritten(self):
        with tempfile.TemporaryDirectory() as tmp:
            codex_root = os.path.join(tmp, "codex-skills")
            user_skill = os.path.join(codex_root, "debug-triage", "SKILL.md")
            os.makedirs(os.path.dirname(user_skill), exist_ok=True)
            with open(user_skill, "w", encoding="utf-8") as f:
                f.write("# user owned\n")

            env = {
                "GASOLINE_SKILL_TARGETS": "codex",
                "GASOLINE_SKILL_SCOPE": "global",
                "GASOLINE_CODEX_SKILLS_DIR": codex_root,
            }
            with patch.dict(os.environ, env, clear=False):
                result = skills.install_bundled_skills(verbose=False)

            self.assertFalse(result["skipped"])
            self.assertGreaterEqual(result["summary"]["skipped_user_owned"], 1)

            with open(user_skill, "r", encoding="utf-8") as f:
                content = f.read()
            self.assertEqual(content, "# user owned\n")


if __name__ == "__main__":
    unittest.main()
