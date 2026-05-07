#!/bin/sh
set -eu

cd "$(dirname "$0")/.."

echo "Starting Flyflor services without rebuilding Docker..."
docker compose up -d qdrant flyflor

echo
echo "Starting Vite Web UI dev server."
echo "Open: http://127.0.0.1:5173"
echo "Backend/API proxy: http://127.0.0.1:18800"
echo

pnpm -C web/frontend dev --host 0.0.0.0
