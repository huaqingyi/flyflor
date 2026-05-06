# Flyflor 架构说明

Flyflor 是一个轻量级多智能体协作架构。它使用 nanobot 负责多渠道会话入口，使用 Flyflor Core 负责任务分发、Web 控制台、黑板、Bridge 生命周期和长期记忆，使用 Codex / Claude / Copilot / OpenCode 作为可协作的执行智能体。

## 当前目标

第一阶段目标不是做一个大而全的 Agent，而是先打通稳定链路：

- nanobot 负责多渠道、session、WebSocket gateway；
- Flyflor Core 负责任务创建、Web 控制台、黑板事件、调度和 OpenAI-compatible 本地接口；
- Qdrant 负责语义记忆检索；
- SQLite 负责黑板和事实源；
- Codex CLI、Claude Code 和 Copilot CLI 安装在同一个容器里，后续通过 Bridge 接管；
- Docker Compose 一键启动整套本地开发环境。

## 架构图

```text
用户 / IM / WebSocket / API
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
        +--> Copilot Worker Bridge（可选）
        +--> OpenCode Bridge（可选）
```

## 单容器设计

当前 Docker Compose 只启动一个服务：

```text
flyflor
```

这个容器内部会启动：

- Flyflor Core / Web Console：宿主机开发入口 `http://127.0.0.1:8080`
- Qdrant：容器内 `http://127.0.0.1:6333`
- nanobot WebSocket gateway：宿主机 `ws://localhost:8765`

Docker 开发模式会把 Web Console 映射到宿主机 `127.0.0.1:8080`，只允许本机访问；顶层 nanobot 仍然暴露 `8765`。Qdrant 不映射到宿主机。

基础镜像和 Node 大版本不写死，可以通过环境变量覆盖：

```bash
QDRANT_IMAGE=qdrant/qdrant:latest \
PYTHON_IMAGE=python:3.12-slim \
NODE_MAJOR=22 \
docker compose build
```

容器内已安装：

- `codex`
- `claude`
- `copilot`
- `nanobot`
- `qdrant`
- Python / Node.js / ripgrep / git 等基础工具

## 目录挂载

所有会变化的数据、配置和源码都从宿主机挂载进容器，方便边跑边开发。

```text
./data/flyflor   -> /data/flyflor       # Flyflor SQLite 等数据
./data/qdrant    -> /data/qdrant        # Qdrant 持久化数据
./data/logs      -> /data/logs          # 后台进程日志
./workspace      -> /workspace          # Agent 工作区
./configs        -> /app/configs        # 内置默认配置
./memory         -> /app/memory         # 身份、策略、长期记忆文件
./flyflor_core   -> /app/flyflor_core   # Flyflor Core 源码
./vendor/nanobot -> /app/vendor/nanobot # nanobot 源码
```

## 启动

### 本机一键安装模式

日常使用优先走本机模式。原因是 Codex / Claude / OpenCode / 浏览器 / 桌面自动化都需要真实宿主机能力，放进 Docker 后很多全局电脑操作无法自然完成。

发布后安装形态：

```bash
curl -fsSL https://raw.githubusercontent.com/huaqingyi/flyflor/main/scripts/install.sh | bash
```

安装脚本需要 Python 3.11 或更新版本。它会优先查找 `python3.13`、`python3.12`、`python3.11`，必要时可以显式指定：

```bash
export FLYFLOR_PYTHON=/path/to/python3.11
curl -fsSL https://raw.githubusercontent.com/huaqingyi/flyflor/main/scripts/install.sh | bash
```

安装器会把 Flyflor 和 vendored nanobot 装进 `~/.flyflor/venv` 私有虚拟环境，再把 `flyflor` 命令链接到 `~/.local/bin`，避免污染系统 Python。

当前仓库未发布时，可以在项目根目录本地安装：

```bash
python3.11 -m pip install --user -e .
python3.11 -m pip install --user -e ./vendor/nanobot
npm install -g @openai/codex @anthropic-ai/claude-code @github/copilot
flyflor setup
```

启动本机主入口：

```bash
flyflor gateway
```

Flyflor 没有初始化前不会启动 `gateway` 或 worker。第一次运行必须先：

```bash
flyflor setup
```

开发/CI 可以使用默认配置：

```bash
flyflor setup --defaults
```

如果暂时没有安装 Qdrant：

```bash
flyflor gateway --skip-qdrant
```

如果要并行测试多个实例，可以改 WebSocket 端口：

```bash
flyflor gateway --websocket-port 18765
```

本机模式的暴露原则：

```text
外部只暴露顶层 flyflor/nanobot 渠道
内部 Core 只绑定 127.0.0.1:8080
内部 Qdrant 只绑定 127.0.0.1:6333
```

也就是说，外部世界只应该接触 IM / WebSocket / 飞书 / 钉钉这类顶层渠道，不直接访问 Flyflor Core 或 Qdrant。

### Docker 开发模式

Docker 只作为隔离开发和回归测试环境，不作为最终电脑操控形态。

```bash
docker compose up --build -d
```

查看容器状态：

```bash
docker compose ps
```

查看日志：

```bash
docker logs -f flyflor
tail -f data/logs/qdrant.log
tail -f data/logs/nanobot.log
```

## 健康检查

本机模式下可以直接检查内部服务：

```bash
curl http://localhost:8080/health
curl http://localhost:6333/
```

Docker 开发模式会暴露本机 Web Console：

```bash
open http://127.0.0.1:8080
```

容器内健康检查：

```bash
docker exec flyflor curl -s http://127.0.0.1:8080/health
docker exec flyflor curl -s http://127.0.0.1:6333/
```

容器内 CLI：

```bash
docker exec flyflor flyflor codex --version
docker exec flyflor flyflor claude --version
docker exec flyflor flyflor nanobot --version
```

## Setup 配置

`flyflor setup` 是 Flyflor 的初始化边界，类似 Hermes Agent 的 setup。它会生成：

```text
~/.flyflor/flyflor.json
~/.flyflor/nanobot/config.json
```

Docker 开发模式中对应：

```text
data/flyflor/flyflor.json
data/flyflor/nanobot/config.json
```

`flyflor.json` 负责 Flyflor 自己的系统配置：

- `primary`：初始模型，也就是 nanobot 打到 Flyflor Core 时使用的模型名和 OpenAI-compatible endpoint；
- `channels`：顶层通讯渠道，最终会写入 nanobot 配置；
- `workers`：可用 CLI/Agent worker，后续 Bridge 会从这里选择可执行工具；
- `bridge.guardianPair`：默认互相守护讨论的两个 worker；
- `initialized`：初始化标记。没有这个标记时，Flyflor 会拒绝启动。

初始化命令：

```bash
flyflor setup
```

交互式 setup 会依次处理：

```text
1. Primary route：nanobot -> Flyflor Core 的 OpenAI-compatible 本地路由
2. Channels：选择 WebSocket、飞书、钉钉、QQ、微信、企业微信、Telegram、Slack、WhatsApp、Email
3. Channel fields：按渠道填写 appId、secret、token、allowFrom、groupPolicy 等字段
4. Workers：选择 Codex、Claude、Copilot、OpenCode、Qwen、Kimi、DeepSeek 等 CLI/Agent 工具
5. Guardian pair：选择默认互相守护讨论的左右两个 worker
```

非交互环境不会运行 `flyflor setup`，避免把 token/secret 从管道里误写入配置。开发环境可以使用：

```bash
flyflor setup --defaults
```

查看 worker：

```bash
flyflor workers
```

默认 worker 预设：

```text
codex          OpenAI Codex CLI
claude         Claude Code
copilot        GitHub Copilot CLI
opencode       OpenCode terminal agent
gemini         Gemini CLI
aider          Aider
goose          Goose
qwen-code      Qwen Code CLI
kimi           Kimi CLI 或用户自定义 wrapper
deepseek-tui   DeepSeek CLI 或用户自定义 wrapper
```

`setup` 会先让你选择启用哪些 worker，然后选择默认讨论组合：

```json
{
  "bridge": {
    "mode": "guardian_pair",
    "autoSelect": false,
    "guardianPair": {
      "left": "codex",
      "right": "claude"
    }
  }
}
```

这个组合不是写死的。用户可以配置多个 CLI/Agent 工具，例如 `codex`、`claude`、`copilot`、`opencode`、`qwen-code`、`kimi`、`deepseek-tui`，再在 Web Console 里临时切换左右守护者。后续调度器会基于任务类型自动选择组合，例如图像理解任务优先选择视觉能力强的工具，代码任务优先选择代码能力强的工具。

Web Console 会检测当前选择的两个 worker 是否真的存在于运行环境。如果命令缺失，并且该 worker 有内置安装方式，页面会询问是否自动安装。安装命令不固定版本，只使用包名安装 registry 当前版本，例如：

```bash
npm install -g @openai/codex
npm install -g @anthropic-ai/claude-code
npm install -g @github/copilot
```

自定义 worker 没有内置安装命令时，Flyflor 只提示缺失，不猜测安装方式。

这里不会限制用户必须用某一种工具。任何 CLI/Agent 工具都可以加到 `workers`：

```json
{
  "workers": {
    "my-worker": {
      "enabled": true,
      "kind": "cli",
      "command": ["my-worker", "--some-flag"],
      "env": {},
      "description": "My custom local agent"
    }
  }
}
```

渠道也在 setup 中声明。默认只开启 WebSocket，其它渠道先保留为 disabled：

```json
{
  "channels": {
    "websocket": {
      "enabled": true,
      "host": "0.0.0.0",
      "port": 8765,
      "allowFrom": ["*"]
    },
    "feishu": { "enabled": false },
    "dingtalk": { "enabled": false },
    "qq": { "enabled": false }
  }
}
```

修改 `flyflor.json` 后重启 `flyflor gateway`，Flyflor 会重新生成 nanobot 配置。

## Codex / Claude 配置隔离

Flyflor 使用的 Codex 和 Claude 必须与用户全局 CLI 配置隔离。

Bridge worker 是系统组件，不应该直接读取你的个人 `~/.codex`、`~/.claude`、MCP 配置、历史会话或登录状态。否则 Flyflor 的调试会污染个人环境，任务执行结果也不可复现。

Flyflor 本机模式会设置隔离环境：

```text
HOME=~/.flyflor/agents/home
CODEX_HOME=~/.flyflor/agents/codex
CLAUDE_CONFIG_DIR=~/.flyflor/agents/claude
COPILOT_CONFIG_DIR=~/.flyflor/agents/copilot
XDG_CONFIG_HOME=~/.flyflor/xdg/config
XDG_CACHE_HOME=~/.flyflor/xdg/cache
XDG_DATA_HOME=~/.flyflor/xdg/data
TMPDIR=~/.flyflor/tmp
```

Docker 模式下对应目录在：

```text
/data/flyflor/agents/home
/data/flyflor/agents/codex
/data/flyflor/agents/claude
/data/flyflor/agents/copilot
```

如果要登录或初始化 Flyflor 专用 Codex / Claude，不要直接运行全局命令，而是运行：

```bash
flyflor codex
flyflor claude
```

Docker 内：

```bash
docker exec -it flyflor flyflor codex
docker exec -it flyflor flyflor claude
```

这样登录态、配置、缓存都会落在 Flyflor 内部，不会碰用户全局配置。

## 创建任务

本机开发时可以直接调用 Flyflor Core：

```bash
curl -X POST http://localhost:8080/tasks \
  -H 'content-type: application/json' \
  -d '{"title":"测试任务","input":"让 Codex 和 Claude 讨论当前项目骨架。"}'
```

通过 nanobot 使用的本地 OpenAI-compatible 接口：

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H 'content-type: application/json' \
  -d '{"model":"flyflor-placeholder","messages":[{"role":"user","content":"创建一个 Flyflor 测试任务"}]}'
```

## 当前已跑通链路

已经验证：

```text
WebSocket -> nanobot -> Flyflor /v1/chat/completions -> SQLite blackboard -> WebSocket delta
```

也就是说，外部渠道进来的消息现在可以通过 nanobot 进入 Flyflor Core，并创建黑板任务。

## 顶层入口：flyflor

Flyflor 不直接实现 QQ、飞书、钉钉、Slack、Telegram、邮件这些渠道。

顶层统一使用 `flyflor gateway`。它内部会启动 nanobot gateway，但对外命名和运维入口都是 Flyflor：

```text
通讯渠道 / WebSocket
        |
        v
flyflor gateway
        |
        v
nanobot gateway（内部）
        |
        v
Flyflor OpenAI-compatible endpoint
        |
        v
Flyflor Core / Blackboard / Bridge
```

当前默认配置已经把 nanobot 的 provider 指向容器内部的 Flyflor Core：

```json
{
  "providers": {
    "vllm": {
      "apiBase": "http://127.0.0.1:8080/v1"
    }
  },
  "agents": {
    "defaults": {
      "workspace": "/workspace",
      "provider": "vllm",
      "model": "flyflor-placeholder"
    }
  }
}
```

所以渠道消息进入 nanobot 后，会被转成 OpenAI-compatible 调用，打到：

```text
http://127.0.0.1:8080/v1/chat/completions
```

Flyflor Core 收到后创建任务并写入 SQLite 黑板。后续 Bridge 会从黑板中接管任务，启动 Codex / Claude 协商。

## 通讯渠道初始化

本机模式下，nanobot 配置放在 Flyflor 内部目录：

```text
~/.flyflor/nanobot/config.json
```

本机模式初始化：

```bash
flyflor init
$EDITOR ~/.flyflor/nanobot/config.json
flyflor gateway
```

Docker 开发模式下，nanobot 的运行目录挂载到宿主机：

```text
./data/flyflor/nanobot -> /data/flyflor/nanobot -> /root/.nanobot
```

这意味着：

- `data/flyflor/nanobot/config.json` 是真实生效的 nanobot 配置；
- 所有渠道 token、登录态、二维码登录状态都会持久化在 `data/flyflor/nanobot`；
- 修改配置后只需要重启容器；
- 不需要在宿主机安装 nanobot，也不需要把渠道逻辑写进 Flyflor。

容器模式初始化流程：

```bash
# 1. 启动整套服务
docker compose up --build -d

# 2. 查看 nanobot 是否启动
tail -f data/logs/nanobot.log

# 3. 编辑真实配置
$EDITOR data/flyflor/nanobot/config.json

# 4. 重启，让新渠道配置生效
docker compose restart flyflor

# 5. 继续看 nanobot 日志，确认渠道连接成功
tail -f data/logs/nanobot.log
```

完整渠道文档在：

```text
vendor/nanobot/docs/chat-apps.md
```

下面只写 Flyflor 场景最常用的接入方式。

### 默认 WebSocket 渠道

WebSocket 默认已经开启：

```json
{
  "channels": {
    "websocket": {
      "enabled": true,
      "host": "0.0.0.0",
      "port": 8765,
      "path": "/",
      "websocketRequiresToken": false,
      "allowFrom": ["*"],
      "streaming": true
    }
  }
}
```

烟测命令：

```bash
docker exec -i flyflor python - <<'PY'
import asyncio
import json
import websockets

async def main():
    async with websockets.connect("ws://127.0.0.1:8765/?client_id=smoke") as ws:
        print(await ws.recv())
        await ws.send(json.dumps({"content": "WebSocket 创建 Flyflor 测试任务"}))
        for _ in range(10):
            msg = await asyncio.wait_for(ws.recv(), timeout=180)
            print(msg)
            if "Flyflor task" in msg or "error" in msg:
                break

asyncio.run(main())
PY
```

成功时，消息会经过：

```text
WebSocket -> nanobot -> Flyflor Core -> SQLite blackboard -> WebSocket 回复
```

注意：nanobot 冷启动后的第一次完整处理可能比较慢，尤其是刚重建镜像或第一次加载会话时。烟测脚本的超时故意设成 180 秒，避免客户端太早断开导致日志中出现 `no active subscribers`。

### 飞书 Feishu

飞书使用长连接，不需要公网 IP。

在飞书开放平台创建应用，开启 Bot 能力，拿到 `appId` 和 `appSecret`，然后在 `~/.flyflor/nanobot/config.json` 或 Docker 的 `data/flyflor/nanobot/config.json` 的 `channels` 中加入：

```json
{
  "channels": {
    "feishu": {
      "enabled": true,
      "appId": "cli_xxx",
      "appSecret": "xxx",
      "encryptKey": "",
      "verificationToken": "",
      "allowFrom": ["ou_YOUR_OPEN_ID"],
      "groupPolicy": "mention",
      "streaming": true,
      "domain": "feishu"
    }
  }
}
```

注意：

- `allowFrom` 初期可以先用 `["*"]` 跑通，跑通后再收紧；
- 群聊建议使用 `"groupPolicy": "mention"`；
- 如果飞书应用没有 CardKit 流式权限，把 `"streaming"` 改成 `false`；
- 修改后执行 `docker compose restart flyflor`。

### 钉钉 DingTalk

钉钉使用 Stream Mode，不需要公网 IP。

在钉钉开放平台创建应用，添加机器人能力，开启 Stream Mode，拿到 `clientId` 和 `clientSecret`：

```json
{
  "channels": {
    "dingtalk": {
      "enabled": true,
      "clientId": "YOUR_APP_KEY",
      "clientSecret": "YOUR_APP_SECRET",
      "allowFrom": ["YOUR_STAFF_ID"]
    }
  }
}
```

修改后：

```bash
docker compose restart flyflor
tail -f data/logs/nanobot.log
```

### QQ

QQ 使用 botpy WebSocket，不需要公网 IP。nanobot 当前文档里标注 QQ 主要支持单聊。

在 QQ 开放平台创建机器人，拿到 `appId` 和 `secret`：

```json
{
  "channels": {
    "qq": {
      "enabled": true,
      "appId": "YOUR_APP_ID",
      "secret": "YOUR_APP_SECRET",
      "allowFrom": ["YOUR_OPENID"],
      "msgFormat": "plain"
    }
  }
}
```

开发期建议先在 QQ 机器人沙箱里添加自己的 QQ 号测试。`YOUR_OPENID` 可以从 nanobot 日志里看。

### Slack

Slack 使用 Socket Mode，不需要公网 URL。

配置示例：

```json
{
  "channels": {
    "slack": {
      "enabled": true,
      "botToken": "xoxb-...",
      "appToken": "xapp-...",
      "allowFrom": ["YOUR_SLACK_USER_ID"],
      "groupPolicy": "mention"
    }
  }
}
```

### Telegram

Telegram 需要从 `@BotFather` 创建 bot，拿到 token：

```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "YOUR_BOT_TOKEN",
      "allowFrom": ["YOUR_USER_ID"]
    }
  }
}
```

国内网络环境下 Telegram 通常需要额外网络条件。Flyflor 不在这一层处理网络代理，容器网络可后续单独设计。

### WhatsApp / 微信扫码登录

这类渠道需要在容器内执行 nanobot 登录命令，二维码登录态会保存到 Flyflor 内部 nanobot 目录。

WhatsApp：

```bash
docker exec -it flyflor flyflor nanobot channels login whatsapp
docker compose restart flyflor
```

微信 Weixin：

```bash
docker exec -it flyflor flyflor nanobot channels login weixin
docker compose restart flyflor
```

如果要强制重新登录：

```bash
docker exec -it flyflor flyflor nanobot channels login weixin --force
```

对应配置：

```json
{
  "channels": {
    "weixin": {
      "enabled": true,
      "allowFrom": ["YOUR_WECHAT_USER_ID"]
    }
  }
}
```

### 企业微信 WeCom

企业微信使用长连接，不需要公网 IP。

```json
{
  "channels": {
    "wecom": {
      "enabled": true,
      "botId": "your_bot_id",
      "secret": "your_bot_secret",
      "allowFrom": ["your_id"]
    }
  }
}
```

### Email

Email 使用 IMAP 收信、SMTP 回信。建议给 nanobot 单独准备一个邮箱账号。

```json
{
  "channels": {
    "email": {
      "enabled": true,
      "consentGranted": true,
      "imapHost": "imap.gmail.com",
      "imapPort": 993,
      "imapUsername": "my-nanobot@gmail.com",
      "imapPassword": "your-app-password",
      "smtpHost": "smtp.gmail.com",
      "smtpPort": 587,
      "smtpUsername": "my-nanobot@gmail.com",
      "smtpPassword": "your-app-password",
      "fromAddress": "my-nanobot@gmail.com",
      "allowFrom": ["your-real-email@gmail.com"]
    }
  }
}
```

## Web Console

Web Console 是 Flyflor 的本地控制台，不属于 nanobot 渠道。nanobot 仍然是顶层多渠道入口；Web Console 用于本机观察、调试和人工介入。

Docker 开发模式打开：

```bash
open http://127.0.0.1:8080
```

当前布局：

```text
┌──────────────────────────────┬──────────────────────────────────────┐
│ Conversation                  │ Blackboard                            │
│ 用户输入与飞花最终回复          │ header: Worker A / Worker B selectors │
│ 输入任务并 Send                │ 左右 worker 微信式讨论气泡              │
│                              │ Result / Metrics / Latest              │
└──────────────────────────────┴──────────────────────────────────────┘
```

右侧黑板 header 的左右 worker 来自 `flyflor.json` 的 `bridge.guardianPair`，也可以在页面中临时切换。切换后创建的新任务会把当前 pair 写入任务 metadata，Bridge daemon 会按任务级 pair 执行。

如果当前选择的 worker 命令不存在，Web Console 会显示缺失提示。对内置 worker，页面会给出自动安装按钮；安装不固定版本，只走包名的当前版本。对自定义 worker，Flyflor 不猜安装方式，只提示你修改 setup 或手动安装。

每个 worker 都可以在页面里配置。点击 Worker A / Worker B 卡片里的 `Configure`，可以修改：

- `enabled`：是否启用；
- `kind`：`cli` 或 `agent`；
- `command`：实际执行命令，JSON array，例如 `["kimi"]` 或 `["my-wrapper", "--print"]`；
- `install`：自动安装命令，JSON array，不默认锁版本；
- `env`：隔离环境变量，JSON object，例如 `{"KIMI_HOME":"{home}/agents/kimi"}`；
- `description`：说明文字。

保存后会写入：

```text
data/flyflor/flyflor.json
```

注意 Docker 开发模式和宿主机模式是两个执行环境。比如宿主机已经安装的 `~/.local/bin/kimi`，容器里不会自动看见；Docker 模式需要在容器里也安装一份，或把 worker command 指向容器内可执行的包装脚本。

如果你只是想看 API 数据，也可以直接调用：

```bash
curl http://localhost:8080/tasks
```

Web Console 的设计定位：

```text
用户打开 Web Console
        |
        v
读取 SQLite blackboard
        |
        +--> 当前任务
        +--> Bridge sessions（下一步）
        +--> 任意左右 worker transcript（下一步）
        +--> 决策 / artifact / tool run（下一步）
```

所以它不是 IM 常驻入口，也不负责接收外部消息。IM 消息永远先进 nanobot，Web Console 是本机运维、黑板观察和人工介入面板。

## 记忆设计

Flyflor 的记忆分三层：

### Markdown 身份文件

```text
memory/SOUL.md
memory/USER.md
memory/POLICY.md
memory/AGENTS.md
```

这些文件用于保存稳定身份、用户偏好、策略边界和 Agent 角色定义。

### SQLite

SQLite 是事实源和审计日志。

保存：

- tasks
- blackboard_events
- bridge_sessions
- decisions
- artifacts
- tool_runs
- memory_facts

### Qdrant

Qdrant 只做语义检索，不做事实源。

Qdrant 中的向量记录应该回指 SQLite 行 ID。

## Bridge 原则

Codex 和 Claude 不应该自由聊天到失控。所有有效输出都应进入黑板，并规范成结构化事件：

- `proposal`
- `critique`
- `decision`
- `patch`
- `verification`
- `question`
- `final_report`

黑板拥有主权，worker 输出只是证据，不是最终事实。

## 下一步开发

建议下一步按这个顺序做：

1. 增加 Bridge Session 表和 API。
2. 实现 Codex / Claude bridge health check。
3. 实现 PTY 进程启动、输入、输出采集。
4. 把 Bridge 输出写入 blackboard_events。
5. 设计 Codex / Claude 协商回合协议。
6. 增加任务恢复和超时熔断。
7. 接入 Qdrant 记忆索引。

## 当前定位

Flyflor 不是要替代 nanobot、Codex 或 Claude。

它的定位是：

```text
多渠道入口 + 结构化黑板 + 可恢复 Bridge 协作 + 长期记忆
```

我们只控制最关键的调度、状态和记忆，把现成工具作为可替换的 worker 使用。
