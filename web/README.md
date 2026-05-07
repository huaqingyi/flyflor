# Flyflor Web Console

`web/` contains the Flyflor launcher and React cockpit UI.

The launcher bundles the frontend, exposes backend APIs, manages local dashboard authentication, and starts or attaches to the Flyflor gateway process. The frontend is designed as an AI Runtime Cockpit: natural dialogue stays on the left, the grouped blackboard stays on the right, and live reasoning remains compact inside the conversation.

## Cockpit Surfaces

- Dialogue panel: user-facing conversation and final Flyflor responses on the left.
- Blackboard panel: always available on the right, grouped by user turn, with Planner and Reviewer worker turns, memory checkpoints, tool calls, review notes, and consensus.
- Runtime signals: gateway state, blackboard scheduler state, active session, model, agent graph, memory inspector, and event flow are surfaced without forcing horizontal scrolling.

## Local Development

```bash
make dev
```

The launcher uses the same core config file as the Flyflor runtime. Internal compatibility names such as `PICOCLAW_BINARY` may still appear in environment variables and build scripts to avoid breaking existing startup paths.

## Build

```bash
make build
```

For Docker-based use, prefer the root `Dockerfile` and `docker-compose.yml`.
