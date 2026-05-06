from __future__ import annotations

import argparse
import getpass
import json
import os
import shutil
import signal
import subprocess
import sys
import time
from pathlib import Path

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


def main(argv: list[str] | None = None) -> None:
    parser = argparse.ArgumentParser(prog="flyflor")
    parser.add_argument("--home", default=os.getenv("FLYFLOR_HOME", "~/.flyflor"), help="Flyflor home directory")
    subparsers = parser.add_subparsers(dest="command")

    setup_parser = subparsers.add_parser("setup", help="Initialize Flyflor model, channels, and workers")
    setup_parser.add_argument("--defaults", action="store_true", help="Write default setup without prompts")
    setup_parser.add_argument("--force", action="store_true", help="Overwrite existing setup")

    gateway_parser = subparsers.add_parser("gateway", help="Start Flyflor gateway on this machine")
    gateway_parser.add_argument("--core-port", type=int, default=DEFAULT_CORE_PORT)
    gateway_parser.add_argument("--qdrant-http-port", type=int, default=DEFAULT_QDRANT_HTTP_PORT)
    gateway_parser.add_argument("--qdrant-grpc-port", type=int, default=DEFAULT_QDRANT_GRPC_PORT)
    gateway_parser.add_argument("--websocket-port", type=int, default=DEFAULT_WEBSOCKET_PORT)
    gateway_parser.add_argument("--nanobot-port", type=int, default=None, help="nanobot health port")
    gateway_parser.add_argument("--skip-qdrant", action="store_true", help="Do not start local qdrant")

    subparsers.add_parser("init", help="Alias for `setup --defaults`")
    subparsers.add_parser("bridge-daemon", help="Run the Flyflor bridge task consumer")
    subparsers.add_parser("workers", help="List configured Flyflor CLI workers")

    subparsers.add_parser("codex", help="Run Codex with Flyflor-isolated config")

    subparsers.add_parser("claude", help="Run Claude Code with Flyflor-isolated config")

    subparsers.add_parser("copilot", help="Run GitHub Copilot CLI with Flyflor-isolated config")

    subparsers.add_parser("nanobot", help="Run nanobot with Flyflor config")

    args, passthrough = parser.parse_known_args(argv)
    home = Path(args.home).expanduser().resolve()

    if args.command == "init":
        run_setup(home, defaults=True, force=False)
        return

    if args.command == "setup":
        run_setup(home, defaults=args.defaults, force=args.force)
        return

    if args.command == "gateway":
        require_initialized(home)
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
        print_workers(home)
        return

    if args.command == "nanobot":
        require_initialized(home)
        nanobot_args = list(passthrough)
        if not any(arg in ("--config", "-c", "--help", "-h", "--version", "-V") for arg in nanobot_args):
            nanobot_args.extend(["--config", str(nanobot_config_path(home))])
        command = ["nanobot", *nanobot_args]
        raise SystemExit(subprocess.call(command, env=runtime_env(home)))

    if args.command == "codex":
        require_initialized(home)
        command = ["codex", *passthrough]
        raise SystemExit(subprocess.call(command, env=runtime_env(home)))

    if args.command == "claude":
        require_initialized(home)
        command = ["claude", *passthrough]
        raise SystemExit(subprocess.call(command, env=runtime_env(home)))

    if args.command == "copilot":
        require_initialized(home)
        command = ["copilot", *passthrough]
        if shutil.which(command[0]) is None:
            print("GitHub Copilot CLI was not found on PATH.", file=sys.stderr)
            print("Install it on the host, or update the `copilot` worker command in `flyflor setup --force`.", file=sys.stderr)
            raise SystemExit(127)
        raise SystemExit(subprocess.call(command, env=runtime_env(home)))

    parser.print_help()


def run_setup(home: Path, *, defaults: bool, force: bool) -> None:
    ensure_layout(home, write_defaults=False)
    setup_path = flyflor_config_path(home)
    if setup_path.exists() and is_initialized(home) and not force:
        print(f"Flyflor is already initialized at {home}")
        print(f"setup config: {setup_path}")
        print("Use `flyflor setup --force` to rewrite it.")
        return

    config = default_flyflor_config(core_port=DEFAULT_CORE_PORT, websocket_port=DEFAULT_WEBSOCKET_PORT)
    if setup_path.exists():
        config = merge_user_setup(load_flyflor_config(home), config)

    if not defaults:
        if not sys.stdin.isatty():
            print("`flyflor setup` needs an interactive terminal.", file=sys.stderr)
            print("Use `flyflor setup --defaults` for non-interactive initialization.", file=sys.stderr)
            raise SystemExit(2)
        config = prompt_setup(config)

    save_flyflor_config(home, config)
    write_nanobot_config_from_setup(home, config)
    print(f"Initialized Flyflor at {home}")
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


def print_workers(home: Path) -> None:
    config = load_flyflor_config(home)
    workers = config.get("workers", {})
    if not isinstance(workers, dict):
        print("No workers configured.")
        return
    for name, value in workers.items():
        if not isinstance(value, dict):
            continue
        enabled = "enabled" if value.get("enabled") else "disabled"
        command = value.get("command", [])
        if isinstance(command, list):
            command_text = " ".join(str(part) for part in command)
        else:
            command_text = str(command)
        print(f"{name:14} {enabled:8} {command_text}")


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

    def start(name: str, command: list[str], extra_env: dict[str, str] | None = None) -> None:
        log_path = home / "logs" / f"{name}.log"
        log = log_path.open("ab", buffering=0)
        child_env = dict(env)
        if extra_env:
            child_env.update(extra_env)
        print(f"Starting {name}: {' '.join(command)}")
        print(f"  log: {log_path}")
        processes.append(subprocess.Popen(command, stdout=log, stderr=subprocess.STDOUT, env=child_env))

    qdrant_bin = shutil.which("qdrant")
    if not skip_qdrant and qdrant_bin:
        qdrant_config = write_qdrant_config(home, qdrant_http_port, qdrant_grpc_port)
        start("qdrant", [qdrant_bin, "--config-path", str(qdrant_config), "--disable-telemetry"])
    elif not skip_qdrant:
        print("Qdrant binary not found. Continuing without semantic memory service.")
        print("Install qdrant later and rerun `flyflor gateway`, or use `--skip-qdrant` deliberately.")

    start("core", [sys.executable, "-m", "flyflor_core"])
    start("bridge", ["flyflor", "bridge-daemon"])

    nanobot_command = ["nanobot", "gateway", "--config", str(nanobot_config_path(home))]
    if nanobot_port is not None:
        nanobot_command.extend(["--port", str(nanobot_port)])
    start("gateway", nanobot_command)

    print()
    print("Flyflor is running.")
    print(f"  home: {home}")
    print(f"  public/top gateway: nanobot channels from {nanobot_config_path(home)}")
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
            "HOME": str(home / "agents" / "home"),
            "CODEX_HOME": str(home / "agents" / "codex"),
            "CLAUDE_CONFIG_DIR": str(home / "agents" / "claude"),
            "COPILOT_CONFIG_DIR": str(home / "agents" / "copilot"),
            "XDG_CONFIG_HOME": str(home / "xdg" / "config"),
            "XDG_CACHE_HOME": str(home / "xdg" / "cache"),
            "XDG_DATA_HOME": str(home / "xdg" / "data"),
            "TMPDIR": str(home / "tmp"),
        }
    )
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
