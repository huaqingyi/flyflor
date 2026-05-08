# Flyflor 飞花

<p align="center">
  <img src="./images/flyflor_avatar.png" alt="Flyflor avatar" width="180" />
</p>

Flyflor（飞花）是一个面向长期协作的个人智能体运行时。它从轻量 agent loop 起步，保留多渠道接入、工具调用、长期记忆、MCP、OpenAI-compatible API、WebUI 和部署能力，并逐步演进到 `DESIGN.md` 描述的可观察、多记忆层、黑板协作架构。

## 核心方向

- **飞花身份**：以“白发、花饰、晶翼、紫粉/青蓝能量”的 IP 视觉作为产品识别，默认语气保持清晰、克制、可靠。
- **可观察运行时**：把请求入口、上下文装配、模型循环、工具治理、事件输出和最终交付拆成可追踪路径。
- **三层记忆**：以 Markdown 宪法层、SQLite/Seahorse 会话层、Qdrant 语义向量层组成长期记忆主干。
- **黑板协作**：复杂任务通过复杂度评估进入 direct、direct-with-watch 或 blackboard 模式，由 Planner/Reviewer 约束收敛。
- **多渠道交付**：保留 CLI、WebUI、Telegram、Discord、Slack、飞书、企业微信、钉钉、邮件、WebSocket 等通道能力。

## 安装

源码开发：

```bash
pip install -e .
```

安装后命令为：

```bash
flyflor --help
```

## 快速开始

初始化配置和工作区：

```bash
flyflor onboard
```

默认配置路径：

```text
~/.flyflor/config.json
```

最小模型配置示例：

```json
{
  "providers": {
    "openrouter": {
      "apiKey": "sk-or-v1-xxx"
    }
  },
  "agents": {
    "defaults": {
      "provider": "openrouter",
      "model": "anthropic/claude-opus-4-6"
    }
  }
}
```

启动 CLI 对话：

```bash
flyflor agent
```

启动网关与 WebUI 通道：

```bash
flyflor gateway
```

## Python SDK

```python
from flyflor import Flyflor

bot = Flyflor.from_config()
result = await bot.run("总结这个工作区的当前状态")
print(result.content)
```

## 项目结构

- `flyflor/`：Python 运行时代码。
- `webui/`：Vite/React WebUI。
- `bridge/`：WhatsApp bridge。
- `docs/`：配置、部署、通道和 SDK 文档。
- `tests/`：Python 与 WebUI 回归测试。
- `DESIGN.md`：飞花目标架构说明。

## 当前定制状态

本仓库已经完成基础品牌迁移：包名、CLI 命令、默认数据目录、WebUI 文案、SDK facade、测试导入和品牌图片均使用 flyflor/飞花。下一阶段可以继续把 `DESIGN.md` 中的黑板复杂度路由、Seahorse/Qdrant/ARMS 记忆侧车按当前 Python 运行时落地。
