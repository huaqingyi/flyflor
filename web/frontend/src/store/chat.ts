import { atom, getDefaultStore } from "jotai"
import { atomWithStorage } from "jotai/utils"

import {
  getInitialActiveSessionId,
  writeStoredSessionId,
} from "@/features/chat/state"

export interface ChatAttachment {
  type: "image" | "audio" | "video" | "file"
  url: string
  filename?: string
  contentType?: string
}

export interface ChatToolCallFunction {
  name?: string
  arguments?: string
}

export interface ChatToolCallExtraContent {
  toolFeedbackExplanation?: string
}

export interface ChatToolCall {
  id?: string
  type?: string
  function?: ChatToolCallFunction
  extraContent?: ChatToolCallExtraContent
}

export type AssistantMessageKind = "normal" | "thought" | "tool_calls"

export type BlackboardSpeaker = "codex" | "copilot" | "flyflor" | "system"
export type BlackboardSide = "left" | "right" | "center"
export type BlackboardPhase =
  | "received"
  | "planning"
  | "review"
  | "tool"
  | "consensus"
  | "memory"
  | "blocked"

export interface BlackboardEntry {
  id: string
  speaker: BlackboardSpeaker
  label: string
  side: BlackboardSide
  content: string
  timestamp: number | string
  phase?: BlackboardPhase
  relatedMessageId?: string
}

export interface ChatMessage {
  id: string
  role: "user" | "assistant"
  content: string
  timestamp: number | string
  kind?: AssistantMessageKind
  attachments?: ChatAttachment[]
  toolCalls?: ChatToolCall[]
}

export interface ContextUsage {
  used_tokens: number
  total_tokens: number
  compress_at_tokens: number
  used_percent: number
}

export type ConnectionState =
  | "disconnected"
  | "connecting"
  | "connected"
  | "error"

export interface ChatStoreState {
  messages: ChatMessage[]
  blackboardEntries: BlackboardEntry[]
  connectionState: ConnectionState
  isTyping: boolean
  activeSessionId: string
  hasHydratedActiveSession: boolean
  contextUsage?: ContextUsage
}

type ChatStorePatch = Partial<ChatStoreState>

// Keep the legacy storage value so existing user preferences survive the rename.
const SHOW_ASSISTANT_DETAILS_STORAGE_KEY = "flyflor:chat-show-runtime-events"

const DEFAULT_CHAT_STATE: ChatStoreState = {
  messages: [],
  blackboardEntries: [],
  connectionState: "disconnected",
  isTyping: false,
  activeSessionId: getInitialActiveSessionId(),
  hasHydratedActiveSession: false,
}

export const chatAtom = atom<ChatStoreState>(DEFAULT_CHAT_STATE)
export const showAssistantDetailsAtom = atomWithStorage<boolean>(
  SHOW_ASSISTANT_DETAILS_STORAGE_KEY,
  true,
)

const store = getDefaultStore()

export function getChatState() {
  return store.get(chatAtom)
}

export function updateChatStore(
  patch:
    | ChatStorePatch
    | ((prev: ChatStoreState) => ChatStorePatch | ChatStoreState),
) {
  store.set(chatAtom, (prev) => {
    const nextPatch = typeof patch === "function" ? patch(prev) : patch
    const next = { ...prev, ...nextPatch }

    if (next.activeSessionId !== prev.activeSessionId) {
      writeStoredSessionId(next.activeSessionId)
    }

    return next
  })
}
