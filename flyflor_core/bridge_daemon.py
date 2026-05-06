from __future__ import annotations

import json
import os
import signal
import shutil
import subprocess
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from flyflor_core.blackboard import Blackboard
from flyflor_core.config import Config
from flyflor_core.setup import load_flyflor_config


@dataclass
class GuardianPair:
    left: str
    right: str


@dataclass
class WorkerRun:
    name: str
    ok: bool
    output: str
    command: list[str]


class BridgeDaemon:
    def __init__(self, config: Config, *, interval: float = 1.0) -> None:
        self.config = config
        self.interval = interval
        self.blackboard = Blackboard(config.database)
        self.running = True

    def run(self) -> None:
        print(f"Flyflor Bridge daemon watching {self.config.database}", flush=True)
        signal.signal(signal.SIGINT, self.stop)
        signal.signal(signal.SIGTERM, self.stop)
        while self.running:
            task = self.blackboard.claim_next_task()
            if task is None:
                time.sleep(self.interval)
                continue
            self.handle_task(task)

    def stop(self, _signum: int | None = None, _frame: object | None = None) -> None:
        self.running = False

    def handle_task(self, task: dict[str, Any]) -> None:
        task_id = int(task["id"])
        pair = guardian_pair_for_task(self.config.home, task)
        payload = {"guardianPair": {"left": pair.left, "right": pair.right}, "runner": "subprocess"}
        print(f"Bridge claimed task #{task_id}: {pair.left} <-> {pair.right}", flush=True)
        self.blackboard.append_event(
            task_id,
            "flyflor",
            "bridge_claimed",
            f"Bridge claimed task with guardian pair {pair.left} <-> {pair.right}",
            payload,
        )

        setup = load_setup(self.config.home)
        user_input = str(task["input"])
        left_prompt = build_worker_prompt("Worker A", pair.left, pair.right, user_input, None)
        left_result = run_worker(self.config.home, setup, pair.left, left_prompt)
        self.blackboard.append_event(
            task_id,
            pair.left,
            "message",
            display_worker_result(left_result),
            {**payload, "command": left_result.command, "ok": left_result.ok},
        )

        right_prompt = build_worker_prompt("Worker B", pair.right, pair.left, user_input, left_result.output)
        right_result = run_worker(self.config.home, setup, pair.right, right_prompt)
        self.blackboard.append_event(
            task_id,
            pair.right,
            "message",
            display_worker_result(right_result),
            {**payload, "command": right_result.command, "ok": right_result.ok},
        )

        final = synthesize_final(user_input, left_result, right_result)
        final_status = "done" if left_result.ok or right_result.ok else "failed_runner"
        self.blackboard.append_event(
            task_id,
            "flyflor",
            "final_answer",
            final,
            {
                **payload,
                "left": {"name": left_result.name, "ok": left_result.ok},
                "right": {"name": right_result.name, "ok": right_result.ok},
            },
        )
        self.blackboard.update_task_status(task_id, final_status)


def load_setup(home: Path) -> dict[str, Any]:
    try:
        return load_flyflor_config(home)
    except (FileNotFoundError, json.JSONDecodeError):
        return {}


def build_worker_prompt(role: str, self_name: str, peer_name: str, user_input: str, peer_output: str | None) -> str:
    peer_section = f"\n\nPeer output from {peer_name}:\n{peer_output}" if peer_output else ""
    return (
        f"You are {role} ({self_name}) in Flyflor guardian-pair discussion.\n"
        f"Your peer is {peer_name}.\n"
        "Answer concisely in Chinese. Focus on the user's request, not on explaining Flyflor internals.\n"
        "If you cannot run because auth/config is missing, say the exact reason.\n\n"
        f"User request:\n{user_input}"
        f"{peer_section}\n\n"
        "Return your best answer or critique now."
    )


def run_worker(home: Path, setup: dict[str, Any], name: str, prompt: str, *, timeout: int = 30) -> WorkerRun:
    worker = worker_config(setup, name)
    command = worker_command(name, worker)
    if not command:
        return WorkerRun(name=name, ok=False, output=f"worker `{name}` has no command configured.", command=[])

    executable = shutil.which(command[0])
    if executable is None:
        return WorkerRun(name=name, ok=False, output=f"command not found: {command[0]}", command=command)

    run_command = adapt_command(home, name, command, prompt)
    env = worker_env(home, worker)
    try:
        result = subprocess.run(
            run_command,
            input=prompt if reads_stdin(name, command) else None,
            text=True,
            capture_output=True,
            timeout=timeout,
            env=env,
            cwd=str(worker_workspace(home)),
        )
    except subprocess.TimeoutExpired as exc:
        partial = (exc.stdout or "") + (exc.stderr or "")
        return WorkerRun(name=name, ok=False, output=clean_output(partial) or f"timed out after {timeout}s", command=run_command)
    except OSError as exc:
        return WorkerRun(name=name, ok=False, output=str(exc), command=run_command)

    output = clean_output((result.stdout or "") + ("\n" + result.stderr if result.stderr else ""))
    ok = result.returncode == 0 and bool(output.strip())
    if not output.strip():
        output = f"exited with code {result.returncode} and produced no output."
    elif result.returncode != 0:
        output = f"exit code {result.returncode}\n{output}"
    return WorkerRun(name=name, ok=ok, output=output, command=run_command)


def worker_config(setup: dict[str, Any], name: str) -> dict[str, Any]:
    workers = setup.get("workers", {})
    if isinstance(workers, dict):
        value = workers.get(name, {})
        if isinstance(value, dict):
            return value
    return {}


def worker_command(name: str, worker: dict[str, Any]) -> list[str]:
    command = worker.get("command")
    if isinstance(command, list) and command:
        return [str(part) for part in command]
    if isinstance(command, str) and command:
        return command.split()
    return [name]


def adapt_command(home: Path, name: str, command: list[str], prompt: str) -> list[str]:
    base = Path(command[0]).name
    if base == "codex":
        return [command[0], "exec", "--skip-git-repo-check", "--color", "never", "-"]
    if base == "claude":
        return [command[0], "--print", "--no-session-persistence", prompt]
    if base == "copilot":
        return [
            command[0],
            "--config-dir",
            str(home / "agents" / "copilot"),
            "--no-color",
            "--stream",
            "off",
            "-s",
            "-p",
            prompt,
        ]
    if "{prompt}" in command:
        return [prompt if part == "{prompt}" else part for part in command]
    return [*command, prompt]


def reads_stdin(name: str, command: list[str]) -> bool:
    return Path(command[0]).name == "codex"


def worker_env(home: Path, worker: dict[str, Any]) -> dict[str, str]:
    env = dict(os.environ)
    raw_env = worker.get("env", {})
    if isinstance(raw_env, dict):
        for key, value in raw_env.items():
            env[str(key)] = str(value).replace("{home}", str(home))
    env.setdefault("HOME", str(home / "agents" / "home"))
    env.setdefault("CODEX_HOME", str(home / "agents" / "codex"))
    env.setdefault("CLAUDE_CONFIG_DIR", str(home / "agents" / "claude"))
    env.setdefault("COPILOT_CONFIG_DIR", str(home / "agents" / "copilot"))
    return env


def worker_workspace(home: Path) -> Path:
    configured = os.getenv("NANOBOT_WORKSPACE") or os.getenv("FLYFLOR_WORKSPACE")
    if configured:
        path = Path(configured)
        path.mkdir(parents=True, exist_ok=True)
        return path
    path = home / "workspace"
    path.mkdir(parents=True, exist_ok=True)
    return path


def clean_output(output: str, *, limit: int = 6000) -> str:
    text = output.replace("\r", "\n")
    lines = [line.rstrip() for line in text.splitlines()]
    compact = "\n".join(line for line in lines if line.strip())
    if len(compact) > limit:
        return compact[-limit:]
    return compact


def display_worker_result(result: WorkerRun) -> str:
    if result.ok:
        return result.output
    return "运行失败：" + add_failure_hint(result)


def add_failure_hint(result: WorkerRun) -> str:
    text = result.output
    lowered = text.lower()
    if "401 unauthorized" in lowered and result.name == "codex":
        return text + "\n\n提示：Flyflor 隔离环境里的 Codex 未登录。运行 `flyflor codex login`。"
    if "401 unauthorized" in lowered and result.name == "copilot":
        return text + "\n\n提示：Flyflor 隔离环境里的 Copilot 未登录。运行 `flyflor copilot login`。"
    if text.startswith("command not found:"):
        return text + f"\n\n提示：worker `{result.name}` 的命令不存在，请在 setup 中修改 command 或安装该 CLI。"
    if "timed out" in lowered:
        return text + "\n\n提示：该 worker 超时，可能正在等待登录、权限确认或网络响应。"
    return text


def synthesize_final(user_input: str, left: WorkerRun, right: WorkerRun) -> str:
    if left.ok and right.ok:
        return (
            f"针对「{user_input}」的双 worker 守护讨论已完成。\n\n"
            f"{left.name} 的结果：\n{left.output}\n\n"
            f"{right.name} 的审查/补充：\n{right.output}"
        )
    if left.ok:
        return f"{right.name} 未能运行，先返回 {left.name} 的结果：\n{left.output}\n\n{right.name} 错误：\n{right.output}"
    if right.ok:
        return f"{left.name} 未能运行，先返回 {right.name} 的结果：\n{right.output}\n\n{left.name} 错误：\n{left.output}"
    return (
        "左右 worker runner 都没有成功运行，所以这不是模型卡住，而是 worker 启动失败。\n\n"
        f"{left.name} 错误：\n{left.output}\n\n"
        f"{right.name} 错误：\n{right.output}"
    )


def guardian_pair_for_task(home: Path, task: dict[str, Any]) -> GuardianPair:
    metadata = parse_json_object(task.get("metadata_json"))
    pair = metadata.get("guardianPair") if isinstance(metadata, dict) else None
    if not isinstance(pair, dict):
        pair = default_guardian_pair(home)
    left = str(pair.get("left", "codex"))
    right = str(pair.get("right", "claude"))
    if left == right:
        right = "claude" if left != "claude" else "codex"
    return GuardianPair(left=left, right=right)


def default_guardian_pair(home: Path) -> dict[str, str]:
    try:
        setup = load_flyflor_config(home)
    except (FileNotFoundError, json.JSONDecodeError):
        return {"left": "codex", "right": "claude"}
    bridge = setup.get("bridge", {})
    pair = bridge.get("guardianPair", {}) if isinstance(bridge, dict) else {}
    if not isinstance(pair, dict):
        return {"left": "codex", "right": "claude"}
    return {"left": str(pair.get("left", "codex")), "right": str(pair.get("right", "claude"))}


def parse_json_object(value: object) -> dict[str, Any]:
    if not isinstance(value, str):
        return {}
    try:
        parsed = json.loads(value)
    except json.JSONDecodeError:
        return {}
    return parsed if isinstance(parsed, dict) else {}


def main() -> None:
    BridgeDaemon(Config.from_env()).run()


if __name__ == "__main__":
    main()
