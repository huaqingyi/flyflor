from __future__ import annotations

import json
import sqlite3
from dataclasses import dataclass
from datetime import datetime, timezone
from importlib import resources
from pathlib import Path
from typing import Any


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat()


@dataclass(frozen=True)
class TaskRecord:
    id: int
    title: str
    status: str
    created_at: str


class Blackboard:
    def __init__(self, database: Path) -> None:
        self.database = database
        self.database.parent.mkdir(parents=True, exist_ok=True)
        self.initialize()

    def connect(self) -> sqlite3.Connection:
        conn = sqlite3.connect(self.database)
        conn.row_factory = sqlite3.Row
        conn.execute("PRAGMA journal_mode=WAL")
        conn.execute("PRAGMA foreign_keys=ON")
        return conn

    def initialize(self) -> None:
        schema = resources.files("flyflor_core").joinpath("schema.sql").read_text()
        with self.connect() as conn:
            conn.executescript(schema)

    def create_task(self, title: str, input_text: str, metadata: dict[str, Any] | None = None) -> TaskRecord:
        now = utc_now()
        metadata_json = json.dumps(metadata or {}, ensure_ascii=False, sort_keys=True)
        with self.connect() as conn:
            cursor = conn.execute(
                """
                INSERT INTO tasks (title, input, status, metadata_json, created_at, updated_at)
                VALUES (?, ?, 'queued', ?, ?, ?)
                """,
                (title, input_text, metadata_json, now, now),
            )
            task_id = int(cursor.lastrowid)
            conn.execute(
                """
                INSERT INTO blackboard_events
                    (task_id, actor, event_type, content, payload_json, created_at)
                VALUES (?, 'system', 'task_created', ?, ?, ?)
                """,
                (task_id, input_text, metadata_json, now),
            )
        return TaskRecord(id=task_id, title=title, status="queued", created_at=now)

    def append_event(
        self,
        task_id: int,
        actor: str,
        event_type: str,
        content: str,
        payload: dict[str, Any] | None = None,
    ) -> int:
        now = utc_now()
        payload_json = json.dumps(payload or {}, ensure_ascii=False, sort_keys=True)
        with self.connect() as conn:
            cursor = conn.execute(
                """
                INSERT INTO blackboard_events
                    (task_id, actor, event_type, content, payload_json, created_at)
                VALUES (?, ?, ?, ?, ?, ?)
                """,
                (task_id, actor, event_type, content, payload_json, now),
            )
            conn.execute("UPDATE tasks SET updated_at = ? WHERE id = ?", (now, task_id))
            return int(cursor.lastrowid)

    def update_task_status(self, task_id: int, status: str) -> None:
        now = utc_now()
        with self.connect() as conn:
            conn.execute("UPDATE tasks SET status = ?, updated_at = ? WHERE id = ?", (status, now, task_id))

    def claim_next_task(self, *, from_status: str = "queued", to_status: str = "discussing") -> dict[str, Any] | None:
        now = utc_now()
        with self.connect() as conn:
            conn.execute("BEGIN IMMEDIATE")
            row = conn.execute(
                """
                SELECT * FROM tasks
                WHERE status = ?
                ORDER BY id DESC
                LIMIT 1
                """,
                (from_status,),
            ).fetchone()
            if row is None:
                conn.commit()
                return None
            task = dict(row)
            conn.execute("UPDATE tasks SET status = ?, updated_at = ? WHERE id = ?", (to_status, now, task["id"]))
            conn.commit()
            task["status"] = to_status
            task["updated_at"] = now
            return task

    def get_task(self, task_id: int) -> dict[str, Any] | None:
        with self.connect() as conn:
            row = conn.execute("SELECT * FROM tasks WHERE id = ?", (task_id,)).fetchone()
            if row is None:
                return None
            return dict(row)

    def list_tasks(self, limit: int = 20) -> list[dict[str, Any]]:
        with self.connect() as conn:
            rows = conn.execute(
                """
                SELECT
                    t.*,
                    (
                        SELECT COUNT(*)
                        FROM blackboard_events e
                        WHERE e.task_id = t.id
                    ) AS event_count
                FROM tasks t
                ORDER BY t.id DESC
                LIMIT ?
                """,
                (limit,),
            ).fetchall()
            return [dict(row) for row in rows]

    def list_events(self, task_id: int) -> list[dict[str, Any]]:
        with self.connect() as conn:
            rows = conn.execute(
                """
                SELECT * FROM blackboard_events
                WHERE task_id = ?
                ORDER BY id ASC
                """,
                (task_id,),
            ).fetchall()
            return [dict(row) for row in rows]
