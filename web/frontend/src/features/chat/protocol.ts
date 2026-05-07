import { toast } from "sonner"

import {
  parseAssistantMessageCreateState,
  parseAssistantMessageUpdateState,
} from "@/features/chat/assistant-message-state"
import {
  buildBlackboardEntriesForAssistantEvent,
  trimBlackboardEntries,
} from "@/features/chat/blackboard"
import { normalizeUnixTimestamp } from "@/features/chat/state"
import {
  type AssistantMessageKind,
  type ChatAttachment,
  type ContextUsage,
  updateChatStore,
} from "@/store/chat"

export interface PicoMessage {
  type: string
  id?: string
  session_id?: string
  timestamp?: number | string
  payload?: Record<string, unknown>
}

function parseAttachments(
  payload: Record<string, unknown>,
): ChatAttachment[] | undefined {
  const raw = payload.attachments
  if (!Array.isArray(raw)) {
    return undefined
  }

  const attachments: ChatAttachment[] = []
  for (const item of raw) {
    if (!item || typeof item !== "object") {
      continue
    }

    const attachment = item as Record<string, unknown>
    const url = typeof attachment.url === "string" ? attachment.url : ""
    if (!url) {
      continue
    }

    const type =
      attachment.type === "audio" ||
      attachment.type === "video" ||
      attachment.type === "file" ||
      attachment.type === "image"
        ? attachment.type
        : "file"

    const filename =
      typeof attachment.filename === "string" ? attachment.filename : undefined
    const contentType =
      typeof attachment.content_type === "string"
        ? attachment.content_type
        : undefined

    attachments.push({
      type,
      url,
      ...(filename ? { filename } : {}),
      ...(contentType ? { contentType } : {}),
    })
  }

  return attachments.length > 0 ? attachments : undefined
}

function parseContextUsage(
  payload: Record<string, unknown>,
): ContextUsage | undefined {
  const raw = payload.context_usage
  if (!raw || typeof raw !== "object") return undefined
  const obj = raw as Record<string, unknown>
  const used = Number(obj.used_tokens)
  const total = Number(obj.total_tokens)
  if (!Number.isFinite(used) || !Number.isFinite(total) || total <= 0)
    return undefined
  return {
    used_tokens: used,
    total_tokens: total,
    compress_at_tokens: Number(obj.compress_at_tokens) || 0,
    used_percent: Number(obj.used_percent) || 0,
  }
}

function hashString(value: string) {
  let hash = 5381
  for (let i = 0; i < value.length; i += 1) {
    hash = (hash * 33) ^ value.charCodeAt(i)
  }
  return (hash >>> 0).toString(36)
}

function blackboardEventKey({
  kind,
  content,
  toolCalls,
}: {
  kind: AssistantMessageKind
  content: string
  toolCalls?: ReturnType<typeof parseAssistantMessageUpdateState>["toolCalls"]
}) {
  if (kind === "normal") return "consensus"
  const toolNames =
    toolCalls
      ?.map((toolCall) => toolCall.function?.name || toolCall.type || "tool")
      .join("|") ?? ""
  return `${kind}:${hashString(`${content}|${toolNames}`)}`
}

export function handlePicoMessage(
  message: PicoMessage,
  expectedSessionId: string,
) {
  if (message.session_id && message.session_id !== expectedSessionId) {
    return
  }

  const payload = message.payload || {}

  switch (message.type) {
    case "message.create":
    case "media.create": {
      const messageId = (payload.message_id as string) || `pico-${Date.now()}`
      const { content, kind, toolCalls } =
        parseAssistantMessageCreateState(payload)
      const attachments = parseAttachments(payload)
      const contextUsage = parseContextUsage(payload)
      const timestamp =
        message.timestamp !== undefined &&
        Number.isFinite(Number(message.timestamp))
          ? normalizeUnixTimestamp(Number(message.timestamp))
          : Date.now()

      updateChatStore((prev) => ({
        messages: [
          ...prev.messages,
          {
            id: messageId,
            role: "assistant",
            content,
            kind,
            ...(toolCalls ? { toolCalls } : {}),
            attachments,
            timestamp,
          },
        ],
        blackboardEntries: trimBlackboardEntries([
          ...prev.blackboardEntries,
          ...buildBlackboardEntriesForAssistantEvent({
            messageId,
            eventKey: blackboardEventKey({ kind, content, toolCalls }),
            content,
            kind,
            timestamp,
            toolCalls,
          }),
        ]),
        isTyping: false,
        ...(contextUsage ? { contextUsage } : {}),
      }))
      break
    }

    case "message.update": {
      const messageId = payload.message_id as string
      const attachments = parseAttachments(payload)
      const contextUsage = parseContextUsage(payload)
      const timestamp =
        message.timestamp !== undefined &&
        Number.isFinite(Number(message.timestamp))
          ? normalizeUnixTimestamp(Number(message.timestamp))
          : Date.now()
      if (!messageId) {
        break
      }

      updateChatStore((prev) => {
        let nextKind: AssistantMessageKind = "normal"
        let nextContent = ""
        let nextToolCalls: ReturnType<
          typeof parseAssistantMessageUpdateState
        >["toolCalls"]
        const messages = (() => {
          let found = false
          const messages = prev.messages.map((msg) => {
            if (msg.id !== messageId) {
              return msg
            }
            found = true
            const { content, kind, toolCalls } =
              parseAssistantMessageUpdateState(payload, msg)
            nextKind = kind
            nextContent = content
            nextToolCalls = toolCalls
            return {
              ...msg,
              id: messageId,
              content,
              kind,
              toolCalls,
              ...(attachments ? { attachments } : {}),
            }
          })
          if (found) {
            return messages
          }

          const { content, kind, toolCalls } =
            parseAssistantMessageUpdateState(payload)
          nextKind = kind
          nextContent = content
          nextToolCalls = toolCalls

          return [
            ...messages,
            {
              id: messageId,
              role: "assistant" as const,
              content,
              kind,
              toolCalls,
              ...(attachments ? { attachments } : {}),
              timestamp,
            },
          ]
        })()
        const eventKey = blackboardEventKey({
          kind: nextKind,
          content: nextContent,
          toolCalls: nextToolCalls,
        })
        const hasSameEvent = prev.blackboardEntries.some((entry) =>
          entry.id.startsWith(`${messageId}:bb:${eventKey}:`),
        )
        return {
          messages,
          blackboardEntries:
            (nextContent || nextToolCalls?.length) && !hasSameEvent
              ? trimBlackboardEntries([
                  ...prev.blackboardEntries,
                  ...buildBlackboardEntriesForAssistantEvent({
                    messageId,
                    eventKey,
                    content: nextContent,
                    kind: nextKind,
                    timestamp,
                    toolCalls: nextToolCalls,
                  }),
                ])
              : prev.blackboardEntries,
          ...(contextUsage ? { contextUsage } : {}),
        }
      })
      break
    }

    case "message.delete": {
      const messageId = payload.message_id as string
      if (!messageId) {
        break
      }

      updateChatStore((prev) => ({
        messages: prev.messages.filter((msg) => msg.id !== messageId),
        blackboardEntries: prev.blackboardEntries.filter(
          (entry) => entry.relatedMessageId !== messageId,
        ),
      }))
      break
    }

    case "typing.start":
      updateChatStore({ isTyping: true })
      break

    case "typing.stop":
      updateChatStore({ isTyping: false })
      break

    case "error": {
      const requestId =
        typeof payload.request_id === "string" ? payload.request_id : ""
      const errorMessage =
        typeof payload.message === "string" ? payload.message : ""

      console.error("Pico error:", payload)
      if (errorMessage) {
        toast.error(errorMessage)
      }
      updateChatStore((prev) => ({
        messages: requestId
          ? prev.messages.filter((msg) => msg.id !== requestId)
          : prev.messages,
        blackboardEntries: requestId
          ? prev.blackboardEntries.filter(
              (entry) => entry.relatedMessageId !== requestId,
            )
          : prev.blackboardEntries,
        isTyping: false,
      }))
      break
    }

    case "pong":
      break

    default:
      console.log("Unknown pico message type:", message.type)
  }
}
