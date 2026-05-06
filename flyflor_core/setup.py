from __future__ import annotations

import json
from copy import deepcopy
from pathlib import Path
from typing import Any


CONFIG_VERSION = 1

CHANNEL_ORDER = [
    "websocket",
    "feishu",
    "dingtalk",
    "qq",
    "weixin",
    "wecom",
    "telegram",
    "slack",
    "whatsapp",
    "email",
]

CHANNEL_PRESETS: dict[str, dict[str, Any]] = {
    "websocket": {
        "label": "WebSocket",
        "description": "local/default channel for smoke tests and API clients",
        "fields": [
            {"name": "host", "label": "Host", "default": "0.0.0.0"},
            {"name": "port", "label": "Port", "default": 8765, "type": "int"},
            {"name": "path", "label": "Path", "default": "/"},
            {
                "name": "websocketRequiresToken",
                "label": "Require token",
                "default": False,
                "type": "bool",
            },
            {"name": "token", "label": "Token", "default": "", "secret": True},
            {"name": "allowFrom", "label": "Allow from", "default": ["*"], "type": "csv"},
            {"name": "streaming", "label": "Streaming", "default": True, "type": "bool"},
        ],
    },
    "feishu": {
        "label": "飞书 / Lark",
        "description": "long connection; no public IP required",
        "fields": [
            {"name": "appId", "label": "App ID"},
            {"name": "appSecret", "label": "App Secret", "secret": True},
            {"name": "encryptKey", "label": "Encrypt Key", "default": "", "secret": True},
            {"name": "verificationToken", "label": "Verification Token", "default": "", "secret": True},
            {"name": "allowFrom", "label": "Allow from", "default": ["*"], "type": "csv"},
            {"name": "groupPolicy", "label": "Group policy", "default": "mention"},
            {"name": "streaming", "label": "Streaming", "default": True, "type": "bool"},
            {"name": "domain", "label": "Domain feishu/lark", "default": "feishu"},
        ],
    },
    "dingtalk": {
        "label": "钉钉",
        "description": "Stream Mode; no public IP required",
        "fields": [
            {"name": "clientId", "label": "Client ID / AppKey"},
            {"name": "clientSecret", "label": "Client Secret / AppSecret", "secret": True},
            {"name": "allowFrom", "label": "Allow from", "default": ["*"], "type": "csv"},
        ],
    },
    "qq": {
        "label": "QQ",
        "description": "botpy WebSocket; private messages are the safest starting point",
        "fields": [
            {"name": "appId", "label": "App ID"},
            {"name": "secret", "label": "App Secret", "secret": True},
            {"name": "allowFrom", "label": "Allow from", "default": ["*"], "type": "csv"},
            {"name": "msgFormat", "label": "Message format plain/markdown", "default": "plain"},
        ],
    },
    "weixin": {
        "label": "微信 / Weixin",
        "description": "QR login; run `flyflor nanobot channels login weixin` after setup",
        "fields": [
            {"name": "allowFrom", "label": "Allow from", "default": ["*"], "type": "csv"},
            {"name": "token", "label": "Saved token", "default": "", "secret": True},
            {"name": "routeTag", "label": "Route tag", "default": ""},
            {"name": "pollTimeout", "label": "Poll timeout seconds", "default": 60, "type": "int"},
        ],
    },
    "wecom": {
        "label": "企业微信 / WeCom",
        "description": "AI Bot long connection; no public IP required",
        "fields": [
            {"name": "botId", "label": "Bot ID"},
            {"name": "secret", "label": "Bot Secret", "secret": True},
            {"name": "allowFrom", "label": "Allow from", "default": ["*"], "type": "csv"},
        ],
    },
    "telegram": {
        "label": "Telegram",
        "description": "BotFather token; may need network support in China",
        "fields": [
            {"name": "token", "label": "Bot token", "secret": True},
            {"name": "allowFrom", "label": "Allow from", "default": ["*"], "type": "csv"},
        ],
    },
    "slack": {
        "label": "Slack",
        "description": "Socket Mode; no public URL required",
        "fields": [
            {"name": "botToken", "label": "Bot token xoxb-*", "secret": True},
            {"name": "appToken", "label": "App token xapp-*", "secret": True},
            {"name": "allowFrom", "label": "Allow from", "default": ["*"], "type": "csv"},
            {"name": "groupPolicy", "label": "Group policy", "default": "mention"},
        ],
    },
    "whatsapp": {
        "label": "WhatsApp",
        "description": "QR login; run `flyflor nanobot channels login whatsapp` after setup",
        "fields": [
            {"name": "allowFrom", "label": "Allow from", "default": ["*"], "type": "csv"},
        ],
    },
    "email": {
        "label": "Email",
        "description": "IMAP inbox + SMTP replies; use a dedicated mailbox",
        "fields": [
            {"name": "consentGranted", "label": "Consent granted", "default": False, "type": "bool"},
            {"name": "imapHost", "label": "IMAP host"},
            {"name": "imapPort", "label": "IMAP port", "default": 993, "type": "int"},
            {"name": "imapUsername", "label": "IMAP username"},
            {"name": "imapPassword", "label": "IMAP password", "secret": True},
            {"name": "smtpHost", "label": "SMTP host"},
            {"name": "smtpPort", "label": "SMTP port", "default": 587, "type": "int"},
            {"name": "smtpUsername", "label": "SMTP username"},
            {"name": "smtpPassword", "label": "SMTP password", "secret": True},
            {"name": "fromAddress", "label": "From address"},
            {"name": "allowFrom", "label": "Allow from", "default": ["*"], "type": "csv"},
            {
                "name": "allowedAttachmentTypes",
                "label": "Allowed attachment MIME types",
                "default": [],
                "type": "csv",
            },
            {"name": "autoReplyEnabled", "label": "Auto reply", "default": True, "type": "bool"},
        ],
    },
}

WORKER_ORDER = [
    "codex",
    "claude",
    "copilot",
    "opencode",
    "qwen-code",
    "kimi",
    "deepseek-tui",
    "gemini",
    "aider",
    "goose",
]

WORKER_PRESETS: dict[str, dict[str, Any]] = {
    "codex": {
        "enabled": True,
        "kind": "cli",
        "command": ["codex"],
        "install": ["npm", "install", "-g", "@openai/codex"],
        "env": {"CODEX_HOME": "{home}/agents/codex"},
        "description": "OpenAI Codex CLI",
    },
    "claude": {
        "enabled": True,
        "kind": "cli",
        "command": ["claude"],
        "install": ["npm", "install", "-g", "@anthropic-ai/claude-code"],
        "env": {"CLAUDE_CONFIG_DIR": "{home}/agents/claude"},
        "description": "Anthropic Claude Code",
    },
    "copilot": {
        "enabled": False,
        "kind": "cli",
        "command": ["copilot"],
        "install": ["npm", "install", "-g", "@github/copilot"],
        "env": {"COPILOT_CONFIG_DIR": "{home}/agents/copilot"},
        "description": "GitHub Copilot CLI",
    },
    "opencode": {
        "enabled": False,
        "kind": "cli",
        "command": ["opencode"],
        "install": ["npm", "install", "-g", "opencode-ai"],
        "env": {},
        "description": "OpenCode terminal agent",
    },
    "qwen-code": {
        "enabled": False,
        "kind": "cli",
        "command": ["qwen"],
        "install": ["npm", "install", "-g", "@qwen-code/qwen-code"],
        "env": {},
        "description": "Qwen Code CLI or user-provided wrapper",
    },
    "kimi": {
        "enabled": False,
        "kind": "cli",
        "command": ["kimi"],
        "install": [
            "sh",
            "-lc",
            "python -m pip install --upgrade uv && uv tool install kimi-cli --force && ln -sf \"$HOME/.local/bin/kimi\" /usr/local/bin/kimi",
        ],
        "env": {},
        "description": "Kimi CLI or user-provided wrapper",
    },
    "deepseek-tui": {
        "enabled": False,
        "kind": "cli",
        "command": ["deepseek-tui"],
        "env": {},
        "description": "DeepSeek CLI or user-provided wrapper",
    },
    "gemini": {
        "enabled": False,
        "kind": "cli",
        "command": ["gemini"],
        "install": ["npm", "install", "-g", "@google/gemini-cli"],
        "env": {},
        "description": "Google Gemini CLI",
    },
    "aider": {
        "enabled": False,
        "kind": "cli",
        "command": ["aider"],
        "install": ["python", "-m", "pip", "install", "--upgrade", "aider-chat"],
        "env": {},
        "description": "Aider coding assistant",
    },
    "goose": {
        "enabled": False,
        "kind": "cli",
        "command": ["goose"],
        "env": {},
        "description": "Block Goose agent",
    },
}


def flyflor_config_path(home: Path) -> Path:
    return home / "flyflor.json"


def is_initialized(home: Path) -> bool:
    path = flyflor_config_path(home)
    if not path.exists():
        return False
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError:
        return False
    return bool(data.get("initialized"))


def load_flyflor_config(home: Path) -> dict[str, Any]:
    path = flyflor_config_path(home)
    if not path.exists():
        raise FileNotFoundError(path)
    return json.loads(path.read_text(encoding="utf-8"))


def save_flyflor_config(home: Path, data: dict[str, Any]) -> None:
    path = flyflor_config_path(home)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def default_flyflor_config(*, core_port: int, websocket_port: int) -> dict[str, Any]:
    return {
        "version": CONFIG_VERSION,
        "initialized": True,
        "primary": {
            "provider": "vllm",
            "model": "flyflor-placeholder",
            "apiBase": f"http://127.0.0.1:{core_port}/v1",
            "apiKey": None,
        },
        "channels": {
            "websocket": {
                "enabled": True,
                "host": "0.0.0.0",
                "port": websocket_port,
                "path": "/",
                "websocketRequiresToken": False,
                "allowFrom": ["*"],
                "streaming": True,
            },
            "feishu": disabled_channel("feishu"),
            "dingtalk": disabled_channel("dingtalk"),
            "qq": disabled_channel("qq"),
            "weixin": disabled_channel("weixin"),
            "wecom": disabled_channel("wecom"),
            "telegram": disabled_channel("telegram"),
            "slack": disabled_channel("slack"),
            "whatsapp": disabled_channel("whatsapp"),
            "email": disabled_channel("email"),
        },
        "workers": {name: deepcopy(WORKER_PRESETS[name]) for name in WORKER_ORDER},
        "bridge": {
            "mode": "guardian_pair",
            "autoSelect": False,
            "guardianPair": {
                "left": "codex",
                "right": "claude",
            },
        },
    }


def merge_user_setup(existing: dict[str, Any], generated: dict[str, Any]) -> dict[str, Any]:
    merged = dict(generated)
    for key in ("primary", "channels", "workers", "bridge"):
        value = existing.get(key)
        if isinstance(value, dict):
            base = dict(merged.get(key, {}))
            for name, item in value.items():
                if isinstance(item, dict) and isinstance(base.get(name), dict):
                    updated = dict(base[name])
                    updated.update(item)
                    base[name] = updated
                else:
                    base[name] = item
            merged[key] = base
    merged["initialized"] = bool(existing.get("initialized", generated.get("initialized", True)))
    merged["version"] = existing.get("version", generated.get("version", CONFIG_VERSION))
    return merged


def disabled_channel(name: str) -> dict[str, Any]:
    channel = {"enabled": False}
    preset = CHANNEL_PRESETS.get(name, {})
    for field in preset.get("fields", []):
        if "default" in field:
            channel[field["name"]] = field["default"]
    return channel
