# 🐳 Docker & Quick Start Guide

> Back to [README](../README.md)

## 🐳 Docker Compose

You can run Flyflor with Docker Compose without installing Go, Node.js, or frontend dependencies locally.

```bash
# 1. Clone this repo
git clone <your-flyflor-repo-url>
cd flyflor

# 2. Prepare config
cp config/config.example.json config/config.json

# 3. Set your API keys
vim config/config.json   # Set provider API keys, bot tokens, etc.

# 4. Start
docker compose up -d flyflor
```

> [!TIP]
> **Docker Users**: By default, the Gateway listens on `127.0.0.1` which is not accessible from the host. If you need to access the health endpoints or expose ports, set `PICOCLAW_GATEWAY_HOST=0.0.0.0` in your environment or update `config.json`.

> [!NOTE]
> The root Compose stack runs the web console, gateway process, and Qdrant together. Use `docker-compose.webdev.yml` only when you want the running container to read freshly built local Web assets without rebuilding the image.

```bash
# 5. Check logs
docker compose logs -f flyflor

# 6. Stop
docker compose down
```

### Web Console

The root `docker-compose.yml` starts the Flyflor web console by default. It provides the browser UI for configuration, chat, and the blackboard.

```bash
docker compose up -d flyflor
```

Open http://localhost:18800 in your browser. The web backend manages the gateway process inside the same container.

> [!WARNING]
> The web console is protected by dashboard password login. **Do not** expose it to untrusted networks or the public internet. See [Web launcher dashboard](configuration.md#web-launcher-dashboard) in the Configuration Guide.

### Agent Mode (One-shot)

```bash
# Ask a question
docker compose run --rm flyflor -m "What is 2+2?"

# Interactive mode
docker compose run --rm flyflor
```

### Update

```bash
docker compose pull qdrant
docker compose up -d --build flyflor
```

### 🚀 Quick Start

> [!TIP]
> Set your API Key in `~/.picoclaw/config.json`. Get API Keys: [Volcengine (CodingPlan)](https://www.volcengine.com/activity/codingplan?utm_campaign=PicoClaw&utm_content=PicoClaw&utm_medium=devrel&utm_source=OWO&utm_term=PicoClaw) (LLM) · [OpenRouter](https://openrouter.ai/keys) (LLM) · [Zhipu](https://open.bigmodel.cn/usercenter/proj-mgmt/apikeys) (LLM). Web search is optional — get a free [Tavily API](https://tavily.com) (1000 free queries/month) or [Brave Search API](https://brave.com/search/api) (2000 free queries/month).

**1. Initialize**

```bash
picoclaw onboard
```

**2. Configure** (`~/.picoclaw/config.json`)

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

> **New**: The `model_list` configuration format allows zero-code provider addition. See [Model Configuration](#model-configuration-model_list) for details.
> `request_timeout` is optional and uses seconds. If omitted or set to `<= 0`, PicoClaw uses the default timeout (120s).

**3. Get API Keys**

* **LLM Provider**: [OpenRouter](https://openrouter.ai/keys) · [Zhipu](https://open.bigmodel.cn/usercenter/proj-mgmt/apikeys) · [Anthropic](https://console.anthropic.com) · [OpenAI](https://platform.openai.com) · [Gemini](https://aistudio.google.com/api-keys)
* **Web Search** (optional):
  * [Brave Search](https://brave.com/search/api) - Paid ($5/1000 queries, ~$5-6/month)
  * [Perplexity](https://www.perplexity.ai) - AI-powered search with chat interface
  * [SearXNG](https://github.com/searxng/searxng) - Self-hosted metasearch engine (free, no API key needed)
  * [Tavily](https://tavily.com) - Optimized for AI Agents (1000 requests/month)
  * DuckDuckGo - Built-in fallback (no API key required)

> **Note**: See `config.example.json` for a complete configuration template.

**4. Chat**

```bash
picoclaw agent -m "What is 2+2?"
```

That's it! You have a working AI assistant in 2 minutes.

---
