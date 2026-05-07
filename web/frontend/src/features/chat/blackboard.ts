import type {
  AssistantMessageKind,
  BlackboardEntry,
  BlackboardPhase,
  BlackboardSpeaker,
  ChatToolCall,
  ChatMessage,
} from "@/store/chat"

const MAX_BLACKBOARD_ENTRIES = 80

const SPEAKER_META: Record<
  BlackboardSpeaker,
  { label: string; side: BlackboardEntry["side"] }
> = {
  codex: { label: "A · Codex Bridge", side: "left" },
  copilot: { label: "B · Copilot Bridge", side: "right" },
  flyflor: { label: "Flyflor", side: "center" },
  system: { label: "Blackboard", side: "center" },
}

function compactContent(content: string, fallback: string) {
  const normalized = content.replace(/\s+/g, " ").trim()
  if (!normalized) return fallback
  if (normalized.length <= 180) return normalized
  return `${normalized.slice(0, 176)}...`
}

function summarizeTask(content: string) {
  return compactContent(content, "用户提交了一个新任务")
}

function createEntry({
  relatedMessageId,
  eventKey,
  speaker,
  phase,
  content,
  timestamp,
  order,
}: {
  relatedMessageId: string
  eventKey?: string
  speaker: BlackboardSpeaker
  phase: BlackboardPhase
  content: string
  timestamp: number
  order: number
}): BlackboardEntry {
  const meta = SPEAKER_META[speaker]
  return {
    id: `${relatedMessageId}:bb:${eventKey ?? phase}:${order}`,
    speaker,
    label: meta.label,
    side: meta.side,
    content,
    timestamp: timestamp + order,
    phase,
    relatedMessageId,
  }
}

export function buildBlackboardEntriesForUserMessage({
  messageId,
  content,
  timestamp,
}: {
  messageId: string
  content: string
  timestamp: number
}): BlackboardEntry[] {
  const task = summarizeTask(content)
  return [
    createEntry({
      relatedMessageId: messageId,
      speaker: "system",
      phase: "received",
      content: `本轮问题：${task}\n阅读方式：这一组只记录本次提问的任务拆解、互检意见、工具检查点和最终结论。`,
      timestamp,
      order: 0,
    }),
    createEntry({
      relatedMessageId: messageId,
      speaker: "codex",
      phase: "planning",
      content:
        "我先拆任务：\n1. 明确用户真正要完成什么。\n2. 判断需要查代码、改 UI、跑验证，还是只需要回答。\n3. 把可执行步骤放到黑板，方便逐项检查。",
      timestamp,
      order: 1,
    }),
    createEntry({
      relatedMessageId: messageId,
      speaker: "copilot",
      phase: "review",
      content:
        "我负责复核：\n1. 看 Codex 有没有误解用户问题。\n2. 找遗漏、风险和边界条件。\n3. 确认最终输出能被用户直接理解。",
      timestamp,
      order: 2,
    }),
    createEntry({
      relatedMessageId: messageId,
      speaker: "flyflor",
      phase: "memory",
      content:
        "记忆检查：读取 Flyflor 的身份、用户偏好和最近上下文。本轮回答要保留自然对话，同时把 bridge 过程清楚展示出来。",
      timestamp,
      order: 3,
    }),
    createEntry({
      relatedMessageId: messageId,
      speaker: "codex",
      phase: "planning",
      content:
        "执行计划：\n1. 先处理最影响体验的问题。\n2. 再补齐架构或界面上的配套信息。\n3. 最后通过构建、测试或截图验证结果。",
      timestamp,
      order: 4,
    }),
    createEntry({
      relatedMessageId: messageId,
      speaker: "copilot",
      phase: "review",
      content:
        "复核标准：黑板不能像日志一样难读。每条记录都要说明“为什么出现、谁负责、下一步是什么”。",
      timestamp,
      order: 5,
    }),
    createEntry({
      relatedMessageId: messageId,
      speaker: "codex",
      phase: "tool",
      content:
        "下一步动作：如果需要动手，我会进入对应文件或工具；如果只是设计判断，我会给出可落地的方案。",
      timestamp,
      order: 6,
    }),
    createEntry({
      relatedMessageId: messageId,
      speaker: "copilot",
      phase: "blocked",
      content:
        "风险提醒：避免只给结论不解释过程；避免把全部事件混在一起，让用户分不清是哪一次提问的黑板。",
      timestamp,
      order: 7,
    }),
    createEntry({
      relatedMessageId: messageId,
      speaker: "flyflor",
      phase: "planning",
      content:
        "本轮检查点：以这次用户问题为边界建立黑板分组。后续模型思考、工具调用和最终回答都会归入同一组。",
      timestamp,
      order: 8,
    }),
  ]
}

export function buildBlackboardConsensusForAssistantMessage({
  messageId,
  content,
  timestamp,
}: {
  messageId: string
  content: string
  timestamp: number
}): BlackboardEntry[] {
  return [
    createEntry({
      relatedMessageId: messageId,
      speaker: "codex",
      phase: "consensus",
      content: `最终回答已生成：${compactContent(content, "Flyflor 已返回最终结果")}`,
      timestamp,
      order: 0,
    }),
    createEntry({
      relatedMessageId: messageId,
      speaker: "copilot",
      phase: "review",
      content:
        "复核完成：最终回答负责交付结果，黑板保留本轮关键判断、执行动作和审查痕迹。",
      timestamp,
      order: 1,
    }),
    createEntry({
      relatedMessageId: messageId,
      speaker: "system",
      phase: "consensus",
      content: "本轮 bridge 已关闭。新的提问会创建新的黑板分组。",
      timestamp,
      order: 2,
    }),
  ]
}

function formatToolCalls(toolCalls?: ChatToolCall[]) {
  if (!toolCalls?.length) return ""
  return toolCalls
    .slice(0, 3)
    .map((toolCall) => {
      const name = toolCall.function?.name || toolCall.type || "tool"
      const explanation = toolCall.extraContent?.toolFeedbackExplanation
      return explanation ? `${name}：${explanation}` : name
    })
    .join("；")
}

export function buildBlackboardEntriesForAssistantEvent({
  messageId,
  eventKey,
  content,
  kind,
  timestamp,
  toolCalls,
}: {
  messageId: string
  eventKey?: string
  content: string
  kind: AssistantMessageKind
  timestamp: number
  toolCalls?: ChatToolCall[]
}): BlackboardEntry[] {
  if (kind === "thought") {
    return [
      createEntry({
        relatedMessageId: messageId,
        eventKey,
        speaker: "codex",
        phase: "planning",
        content: `Codex 思考：${compactContent(content, "正在拆解下一步动作")}`,
        timestamp,
        order: 0,
      }),
      createEntry({
        relatedMessageId: messageId,
        eventKey,
        speaker: "copilot",
        phase: "review",
        content: "Copilot 复核这段思考：确认它是否回答了用户问题，是否需要补充约束、回滚点或用户可见说明。",
        timestamp,
        order: 1,
      }),
    ]
  }

  if (kind === "tool_calls") {
    const tools = compactContent(
      formatToolCalls(toolCalls) || content,
      "准备调用工具执行当前检查点",
    )
    return [
      createEntry({
        relatedMessageId: messageId,
        eventKey,
        speaker: "flyflor",
        phase: "tool",
        content: `工具检查点：${tools}\n说明：这一步会产生外部副作用或读取结果，需要进入黑板留痕。`,
        timestamp,
        order: 0,
      }),
      createEntry({
        relatedMessageId: messageId,
        eventKey,
        speaker: "copilot",
        phase: "review",
        content: "工具审查：检查权限范围、可能副作用、执行结果是否可信，以及是否需要写回 SQLite/Qdrant 记忆。",
        timestamp,
        order: 1,
      }),
    ]
  }

  if (!content.trim()) {
    return []
  }

  return buildBlackboardConsensusForAssistantMessage({
    messageId,
    content,
    timestamp,
  })
}

export function deriveBlackboardEntries(messages: ChatMessage[]) {
  const entries: BlackboardEntry[] = []
  for (const message of messages.slice(-12)) {
    const timestamp =
      typeof message.timestamp === "number" ? message.timestamp : Date.now()
    if (message.role === "user") {
      entries.push(
        ...buildBlackboardEntriesForUserMessage({
          messageId: message.id,
          content: message.content,
          timestamp,
        }),
      )
      continue
    }
    if (message.role === "assistant") {
      entries.push(
        ...buildBlackboardEntriesForAssistantEvent({
          messageId: message.id,
          content: message.content,
          kind: message.kind ?? "normal",
          timestamp,
          toolCalls: message.toolCalls,
        }),
      )
    }
  }
  return trimBlackboardEntries(entries)
}

export function trimBlackboardEntries(entries: BlackboardEntry[]) {
  if (entries.length <= MAX_BLACKBOARD_ENTRIES) return entries
  return entries.slice(entries.length - MAX_BLACKBOARD_ENTRIES)
}

export interface BlackboardTurnGroup {
  id: string
  title: string
  timestamp: number | string
  entries: BlackboardEntry[]
  messageIds: Set<string>
  status: "running" | "complete" | "blocked"
}

export function buildBlackboardTurnGroups({
  messages,
  entries,
}: {
  messages: ChatMessage[]
  entries: BlackboardEntry[]
}): BlackboardTurnGroup[] {
  const groups: BlackboardTurnGroup[] = []
  let current: BlackboardTurnGroup | null = null
  const messageToGroup = new Map<string, BlackboardTurnGroup>()

  for (const message of messages) {
    if (message.role === "user") {
      current = {
        id: message.id,
        title: summarizeTask(message.content),
        timestamp: message.timestamp,
        entries: [],
        messageIds: new Set([message.id]),
        status: "running",
      }
      groups.push(current)
      messageToGroup.set(message.id, current)
      continue
    }

    if (current) {
      current.messageIds.add(message.id)
      messageToGroup.set(message.id, current)
    }
  }

  if (groups.length === 0) {
    return []
  }

  for (const entry of entries) {
    const relatedMessageId = entry.relatedMessageId
    if (!relatedMessageId) continue
    const group = messageToGroup.get(relatedMessageId)
    if (!group) continue
    group.entries.push(entry)
    if (entry.phase === "consensus" && group.status !== "blocked") {
      group.status = "complete"
    }
  }

  return groups.filter((group) => group.entries.length > 0)
}
