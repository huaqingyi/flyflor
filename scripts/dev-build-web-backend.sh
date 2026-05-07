#!/bin/sh
set -eu

cd "$(dirname "$0")/.."

builder_compose="docker-compose.dev-build.yml"
output="build/dev/flyflor-web"

mkdir -p build/dev

echo "Starting reusable Flyflor builder container..."
docker compose -f "$builder_compose" --profile dev up -d flyflor-builder

echo "Building frontend dist + flyflor-web inside the builder container..."
docker compose -f "$builder_compose" --profile dev exec -T flyflor-builder sh -c \
  'export PATH=/usr/local/go/bin:$PATH; make -C web build OUTPUT=/src/build/dev/flyflor-web PICOCLAW_BINARY=/usr/local/bin/flyflor'

if [ ! -f "$output" ]; then
  echo "Expected $output, but it was not created." >&2
  exit 1
fi

if docker ps --format '{{.Names}}' | grep -qx flyflor; then
  echo "Copying $output into the running flyflor container..."
  docker cp "$output" flyflor:/usr/local/bin/flyflor-web
  echo "Restarting flyflor without rebuilding the image..."
  docker restart flyflor >/dev/null
  echo "Updated http://127.0.0.1:18800"
else
  echo "Built $output. The flyflor container is not running, so nothing was copied."
fi
