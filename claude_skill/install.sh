#!/bin/bash
# install.sh — Install the Strum Claude Code skill.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SKILL_SRC="$SCRIPT_DIR/gasoline"

if [ ! -d "$SKILL_SRC" ]; then
  echo "Error: strum skill directory not found at $SKILL_SRC" >&2
  exit 1
fi

# --- Install matching MCP server ---
install_mcp_server() {
  if ! command -v npm &>/dev/null; then
    echo "npm not found — skipping MCP server install."
    return
  fi

  echo ""
  echo "Install the Strum MCP server (strum)."
  echo "  To match your Chrome extension version, check chrome://extensions"
  echo "  and find the version number next to 'Strum AI DevTools'."
  echo ""
  read -rp "Enter extension version to match (or press Enter for latest): " version

  if [ -z "$version" ]; then
    echo "Installing latest strum..."
    npm install -g strum
    return
  fi

  # Validate version exists on npm
  echo "Checking if strum@${version} exists on npm..."
  while true; do
    if npm view "strum@${version}" version &>/dev/null; then
      npm install -g "strum@${version}"
      return
    else
      echo "ERROR: Version ${version} not found on npm."
      echo "  Available versions:"
      npm view strum versions --json 2>/dev/null \
        | tr -d '[]",' | xargs -n1 | tail -5 | sed 's/^/    /'
      echo ""
      read -rp "Enter a different version (or press Enter for latest): " version
      if [ -z "$version" ]; then
        echo "Installing latest strum..."
        npm install -g strum
        return
      fi
    fi
  done
}

# Check if we're inside a git project
IN_GIT=false
GIT_ROOT=""
if git rev-parse --show-toplevel > /dev/null 2>&1; then
  IN_GIT=true
  GIT_ROOT="$(git rev-parse --show-toplevel)"
fi

GLOBAL_DIR="$HOME/.claude/skills/strum"
PROJECT_DIR=""
if $IN_GIT; then
  PROJECT_DIR="$GIT_ROOT/.claude/skills/strum"
fi

install_skill() {
  local dest="$1"
  if [ -d "$dest" ]; then
    echo ""
    echo "WARNING: Existing skill found at $dest"
    echo "  The previous version will be deleted and replaced."
    read -rp "Overwrite? [y/N]: " overwrite
    case "$overwrite" in
      [Yy]*)
        rm -rf "$dest"
        ;;
      *)
        echo "Skipping skill install."
        return
        ;;
    esac
  fi
  mkdir -p "$dest"
  cp -r "$SKILL_SRC/"* "$dest/"
  echo "Installed strum skill to $dest"
}

if $IN_GIT; then
  echo "You are inside a git project: $GIT_ROOT"
  echo ""
  echo "Where do you want to install the Strum skill?"
  echo "  1) Global  (~/.claude/skills/strum) — available in all projects"
  echo "  2) Project (.claude/skills/strum)   — this project only"
  echo ""
  read -rp "Choose [1/2]: " choice

  case "$choice" in
    1) install_skill "$GLOBAL_DIR" ;;
    2) install_skill "$PROJECT_DIR" ;;
    *)
      echo "Invalid choice. Aborting." >&2
      exit 1
      ;;
  esac
else
  echo "Not inside a git project. Installing globally."
  install_skill "$GLOBAL_DIR"
fi

# Install MCP server to match the Chrome extension version
install_mcp_server
