# 🐳 Docker 与快速开始

> 返回 [README](../project/README.zh.md)

## 🐳 Docker Compose

您也可以使用 Docker Compose 运行 Flyflor，无需在本地安装 Go、Node.js 或前端依赖。

```bash
# 1. 克隆仓库
git clone <your-flyflor-repo-url>
cd flyflor

# 2. 准备配置
cp config/config.example.json config/config.json

# 3. 填写 API Key 等配置
vim config/config.json   # 设置 Provider API Key、Bot Token 等

# 4. 正式启动
docker compose up -d flyflor
```

> [!TIP]
> 根目录 Compose 会同时启动 Web 控制台、Gateway 进程和 Qdrant。只有在你希望运行中的容器直接读取本地新构建的 Web 静态资源时，才需要叠加 `docker-compose.webdev.yml`。

```bash
# 5. 查看日志
docker compose logs -f flyflor

# 6. 停止
docker compose down
```

### Web 控制台

根目录 `docker-compose.yml` 默认启动 Flyflor Web 控制台，提供基于浏览器的配置、聊天和黑板界面。

```bash
docker compose up -d flyflor
```

在浏览器中打开 <http://localhost:18800>。Web 后端会在同一容器内管理 Gateway 进程。

> [!WARNING]
> Web 控制台通过 dashboard 登录密码保护。**不要**将它暴露到不可信网络或公网。完整说明见 [配置指南](configuration.md) 中的「Web 启动器控制台」一节。

### Agent 模式 (一次性运行)

```bash
# 提问
docker compose run --rm flyflor -m "2+2 等于几？"

# 交互模式
docker compose run --rm flyflor
```

### 更新镜像

```bash
docker compose pull qdrant
docker compose up -d --build flyflor
```

---

## 🚀 快速开始

> [!TIP]
> 在 `~/.picoclaw/config.json` 中设置您的 API Key。获取 API Key: [火山引擎 (CodingPlan)](https://www.volcengine.com/activity/codingplan?utm_campaign=PicoClaw&utm_content=PicoClaw&utm_medium=devrel&utm_source=OWO&utm_term=PicoClaw) (LLM) · [OpenRouter](https://openrouter.ai/keys) (LLM) · [Zhipu (智谱)](https://open.bigmodel.cn/usercenter/proj-mgmt/apikeys) (LLM)。网络搜索是 **可选的** — 获取免费的 [Tavily API](https://tavily.com) (每月 1000 次免费查询) 或 [Brave Search API](https://brave.com/search/api) (每月 2000 次免费查询)。

**1. 初始化 (Initialize)**

```bash
picoclaw onboard
```

**2. 配置 (Configure)** (`~/.picoclaw/config.json`)

```json
{
  "agents": {
    "defaults": {
      "workspace": "~/.picoclaw/workspace",
      "model_name": "gpt-5.4",
      "max_tokens": 8192,
      "temperature": 0.7,
      "max_tool_iterations": 20
    }
  },
  "model_list": [
    {
      "model_name": "ark-code-latest",
      "provider": "volcengine",
      "model": "ark-code-latest",
      "api_keys": ["sk-your-api-key"],
      "api_base":"https://ark.cn-beijing.volces.com/api/coding/v3"
    },
    {
      "model_name": "gpt-5.4",
      "provider": "openai",
      "model": "gpt-5.4",
      "api_keys": ["your-api-key"],
      "request_timeout": 300
    },
    {
      "model_name": "claude-sonnet-4.6",
      "provider": "anthropic",
      "model": "claude-sonnet-4.6",
      "api_keys": ["your-anthropic-key"]
    }
  ],
  "tools": {
    "web": {
      "enabled": true,
      "fetch_limit_bytes": 10485760,
      "format": "plaintext",
      "brave": {
        "enabled": false,
        "api_key": "YOUR_BRAVE_API_KEY",
        "max_results": 5
      },
      "tavily": {
        "enabled": false,
        "api_key": "YOUR_TAVILY_API_KEY",
        "max_results": 5
      },
      "duckduckgo": {
        "enabled": true,
        "max_results": 5
      },
      "perplexity": {
        "enabled": false,
        "api_key": "YOUR_PERPLEXITY_API_KEY",
        "max_results": 5
      },
      "searxng": {
        "enabled": false,
        "base_url": "http://your-searxng-instance:8888",
        "max_results": 5
      }
    }
  }
}
```

> **新功能**: `model_list` 配置格式支持零代码添加 provider。详见[模型配置](providers.zh.md#模型配置-model_list)章节。
> `request_timeout` 为可选项，单位为秒。若省略或设置为 `<= 0`，PicoClaw 使用默认超时（120 秒）。

**3. 获取 API Key**

* **LLM 提供商**: [OpenRouter](https://openrouter.ai/keys) · [Zhipu](https://open.bigmodel.cn/usercenter/proj-mgmt/apikeys) · [Anthropic](https://console.anthropic.com) · [OpenAI](https://platform.openai.com) · [Gemini](https://aistudio.google.com/api-keys)
* **网络搜索** (可选):
  * [Brave Search](https://brave.com/search/api) - 付费 ($5/1000 次查询，约 $5-6/月)
  * [Perplexity](https://www.perplexity.ai) - AI 驱动的搜索与聊天界面
  * [SearXNG](https://github.com/searxng/searxng) - 自建元搜索引擎（免费，无需 API Key）
  * [Tavily](https://tavily.com) - 专为 AI Agent 优化 (1000 请求/月)
  * DuckDuckGo - 内置回退（无需 API Key）

> **注意**: 完整的配置模板请参考 `config.example.json`。

**4. 对话 (Chat)**

```bash
picoclaw agent -m "2+2 等于几？"
```

就是这样！您在 2 分钟内就拥有了一个可工作的 AI 助手。

---
