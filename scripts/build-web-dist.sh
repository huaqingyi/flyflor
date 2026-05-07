#!/bin/sh
set -eu

cd "$(dirname "$0")/.."

pnpm -C web/frontend build:backend

cat <<'MSG'

Built web/backend/dist.

Fast path:
  If flyflor was started with docker-compose.webdev.yml and a backend that
  supports FLYFLOR_WEB_DIST_DIR, refresh http://127.0.0.1:18800.

No Docker rebuild is required for frontend-only changes.
MSG
