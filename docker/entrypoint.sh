#!/bin/bash
set -eu

mkdir -p \
  /data/flyflor \
  /data/flyflor/agents/home \
  /data/flyflor/agents/codex \
  /data/flyflor/agents/claude \
  /data/flyflor/agents/copilot \
  /data/flyflor/nanobot \
  /data/flyflor/xdg/config \
  /data/flyflor/xdg/cache \
  /data/flyflor/xdg/data \
  /data/flyflor/tmp \
  /data/qdrant/storage \
  /data/logs \
  /workspace

export HOME=/data/flyflor/agents/home
export CODEX_HOME=/data/flyflor/agents/codex
export CLAUDE_CONFIG_DIR=/data/flyflor/agents/claude
export COPILOT_CONFIG_DIR=/data/flyflor/agents/copilot
export XDG_CONFIG_HOME=/data/flyflor/xdg/config
export XDG_CACHE_HOME=/data/flyflor/xdg/cache
export XDG_DATA_HOME=/data/flyflor/xdg/data
export TMPDIR=/data/flyflor/tmp

if [ -L /root/.nanobot ] && [ "$(readlink /root/.nanobot)" = "/data/flyflor/nanobot" ]; then
  :
else
  if [ -d /root/.nanobot ]; then
    if [ ! -f /data/flyflor/nanobot/config.json ] && [ -f /root/.nanobot/config.json ]; then
      cp /root/.nanobot/config.json /data/flyflor/nanobot/config.json
    fi
    rm -rf /root/.nanobot
  fi
  ln -s /data/flyflor/nanobot /root/.nanobot
fi

for pair in \
  "/root/.codex:/data/flyflor/agents/codex" \
  "/root/.claude:/data/flyflor/agents/claude" \
  "/root/.copilot:/data/flyflor/agents/copilot"
do
  link="${pair%%:*}"
  target="${pair#*:}"
  if [ -L "$link" ] && [ "$(readlink "$link")" = "$target" ]; then
    :
  else
    rm -rf "$link"
    ln -s "$target" "$link"
  fi
done

flyflor setup --defaults

start_background() {
  name="$1"
  shift
  echo "Starting ${name}: $*"
  "$@" >>"/data/logs/${name}.log" 2>&1 &
}

if [ -x /qdrant/qdrant ]; then
  start_background qdrant /qdrant/qdrant
else
  echo "Qdrant binary not found at /qdrant/qdrant" >&2
  exit 1
fi

start_background nanobot nanobot gateway --config /data/flyflor/nanobot/config.json
start_background bridge flyflor bridge-daemon

exec "$@"
