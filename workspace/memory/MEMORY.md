# Long-term Memory

This Markdown layer stores stable identity, explicit user preferences, and
high-signal project notes. It is the readable "soul/profile" layer of Flyflor's
three-layer memory system.

## User Information

- The user is building Flyflor, an AI runtime cockpit with visible bridge discussion and long-term learning.

## Preferences

- Blackboard dialogue must be grouped by each user question.
- The default blackboard view should show the latest turn, with older turn groups clickable.
- Bridge dialogue should be detailed and understandable, not hidden until consensus.
- User-facing text should say Flyflor, not PicoClaw.

## Important Notes

- Memory architecture must have three active layers:
  - Markdown: identity, user profile, stable preferences, project constitution.
  - SQLite/Seahorse: structured session timeline, events, summaries, and exact retrieval.
  - Qdrant: semantic vector memories extracted from compressed facts, preferences, decisions, and lessons.
- Qdrant memory should be queried before model calls and indexed after meaningful user/assistant turns.

## Configuration

- Default context manager should be Seahorse with semantic Qdrant memory enabled when QDRANT_URL is available.
