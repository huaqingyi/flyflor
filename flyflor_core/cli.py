from __future__ import annotations

import argparse
import getpass
import importlib.metadata
import json
import os
import platform
import shutil
import signal
import subprocess
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

try:
    from rich import box
    from rich.console import Console
    from rich.panel import Panel
    from rich.progress import BarColumn, Progress, SpinnerColumn, TextColumn, TimeElapsedColumn
    from rich.table import Table
    from rich.text import Text
except ImportError:  # pragma: no cover - exercised only before dependencies are installed.
    box = None
    Console = None
    Panel = None
    Progress = None
    SpinnerColumn = None
    BarColumn = None
    TextColumn = None
    TimeElapsedColumn = None
    Table = None
    Text = None

from flyflor_core.setup import (
    CHANNEL_ORDER,
    CHANNEL_PRESETS,
    WORKER_ORDER,
    default_flyflor_config,
    flyflor_config_path,
    is_initialized,
    load_flyflor_config,
    merge_user_setup,
    save_flyflor_config,
)
from flyflor_core.bridge_daemon import main as bridge_daemon_main


DEFAULT_CORE_PORT = 8080
DEFAULT_QDRANT_HTTP_PORT = 6333
DEFAULT_QDRANT_GRPC_PORT = 6334
DEFAULT_WEBSOCKET_PORT = 8765

RICH_AVAILABLE = Console is not None
console = Console() if Console is not None else None


def main(argv: list[str] | None = None) -> None:
    argv = list(sys.argv[1:] if argv is None else argv)
    internal_command_index = next((index for index, item in enumerate(argv) if item == "internal-nanobot"), None)
    if internal_command_index is not None:
        home = Path(os.getenv("FLYFLOR_HOME", "~/.flyflor")).expanduser().resolve()
        if "--home" in argv:
            index = argv.index("--home")
            if index + 1 < len(argv):
                home = Path(argv[index + 1]).expanduser().resolve()
        elif any(item.startswith("--home=") for item in argv):
            value = next(item.split("=", 1)[1] for item in argv if item.startswith("--home="))
            home = Path(value).expanduser().resolve()
        run_internal_nanobot(home, argv[internal_command_index + 1 :])
        return

    parser = argparse.ArgumentParser(
        prog="flyflor",
        description="Flyflor CLI for setup, gateway runtime, diagnostics, logs, and isolated worker tools.",
    )
    parser.add_argument("--home", default=os.getenv("FLYFLOR_HOME", "~/.flyflor"), help="Flyflor home directory")
    parser.add_argument("--version", "-V", action="version", version=f"flyflor {flyflor_version()}")
    subparsers = parser.add_subparsers(dest="command")

    setup_parser = subparsers.add_parser("setup", help="Initialize Flyflor model, channels, and workers")
    setup_parser.add_argument("--defaults", action="store_true", help="Use defaults without prompts; refreshes existing setup safely")
    setup_parser.add_argument("--force", action="store_true", help="Reset setup from built-in defaults before writing")

    gateway_parser = subparsers.add_parser("gateway", help="Start Flyflor gateway on this machine")
    gateway_parser.add_argument(
        "gateway_action",
        nargs="?",
        choices=["run", "status"],
        default="run",
        help="Gateway action. `run` is the foreground gateway, `status` checks local endpoints.",
    )
    gateway_parser.add_argument("--core-port", type=int, default=DEFAULT_CORE_PORT)
    gateway_parser.add_argument("--qdrant-http-port", type=int, default=DEFAULT_QDRANT_HTTP_PORT)
    gateway_parser.add_argument("--qdrant-grpc-port", type=int, default=DEFAULT_QDRANT_GRPC_PORT)
    gateway_parser.add_argument("--websocket-port", type=int, default=DEFAULT_WEBSOCKET_PORT)
    gateway_parser.add_argument("--nanobot-port", type=int, default=None, help="nanobot health port")
    gateway_parser.add_argument("--skip-qdrant", action="store_true", help="Do not start local qdrant")
    gateway_parser.add_argument("--json", action="store_true", help="Print machine-readable status for `gateway status`")

    subparsers.add_parser("init", help="Alias for `setup --defaults`")
    subparsers.add_parser("bridge-daemon", help="Run the Flyflor bridge task consumer")

    workers_parser = subparsers.add_parser("workers", help="List, inspect, or install configured Flyflor CLI workers")
    workers_parser.add_argument(
        "workers_action",
        nargs="?",
        choices=["list", "status", "install", "init", "config", "configure", "run"],
        default="list",
    )
    workers_parser.add_argument("worker_name", nargs="?", help="Worker name for `workers install`")
    workers_parser.add_argument("--all", action="store_true", help="Apply `workers init` to all enabled workers with init commands")
    workers_parser.add_argument("--dry-run", action="store_true", help="Print the init/install command without running it")
    workers_parser.add_argument("--json", action="store_true", help="Print machine-readable worker data")

    status_parser = subparsers.add_parser("status", help="Show Flyflor setup, runtime, and worker status")
    status_parser.add_argument("--deep", action="store_true", help="Probe local HTTP endpoints")
    status_parser.add_argument("--json", action="store_true", help="Print machine-readable status")

    doctor_parser = subparsers.add_parser("doctor", help="Diagnose Flyflor configuration and dependencies")
    doctor_parser.add_argument("--fix", action="store_true", help="Repair layout/config files where it is safe")
    doctor_parser.add_argument("--json", action="store_true", help="Print machine-readable diagnostics")

    config_parser = subparsers.add_parser("config", help="Show, edit, and check Flyflor config files")
    config_parser.add_argument(
        "config_action",
        nargs="?",
        choices=["show", "path", "nanobot-path", "edit", "check"],
        default="show",
    )
    config_parser.add_argument("--json", action="store_true", help="Print raw JSON for `config show`")

    logs_parser = subparsers.add_parser("logs", help="View Flyflor logs")
    logs_parser.add_argument("log_name", nargs="?", default="gateway", help="Log name, or `list`")
    logs_parser.add_argument("-n", "--lines", type=int, default=80, help="Number of lines to show")
    logs_parser.add_argument("-f", "--follow", action="store_true", help="Follow the log like tail -f")

    run_parser = subparsers.add_parser("run", help="Run any configured worker with Flyflor isolation")
    run_parser.add_argument("worker_name", help="Configured worker name, for example codex, claude, opencode")

    args, passthrough = parser.parse_known_args(argv)
    home = Path(args.home).expanduser().resolve()

    if args.command is None:
        run_textual_dashboard(home)
        return

    if args.command == "init":
        run_setup(home, defaults=True, force=False)
        return

    if args.command == "setup":
        run_setup(home, defaults=args.defaults, force=args.force)
        return

    if args.command == "gateway":
        require_initialized(home)
        if args.gateway_action == "status":
            print_gateway_status(
                home=home,
                core_port=args.core_port,
                qdrant_http_port=args.qdrant_http_port,
                nanobot_port=args.nanobot_port,
                as_json=args.json,
            )
        else:
            run_gateway(
                home=home,
                core_port=args.core_port,
                qdrant_http_port=args.qdrant_http_port,
                qdrant_grpc_port=args.qdrant_grpc_port,
                websocket_port=args.websocket_port,
                nanobot_port=args.nanobot_port,
                skip_qdrant=args.skip_qdrant,
            )
        return

    if args.command == "bridge-daemon":
        require_initialized(home)
        os.environ.setdefault("FLYFLOR_HOME", str(home))
        os.environ.setdefault("FLYFLOR_DATABASE", str(home / "flyflor.db"))
        bridge_daemon_main()
        return

    if args.command == "workers":
        require_initialized(home)
        handle_workers(
            home,
            action=args.workers_action,
            name=args.worker_name,
            all_workers=args.all,
            dry_run=args.dry_run,
            as_json=args.json,
        )
        return

    if args.command == "status":
        print_status(home, deep=args.deep, as_json=args.json)
        return

    if args.command == "doctor":
        run_doctor(home, fix=args.fix, as_json=args.json)
        return

    if args.command == "config":
        handle_config(home, action=args.config_action, as_json=args.json)
        return

    if args.command == "logs":
        handle_logs(home, log_name=args.log_name, lines=args.lines, follow=args.follow)
        return

    if args.command == "run":
        require_initialized(home)
        run_configured_worker(home, args.worker_name, passthrough)
        return

    parser.print_help()


def run_setup(home: Path, *, defaults: bool, force: bool) -> None:
    ensure_layout(home, write_defaults=False)
    setup_path = flyflor_config_path(home)
    existing = setup_path.exists() and is_initialized(home)
    generated = default_flyflor_config(core_port=DEFAULT_CORE_PORT, websocket_port=DEFAULT_WEBSOCKET_PORT)

    if force:
        config = generated
    elif setup_path.exists():
        config = merge_user_setup(load_flyflor_config(home), generated)
    else:
        config = generated

    if existing and defaults and not force:
        save_flyflor_config(home, config)
        write_nanobot_config_from_setup(home, config)
        rich_print(
            f"[green]Refreshed Flyflor setup at {home}[/]\n"
            f"setup config: {setup_path}\n"
            f"nanobot config: {nanobot_config_path(home)}"
            if console is not None
            else f"Refreshed Flyflor setup at {home}\n"
            f"setup config: {setup_path}\n"
            f"nanobot config: {nanobot_config_path(home)}"
        )
        return

    if not defaults:
        if not sys.stdin.isatty():
            print("`flyflor setup` needs an interactive terminal.", file=sys.stderr)
            if existing:
                print("Use `flyflor setup --defaults` to refresh existing setup non-interactively.", file=sys.stderr)
                print("Use `flyflor setup --force --defaults` to reset to built-in defaults.", file=sys.stderr)
            else:
                print("Use `flyflor setup --defaults` for non-interactive initialization.", file=sys.stderr)
            raise SystemExit(2)
        if existing:
            print_header("Reconfigure Setup", "Press Enter to keep existing values, or type a new value.")
        config = prompt_setup(config)

    save_flyflor_config(home, config)
    write_nanobot_config_from_setup(home, config)
    if existing and force:
        action = "Reset Flyflor setup"
    elif existing:
        action = "Updated Flyflor setup"
    else:
        action = "Initialized Flyflor"
    print(f"{action} at {home}")
    print(f"setup config: {setup_path}")
    print(f"nanobot config: {nanobot_config_path(home)}")


def require_initialized(home: Path) -> None:
    if is_initialized(home):
        ensure_layout(home, write_defaults=False)
        migrate_setup_config(home)
        return
    if os.getenv("FLYFLOR_AUTO_INIT") == "1":
        run_setup(home, defaults=True, force=False)
        return
    print(f"Flyflor is not initialized at {home}", file=sys.stderr)
    print("Run `flyflor setup` first.", file=sys.stderr)
    raise SystemExit(2)


def migrate_setup_config(home: Path) -> None:
    current = load_flyflor_config(home)
    generated = default_flyflor_config(core_port=DEFAULT_CORE_PORT, websocket_port=DEFAULT_WEBSOCKET_PORT)
    merged = merge_user_setup(current, generated)
    if merged != current:
        save_flyflor_config(home, merged)
        write_nanobot_config_from_setup(home, merged)


def prompt_setup(config: dict[str, object]) -> dict[str, object]:
    print("Flyflor setup")
    print("This initializes the top-level gateway, channel bindings, and isolated CLI/Agent workers.")
    print()

    primary = config["primary"]
    assert isinstance(primary, dict)
    print("Primary route")
    print("  Keep these defaults unless you are replacing Flyflor Core's internal OpenAI-compatible endpoint.")
    primary["provider"] = prompt("Nanobot provider key", str(primary.get("provider", "vllm")))
    primary["model"] = prompt("Nanobot model name", str(primary.get("model", "flyflor-placeholder")))
    primary["apiBase"] = prompt("Nanobot API base", str(primary.get("apiBase", f"http://127.0.0.1:{DEFAULT_CORE_PORT}/v1")))
    api_key = prompt_secret("Nanobot API key", optional_text(primary.get("apiKey")))
    primary["apiKey"] = api_key or None
    print()

    channels = config["channels"]
    assert isinstance(channels, dict)

    print("Channels")
    print_choice_help(CHANNEL_ORDER, CHANNEL_PRESETS)
    default_channels = ",".join(name for name in CHANNEL_ORDER if is_enabled_channel(channels.get(name)))
    selected_channels = prompt_multi("Enable channels", default_channels or "websocket", CHANNEL_ORDER)
    for name in CHANNEL_ORDER:
        value = channels.setdefault(name, {"enabled": False})
        if not isinstance(value, dict):
            value = {"enabled": False}
            channels[name] = value
        value["enabled"] = name in selected_channels
        if value["enabled"]:
            prompt_channel(name, value)
    print()

    workers = config["workers"]
    assert isinstance(workers, dict)

    print("CLI/Agent workers")
    print_worker_help(workers)
    ordered_worker_names = [name for name in WORKER_ORDER if name in workers] + [
        name for name in workers if name not in WORKER_ORDER
    ]
    default_workers = ",".join(name for name in ordered_worker_names if is_enabled_worker(workers.get(name)))
    selected_workers = prompt_multi("Enable workers", default_workers or "codex,claude", ordered_worker_names)
    for name in ordered_worker_names:
        value = workers[name]
        if not isinstance(value, dict):
            continue
        value["enabled"] = name in selected_workers
        if value["enabled"]:
            command = value.get("command", [])
            command_default = " ".join(str(part) for part in command) if isinstance(command, list) else str(command)
            value["kind"] = prompt("Worker kind cli/agent", str(value.get("kind", "cli")))
            value["command"] = prompt_command("Worker command", command_default)

    while prompt_bool("Add custom worker", False):
        name = prompt("Worker name", "custom-worker").strip()
        command = prompt("Worker command", name).strip()
        if name and command:
            workers[name] = {
                "enabled": True,
                "kind": "cli",
                "command": command.split(),
                "env": {},
                "description": "User configured worker",
            }
            selected_workers.add(name)

    configure_bridge_pair(config, selected_workers)
    return config


def configure_bridge_pair(config: dict[str, object], selected_workers: set[str]) -> None:
    workers = config.get("workers", {})
    if not isinstance(workers, dict):
        workers = {}
    enabled = [
        name
        for name, value in workers.items()
        if name in selected_workers and isinstance(value, dict) and value.get("enabled")
    ]
    if len(enabled) < 2:
        print("Bridge guardian pair needs at least two enabled workers; keeping current/default pair.")
        return

    bridge = config.setdefault("bridge", {})
    if not isinstance(bridge, dict):
        bridge = {}
        config["bridge"] = bridge
    pair = bridge.setdefault("guardianPair", {})
    if not isinstance(pair, dict):
        pair = {}
        bridge["guardianPair"] = pair

    print()
    print("Guardian discussion pair")
    print("  Choose the two CLI/Agent workers that will mutually review high-risk tasks.")
    print(f"  Enabled workers: {', '.join(enabled)}")
    left_default = str(pair.get("left") if pair.get("left") in enabled else enabled[0])
    right_default = str(pair.get("right") if pair.get("right") in enabled and pair.get("right") != left_default else enabled[1])
    pair["left"] = prompt_choice("Left guardian", left_default, enabled)
    right_options = [name for name in enabled if name != pair["left"]]
    pair["right"] = prompt_choice("Right guardian", right_default if right_default in right_options else right_options[0], right_options)
    bridge["mode"] = prompt("Bridge mode", str(bridge.get("mode", "guardian_pair")))
    bridge["autoSelect"] = prompt_bool("Let Flyflor auto-select pair later", bool(bridge.get("autoSelect", False)))


def print_choice_help(names: list[str], presets: dict[str, dict[str, object]]) -> None:
    for name in names:
        preset = presets.get(name, {})
        label = preset.get("label", name)
        description = preset.get("description", "")
        print(f"  {name:10} {label} - {description}")


def print_worker_help(workers: dict[str, object]) -> None:
    ordered = [name for name in WORKER_ORDER if name in workers] + [name for name in workers if name not in WORKER_ORDER]
    for name in ordered:
        value = workers.get(name)
        if not isinstance(value, dict):
            continue
        description = value.get("description", "")
        command = value.get("command", [])
        command_text = " ".join(str(part) for part in command) if isinstance(command, list) else str(command)
        print(f"  {name:14} {command_text:24} {description}")


def prompt_channel(name: str, value: dict[str, object]) -> None:
    preset = CHANNEL_PRESETS.get(name, {})
    label = preset.get("label", name)
    print(f"Configure {label}")
    for field in preset.get("fields", []):
        if not isinstance(field, dict):
            continue
        field_name = str(field["name"])
        if field_name == "token" and name == "websocket" and not value.get("websocketRequiresToken"):
            value[field_name] = value.get(field_name, field.get("default", ""))
            continue
        default = value.get(field_name, field.get("default", ""))
        value[field_name] = prompt_field(field, default)


def prompt_field(field: dict[str, object], default: object) -> object:
    label = str(field.get("label", field.get("name", "Value")))
    kind = str(field.get("type", "str"))
    if kind == "bool":
        return prompt_bool(label, bool(default))
    if kind == "int":
        return prompt_int(label, default)
    if kind == "csv":
        return prompt_csv(label, default)
    if field.get("secret"):
        return prompt_secret(label, optional_text(default))
    return prompt(label, optional_text(default))


def prompt(label: str, default: str) -> str:
    value = input(f"{label} [{default}]: ").strip()
    return value or default


def prompt_secret(label: str, default: str) -> str:
    suffix = "set" if default else "empty"
    try:
        value = getpass.getpass(f"{label} [{suffix}, press Enter to keep]: ").strip()
    except (EOFError, getpass.GetPassWarning):
        value = input(f"{label} [{suffix}, press Enter to keep]: ").strip()
    return value or default


def prompt_bool(label: str, default: bool) -> bool:
    suffix = "Y/n" if default else "y/N"
    value = input(f"{label} [{suffix}]: ").strip().lower()
    if not value:
        return default
    return value in ("y", "yes", "1", "true", "on")


def prompt_int(label: str, default: object) -> int:
    while True:
        value = prompt(label, optional_text(default))
        try:
            return int(value)
        except ValueError:
            print("Please enter an integer.")


def prompt_csv(label: str, default: object) -> list[str]:
    default_items = default if isinstance(default, list) else [str(default)] if default else []
    raw = prompt(label + " comma list", ",".join(str(item) for item in default_items))
    return [item.strip() for item in raw.split(",") if item.strip()]


def prompt_command(label: str, default: str) -> list[str]:
    raw = prompt(label, default)
    return raw.split()


def prompt_multi(label: str, default: str, allowed: list[str]) -> set[str]:
    allowed_set = set(allowed)
    while True:
        raw = prompt(label + " comma list", default).strip()
        if not raw:
            return set()
        selected = {item.strip() for item in raw.split(",") if item.strip()}
        unknown = sorted(selected - allowed_set)
        if not unknown:
            return selected
        print(f"Unknown choices: {', '.join(unknown)}")


def prompt_choice(label: str, default: str, allowed: list[str]) -> str:
    allowed_set = set(allowed)
    while True:
        value = prompt(label, default).strip()
        if value in allowed_set:
            return value
        print(f"Choose one of: {', '.join(allowed)}")


def optional_text(value: object) -> str:
    if value is None:
        return ""
    if isinstance(value, list):
        return ",".join(str(item) for item in value)
    return str(value)


def is_enabled_channel(value: object) -> bool:
    return isinstance(value, dict) and bool(value.get("enabled"))


def is_enabled_worker(value: object) -> bool:
    return isinstance(value, dict) and bool(value.get("enabled"))


def flyflor_version() -> str:
    try:
        return importlib.metadata.version("flyflor-core")
    except importlib.metadata.PackageNotFoundError:
        return "0.1.0"


def rich_print(*objects: object) -> None:
    if console is not None:
        console.print(*objects)
    else:
        print(*objects)


def print_header(title: str, subtitle: str | None = None) -> None:
    if console is None or Panel is None or Text is None:
        print(title)
        if subtitle:
            print(subtitle)
        return
    text = Text(title, style="bold cyan")
    if subtitle:
        text.append("\n")
        text.append(subtitle, style="dim")
    console.print(Panel.fit(text, title="[bold magenta]Flyflor[/]", border_style="cyan"))


def run_with_progress(command: list[str], *, env: dict[str, str], label: str) -> int:
    if console is None or Progress is None or SpinnerColumn is None or TextColumn is None:
        return subprocess.call(command, env=env)
    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        BarColumn(),
        TimeElapsedColumn(),
        console=console,
        transient=True,
    ) as progress:
        task = progress.add_task(label, total=None)
        process = subprocess.Popen(command, env=env)
        while process.poll() is None:
            time.sleep(0.1)
        progress.update(task, completed=1)
        return process.returncode or 0


def load_setup_or_empty(home: Path) -> dict[str, object]:
    try:
        return load_flyflor_config(home)
    except (FileNotFoundError, json.JSONDecodeError):
        return {}


def redact_config(value: object) -> object:
    if isinstance(value, dict):
        redacted: dict[str, object] = {}
        for key, item in value.items():
            lowered = key.lower()
            is_secret = any(secret in lowered for secret in ("key", "secret", "token", "password"))
            if is_secret and not isinstance(item, bool):
                redacted[key] = "set" if item else None
            else:
                redacted[key] = redact_config(item)
        return redacted
    if isinstance(value, list):
        return [redact_config(item) for item in value]
    return value


def merged_setup(home: Path) -> dict[str, object]:
    generated = default_flyflor_config(core_port=DEFAULT_CORE_PORT, websocket_port=DEFAULT_WEBSOCKET_PORT)
    current = load_setup_or_empty(home)
    if current:
        return merge_user_setup(current, generated)
    return generated


def print_json(data: object) -> None:
    print(json.dumps(data, ensure_ascii=False, indent=2))


def command_text(command: object) -> str:
    if isinstance(command, list):
        return " ".join(str(part) for part in command)
    return str(command or "")


def normalize_command(command: object, fallback: str) -> list[str]:
    if isinstance(command, str):
        command = command.split()
    if not isinstance(command, list) or not command:
        command = [fallback]
    return [str(part) for part in command if str(part)]


def detect_version(executable: str) -> str | None:
    for args in ([executable, "--version"], [executable, "version"]):
        try:
            result = subprocess.run(args, text=True, capture_output=True, timeout=5)
        except (OSError, subprocess.TimeoutExpired):
            continue
        text = ((result.stdout or "") + "\n" + (result.stderr or "")).strip()
        if result.returncode == 0 and text:
            return text.splitlines()[0][:160]
    return None


def detect_worker(name: str, worker: dict[str, object]) -> dict[str, object]:
    command = normalize_command(worker.get("command"), name)
    executable = shutil.which(command[0])
    install = worker.get("install")
    if isinstance(install, str):
        install = install.split()
    if not isinstance(install, list):
        install = []
    init = worker.get("init")
    if isinstance(init, str):
        init = init.split()
    if not isinstance(init, list):
        init = []
    configure = worker.get("configure")
    if isinstance(configure, str):
        configure = configure.split()
    if not isinstance(configure, list):
        configure = []
    return {
        "name": name,
        "enabled": bool(worker.get("enabled")),
        "kind": str(worker.get("kind") or "cli"),
        "command": command,
        "available": executable is not None,
        "path": executable,
        "version": detect_version(command[0]) if executable else None,
        "install": [str(part) for part in install],
        "init": [str(part) for part in init],
        "configure": [str(part) for part in configure],
        "needs_init": bool(init),
        "description": str(worker.get("description") or ""),
    }


def worker_statuses(home: Path) -> dict[str, dict[str, object]]:
    config = merged_setup(home)
    workers = config.get("workers", {})
    if not isinstance(workers, dict):
        return {}
    result: dict[str, dict[str, object]] = {}
    for name, value in workers.items():
        if not isinstance(value, dict):
            continue
        result[str(name)] = detect_worker(str(name), value)
    return result


def print_workers(home: Path, *, as_json: bool = False) -> None:
    statuses = worker_statuses(home)
    if as_json:
        print_json({"workers": statuses, "toolProfiles": merged_setup(home).get("toolProfiles", {})})
        return
    if not statuses:
        print("No workers configured.")
        return
    if console is not None and Table is not None:
        print_header("Worker Inventory", "Configured CLI/TUI tools, availability, and initialization commands")
        table = Table(box=getattr(box, "SIMPLE_HEAVY", None) if box is not None else None)
        table.add_column("Worker", style="bold")
        table.add_column("State")
        table.add_column("Command")
        table.add_column("Init")
        table.add_column("Version", overflow="fold")
        for name, value in statuses.items():
            enabled = "[green]enabled[/]" if value["enabled"] else "[dim]disabled[/]"
            available = "[green]ok[/]" if value["available"] else "[yellow]missing[/]"
            init = command_text(value.get("init")) or "[dim]-[/]"
            version = str(value.get("version") or "")
            table.add_row(name, f"{enabled}\n{available}", command_text(value["command"]), init, version)
        console.print(table)
        return
    for name, value in statuses.items():
        enabled = "enabled" if value["enabled"] else "disabled"
        available = "ok" if value["available"] else "missing"
        init = "init" if value.get("init") else "-"
        print(f"{name:14} {enabled:8} {available:8} {init:5} {command_text(value['command'])}")


def handle_workers(
    home: Path,
    *,
    action: str,
    name: str | None,
    all_workers: bool,
    dry_run: bool,
    as_json: bool,
) -> None:
    if action == "list":
        print_workers(home, as_json=as_json)
        return
    if action == "status":
        print_workers(home, as_json=as_json)
        return
    if action == "install":
        if not name:
            print("worker name is required for `flyflor workers install`", file=sys.stderr)
            raise SystemExit(2)
        install_worker(home, name, dry_run=dry_run)
        return
    if action == "init":
        init_workers(home, name=name, all_workers=all_workers, dry_run=dry_run)
        return
    if action in ("config", "configure"):
        configure_worker(home, name=name, dry_run=dry_run)
        return
    if action == "run":
        if not name:
            print("worker name is required for `flyflor workers run`", file=sys.stderr)
            raise SystemExit(2)
        run_configured_worker(home, name, [])
        return
    raise SystemExit(2)


def install_worker(home: Path, name: str, *, dry_run: bool) -> None:
    config = merged_setup(home)
    workers = config.get("workers", {})
    worker = workers.get(name) if isinstance(workers, dict) else None
    if not isinstance(worker, dict):
        print(f"Unknown worker: {name}", file=sys.stderr)
        raise SystemExit(2)
    install = worker.get("install")
    if isinstance(install, str):
        install = install.split()
    if not isinstance(install, list) or not install:
        print(f"Worker `{name}` has no install command.", file=sys.stderr)
        raise SystemExit(2)
    command = [str(part) for part in install]
    rich_print(f"[bold]Installing {name}:[/] {' '.join(command)}" if console is not None else f"Installing {name}: {' '.join(command)}")
    if dry_run:
        return
    raise SystemExit(run_with_progress(command, env=os.environ.copy(), label=f"Installing {name}"))


def worker_command_field(worker: dict[str, object], field: str) -> list[str]:
    command = worker.get(field)
    if isinstance(command, str):
        command = command.split()
    if isinstance(command, list) and command:
        return [str(part) for part in command]
    return []


def worker_env(home: Path, worker: dict[str, object]) -> dict[str, str]:
    env = runtime_env(home)
    worker_name = str(worker.get("name") or "")
    if worker_name:
        env.update(tool_profile_env(home, worker_name=worker_name))
    configured = worker.get("env")
    if isinstance(configured, dict):
        for key, value in configured.items():
            if isinstance(key, str) and isinstance(value, str):
                env[key] = value.format(home=str(home))
    return env


def tool_profile_env(home: Path, *, worker_name: str | None = None) -> dict[str, str]:
    env: dict[str, str] = {}
    setup = merged_setup(home)
    profiles = setup.get("toolProfiles", {})
    if not isinstance(profiles, dict):
        profiles = {}
    base = profiles.get("base", {})
    if isinstance(base, dict):
        mapping = {
            "HOME": base.get("home"),
            "XDG_CONFIG_HOME": base.get("xdgConfig"),
            "XDG_CACHE_HOME": base.get("xdgCache"),
            "XDG_DATA_HOME": base.get("xdgData"),
            "TMPDIR": base.get("tmp"),
        }
        for key, value in mapping.items():
            if isinstance(value, str) and value:
                env[key] = value.format(home=str(home))
    if worker_name:
        profile = profiles.get(worker_name, {})
        if isinstance(profile, dict):
            configured = profile.get("env")
            if isinstance(configured, dict):
                for key, value in configured.items():
                    if isinstance(key, str) and isinstance(value, str):
                        env[key] = value.format(home=str(home))
    return env


def run_configured_worker(home: Path, name: str, passthrough: list[str]) -> None:
    config = merged_setup(home)
    workers = config.get("workers", {})
    worker = workers.get(name) if isinstance(workers, dict) else None
    if not isinstance(worker, dict):
        print(f"Unknown worker: {name}", file=sys.stderr)
        print("Run `flyflor workers` to list configured workers.", file=sys.stderr)
        raise SystemExit(2)
    command = normalize_command(worker.get("command"), name)
    worker["name"] = name
    executable = shutil.which(command[0])
    if executable is None:
        install = command_text(worker.get("install"))
        print(f"Worker `{name}` command not found: {command[0]}", file=sys.stderr)
        if install:
            print(f"Install it with: flyflor workers install {name}", file=sys.stderr)
        raise SystemExit(127)
    raise SystemExit(subprocess.call([*command, *passthrough], env=worker_env(home, worker)))


def configure_worker(home: Path, *, name: str | None, dry_run: bool) -> None:
    if not name:
        print("worker name is required for `flyflor workers config`", file=sys.stderr)
        raise SystemExit(2)
    config = merged_setup(home)
    workers = config.get("workers", {})
    worker = workers.get(name) if isinstance(workers, dict) else None
    if not isinstance(worker, dict):
        print(f"Unknown worker: {name}", file=sys.stderr)
        raise SystemExit(2)
    command = worker_command_field(worker, "configure") or worker_command_field(worker, "init")
    worker["name"] = name
    if not command:
        print(f"Worker `{name}` has no configure/init command.", file=sys.stderr)
        raise SystemExit(2)
    rich_print(
        f"[bold cyan]Configuring {name}:[/] {' '.join(command)}"
        if console is not None
        else f"Configuring {name}: {' '.join(command)}"
    )
    if dry_run:
        return
    if not sys.stdin.isatty():
        print("`flyflor workers config` runs an interactive TUI/auth command and needs a terminal.", file=sys.stderr)
        raise SystemExit(2)
    if shutil.which(command[0]) is None:
        print(f"Worker `{name}` command not found: {command[0]}", file=sys.stderr)
        print(f"Install it with: flyflor workers install {name}", file=sys.stderr)
        raise SystemExit(127)
    raise SystemExit(subprocess.call(command, env=worker_env(home, worker)))


def init_workers(home: Path, *, name: str | None, all_workers: bool, dry_run: bool) -> None:
    config = merged_setup(home)
    workers = config.get("workers", {})
    if not isinstance(workers, dict):
        print("No workers configured.", file=sys.stderr)
        raise SystemExit(2)

    targets: list[tuple[str, dict[str, object]]] = []
    if all_workers:
        for worker_name, worker in workers.items():
            if isinstance(worker, dict) and worker.get("enabled") and worker.get("init"):
                targets.append((str(worker_name), worker))
    else:
        if not name:
            print("worker name is required for `flyflor workers init`, or pass --all", file=sys.stderr)
            raise SystemExit(2)
        worker = workers.get(name)
        if not isinstance(worker, dict):
            print(f"Unknown worker: {name}", file=sys.stderr)
            raise SystemExit(2)
        targets.append((name, worker))

    if not targets:
        print("No enabled workers with init commands.")
        return
    if not dry_run and not sys.stdin.isatty():
        print("`flyflor workers init` runs interactive TUI/auth commands and needs a terminal.", file=sys.stderr)
        print("Use `--dry-run` to inspect commands in non-interactive environments.", file=sys.stderr)
        raise SystemExit(2)

    for worker_name, worker in targets:
        init = worker.get("init")
        if isinstance(init, str):
            init = init.split()
        if not isinstance(init, list) or not init:
            print(f"Worker `{worker_name}` has no init command.")
            continue
        command = [str(part) for part in init]
        worker["name"] = worker_name
        rich_print(
            f"[bold cyan]Initializing {worker_name}:[/] {' '.join(command)}"
            if console is not None
            else f"Initializing {worker_name}: {' '.join(command)}"
        )
        executable = shutil.which(command[0])
        if executable is None:
            print(f"Worker `{worker_name}` command not found: {command[0]}", file=sys.stderr)
            if dry_run:
                continue
            raise SystemExit(127)
        if dry_run:
            continue
        code = subprocess.call(command, env=worker_env(home, worker))
        if code != 0:
            raise SystemExit(code)


def probe_http(url: str, *, timeout: float = 1.5) -> dict[str, object]:
    try:
        with urllib.request.urlopen(url, timeout=timeout) as response:
            body = response.read(160).decode("utf-8", errors="replace")
            return {"ok": True, "status": response.status, "url": url, "sample": body.strip()}
    except urllib.error.HTTPError as exc:
        return {"ok": False, "status": exc.code, "url": url, "error": str(exc)}
    except OSError as exc:
        return {"ok": False, "url": url, "error": str(exc)}


def gateway_status_payload(
    *,
    home: Path,
    core_port: int,
    qdrant_http_port: int,
    nanobot_port: int | None,
) -> dict[str, object]:
    checks: dict[str, object] = {
        "core": probe_http(f"http://127.0.0.1:{core_port}/health"),
        "qdrant": probe_http(f"http://127.0.0.1:{qdrant_http_port}/"),
    }
    if nanobot_port is not None:
        checks["nanobot"] = probe_http(f"http://127.0.0.1:{nanobot_port}/health")
    return {
        "home": str(home),
        "logs": str(home / "logs"),
        "config": str(flyflor_config_path(home)),
        "nanobot_config": str(nanobot_config_path(home)),
        "checks": checks,
    }


def print_gateway_status(
    *,
    home: Path,
    core_port: int,
    qdrant_http_port: int,
    nanobot_port: int | None,
    as_json: bool,
) -> None:
    payload = gateway_status_payload(
        home=home,
        core_port=core_port,
        qdrant_http_port=qdrant_http_port,
        nanobot_port=nanobot_port,
    )
    if as_json:
        print_json(payload)
        return
    if console is not None and Table is not None:
        print_header("Gateway Status", f"home: {payload['home']}")
        table = Table(box=getattr(box, "SIMPLE", None) if box is not None else None)
        table.add_column("Service", style="bold")
        table.add_column("State")
        table.add_column("Endpoint")
        table.add_column("Detail", overflow="fold")
        checks = payload["checks"]
        assert isinstance(checks, dict)
        for name, check in checks.items():
            assert isinstance(check, dict)
            state = "[green]ok[/]" if check.get("ok") else "[yellow]down[/]"
            table.add_row(str(name), state, str(check.get("url") or ""), str(check.get("error") or check.get("sample") or ""))
        console.print(table)
        return
    print("Flyflor gateway status")
    print(f"  home: {payload['home']}")
    checks = payload["checks"]
    assert isinstance(checks, dict)
    for name, check in checks.items():
        assert isinstance(check, dict)
        state = "ok" if check.get("ok") else "down"
        detail = check.get("url")
        if check.get("error"):
            detail = f"{detail} ({check['error']})"
        print(f"  {name:8} {state:5} {detail}")


def status_payload(home: Path, *, deep: bool) -> dict[str, object]:
    initialized = is_initialized(home)
    setup = load_setup_or_empty(home)
    primary = setup.get("primary", {}) if isinstance(setup, dict) else {}
    bridge = setup.get("bridge", {}) if isinstance(setup, dict) else {}
    payload: dict[str, object] = {
        "version": flyflor_version(),
        "python": platform.python_version(),
        "platform": platform.platform(),
        "home": str(home),
        "initialized": initialized,
        "config": str(flyflor_config_path(home)),
        "config_exists": flyflor_config_path(home).exists(),
        "nanobot_config": str(nanobot_config_path(home)),
        "nanobot_config_exists": nanobot_config_path(home).exists(),
        "primary": redact_config(primary),
        "bridge": bridge,
        "workers": worker_statuses(home) if initialized else {},
    }
    if deep:
        payload["gateway"] = gateway_status_payload(
            home=home,
            core_port=DEFAULT_CORE_PORT,
            qdrant_http_port=DEFAULT_QDRANT_HTTP_PORT,
            nanobot_port=None,
        )
    return payload


def print_status(home: Path, *, deep: bool, as_json: bool) -> None:
    payload = status_payload(home, deep=deep)
    if as_json:
        print_json(payload)
        return
    if console is not None and Table is not None:
        print_header("System Status", f"{payload['platform']} · Python {payload['python']}")
        summary = Table(box=getattr(box, "SIMPLE", None) if box is not None else None)
        summary.add_column("Key", style="bold")
        summary.add_column("Value", overflow="fold")
        summary.add_row("version", str(payload["version"]))
        summary.add_row("home", str(payload["home"]))
        summary.add_row("initialized", "[green]yes[/]" if payload["initialized"] else "[yellow]no[/]")
        summary.add_row("config", f"{payload['config']} ({'exists' if payload['config_exists'] else 'missing'})")
        summary.add_row(
            "nanobot config",
            f"{payload['nanobot_config']} ({'exists' if payload['nanobot_config_exists'] else 'missing'})",
        )
        primary = payload.get("primary")
        if isinstance(primary, dict):
            summary.add_row("primary", f"{primary.get('provider', '?')} / {primary.get('model', '?')}")
        workers = payload.get("workers")
        if isinstance(workers, dict) and workers:
            available = sum(1 for worker in workers.values() if isinstance(worker, dict) and worker.get("available"))
            enabled = sum(1 for worker in workers.values() if isinstance(worker, dict) and worker.get("enabled"))
            summary.add_row("workers", f"{available}/{len(workers)} available, {enabled} enabled")
        console.print(summary)
        if deep and isinstance(payload.get("gateway"), dict):
            console.print()
            print_gateway_status(
                home=home,
                core_port=DEFAULT_CORE_PORT,
                qdrant_http_port=DEFAULT_QDRANT_HTTP_PORT,
                nanobot_port=None,
                as_json=False,
            )
        return
    print("Flyflor status")
    print(f"  version: {payload['version']}")
    print(f"  home: {payload['home']}")
    print(f"  initialized: {'yes' if payload['initialized'] else 'no'}")
    print(f"  config: {payload['config']} ({'exists' if payload['config_exists'] else 'missing'})")
    print(f"  nanobot config: {payload['nanobot_config']} ({'exists' if payload['nanobot_config_exists'] else 'missing'})")
    primary = payload.get("primary")
    if isinstance(primary, dict):
        print(f"  primary: {primary.get('provider', '?')} / {primary.get('model', '?')}")
    workers = payload.get("workers")
    if isinstance(workers, dict) and workers:
        available = sum(1 for worker in workers.values() if isinstance(worker, dict) and worker.get("available"))
        enabled = sum(1 for worker in workers.values() if isinstance(worker, dict) and worker.get("enabled"))
        print(f"  workers: {available}/{len(workers)} available, {enabled} enabled")
    if deep and isinstance(payload.get("gateway"), dict):
        print()
        print_gateway_status(
            home=home,
            core_port=DEFAULT_CORE_PORT,
            qdrant_http_port=DEFAULT_QDRANT_HTTP_PORT,
            nanobot_port=None,
            as_json=False,
        )


def run_doctor(home: Path, *, fix: bool, as_json: bool) -> None:
    checks: list[dict[str, object]] = []

    def add(name: str, ok: bool, detail: str, fixable: bool = False) -> None:
        checks.append({"name": name, "ok": ok, "detail": detail, "fixable": fixable})

    if fix:
        ensure_layout(home, write_defaults=False)
        if is_initialized(home):
            migrate_setup_config(home)

    add("python", sys.version_info >= (3, 11), platform.python_version())
    add("home", home.exists(), str(home), fixable=True)
    add("setup", is_initialized(home), str(flyflor_config_path(home)), fixable=False)
    add("nanobot command", shutil.which("nanobot") is not None, shutil.which("nanobot") or "not found")
    add("qdrant command", shutil.which("qdrant") is not None, shutil.which("qdrant") or "not found; gateway can run with --skip-qdrant")

    if is_initialized(home):
        setup = load_flyflor_config(home)
        if fix and not nanobot_config_path(home).exists():
            write_nanobot_config_from_setup(home, setup)
    add("nanobot config", nanobot_config_path(home).exists(), str(nanobot_config_path(home)), fixable=True)

    if is_initialized(home):
        for name, status in worker_statuses(home).items():
            if status.get("enabled"):
                add(f"worker:{name}", bool(status.get("available")), str(status.get("path") or command_text(status.get("command"))))
                add(
                    f"worker-init:{name}",
                    bool(status.get("init")),
                    command_text(status.get("init")) or "no init command configured",
                )

    ok = all(bool(check["ok"]) or check["name"] == "qdrant command" for check in checks)
    payload = {"ok": ok, "home": str(home), "checks": checks}
    if as_json:
        print_json(payload)
    else:
        if console is not None and Table is not None:
            print_header("Doctor", "Configuration, dependency, and worker initialization checks")
            table = Table(box=getattr(box, "SIMPLE", None) if box is not None else None)
            table.add_column("State")
            table.add_column("Check", style="bold")
            table.add_column("Detail", overflow="fold")
            for check in checks:
                state = "[green]ok[/]" if check["ok"] else "[red]fail[/]"
                table.add_row(state, str(check["name"]), str(check["detail"]))
            console.print(table)
        else:
            print("Flyflor doctor")
            for check in checks:
                state = "ok" if check["ok"] else "fail"
                print(f"  {state:4} {check['name']}: {check['detail']}")
        if not ok:
            print()
            rich_print(
                "[yellow]Run `flyflor setup` if setup is missing. Use `flyflor doctor --fix` for safe layout/config repairs.[/]"
                if console is not None
                else "Run `flyflor setup` if setup is missing. Use `flyflor doctor --fix` for safe layout/config repairs."
            )
    if not ok:
        raise SystemExit(1)


def handle_config(home: Path, *, action: str, as_json: bool) -> None:
    if action == "path":
        print(flyflor_config_path(home))
        return
    if action == "nanobot-path":
        print(nanobot_config_path(home))
        return
    if action == "edit":
        editor = os.getenv("VISUAL") or os.getenv("EDITOR")
        if not editor:
            print("Set VISUAL or EDITOR to edit config from the CLI.", file=sys.stderr)
            raise SystemExit(2)
        ensure_layout(home, write_defaults=False)
        raise SystemExit(subprocess.call([editor, str(flyflor_config_path(home))]))
    if action == "check":
        run_doctor(home, fix=False, as_json=as_json)
        return
    if action == "show":
        data = load_setup_or_empty(home)
        if as_json:
            print_json(data)
        else:
            print_json(redact_config(data))
        return
    raise SystemExit(2)


def handle_logs(home: Path, *, log_name: str, lines: int, follow: bool) -> None:
    logs_dir = home / "logs"
    if log_name == "list":
        if not logs_dir.exists():
            print(f"No log directory: {logs_dir}")
            return
        if console is not None and Table is not None:
            print_header("Logs", str(logs_dir))
            table = Table(box=getattr(box, "SIMPLE", None) if box is not None else None)
            table.add_column("Name", style="bold")
            table.add_column("Size", justify="right")
            table.add_column("Path", overflow="fold")
            for path in sorted(logs_dir.glob("*.log")):
                table.add_row(path.stem, str(path.stat().st_size), str(path))
            console.print(table)
            return
        for path in sorted(logs_dir.glob("*.log")):
            size = path.stat().st_size
            print(f"{path.stem:16} {size:10} {path}")
        return

    path = logs_dir / f"{log_name}.log"
    if not path.exists():
        print(f"Log file not found: {path}", file=sys.stderr)
        print("Run `flyflor logs list` to see available logs.", file=sys.stderr)
        raise SystemExit(2)
    command = ["tail", "-n", str(max(lines, 0))]
    if follow:
        command.append("-f")
    command.append(str(path))
    raise SystemExit(subprocess.call(command))


def run_textual_dashboard(home: Path) -> None:
    try:
        from flyflor_core.tui import run
    except ImportError as exc:
        print("Textual is not installed. Install Flyflor with its Python dependencies first.", file=sys.stderr)
        print(f"Import error: {exc}", file=sys.stderr)
        raise SystemExit(2) from exc
    if not is_initialized(home):
        print(f"Flyflor is not initialized at {home}", file=sys.stderr)
        print("Run `flyflor setup` first, or `flyflor setup --defaults` in non-interactive environments.", file=sys.stderr)
        raise SystemExit(2)
    run(home)


def run_internal_nanobot(home: Path, args: list[str]) -> None:
    require_initialized(home)
    nanobot_args = list(args)
    if not any(arg in ("--config", "-c", "--help", "-h", "--version", "-V") for arg in nanobot_args):
        nanobot_args.extend(["--config", str(nanobot_config_path(home))])
    command = ["nanobot", *nanobot_args]
    raise SystemExit(subprocess.call(command, env=runtime_env(home)))


def run_gateway(
    *,
    home: Path,
    core_port: int,
    qdrant_http_port: int,
    qdrant_grpc_port: int,
    websocket_port: int,
    nanobot_port: int | None,
    skip_qdrant: bool,
) -> None:
    ensure_layout(home, core_port=core_port, websocket_port=websocket_port, write_defaults=False)
    setup_config = load_flyflor_config(home)
    write_nanobot_config_from_setup(home, setup_config)
    env = runtime_env(home, core_port=core_port, qdrant_http_port=qdrant_http_port)
    processes: list[subprocess.Popen[bytes]] = []
    print_header("Gateway Runtime", "Starting Flyflor Core, bridge daemon, and internal channel gateway")

    def start(name: str, command: list[str], extra_env: dict[str, str] | None = None) -> None:
        log_path = home / "logs" / f"{name}.log"
        log = log_path.open("ab", buffering=0)
        child_env = dict(env)
        if extra_env:
            child_env.update(extra_env)
        rich_print(
            f"[bold]Starting {name}:[/] {' '.join(command)}\n[dim]log: {log_path}[/]"
            if console is not None
            else f"Starting {name}: {' '.join(command)}\n  log: {log_path}"
        )
        processes.append(subprocess.Popen(command, stdout=log, stderr=subprocess.STDOUT, env=child_env))

    services: list[tuple[str, list[str]]] = []
    qdrant_bin = shutil.which("qdrant")
    if not skip_qdrant and qdrant_bin:
        qdrant_config = write_qdrant_config(home, qdrant_http_port, qdrant_grpc_port)
        services.append(("qdrant", [qdrant_bin, "--config-path", str(qdrant_config), "--disable-telemetry"]))
    elif not skip_qdrant:
        rich_print(
            "[yellow]Qdrant binary not found.[/] Continuing without semantic memory service.\n"
            "Install qdrant later and rerun `flyflor gateway`, or use `--skip-qdrant` deliberately."
            if console is not None
            else "Qdrant binary not found. Continuing without semantic memory service.\n"
            "Install qdrant later and rerun `flyflor gateway`, or use `--skip-qdrant` deliberately."
        )

    services.append(("core", [sys.executable, "-m", "flyflor_core"]))
    services.append(("bridge", ["flyflor", "bridge-daemon"]))

    nanobot_command = ["flyflor", "--home", str(home), "internal-nanobot", "gateway"]
    if nanobot_port is not None:
        nanobot_command.extend(["--port", str(nanobot_port)])
    services.append(("gateway", nanobot_command))

    if console is not None and Progress is not None and SpinnerColumn is not None and TextColumn is not None:
        with Progress(
            SpinnerColumn(),
            TextColumn("[progress.description]{task.description}"),
            BarColumn(),
            TimeElapsedColumn(),
            console=console,
            transient=True,
        ) as progress:
            task = progress.add_task("Starting services", total=len(services))
            for name, command in services:
                start(name, command)
                progress.advance(task)
    else:
        for name, command in services:
            start(name, command)

    if console is not None and Table is not None:
        table = Table(box=getattr(box, "SIMPLE", None) if box is not None else None)
        table.add_column("Endpoint", style="bold")
        table.add_column("Value", overflow="fold")
        table.add_row("home", str(home))
        table.add_row("public gateway config", str(nanobot_config_path(home)))
        table.add_row("internal core", f"http://127.0.0.1:{core_port}")
        table.add_row("internal qdrant", f"http://127.0.0.1:{qdrant_http_port}")
        console.print(table)
        rich_print("[bold green]Flyflor is running.[/] Press Ctrl-C to stop.")
    else:
        print()
        print("Flyflor is running.")
        print(f"  home: {home}")
        print(f"  public/top gateway config: {nanobot_config_path(home)}")
        print(f"  internal core: http://127.0.0.1:{core_port}")
        print(f"  internal qdrant: http://127.0.0.1:{qdrant_http_port}")
        print("Press Ctrl-C to stop.")

    def stop(_signum: int | None = None, _frame: object | None = None) -> None:
        for process in reversed(processes):
            if process.poll() is None:
                process.terminate()
        deadline = time.time() + 8
        for process in processes:
            remaining = max(deadline - time.time(), 0.1)
            try:
                process.wait(timeout=remaining)
            except subprocess.TimeoutExpired:
                process.kill()
        raise SystemExit(0)

    signal.signal(signal.SIGINT, stop)
    signal.signal(signal.SIGTERM, stop)

    while True:
        for process in processes:
            code = process.poll()
            if code is not None:
                print(f"A Flyflor child process exited with code {code}. Stopping gateway.")
                stop()
        time.sleep(1)


def ensure_layout(
    home: Path,
    *,
    core_port: int = DEFAULT_CORE_PORT,
    websocket_port: int = DEFAULT_WEBSOCKET_PORT,
    write_defaults: bool = True,
) -> None:
    for path in (
        home,
        home / "logs",
        home / "qdrant" / "storage",
        home / "nanobot",
        home / "workspace",
        home / "memory",
        home / "agents" / "home",
        home / "agents" / "codex",
        home / "agents" / "claude",
        home / "agents" / "copilot",
        home / "agents" / "opencode",
        home / "agents" / "qwen",
        home / "agents" / "kimi",
        home / "agents" / "deepseek",
        home / "agents" / "gemini",
        home / "xdg" / "config",
        home / "xdg" / "cache",
        home / "xdg" / "data",
        home / "tmp",
    ):
        path.mkdir(parents=True, exist_ok=True)

    if write_defaults and not flyflor_config_path(home).exists():
        config = default_flyflor_config(core_port=core_port, websocket_port=websocket_port)
        save_flyflor_config(home, config)
        write_nanobot_config_from_setup(home, config)


def nanobot_config_path(home: Path) -> Path:
    return home / "nanobot" / "config.json"


def runtime_env(home: Path, *, core_port: int = DEFAULT_CORE_PORT, qdrant_http_port: int = DEFAULT_QDRANT_HTTP_PORT) -> dict[str, str]:
    env = dict(os.environ)
    env.update(
        {
            "FLYFLOR_HOME": str(home),
            "FLYFLOR_DATABASE": str(home / "flyflor.db"),
            "FLYFLOR_HOST": "127.0.0.1",
            "FLYFLOR_PORT": str(core_port),
            "QDRANT_URL": f"http://127.0.0.1:{qdrant_http_port}",
            "QDRANT_COLLECTION": "flyflor_memory",
        }
    )
    env.update(tool_profile_env(home))
    return env


def write_qdrant_config(home: Path, http_port: int, grpc_port: int) -> Path:
    path = home / "qdrant" / "config.yaml"
    path.write_text(
        "\n".join(
            [
                "storage:",
                f"  storage_path: {home / 'qdrant' / 'storage'}",
                "service:",
                "  host: 127.0.0.1",
                f"  http_port: {http_port}",
                f"  grpc_port: {grpc_port}",
                "telemetry_disabled: true",
                "",
            ]
        ),
        encoding="utf-8",
    )
    return path


def write_nanobot_config_from_setup(home: Path, setup_config: dict[str, object]) -> None:
    nanobot_config_path(home).write_text(
        json.dumps(default_nanobot_config(home, setup_config), ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )


def default_nanobot_config(home: Path, setup_config: dict[str, object]) -> dict[str, object]:
    primary = setup_config.get("primary", {})
    if not isinstance(primary, dict):
        primary = {}
    channels = setup_config.get("channels", {})
    if not isinstance(channels, dict):
        channels = {}
    return {
        "providers": {
            "vllm": {
                "apiKey": primary.get("apiKey"),
                "apiBase": primary.get("apiBase", f"http://127.0.0.1:{DEFAULT_CORE_PORT}/v1"),
            }
        },
        "agents": {
            "defaults": {
                "workspace": str(home / "workspace"),
                "provider": "vllm",
                "model": primary.get("model", "flyflor-placeholder"),
                "disabledSkills": ["memory", "cron"],
            }
        },
        "channels": {name: value for name, value in channels.items() if isinstance(value, dict) and value.get("enabled")},
    }


if __name__ == "__main__":
    main()
