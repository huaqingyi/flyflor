#!/usr/bin/env bash
set -euo pipefail

FLYFLOR_REPO_URL="${FLYFLOR_REPO_URL:-https://github.com/huaqingyi/flyflor.git}"
FLYFLOR_INSTALL_DIR="${FLYFLOR_INSTALL_DIR:-$HOME/.flyflor/src}"
FLYFLOR_VENV="${FLYFLOR_VENV:-$HOME/.flyflor/venv}"
FLYFLOR_NPM_PREFIX="${FLYFLOR_NPM_PREFIX:-$HOME/.local}"
FLYFLOR_NPM_BIN_DIR="$FLYFLOR_NPM_PREFIX/bin"
FLYFLOR_SKIP_NODE_TOOLS="${FLYFLOR_SKIP_NODE_TOOLS:-0}"

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

need git
if [ "$FLYFLOR_SKIP_NODE_TOOLS" != "1" ]; then
  need npm
fi

python_is_supported() {
  "$1" - <<'PY' >/dev/null 2>&1
import sys
raise SystemExit(0 if sys.version_info >= (3, 11) else 1)
PY
}

choose_python() {
  if [ -n "${FLYFLOR_PYTHON:-}" ]; then
    if command -v "$FLYFLOR_PYTHON" >/dev/null 2>&1 && python_is_supported "$FLYFLOR_PYTHON"; then
      printf '%s\n' "$FLYFLOR_PYTHON"
      return
    fi
    echo "FLYFLOR_PYTHON must point to Python >= 3.11: $FLYFLOR_PYTHON" >&2
    exit 1
  fi

  for candidate in python3.13 python3.12 python3.11 python3; do
    if command -v "$candidate" >/dev/null 2>&1 && python_is_supported "$candidate"; then
      printf '%s\n' "$candidate"
      return
    fi
  done

  cat >&2 <<'MSG'
Flyflor requires Python >= 3.11.
Install a newer Python, or rerun with:

  export FLYFLOR_PYTHON=/path/to/python3.11
  curl -fsSL https://raw.githubusercontent.com/huaqingyi/flyflor/main/scripts/install.sh | bash

MSG
  exit 1
}

FLYFLOR_PYTHON_BIN="$(choose_python)"
FLYFLOR_BIN_DIR="${FLYFLOR_BIN_DIR:-$HOME/.local/bin}"

mkdir -p "$FLYFLOR_BIN_DIR"
mkdir -p "$FLYFLOR_NPM_PREFIX"

if [ -d "$FLYFLOR_INSTALL_DIR/.git" ]; then
  git -C "$FLYFLOR_INSTALL_DIR" pull --ff-only
else
  mkdir -p "$(dirname "$FLYFLOR_INSTALL_DIR")"
  git clone "$FLYFLOR_REPO_URL" "$FLYFLOR_INSTALL_DIR"
fi

"$FLYFLOR_PYTHON_BIN" -m venv "$FLYFLOR_VENV"
"$FLYFLOR_VENV/bin/python" -m pip install --upgrade pip
"$FLYFLOR_VENV/bin/python" -m pip install -e "$FLYFLOR_INSTALL_DIR"
"$FLYFLOR_VENV/bin/python" -m pip install -e "$FLYFLOR_INSTALL_DIR/vendor/nanobot"

ln -sf "$FLYFLOR_VENV/bin/flyflor" "$FLYFLOR_BIN_DIR/flyflor"
ln -sf "$FLYFLOR_VENV/bin/flyflor-core" "$FLYFLOR_BIN_DIR/flyflor-core"
ln -sf "$FLYFLOR_VENV/bin/flyflor-bridge" "$FLYFLOR_BIN_DIR/flyflor-bridge"

if [ "$FLYFLOR_SKIP_NODE_TOOLS" != "1" ]; then
  npm install --prefix "$FLYFLOR_NPM_PREFIX" -g @openai/codex @anthropic-ai/claude-code @github/copilot
fi

if ! command -v qdrant >/dev/null 2>&1; then
  cat <<'MSG'

Qdrant was not found in PATH.
Flyflor can still start with:

  flyflor gateway --skip-qdrant

For semantic memory, install qdrant separately, then rerun:

  flyflor gateway

MSG
fi

FLYFLOR_BIN="$FLYFLOR_BIN_DIR/flyflor"

export PATH="$FLYFLOR_BIN_DIR:$FLYFLOR_NPM_BIN_DIR:$PATH"

if [ ! -x "$FLYFLOR_BIN" ] && ! command -v flyflor >/dev/null 2>&1; then
  cat <<MSG

Flyflor was installed, but '$FLYFLOR_BIN_DIR' is not in PATH.
Add this to your shell profile:

  export PATH="$FLYFLOR_BIN_DIR:\$PATH"

MSG
fi

if [ "$FLYFLOR_SKIP_NODE_TOOLS" != "1" ] && ! command -v codex >/dev/null 2>&1; then
  cat <<MSG

Node CLI tools were installed under '$FLYFLOR_NPM_PREFIX', but '$FLYFLOR_NPM_BIN_DIR' is not in PATH.
Add this to your shell profile:

  export PATH="$FLYFLOR_NPM_BIN_DIR:\$PATH"

MSG
fi

if [ -x "$FLYFLOR_BIN" ]; then
  "$FLYFLOR_BIN" init
else
  flyflor init
fi

cat <<'MSG'

Flyflor installed.

Start:
  flyflor

Start gateway:
  flyflor gateway

Open Web Console after gateway starts:
  open http://127.0.0.1:8080

Configure isolated TUI tools:
  flyflor workers config codex
  flyflor workers config claude
  flyflor workers config copilot

Configure channels:
  flyflor setup

MSG
