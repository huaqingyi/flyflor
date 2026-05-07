# Flyflor Development Workflow

> Back to [Operations](README.md)

This guide documents the fast local development loop for Flyflor. The main rule is simple: do not run `docker compose up -d --build flyflor` for every UI iteration.

## Command Matrix

| Scenario | Command | Rebuilds Docker image |
| --- | --- | --- |
| Web UI hot reload | `make dev-webui` | No |
| Build frontend dist only | `make build-web-dist` | No |
| Rebuild `flyflor` CLI and copy it into the running container | `make dev-build-cli` | No |
| Rebuild `flyflor-web` and copy it into the running container | `make dev-build-web-backend` | No |
| Validate Dockerfile / final image | `docker compose up -d --build flyflor` | Yes |

## Web UI Hot Reload

Use this for normal Web UI work:

```bash
make dev-webui
```

Open:

```text
http://127.0.0.1:5173
```

The Vite dev server proxies `/api`, `/pico/ws`, and `/pico/media` to the Docker backend on `http://127.0.0.1:18800`.

Use this for changes under `web/frontend/src/**`, layout, blackboard UI, styling, icons, and favicon work.

## Build Frontend Dist Only

Use this when you need production frontend output in `web/backend/dist`:

```bash
make build-web-dist
```

This runs TypeScript checks and Vite production build without touching Docker.

## Rebuild Web Backend Without Rebuilding Docker

Use this when `flyflor-web` itself must change:

```bash
make dev-build-web-backend
```

It uses the reusable `flyflor-builder` container with persistent Docker volumes for:

- Go module cache
- Go build cache
- pnpm store

Then it copies the new `build/dev/flyflor-web` binary into the running `flyflor` container and restarts that container.

## Rebuild CLI Without Rebuilding Docker

Use this when the `flyflor` CLI/TUI entrypoint itself changes:

```bash
make dev-build-cli
```

It reuses the same builder container and Go caches, builds `build/dev/flyflor`, then copies it into the running container at `/usr/local/bin/flyflor`.

Test it with:

```bash
docker exec -it flyflor flyflor
```

## Mounted Frontend Dist For Port 18800

The Web backend supports:

```bash
FLYFLOR_WEB_DIST_DIR=/source/web/backend/dist
```

Start the runtime with:

```bash
docker compose -f docker-compose.yml -f docker-compose.webdev.yml up -d flyflor
```

Then frontend-only changes can be applied to `http://127.0.0.1:18800` with:

```bash
make build-web-dist
```

If the running `flyflor-web` binary is from an older image and does not support `FLYFLOR_WEB_DIST_DIR`, run `make dev-build-web-backend` once.

## When To Use Full Docker Build

Use full image rebuilds only when changing:

- `Dockerfile`
- OS packages or runtime image dependencies
- container entrypoint behavior
- release / CI validation

Avoid full image rebuilds for frontend layout, TUI styling, icons, static assets, and ordinary TypeScript component work.
