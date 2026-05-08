# Blackboard Workbench

Flyflor uses the blackboard workbench only when a turn needs structured
coordination. Simple turns stay in the direct agent loop. This keeps latency
low for ordinary chat while preserving a stronger path for multi-step work.

## Design Rule

Prefer conventions over configuration.

The blackboard workbench has stable runtime conventions:

- simple turns use `direct`
- ambiguous turns use `direct-with-watch`
- complex turns use `blackboard`
- blackboard discussions should converge in 3 rounds
- 5 rounds is the hard upper bound
- livelock stops the discussion before the round limit when possible
- unresolved discussions are handed back to the user as a decision form
- reusable lessons are emitted as methodology reflection drafts

Configuration should only adjust deployment policy, not redefine the blackboard
protocol. Thresholds are configurable; scoring weights, round limits, livelock
rules, and the decision-form schema are conventions.

## Turn Modes

`direct`

The normal single-agent path. The blackboard prompt is not injected and the
blackboard scheduler does not acquire a turn lease.

`direct-with-watch`

The gray zone. The turn starts as direct, but the runtime watches for signs that
the task needs coordination. If it sees tool churn or repeated failure, it
restores the session to the start of the turn and reruns the same user request
in `blackboard` mode.

`blackboard`

The workbench path. The system prompt includes the planner/reviewer workbench,
convergence rules, deadlock handoff rules, and reflection handoff rules.

## Complexity Routing

The blackboard router uses structural features from the current message and
recent session history:

- token estimate
- code blocks
- diff or stack trace markers
- subtask count
- plan, implementation, verification, and review intent
- conversation depth
- recent tool-call density
- continuation references
- media references
- risk intent
- explicit blackboard or multi-agent requests
- cross-file or multi-step signals

The scoring weights are fixed in code. This is intentional: changing weights in
config makes behavior difficult to reason about and hard to document. Operators
can tune only the two thresholds:

```json
{
  "agents": {
    "defaults": {
      "blackboard_routing": {
        "enabled": true,
        "direct_threshold": 0.35,
        "threshold": 0.55,
        "allow_auto_escalation": true
      }
    }
  }
}
```

Mode selection:

- score `< direct_threshold`: `direct`
- score `>= direct_threshold` and `< threshold`: `direct-with-watch`
- score `>= threshold`: `blackboard`
- hard-gate signals bypass the score and go straight to `blackboard`

Hard gates are intentionally sparse. They cover explicit blackboard requests,
very large inputs, implementation plus verification, implementation plus review,
and cross-file workflows with implementation, verification, or review intent.
Risk intent alone is not a hard gate.

## Auto Escalation

`direct-with-watch` escalates to `blackboard` when one of these runtime signals
appears:

- at least 3 tool executions in the watched turn
- the same tool fails twice in a row
- the turn enters a second LLM/tool iteration while still needing tools
- proactive context compression detects budget pressure

Escalation is implemented as rollback and rerun. The direct attempt restores the
session to the turn restore point, emits `agent.blackboard.escalated`, and the
agent loop retries once in `blackboard` mode.

## Convergence

Blackboard discussion follows a fixed convergence budget:

- target maximum: 3 rounds
- hard maximum: 5 rounds

The workbench should stop early when it detects livelock:

- two consecutive rounds add no new facts
- the same disagreement repeats
- the same blocker remains open
- the same failing tool path is retried

When the workbench cannot converge, it must stop debating and ask the user for a
decision.

## Decision Form

Deadlock handoff uses a Markdown fenced block with the `flyflor-decision-form`
info string. The block contains JSON so TUI and WebUI can render it later as
single-select, multi-select, and custom input controls.

```flyflor-decision-form
{
  "version": 1,
  "title": "Decision needed",
  "summary": "One short sentence describing why Flyflor needs user input.",
  "single_select": {
    "id": "path",
    "label": "Choose one path",
    "options": [
      {
        "id": "recommended",
        "label": "Recommended",
        "description": "Why this is safest."
      }
    ]
  },
  "multi_select": {
    "id": "constraints",
    "label": "Optional constraints",
    "options": [
      {
        "id": "add_tests",
        "label": "Add tests",
        "description": "Include verification before final delivery."
      }
    ]
  },
  "custom_input": {
    "id": "notes",
    "label": "Additional context",
    "placeholder": "Tell Flyflor any missing constraint."
  }
}
```

Until TUI/WebUI render this as native controls, the same block remains readable
as plain Markdown and can be answered manually by the user.

## Reflection Drafts

Reflection storage is not part of this first step, but the workbench already has
a stable output convention for future memory ingestion. When a turn reveals a
reusable method, the answer may include:

```markdown
## Methodology Reflection Draft

- Situation: When this method applies.
- Method: The repeatable approach.
- Avoid: A pattern that failed or caused delay.
- Next-time hint: A short retrieval cue for future turns.
```

ARMS stores these drafts as isolated methodology memory. It does not store user
facts, project facts, ordinary assistant outcomes, or general semantic memory.
Before future planning, Flyflor can retrieve related ARMS methods and inject
them as `METHODOLOGY_MEMORY`.

## Events

The runtime emits:

- `agent.complexity.assessed`: score, mode, thresholds, reasons, features
- `agent.blackboard.escalated`: direct-with-watch rollback and rerun reason

These events are observational. The authoritative behavior lives in the agent
loop mode and the blackboard prompt convention.
