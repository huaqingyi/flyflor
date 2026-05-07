#!/bin/sh
set -eu

cd "$(dirname "$0")/.."

builder_compose="docker-compose.dev-build.yml"
output="build/dev/flyflor"

mkdir -p build/dev

echo "Starting reusable Flyflor builder container..."
docker compose -f "$builder_compose" --profile dev up -d flyflor-builder

echo "Building flyflor CLI inside the builder container..."
docker compose -f "$builder_compose" --profile dev exec -T flyflor-builder sh -c \
  'export PATH=/usr/local/go/bin:$PATH; go build -tags goolm,stdjson -o /src/build/dev/flyflor ./cmd/picoclaw'

if [ ! -f "$output" ]; then
  echo "Expected $output, but it was not created." >&2
  exit 1
fi

if docker ps --format '{{.Names}}' | grep -qx flyflor; then
  echo "Copying $output into the running flyflor container..."
  docker cp "$output" flyflor:/usr/local/bin/flyflor
  docker exec flyflor sh -lc 'ln -sf /usr/local/bin/flyflor /usr/local/bin/picoclaw'
  echo "Updated /usr/local/bin/flyflor. Try: docker exec -it flyflor flyflor"
else
  echo "Built $output. The flyflor container is not running, so nothing was copied."
fi
