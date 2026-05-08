# Flyflor 迭代状态日志

更新时间：2026-05-08

这份日志用于把当前实现进度和 `DESIGN.md` 对齐，方便每轮开发后判断已经落到哪一步、下一步该做什么。

## 状态图例

- `完成`：已有可运行实现，并经过基本验证。
- `部分完成`：已有垂直切片或 prompt/配置约定，但还不是完整架构实现。
- `待实现`：设计中有要求，当前仓库还没有对应实现。

## 当前运行状态

- Docker dev 容器：`flyflor-dev` 正常运行。
- Gateway：`flyflor-gateway` 正常运行。
- WebUI：http://127.0.0.1:8765
- WebUI 密码：`flyflor-dev`
- 模型配置：`fastai` / `gpt-5.5` / `reasoningEffort=high`
- 本地运行配置：`docker/.flyflor/config.json`，已被 `.gitignore` 忽略。

## 设计对齐进度

| DESIGN 章节 | 目标能力 | 当前状态 | 说明 |
| --- | --- | --- | --- |
| 1. 总体架构 | 可观察智能体运行时：入口、复杂度、上下文、LLM、工具、交付 | 部分完成 | Python 运行时已有入口、上下文、LLM、工具和 WebUI；复杂度路由/事件体系还未完整实现。 |
| 2. 三种执行模式 | `direct` / `direct-with-watch` / `blackboard` | 部分完成 | 已新增 `blackboard` 配置，并默认进入 `blackboard`；暂未实现自动 direct/direct-with-watch 路由。 |
| 3. 复杂度计算 | 根据输入特征评分并决定模式 | 待实现 | 当前默认强制黑板讨论模式；复杂度 scorer 和阈值路由还未落地。 |
| 4. 黑板工作台 | Planner/Reviewer、会话 lease、收敛、deadlock 交还 | 部分完成 | 已通过系统提示注入可见 `黑板/讨论/答复` 输出契约；尚未实现独立 scheduler、lease、worker 状态机。 |
| 5. 三层记忆主干 | Markdown / SQLite-Seahorse / Qdrant | 部分完成 | 当前已有 Markdown workspace、session JSONL/manager、Dream memory；Seahorse/Qdrant 设计尚未作为完整模块落地。 |
| 6. 向量索引流程 | 写入、抽取、embedding、Qdrant 搜索召回 | 待实现 | 暂未实现 Qdrant 语义记忆流水线。 |
| 7. ARMS 方法论记忆 | 黑板反思抽取、ARMS store/search | 待实现 | 暂未实现 ARMS。 |
| 8. 记忆唤醒 | Markdown、会话、Qdrant、ARMS 按优先级进入上下文 | 部分完成 | 当前 ContextBuilder 已装配 identity、bootstrap、memory、skills、recent history；Qdrant/ARMS 召回未实现。 |
| 9. 运行时可观察性 | runtime events 暴露给 UI、日志和黑板面板 | 部分完成 | Docker logs/WebSocket 事件已有基础能力；尚未实现 `agent.blackboard.*`、复杂度事件和 UI 黑板面板。 |
| 10. 当前设计边界 | prompt 协作先行，逐步产品化 worker/decision form | 部分完成 | 当前第一步正是 prompt 协作垂直切片。 |
| 11. 一句话总结 | direct/blackboard + 记忆 + ARMS 的完整闭环 | 待实现 | 还处在黑板默认模式和可见讨论的第一阶段。 |

## 已完成迭代

### 2026-05-08 / 迭代 1：默认黑板讨论模式

目标：默认进入黑板和讨论模式，让 WebUI 对话中能直接看到讨论内容。

完成项：

- 新增 `BlackboardConfig`：
  - `enabled`
  - `mode`
  - `showDiscussion`
  - `directThreshold`
  - `threshold`
  - `allowAutoEscalation`
- `Config` 新增顶层 `blackboard`。
- `AgentLoop` 接收 `blackboard_config` 并传给 `ContextBuilder`。
- `ContextBuilder` 默认注入 `agent/blackboard_discussion.md`。
- 新增黑板讨论提示模板，要求输出：
  - `## 黑板`
  - `## 讨论`
  - `## 答复`
- WebSocket bootstrap 增加 `blackboard_mode`。
- WebUI 顶部和空态显示“黑板讨论”模式。
- Docker 本地配置启用：
  - `blackboard.enabled=true`
  - `blackboard.mode=blackboard`
  - `blackboard.showDiscussion=true`

验证：

- `docker compose exec dev flyflor -h` 正常。
- bootstrap 返回 `blackboard_mode=blackboard`。
- 实际 WebUI 对话日志已经出现 `## 黑板`、`## 讨论`。
- 前端定向测试：23 passed。
- Python 定向测试：117 passed；追加复测 47 passed。

遗留问题：

- 当前是 prompt-level 黑板，不是独立 Planner/Reviewer worker。
- 没有复杂度自动路由。
- 没有黑板状态面板和 runtime events。

## 下一步候选

1. 实现复杂度计算最小版本。
   - 输入：用户消息、媒体数量、历史深度、关键词、代码块数量。
   - 输出：`mode`、`score`、`reasons`。
   - 先记录日志和 WebUI 可见，不急着自动切模式。

2. 实现黑板事件流。
   - `agent.turn.start`
   - `agent.complexity.assessed`
   - `agent.blackboard.started`
   - `agent.blackboard.updated`
   - `agent.turn.end`

3. 实现 WebUI 黑板面板。
   - 默认展示当前模式、复杂度分数、Planner/Reviewer 摘要。
   - 从消息正文里拆 `## 黑板` / `## 讨论` 只是过渡方案。

4. 实现真正的 Planner/Reviewer 协作循环。
   - 先做同一模型内的两阶段调用。
   - 再考虑独立 worker、lease、deadlock 检测。

5. 补齐记忆系统。
   - 先稳定 Markdown + session 历史。
   - 再引入 Qdrant/ARMS。

## 更新规则

每次完成一个迭代后，更新三处：

1. `当前运行状态`：端口、容器、模型、配置是否变化。
2. `设计对齐进度`：把对应章节状态从 `待实现` 推到 `部分完成` 或 `完成`。
3. `已完成迭代`：追加日期、目标、完成项、验证和遗留问题。
