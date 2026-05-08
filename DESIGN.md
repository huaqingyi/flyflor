# Flyflor 智能体架构设计总结

本文总结当前 Flyflor 智能体架构的实际设计，包括反思、向量索引、三层记忆、记忆唤醒、黑板协作和复杂度计算。

## 1. 总体架构

Flyflor 是一个可观察的智能体运行时，而不是单纯聊天客户端。一次用户请求会经过：

1. 入口归一化：渠道、会话、媒体、技能、沙箱配置和路由结果被规范成 `DispatchRequest`。
2. 复杂度评估：决定本轮走 `direct`、`direct-with-watch` 还是 `blackboard`。
3. 上下文装配：从 Markdown、SQLite/Seahorse、Qdrant 和可选 ARMS 中唤醒相关记忆。
4. LLM 主循环：执行模型调用、工具调用、转向消息、子任务结果和中断处理。
5. 工具治理：Hook、Sandbox Box、敏感信息过滤、工具结果入库和事件发布。
6. 最终交付：输出用户可读答案，并把可记忆内容写回上下文管理器。

核心路径在：

- `pkg/agent/agent.go`
- `pkg/agent/turn_coord.go`
- `pkg/agent/pipeline_setup.go`
- `pkg/agent/pipeline_execute.go`
- `pkg/agent/context_manager.go`

## 2. 三种执行模式

Flyflor 当前有三种黑板路由模式。

`direct`：普通单智能体路径。不会注入黑板提示，不申请黑板会话租约，延迟最低。

`direct-with-watch`：灰区任务先按 direct 执行，但运行时观察工具 churn、重复失败、上下文压力和多轮工具迭代。一旦发现任务需要协作，会回滚到本轮开始时的 restore point，然后以 blackboard 模式重跑一次。

`blackboard`：复杂任务进入黑板工作台。系统提示中会注入 Planner/Reviewer、收敛预算、死锁交还和反思草稿规范。

这三种模式由 `BlackboardMode` 表示：

- `direct`
- `direct-with-watch`
- `blackboard`

## 3. 复杂度计算

复杂度计算在 `pkg/agent/blackboard_complexity.go`。

输入特征来自当前用户消息、媒体和最近会话历史，主要包括：

- token 估算
- 代码块数量
- diff 或 stacktrace 标记
- 子任务数量
- 是否请求方案、实现、验证、复核
- 会话深度
- 最近工具调用密度
- 是否引用前文继续
- 是否包含媒体
- 是否有风险意图，例如 shell、网络、提交、删除
- 是否显式要求黑板、多智能体、规划器、复核器
- 是否跨文件或多步骤

打分逻辑是固定权重加和后 clamp 到 `[0, 1]`。默认阈值：

- `< 0.35`：`direct`
- `>= 0.35` 且 `< 0.55`：`direct-with-watch`
- `>= 0.55`：`blackboard`

配置只开放：

- `enabled`
- `direct_threshold`
- `threshold`
- `allow_auto_escalation`

权重、黑板轮数和死锁规则是运行时约定，不建议按部署随意调参。

硬门槛会绕过普通分数，直接进入 blackboard：

- 显式要求黑板或多智能体
- 超大输入或多个代码块
- 同时要求实现和验证
- 同时要求实现和复核
- 跨文件工作流且带实现、验证或复核意图

运行时升级条件：

- watched turn 内工具执行达到 3 次
- 同一工具连续失败 2 次
- 第二轮 LLM 仍需要工具
- 主动上下文压缩发现预算压力

升级时会恢复会话历史和摘要，发出 `agent.blackboard.escalated` 事件，并在第二次尝试中强制 blackboard。

## 4. 黑板工作台

黑板工作台在 `pkg/agent/blackboard_scheduler.go`。

默认 worker：

- `flyflor-planner`：拆解目标、提炼上下文、提出执行路径和验证点。
- `flyflor-reviewer`：复核约束、遗漏、风险、边界条件和最终可读性。

调度规则：

- 每个会话同一时间只能有一个 blackboard turn，通过 session lease 防止并发交叉。
- 每个 worker 默认 `MaxConcurrency = 1`。
- worker 上下文预算约 12000 runes。
- 目标 3 轮内收敛，硬上限 5 轮。
- 两轮没有新事实、重复争议、同一 blocker 未解除、重复失败工具路径时视为 livelock。

如果无法收敛，黑板不继续内部争论，而是向用户返回 `flyflor-decision-form` fenced block。WebUI/TUI 可以把它渲染成单选、多选和自定义输入；纯 Markdown 环境也能直接阅读。

黑板提示通过 prompt contributor 注入，仅在 `BlackboardModeBlackboard` 生效。

## 5. 三层记忆主干

当前长期记忆主干是三层：

### 5.1 Markdown 层

位置：

- `workspace/AGENT.md`
- `workspace/SOUL.md`
- `workspace/USER.md`
- `workspace/memory/MEMORY.md`

职责：

- 智能体身份、行为原则和语气
- 用户偏好和稳定工作方式
- 项目长期事实和高信号约束

这层是人可读、可编辑的宪法层。ContextBuilder 会跟踪文件 mtime，文件变化后下一轮自动进入上下文。

### 5.2 SQLite/Seahorse 层

位置和实现：

- `pkg/seahorse`
- 默认数据库：`workspace/sessions/seahorse.db`
- 会话 JSONL 后端：`pkg/memory`、`pkg/session`

职责：

- 保存结构化会话时间线
- 记录 tool use、tool result、media part 和 reasoning content
- 做短期/中期上下文装配
- 在预算压力下压缩摘要
- 提供 grep/expand 类精确检索工具

Seahorse 是默认 `context_manager`，负责每轮 `Assemble`、`Ingest`、`Compact`、`Clear`。

### 5.3 Qdrant 语义向量层

位置：

- `pkg/semanticmemory`

职责：

- 从有意义的 user/assistant 消息中抽取偏好、项目事实、记忆设计要求和交付经验。
- 生成 embedding。
- 写入 Qdrant collection。
- 下一轮根据当前 query 做语义召回。

默认 collection 是 `flyflor_memories`，默认维度 256，默认 `top_k = 6`。没有外部 embedding 配置时使用 hash embedding；配置 OpenAI-like embedding 后可使用真实向量模型。

Qdrant 层输出格式为 `VECTOR_MEMORY`，提示模型把它当作有用线索，而不是高于用户当前指令的事实。

## 6. 向量索引流程

Qdrant 向量索引发生在 `ContextManager.Ingest`。

写入路径：

1. 用户消息或助手消息被持久化到 session。
2. `turnState.ingestMessage` 调用当前 `ContextManager.Ingest`。
3. Seahorse 先写 SQLite。
4. 如果 semantic memory 已启用，`semanticmemory.Manager.IndexMessage` 抽取候选记忆。
5. 每条候选记忆生成 embedding。
6. 使用稳定 UUID 写入 Qdrant，payload 包含 text、kind、session_key、role、created_at、source、schema_version。

抽取规则：

- user 消息：偏好、项目事实、记忆设计要求、长文本摘要。
- assistant 消息：交付结果、验证经验、风险和设计结论。
- `Methodology Reflection Draft` 会先被剥离，避免方法论反思混入普通语义记忆。

召回路径：

1. `SetupTurn` 调用 `ContextManager.Assemble`。
2. 当前 user message 作为 query。
3. Seahorse 先装配会话上下文和摘要。
4. Qdrant 搜索 topK 并按 `min_score` 过滤。
5. 命中项追加到 summary/context block 中。

## 7. 反思与 ARMS 方法论记忆

反思系统不是普通记忆的一部分，而是隔离的方法论记忆侧车。

反思入口是黑板提示中的输出约定：

```markdown
## Methodology Reflection Draft

- Situation: When this method applies.
- Method: The repeatable approach.
- Avoid: What failed or caused delay.
- Next-time hint: A short cue for retrieval before future planning.
```

ARMS 只索引 assistant 消息里的 `Methodology Reflection Draft` section。用户文本、普通助手总结、项目事实、偏好和会话摘要都不会进入 ARMS。

实现位置：

- `pkg/armsmemory`
- `pkg/agent/context_arms.go`

写入路径：

1. blackboard turn 产出 Methodology Reflection Draft。
2. `armsContextManager.Ingest` 包装基础 ContextManager。
3. `armsmemory.ExtractMethodologies` 严格抽取反思 section。
4. 写入 ARMS store。

读取路径：

1. `armsContextManager.Assemble` 先调用基础 ContextManager。
2. 用当前 query 搜索 ARMS。
3. 命中结果追加为 `METHODOLOGY_MEMORY`。
4. 模型把它作为规划和复核的可复用方法，而不是用户事实。

本地 ARMS driver：

- SQLite 是 source of truth。
- HNSW 用于语义近邻。
- R-tree 用于 8 维“方法论空间”检索。
- 8 个坐标轴大致对应 code、docs、architecture、deploy、test、blackboard、risk、reflection/memory。

ARMS 也支持 HTTP driver，接口包括 upsert、search 和 status。

## 8. 记忆唤醒

“唤醒”发生在模型调用前的 `Assemble` 阶段，而不是模型回答后。

当前顺序：

1. Markdown 层由 ContextBuilder 作为静态/半静态 prompt 贡献者进入系统提示。
2. Seahorse 根据 session key 和预算装配最近消息、摘要和压缩上下文。
3. Qdrant 用当前 user query 召回 `VECTOR_MEMORY`。
4. ARMS 如果启用，再用同一 query 召回 `METHODOLOGY_MEMORY`。
5. BuildMessages 把这些内容合并到最终 provider messages。

记忆优先级原则：

- 当前用户指令最高。
- Markdown 身份和用户偏好是长期约束。
- Seahorse 会话上下文提供最近事实和过程。
- Qdrant 是相关事实线索，需要被核对。
- ARMS 是方法论建议，只影响“怎么做”，不直接当事实。

## 9. 运行时可观察性

架构通过 runtime events 把内部状态暴露给 UI、日志和黑板面板。

关键事件：

- `agent.turn.start`
- `agent.turn.end`
- `agent.complexity.assessed`
- `agent.blackboard.escalated`
- `agent.sandbox.assessed`
- `agent.llm.request`
- `agent.llm.response`
- `agent.tool.exec_start`
- `agent.tool.exec_end`
- `agent.tool.exec_skipped`
- `agent.context.compress`
- `agent.steering.injected`

复杂度事件会带上 mode、score、threshold、reasons、hardGate 和完整 features，便于解释为什么某轮进了黑板或保持 direct。

## 10. 当前设计边界

已经实现：

- 默认 Seahorse context manager。
- Qdrant semantic memory 的索引、召回和 session delete。
- ARMS 本地/HTTP 配置、严格反思抽取、本地 SQLite + HNSW + R-tree。
- blackboard complexity routing。
- direct-with-watch 自动升级、回滚、重跑。
- 黑板 session lease、默认 Planner/Reviewer、收敛和 deadlock 提示约定。

仍属于约定或待继续产品化的部分：

- Planner/Reviewer 目前主要通过 prompt 协作约定体现，不是完全独立的外部进程。
- `flyflor-decision-form` 已有 Markdown/JSON 约定，UI 原生控件渲染仍需继续完善。
- Methodology Reflection Draft 已可被 ARMS 吃入，但是否每个复杂 turn 都产出反思仍依赖模型遵守提示。
- Qdrant 抽取规则目前是启发式关键词和压缩文本，后续可换成更强的结构化抽取器。

## 11. 一句话总结

Flyflor 当前架构是：用复杂度计算决定 direct 或 blackboard；用黑板把复杂工作变成可观察、可收敛、可交还的协作流程；用 Markdown、SQLite/Seahorse、Qdrant 构成三层记忆主干；再用 ARMS 隔离存储黑板反思，让未来任务在规划前唤醒可复用方法。
