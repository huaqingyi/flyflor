# Architecture

Internal architecture notes for major runtime mechanisms and subsystem design.

- [Steering](steering.md): injecting messages into a running agent loop between tool calls.
- [SubTurn Mechanism](subturn.md): sub-agent coordination, concurrency control, and lifecycle handling.
- [Session System](session-system.md): session scope allocation, JSONL persistence, alias compatibility, and migration. ([ZH](session-system.zh.md))
- [Routing System](routing-system.md): agent dispatch, session policy selection, and light/heavy model routing. ([ZH](routing-system.zh.md))
- [Blackboard Workbench](blackboard-workbench.md): complexity routing, blackboard convergence, deadlock decision forms, and reflection drafts.
- [ARMS Methodology Memory](arms-methodology-memory.md): isolated methodology and self-growth memory for reflection drafts.
- [Runtime Events](runtime-events.md): runtime event envelope, centralized event logging, filters, and examples. ([ZH](runtime-events.zh.md))
- [Hook System Guide](hooks/README.md): current hook architecture and protocol details.
- [Sandbox Box](sandbox-box.md): execution policy, `/yolo`, tool risk decisions, and approval handoff.
- [Agent Refactor](agent-refactor/README.md): notes and checkpoints for the agent refactor work.

For proposal-style or exploratory docs, also see [`../design/`](../design/).
