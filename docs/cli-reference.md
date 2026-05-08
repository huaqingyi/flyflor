# CLI Reference

| Command | Description |
|---------|-------------|
| `flyflor onboard` | Initialize config & workspace at `~/.flyflor/` |
| `flyflor onboard --wizard` | Launch the interactive onboarding wizard |
| `flyflor onboard -c <config> -w <workspace>` | Initialize or refresh a specific instance config and workspace |
| `flyflor agent -m "..."` | Chat with the agent |
| `flyflor agent -w <workspace>` | Chat against a specific workspace |
| `flyflor agent -w <workspace> -c <config>` | Chat against a specific workspace/config |
| `flyflor agent` | Interactive chat mode |
| `flyflor agent --no-markdown` | Show plain-text replies |
| `flyflor agent --logs` | Show runtime logs during chat |
| `flyflor serve` | Start the OpenAI-compatible API |
| `flyflor gateway` | Start the gateway |
| `flyflor status` | Show status |
| `flyflor provider login openai-codex` | OAuth login for providers |
| `flyflor channels login <channel>` | Authenticate a channel interactively |
| `flyflor channels status` | Show channel status |

Interactive mode exits: `exit`, `quit`, `/exit`, `/quit`, `:q`, or `Ctrl+D`.
