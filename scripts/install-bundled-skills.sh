#!/usr/bin/env bash
# install-bundled-skills.sh
# Install bundled Gasoline skills from source tree for manual/local builds.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SKILLS_SRC_DIR="${GASOLINE_BUNDLED_SKILLS_DIR:-$PROJECT_ROOT/npm/gasoline-mcp/skills}"
MARKER="<!-- gasoline-managed-skill"

SCOPE="${GASOLINE_SKILL_SCOPE:-global}"
TARGETS_RAW="${GASOLINE_SKILL_TARGETS:-${GASOLINE_SKILL_TARGET:-claude,codex,gemini}}"
PROJECT_SCOPE_ROOT="${GASOLINE_PROJECT_ROOT:-$(pwd)}"

CREATED=0
UPDATED=0
UNCHANGED=0
SKIPPED=0
LEGACY_REMOVED=0
ERRORS=0

case "$SCOPE" in
  global|project|all) ;;
  *)
    echo "Invalid GASOLINE_SKILL_SCOPE='$SCOPE' (expected: global, project, all)" >&2
    exit 1
    ;;
esac

if [ ! -d "$SKILLS_SRC_DIR" ] || [ ! -f "$SKILLS_SRC_DIR/skills.json" ]; then
  echo "Bundled skills directory not found: $SKILLS_SRC_DIR" >&2
  exit 1
fi

agent_global_root() {
  local agent="$1"
  case "$agent" in
    claude)
      printf "%s\n" "${GASOLINE_CLAUDE_SKILLS_DIR:-$HOME/.claude/skills}"
      ;;
    codex)
      local codex_home="${CODEX_HOME:-$HOME/.codex}"
      printf "%s\n" "${GASOLINE_CODEX_SKILLS_DIR:-$codex_home/skills}"
      ;;
    gemini)
      local gemini_home="${GEMINI_HOME:-$HOME/.gemini}"
      printf "%s\n" "${GASOLINE_GEMINI_SKILLS_DIR:-$gemini_home/skills}"
      ;;
    *)
      return 1
      ;;
  esac
}

agent_project_root() {
  local agent="$1"
  case "$agent" in
    claude)
      printf "%s\n" "$PROJECT_SCOPE_ROOT/.claude/skills"
      ;;
    codex)
      printf "%s\n" "$PROJECT_SCOPE_ROOT/.codex/skills"
      ;;
    gemini)
      printf "%s\n" "$PROJECT_SCOPE_ROOT/.gemini/skills"
      ;;
    *)
      return 1
      ;;
  esac
}

skill_dest_path() {
  local agent="$1"
  local root="$2"
  local skill_id="$3"
  if [ "$agent" = "codex" ]; then
    printf "%s\n" "$root/$skill_id/SKILL.md"
  else
    printf "%s\n" "$root/$skill_id.md"
  fi
}

remove_legacy_skill() {
  local agent="$1"
  local root="$2"
  local skill_id="$3"
  local legacy_id="gasoline-$skill_id"
  local legacy_path
  legacy_path="$(skill_dest_path "$agent" "$root" "$legacy_id")"

  if [ -f "$legacy_path" ] && grep -q "$MARKER" "$legacy_path"; then
    rm -f "$legacy_path" || true
    if [ "$agent" = "codex" ]; then
      rmdir "$(dirname "$legacy_path")" 2>/dev/null || true
    fi
    LEGACY_REMOVED=$((LEGACY_REMOVED + 1))
  fi
}

install_skill() {
  local agent="$1"
  local root="$2"
  local skill_dir="$3"
  local skill_id
  skill_id="$(basename "$skill_dir")"
  local src_file="$skill_dir/SKILL.md"

  if [ ! -f "$src_file" ]; then
    return
  fi

  local dest
  dest="$(skill_dest_path "$agent" "$root" "$skill_id")"
  mkdir -p "$(dirname "$dest")"

  local tmp_file
  tmp_file="$(mktemp)"
  {
    printf "%s id:%s version:1 -->\n" "$MARKER" "$skill_id"
    cat "$src_file"
  } >"$tmp_file"

  if [ -f "$dest" ]; then
    if cmp -s "$tmp_file" "$dest"; then
      UNCHANGED=$((UNCHANGED + 1))
      rm -f "$tmp_file"
      return
    fi
    if ! grep -q "$MARKER" "$dest"; then
      SKIPPED=$((SKIPPED + 1))
      rm -f "$tmp_file"
      return
    fi
    if cp "$tmp_file" "$dest"; then
      UPDATED=$((UPDATED + 1))
    else
      ERRORS=$((ERRORS + 1))
    fi
  else
    if cp "$tmp_file" "$dest"; then
      CREATED=$((CREATED + 1))
    else
      ERRORS=$((ERRORS + 1))
    fi
  fi

  rm -f "$tmp_file"
  remove_legacy_skill "$agent" "$root" "$skill_id"
}

for agent in $(printf "%s" "$TARGETS_RAW" | tr ',' ' '); do
  case "$agent" in
    claude|codex|gemini) ;;
    *)
      echo "Skipping unknown agent: $agent"
      continue
      ;;
  esac

  roots=""
  if [ "$SCOPE" = "global" ] || [ "$SCOPE" = "all" ]; then
    roots="$(agent_global_root "$agent")"
  fi
  if [ "$SCOPE" = "project" ] || [ "$SCOPE" = "all" ]; then
    if [ -n "$roots" ]; then
      roots="$roots
$(agent_project_root "$agent")"
    else
      roots="$(agent_project_root "$agent")"
    fi
  fi

  while IFS= read -r root; do
    [ -z "$root" ] && continue
    for skill_dir in "$SKILLS_SRC_DIR"/*; do
      [ -d "$skill_dir" ] || continue
      install_skill "$agent" "$root" "$skill_dir"
    done
  done <<EOF_ROOTS
$roots
EOF_ROOTS

done

echo "Skills installed (${TARGETS_RAW} / ${SCOPE}): created=${CREATED} updated=${UPDATED} unchanged=${UNCHANGED} skipped=${SKIPPED} legacy_removed=${LEGACY_REMOVED} errors=${ERRORS}"

if [ "$ERRORS" -gt 0 ]; then
  exit 1
fi
