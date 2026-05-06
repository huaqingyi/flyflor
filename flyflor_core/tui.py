from __future__ import annotations

from pathlib import Path

from textual.app import App, ComposeResult
from textual.containers import Horizontal, Vertical
from textual.widgets import DataTable, Footer, Header, Static

from flyflor_core.cli import (
    DEFAULT_CORE_PORT,
    DEFAULT_QDRANT_HTTP_PORT,
    command_text,
    gateway_status_payload,
    status_payload,
    worker_statuses,
)


class FlyflorTui(App[None]):
    CSS = """
    Screen {
        background: #0f1419;
    }

    #summary {
        height: 8;
        padding: 1 2;
        border: solid #38bdf8;
        margin: 1;
    }

    #body {
        height: 1fr;
    }

    #steps {
        width: 34;
        margin: 0 0 1 1;
    }

    #workers {
        width: 2fr;
        margin: 0 1 1 1;
    }

    #checks {
        width: 1fr;
        margin: 0 1 1 0;
    }
    """

    BINDINGS = [
        ("r", "refresh", "Refresh"),
        ("q", "quit", "Quit"),
    ]

    def __init__(self, home: Path) -> None:
        super().__init__()
        self.home = home

    def compose(self) -> ComposeResult:
        yield Header(show_clock=True)
        yield Static(id="summary")
        with Horizontal(id="body"):
            with Vertical():
                yield DataTable(id="steps")
            with Vertical():
                yield DataTable(id="workers")
            with Vertical():
                yield DataTable(id="checks")
        yield Footer()

    def on_mount(self) -> None:
        self.title = "Flyflor"
        self.sub_title = str(self.home)
        self.refresh_data()

    def action_refresh(self) -> None:
        self.refresh_data()

    def refresh_data(self) -> None:
        self.render_summary()
        self.render_steps()
        self.render_workers()
        self.render_gateway_checks()

    def render_summary(self) -> None:
        payload = status_payload(self.home, deep=False)
        primary = payload.get("primary") if isinstance(payload.get("primary"), dict) else {}
        workers = payload.get("workers") if isinstance(payload.get("workers"), dict) else {}
        enabled = sum(1 for worker in workers.values() if isinstance(worker, dict) and worker.get("enabled"))
        available = sum(1 for worker in workers.values() if isinstance(worker, dict) and worker.get("available"))
        text = "\n".join(
            [
                f"Home: {payload['home']}",
                f"Initialized: {'yes' if payload['initialized'] else 'no'}",
                f"Primary: {primary.get('provider', '?')} / {primary.get('model', '?')}",
                f"Workers: {available}/{len(workers)} available, {enabled} enabled",
                "Next: configure missing or uninitialized workers, then start gateway.",
            ]
        )
        self.query_one("#summary", Static).update(text)

    def render_steps(self) -> None:
        table = self.query_one("#steps", DataTable)
        table.clear(columns=True)
        table.add_columns("Step", "Action")
        table.add_row("1", "Setup primary route")
        table.add_row("2", "Enable channels")
        table.add_row("3", "Install TUI tools")
        table.add_row("4", "Configure tool profiles")
        table.add_row("5", "Choose guardian pair")
        table.add_row("6", "Run gateway")

    def render_workers(self) -> None:
        table = self.query_one("#workers", DataTable)
        table.clear(columns=True)
        table.add_columns("Worker", "State", "Command", "Config entry", "Env profile")
        for name, item in worker_statuses(self.home).items():
            state = "enabled" if item.get("enabled") else "disabled"
            availability = "ok" if item.get("available") else "missing"
            table.add_row(
                name,
                f"{state}/{availability}",
                command_text(item.get("command")),
                command_text(item.get("configure")),
                f"{self.home}/agents/{profile_dir_name(name)}",
            )

    def render_gateway_checks(self) -> None:
        table = self.query_one("#checks", DataTable)
        table.clear(columns=True)
        table.add_columns("Service", "State", "Endpoint")
        payload = gateway_status_payload(
            home=self.home,
            core_port=DEFAULT_CORE_PORT,
            qdrant_http_port=DEFAULT_QDRANT_HTTP_PORT,
            nanobot_port=None,
        )
        checks = payload.get("checks", {})
        if not isinstance(checks, dict):
            return
        for name, check in checks.items():
            if not isinstance(check, dict):
                continue
            table.add_row(str(name), "ok" if check.get("ok") else "down", str(check.get("url") or ""))


def run(home: Path) -> None:
    FlyflorTui(home).run()


def profile_dir_name(worker_name: str) -> str:
    return {
        "qwen-code": "qwen",
        "deepseek-tui": "deepseek",
    }.get(worker_name, worker_name)
