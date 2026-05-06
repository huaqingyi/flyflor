# Flyflor Architecture

Flyflor is a lightweight multi-agent orchestration architecture built around a nanobot multi-channel gateway, a Web blackboard console, configurable CLI/Agent workers, and an auditable memory layer.

## Goal

The first version is intentionally conservative:

- use `nanobot` for multi-channel sessions, routing, and chat app stability;
- keep Flyflor's own orchestration core outside nanobot as a small sidecar service;
- use a SQLite blackboard as the source of truth;
- use Qdrant only for semantic retrieval, not as the truth store;
- connect Codex, Claude, Copilot, Kimi, OpenCode, Qwen, and custom CLI/Agent workers through bridge adapters;
- support `curl -fsSL ... | bash` host installation and Docker Compose development.

## High-Level Shape

```text
Chat Apps / WebSocket / API
        |
        v
nanobot gateway
        |
        v
Flyflor Core
        |
        +--> SQLite Blackboard
        +--> Qdrant Semantic Memory
        +--> Codex Worker Bridge
        +--> Claude Worker Bridge
        +--> Copilot / Kimi / OpenCode / Custom Workers
```

## Responsibility Split

### nanobot

nanobot is the channel and session shell.

It should own:

- Telegram, Discord, Slack, Feishu, DingTalk, QQ, WeCom, WeChat, email, and other channel adapters;
- per-channel user/session identity;
- channel-specific formatting, media, threading, and retries;
- forwarding complex work to Flyflor Core.

It should not own:

- multi-agent arbitration;
- blackboard state;
- long-term memory policy;
- Codex/Claude bridge lifecycle.

### Flyflor Core

Flyflor Core is the orchestration sidecar.

It owns:

- task creation and lifecycle;
- agent roles and bridge sessions;
- blackboard event validation;
- deliberation rounds;
- decision and veto policy;
- checkpointing and recovery;
- memory consolidation hooks.

### Bridges

Bridge adapters are controlled terminals or APIs for real worker agents.

Initial bridge targets:

- `codex`: OpenAI Codex CLI;
- `claude`: Claude Code CLI;
- `copilot`: GitHub Copilot CLI;
- `kimi`, `opencode`, `qwen-code`, `gemini`, `aider`, and custom commands.

The bridge layer must not rely on free-form chat alone. Every meaningful bridge output should be normalized into blackboard events such as:

- `proposal`
- `critique`
- `decision`
- `patch`
- `verification`
- `question`
- `final_report`

## Memory Design

Flyflor memory has three layers.

### Markdown Identity

Human-readable identity and policy files:

- `SOUL.md`: agent voice and durable identity;
- `USER.md`: stable user preferences;
- `POLICY.md`: operating rules and safety boundaries;
- `AGENTS.md`: role definitions for Codex, Claude, and future agents.

These files are small, reviewable, and versionable.

### SQLite

SQLite is the durable fact store and audit log.

It stores:

- tasks;
- blackboard events;
- decisions;
- artifacts;
- tool runs;
- bridge sessions;
- structured memory facts;
- consolidation cursors.

### Qdrant

Qdrant stores embeddings for semantic retrieval.

Qdrant records should point back to SQLite row IDs. It should not become the canonical source of truth.

## Long-Task Loop

A long task should follow this loop:

1. nanobot receives the user request and forwards it to Flyflor Core.
2. Flyflor creates a task and task brief.
3. Flyflor retrieves relevant memory from SQLite and Qdrant.
4. Flyflor opens bridge sessions for the selected workers.
5. Workers discuss through structured blackboard turns.
6. Flyflor decides whether to continue, ask the user, execute, verify, or stop.
7. Results and artifacts are persisted.
8. A memory consolidator extracts durable facts after completion.

## Bridge Risk Rules

The blackboard has sovereignty. Worker output is input evidence, not final truth.

Rules:

- all bridge output must be captured;
- all final decisions must be structured;
- every task has a max round count;
- vetoes must include category and evidence;
- stalled bridge sessions must be interrupted and checkpointed;
- user approval is required for destructive or high-risk actions.

## Install

Host install:

```bash
curl -fsSL https://raw.githubusercontent.com/huaqingyi/flyflor/main/scripts/install.sh | bash
```

The installer needs Python 3.11 or newer. It will prefer `python3.13`, `python3.12`, then `python3.11`; export `FLYFLOR_PYTHON=/path/to/python3.11` before running the pipe if needed. It clones this repo, installs Flyflor and its internal channel gateway dependency into a private venv under `~/.flyflor/venv`, links the `flyflor` command into `~/.local/bin`, and installs Node-based worker CLIs into a user-level prefix. Override paths with `FLYFLOR_INSTALL_DIR`, `FLYFLOR_VENV`, `FLYFLOR_BIN_DIR`, or `FLYFLOR_NPM_PREFIX`.

Start:

```bash
flyflor
```

Start the gateway:

```bash
flyflor gateway
```

Open the Web Console:

```bash
open http://127.0.0.1:8080
```

## Local Development

Clone dependencies and start the core stack:

```bash
docker compose up --build
```

The default stack starts one Flyflor appliance container with:

- Flyflor Core / Web Console on `http://127.0.0.1:8080`;
- Qdrant inside the same container on `http://127.0.0.1:6333`;
- internal channel gateway on `ws://localhost:8765`;
- SQLite data under `./data/flyflor`.

Health check:

```bash
curl http://localhost:8080/health
docker exec flyflor curl -s http://127.0.0.1:6333/
```

Create a test task:

```bash
curl -X POST http://localhost:8080/tasks \
  -H 'content-type: application/json' \
  -d '{"title":"Test bridge task","input":"Have Codex and Claude discuss the project skeleton."}'
```

OpenAI-compatible smoke test used by the channel gateway:

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H 'content-type: application/json' \
  -d '{"model":"flyflor-placeholder","messages":[{"role":"user","content":"Create a Flyflor task"}]}'
```

Worker tools are managed through Flyflor:

```bash
docker exec flyflor flyflor workers
docker exec -it flyflor flyflor workers config codex
docker exec -it flyflor flyflor workers config claude
```

Source code is mounted into the running containers by Docker Compose:

- `./data/flyflor` -> `/data/flyflor`
- `./data/qdrant` -> `/data/qdrant`
- `./data/nanobot` -> `/data/nanobot`
- `./data/logs` -> `/data/logs`
- `./workspace` -> `/workspace`
- `./flyflor_core` -> `/app/flyflor_core`
- `./vendor/nanobot` -> `/app/vendor/nanobot`
- `./memory` -> `/app/memory`
- `./configs` -> `/app/configs`

## Repository Layout

```text
.
├── README.md
├── Dockerfile
├── docker-compose.yml
├── configs/
│   └── nanobot/
├── flyflor_core/
│   ├── __main__.py
│   ├── blackboard.py
│   ├── config.py
│   ├── server.py
│   ├── schema.sql
│   └── bridges/
├── memory/
│   ├── SOUL.md
│   ├── USER.md
│   ├── POLICY.md
│   └── AGENTS.md
└── vendor/
    └── nanobot/
```

## Near-Term Milestones

1. Wire nanobot to Flyflor Core through HTTP.
2. Improve worker bridge process management.
3. Normalize bridge output into blackboard events.
4. Add task recovery from SQLite checkpoints.
5. Add Qdrant indexing for completed task summaries.
6. Add memory consolidation into markdown and structured facts.
