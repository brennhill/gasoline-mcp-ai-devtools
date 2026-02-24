"""Bundled skill installer for the PyPI wrapper.

Purpose: Install/update managed bundled skills into supported agent skill directories.
Why: Brings PyPI `--install` behavior to parity with npm skill management semantics.
Docs: docs/features/feature/enhanced-cli-config/index.md
"""

import json
import os
from pathlib import Path

MANAGED_MARKER = "<!-- gasoline-managed-skill"
DEFAULT_AGENTS = ["claude", "codex", "gemini"]


def _parse_bool_env(name):
    value = os.environ.get(name)
    if not value:
        return False
    normalized = str(value).strip().lower()
    return normalized in {"1", "true", "yes", "on"}


def parse_agents():
    """Parse target agents from GASOLINE_SKILL_TARGETS or GASOLINE_SKILL_TARGET."""
    raw = os.environ.get("GASOLINE_SKILL_TARGETS") or os.environ.get("GASOLINE_SKILL_TARGET")
    if not raw:
        return list(DEFAULT_AGENTS)

    requested = [part.strip().lower() for part in raw.split(",") if part.strip()]
    filtered = [agent for agent in requested if agent in DEFAULT_AGENTS]
    return filtered if filtered else list(DEFAULT_AGENTS)


def parse_scope(default_scope="global"):
    """Parse install scope from GASOLINE_SKILL_SCOPE."""
    raw = str(os.environ.get("GASOLINE_SKILL_SCOPE", default_scope)).strip().lower()
    if raw in {"global", "project", "all"}:
        return raw
    return default_scope


def _project_root():
    override = os.environ.get("GASOLINE_PROJECT_ROOT")
    if override:
        return Path(override).expanduser().resolve()
    return Path.cwd().resolve()


def _dedupe_paths(paths):
    deduped = []
    seen = set()
    for p in paths:
        key = str(p.resolve())
        if key in seen:
            continue
        seen.add(key)
        deduped.append(p)
    return deduped


def get_agent_roots(agent, scope):
    """Get install roots for an agent and scope."""
    home = Path.home()
    project_root = _project_root()

    global_roots = {
        "claude": Path(os.environ.get("GASOLINE_CLAUDE_SKILLS_DIR", home / ".claude" / "skills")),
        "codex": Path(
            os.environ.get(
                "GASOLINE_CODEX_SKILLS_DIR",
                Path(os.environ.get("CODEX_HOME", home / ".codex")) / "skills",
            )
        ),
        "gemini": Path(
            os.environ.get(
                "GASOLINE_GEMINI_SKILLS_DIR",
                Path(os.environ.get("GEMINI_HOME", home / ".gemini")) / "skills",
            )
        ),
    }

    project_roots = {
        "claude": project_root / ".claude" / "skills",
        "codex": project_root / ".codex" / "skills",
        "gemini": project_root / ".gemini" / "skills",
    }

    roots = []
    if scope in {"global", "all"}:
        roots.append(global_roots[agent])
    if scope in {"project", "all"}:
        roots.append(project_roots[agent])

    return _dedupe_paths([Path(p).expanduser() for p in roots])


def _skill_file_path(agent, root_dir, skill_id):
    if agent == "codex":
        return root_dir / skill_id / "SKILL.md"
    return root_dir / f"{skill_id}.md"


def _build_managed_content(skill):
    body = skill["body"].rstrip("\n") + "\n"
    return f"{MANAGED_MARKER} id:{skill['id']} version:{skill['version']} -->\n{body}"


def _safe_write_managed_file(path, content):
    path.parent.mkdir(parents=True, exist_ok=True)

    if path.exists():
        existing = path.read_text(encoding="utf-8")
        if existing == content:
            return "unchanged"
        if MANAGED_MARKER not in existing:
            return "skipped_user_owned"
        path.write_text(content, encoding="utf-8")
        return "updated"

    path.write_text(content, encoding="utf-8")
    return "created"


def _remove_legacy_skill(agent, root_dir, skill_id):
    legacy_id = f"gasoline-{skill_id}"
    legacy_path = _skill_file_path(agent, root_dir, legacy_id)
    if not legacy_path.exists():
        return False

    try:
        existing = legacy_path.read_text(encoding="utf-8")
        if MANAGED_MARKER not in existing:
            return False
        legacy_path.unlink()
        if agent == "codex":
            try:
                legacy_path.parent.rmdir()
            except OSError:
                pass
        return True
    except OSError:
        return False


def _load_bundled_catalog():
    skills_root = Path(__file__).resolve().parent / "skills"
    manifest_path = skills_root / "skills.json"
    if not manifest_path.exists():
        raise RuntimeError(f"skills manifest not found: {manifest_path}")

    try:
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as err:
        raise RuntimeError(f"invalid skills manifest JSON: {err}") from err

    raw_skills = manifest.get("skills")
    if not isinstance(raw_skills, list):
        raise RuntimeError("invalid skills manifest: expected { skills: [] }")

    skills = []
    warnings = []
    for entry in raw_skills:
        skill_id = entry.get("id") if isinstance(entry, dict) else None
        if not skill_id:
            continue

        skill_path = skills_root / skill_id / "SKILL.md"
        if not skill_path.exists():
            warnings.append(f"missing bundled skill file: {skill_path}")
            continue

        skills.append({
            "id": skill_id,
            "version": entry.get("version", 1),
            "body": skill_path.read_text(encoding="utf-8").rstrip("\n") + "\n",
        })

    return skills, warnings


def install_bundled_skills(verbose=False):
    """Install bundled skills for configured agents/scope.

    Returns a dict with install summary and warnings.
    """
    if _parse_bool_env("GASOLINE_SKIP_SKILL_INSTALL") or _parse_bool_env("GASOLINE_SKIP_SKILLS_INSTALL"):
        return {
            "skipped": True,
            "reason": "disabled_by_env",
            "agents": [],
            "scope": parse_scope(),
            "warnings": [],
            "summary": {
                "created": 0,
                "updated": 0,
                "unchanged": 0,
                "skipped_user_owned": 0,
                "legacy_removed": 0,
                "errors": 0,
            },
        }

    skills, warnings = _load_bundled_catalog()
    if not skills:
        return {
            "skipped": True,
            "reason": "no_bundled_skills",
            "agents": [],
            "scope": parse_scope(),
            "warnings": warnings,
            "summary": {
                "created": 0,
                "updated": 0,
                "unchanged": 0,
                "skipped_user_owned": 0,
                "legacy_removed": 0,
                "errors": 0,
            },
        }

    agents = parse_agents()
    scope = parse_scope()
    summary = {
        "created": 0,
        "updated": 0,
        "unchanged": 0,
        "skipped_user_owned": 0,
        "legacy_removed": 0,
        "errors": 0,
    }

    for agent in agents:
        for root_dir in get_agent_roots(agent, scope):
            for skill in skills:
                try:
                    content = _build_managed_content(skill)
                    out_path = _skill_file_path(agent, root_dir, skill["id"])
                    status = _safe_write_managed_file(out_path, content)
                    if status in summary:
                        summary[status] += 1
                    else:
                        summary["errors"] += 1

                    if verbose and status != "unchanged":
                        print(
                            f"[gasoline-mcp] skills {status}: "
                            f"{agent}:{skill['id']} -> {out_path}"
                        )

                    if _remove_legacy_skill(agent, root_dir, skill["id"]):
                        summary["legacy_removed"] += 1
                except OSError as err:
                    summary["errors"] += 1
                    warnings.append(
                        f"failed to install skill {skill['id']} for {agent} ({root_dir}): {err}"
                    )

    return {
        "skipped": False,
        "reason": None,
        "agents": agents,
        "scope": scope,
        "warnings": warnings,
        "summary": summary,
    }
