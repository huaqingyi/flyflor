import {
  IconBook2,
  IconChevronLeft,
  IconChevronRight,
  IconGitBranch,
  IconMessageCircle2,
  IconRefresh,
  IconX,
} from "@tabler/icons-react"
import { useEffect, useMemo, useState } from "react"

import {
  buildBlackboardTurnGroups,
  type BlackboardTurnGroup,
} from "@/features/chat/blackboard"
import { formatMessageTime } from "@/hooks/use-pico-chat"
import { cn } from "@/lib/utils"
import type {
  BlackboardEntry,
  BlackboardPhase,
  BlackboardSpeaker,
  ChatMessage,
} from "@/store/chat"

interface BlackboardPanelProps {
  entries: BlackboardEntry[]
  messages: ChatMessage[]
  isTyping?: boolean
  defaultOpen?: boolean
  lockedOpen?: boolean
  className?: string
  variant?: "inline" | "side"
}

const SPEAKER_STYLE: Record<
  BlackboardSpeaker,
  { avatar: string; name: string; className: string; bubble: string }
> = {
  codex: {
    avatar: "A",
    name: "Codex Bridge",
    className: "bg-cyan-500 text-white",
    bubble:
      "border-cyan-300/70 bg-cyan-50 text-cyan-950 dark:border-cyan-500/50 dark:bg-cyan-950/30 dark:text-cyan-50",
  },
  copilot: {
    avatar: "B",
    name: "Copilot Guard",
    className: "bg-rose-500 text-white",
    bubble:
      "border-rose-300/70 bg-rose-50 text-rose-950 dark:border-rose-500/50 dark:bg-rose-950/30 dark:text-rose-50",
  },
  flyflor: {
    avatar: "F",
    name: "Flyflor",
    className: "bg-violet-500 text-white",
    bubble:
      "border-violet-300/70 bg-violet-50 text-violet-950 dark:border-violet-500/50 dark:bg-violet-950/30 dark:text-violet-50",
  },
  system: {
    avatar: "·",
    name: "Blackboard",
    className: "bg-zinc-500 text-white",
    bubble:
      "border-border/60 bg-muted/40 text-muted-foreground dark:bg-muted/20",
  },
}

const PHASE_LABEL: Record<BlackboardPhase, string> = {
  received: "接收",
  planning: "拆解",
  review: "审查",
  tool: "工具",
  consensus: "结论",
  memory: "记忆",
  blocked: "风险",
}

const PHASE_HELP: Record<BlackboardPhase, string> = {
  received: "本轮问题边界",
  planning: "怎么做",
  review: "哪里可能错",
  tool: "执行或验证",
  consensus: "交付结果",
  memory: "使用了哪些上下文",
  blocked: "需要留意的限制",
}

const PHASE_STYLE: Record<BlackboardPhase, string> = {
  received: "border-sky-300 bg-sky-50 text-sky-700 dark:border-sky-500/40 dark:bg-sky-950/30 dark:text-sky-200",
  planning:
    "border-cyan-300 bg-cyan-50 text-cyan-700 dark:border-cyan-500/40 dark:bg-cyan-950/30 dark:text-cyan-200",
  review:
    "border-rose-300 bg-rose-50 text-rose-700 dark:border-rose-500/40 dark:bg-rose-950/30 dark:text-rose-200",
  tool: "border-amber-300 bg-amber-50 text-amber-700 dark:border-amber-500/40 dark:bg-amber-950/30 dark:text-amber-200",
  consensus:
    "border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-500/40 dark:bg-emerald-950/30 dark:text-emerald-200",
  memory:
    "border-violet-300 bg-violet-50 text-violet-700 dark:border-violet-500/40 dark:bg-violet-950/30 dark:text-violet-200",
  blocked:
    "border-orange-300 bg-orange-50 text-orange-700 dark:border-orange-500/40 dark:bg-orange-950/30 dark:text-orange-200",
}

function PhaseChip({ phase }: { phase?: BlackboardPhase }) {
  if (!phase) return null
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center rounded border px-1.5 py-0.5 text-[10px] font-semibold leading-none",
        PHASE_STYLE[phase],
      )}
    >
      {PHASE_LABEL[phase]}
    </span>
  )
}

function groupStatusLabel(group: BlackboardTurnGroup) {
  if (group.status === "complete") return "已完成"
  if (group.status === "blocked") return "需处理"
  return "进行中"
}

function groupStatusClass(group: BlackboardTurnGroup) {
  if (group.status === "complete") {
    return "text-emerald-600 dark:text-emerald-300"
  }
  if (group.status === "blocked") {
    return "text-orange-600 dark:text-orange-300"
  }
  return "text-cyan-600 dark:text-cyan-300"
}

function formatGroupTimestamp(group: BlackboardTurnGroup) {
  const timestamp = formatMessageTime(group.timestamp)
  return timestamp || "刚刚"
}

function BlackboardAvatar({ speaker }: { speaker: BlackboardSpeaker }) {
  const style = SPEAKER_STYLE[speaker]
  return (
    <div
      className={cn(
        "flex size-7 shrink-0 items-center justify-center rounded-md text-xs font-semibold shadow-sm",
        style.className,
      )}
    >
      {style.avatar}
    </div>
  )
}

function EntryReason({ phase }: { phase?: BlackboardPhase }) {
  if (!phase) return null
  return (
    <span className="text-muted-foreground/70 text-[10px]">
      {PHASE_HELP[phase]}
    </span>
  )
}

function BlackboardBubble({ entry }: { entry: BlackboardEntry }) {
  const style = SPEAKER_STYLE[entry.speaker]
  const timestamp = formatMessageTime(entry.timestamp)
  const isRight = entry.side === "right"
  const isCenter = entry.side === "center"

  if (isCenter) {
    return (
      <div className="border-border/60 bg-background/70 rounded-md border px-3 py-2.5 text-xs leading-relaxed">
        <div className="mb-1.5 flex flex-wrap items-center gap-2">
          <BlackboardAvatar speaker={entry.speaker} />
          <span className="text-foreground font-semibold">{style.name}</span>
          <PhaseChip phase={entry.phase} />
          <EntryReason phase={entry.phase} />
          {timestamp && (
            <span className="text-muted-foreground/70 ml-auto">{timestamp}</span>
          )}
        </div>
        <div className="text-muted-foreground whitespace-pre-wrap break-words [overflow-wrap:anywhere]">
          {entry.content}
        </div>
      </div>
    )
  }

  return (
    <div
      className={cn(
        "flex w-full gap-2",
        isRight ? "justify-end" : "justify-start",
      )}
    >
      {!isRight && <BlackboardAvatar speaker={entry.speaker} />}
      <div
        className={cn(
          "flex max-w-[86%] flex-col gap-1",
          isRight ? "items-end" : "items-start",
        )}
      >
        <div className="text-muted-foreground/75 flex max-w-full flex-wrap items-center gap-1.5 px-1 text-[11px]">
          <span className="font-medium">{style.name}</span>
          <PhaseChip phase={entry.phase} />
          <EntryReason phase={entry.phase} />
          {timestamp && <span>{timestamp}</span>}
        </div>
        <div
          className={cn(
            "whitespace-pre-wrap break-words rounded-md border px-3 py-2 text-[13px] leading-relaxed shadow-sm [overflow-wrap:anywhere]",
            style.bubble,
          )}
        >
          {entry.content}
        </div>
      </div>
      {isRight && <BlackboardAvatar speaker={entry.speaker} />}
    </div>
  )
}

function BlackboardSideContent({
  groups,
  selectedGroup,
  latestGroup,
  isTyping,
  onSelectGroup,
}: {
  groups: BlackboardTurnGroup[]
  selectedGroup: BlackboardTurnGroup
  latestGroup: BlackboardTurnGroup | null
  isTyping: boolean
  onSelectGroup: (groupId: string | null) => void
}) {
  return (
    <div className="border-border/60 flex min-h-0 flex-1 overflow-hidden border-t">
      <div className="border-border/60 bg-muted/15 flex w-12 shrink-0 flex-col border-r">
        <div className="border-border/60 text-muted-foreground border-b px-1 py-1.5 text-center text-[10px] font-semibold">
          {groups.length} 轮
        </div>
        <div className="min-h-0 flex-1 space-y-1 overflow-x-hidden overflow-y-auto px-1 py-1.5">
          {groups.map((group, index) => {
            const selected = selectedGroup.id === group.id
            return (
              <button
                key={group.id}
                type="button"
                onClick={() => onSelectGroup(group.id)}
                title={`第 ${index + 1} 轮：${group.title}`}
                className={cn(
                  "relative flex h-8 w-full items-center justify-center rounded-md border text-[11px] font-semibold transition",
                  selected
                    ? "border-violet-500/70 bg-violet-50 text-violet-700 shadow-sm dark:bg-violet-950/40 dark:text-violet-100"
                    : "border-transparent text-muted-foreground hover:border-border/70 hover:bg-background/70 hover:text-foreground",
                )}
              >
                {index + 1}
                <span
                  className={cn(
                    "absolute right-1 top-1 size-1.5 rounded-full",
                    group.status === "complete" &&
                      "bg-emerald-500 dark:bg-emerald-300",
                    group.status === "blocked" &&
                      "bg-orange-500 dark:bg-orange-300",
                    group.status === "running" &&
                      "bg-cyan-500 dark:bg-cyan-300",
                  )}
                />
              </button>
            )
          })}
        </div>
      </div>

      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        <div className="min-h-0 flex-1 overflow-x-hidden overflow-y-auto px-2.5 py-2.5">
          <div className="mb-2 flex items-center gap-2 text-[11px] font-semibold">
            <IconBook2 className="size-3.5 shrink-0 text-violet-500" />
            <span>本轮 bridge 对话</span>
            <span className="text-muted-foreground ml-auto font-normal">
              按发生顺序
            </span>
          </div>
          <div className="flex flex-col gap-3">
            {selectedGroup.entries.map((entry) => (
              <BlackboardBubble key={entry.id} entry={entry} />
            ))}
            {isTyping && selectedGroup.id === latestGroup?.id && (
              <div className="text-muted-foreground flex justify-center text-xs">
                bridge 正在补充本轮事件...
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

export function BlackboardPanel({
  entries,
  messages,
  isTyping = false,
  defaultOpen = false,
  lockedOpen = false,
  className,
  variant = "inline",
}: BlackboardPanelProps) {
  const groups = useMemo(
    () => buildBlackboardTurnGroups({ messages, entries }),
    [entries, messages],
  )
  const latestGroup = groups.at(-1) ?? null
  const [selectedGroupId, setSelectedGroupId] = useState<string | null>(null)
  const [isOpen, setIsOpen] = useState(defaultOpen || lockedOpen)
  const open = lockedOpen || isOpen
  const selectedGroup =
    groups.find((group) => group.id === selectedGroupId) ?? latestGroup
  const selectedGroupIndex = selectedGroup
    ? groups.findIndex((group) => group.id === selectedGroup.id)
    : -1
  const canSelectPreviousGroup = selectedGroupIndex > 0
  const canSelectNextGroup =
    selectedGroupIndex >= 0 && selectedGroupIndex < groups.length - 1
  const selectedGroupIsLatest = selectedGroup?.id === latestGroup?.id

  const selectGroupByIndex = (index: number) => {
    const group = groups[index]
    if (group) setSelectedGroupId(group.id)
  }

  useEffect(() => {
    if (
      selectedGroupId &&
      !groups.some((group) => group.id === selectedGroupId)
    ) {
      setSelectedGroupId(null)
    }
  }, [groups, selectedGroupId])

  if (groups.length === 0 || !selectedGroup) {
    return (
      <section
        className={cn(
          "border-border/60 bg-background/80 rounded-lg border shadow-sm",
          variant === "side" && "flex h-full min-h-0 flex-col overflow-hidden",
          className,
        )}
      >
        <div className="flex items-center justify-between gap-3 px-3 py-2.5">
          <div className="flex items-center gap-2">
            <div className="bg-background border-border flex size-7 items-center justify-center rounded-md border">
              <IconMessageCircle2 className="size-4 text-cyan-600" />
            </div>
            <div>
              <div className="text-sm font-semibold">黑板</div>
              <div className="text-muted-foreground text-[11px]">
                等待第一轮提问
              </div>
            </div>
          </div>
        </div>
        <div className="flex min-h-0 flex-1 items-center justify-center px-4 py-8">
          <div className="text-muted-foreground max-w-64 text-center text-sm leading-relaxed">
            每次你向 Flyflor 提问后，本轮 bridge 过程会按分组记录在这里。
          </div>
        </div>
      </section>
    )
  }

  return (
    <section
      className={cn(
        "border-border/60 bg-background/80 rounded-lg border shadow-sm",
        variant === "side" && "flex h-full min-h-0 flex-col overflow-hidden",
        className,
      )}
    >
      <div className="flex items-center justify-between gap-3 px-3 py-2.5">
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <div className="bg-background border-border flex size-7 items-center justify-center rounded-md border">
            <IconMessageCircle2 className="size-4 text-cyan-600" />
          </div>
          <div className="min-w-0">
            <div className="flex min-w-0 items-center gap-2 text-sm font-semibold">
              <span className="shrink-0">黑板</span>
              {variant === "side" && open && (
                <span className="text-muted-foreground min-w-0 truncate text-[11px] font-medium">
                  · 第 {selectedGroupIndex + 1}/{groups.length} 轮 ·{" "}
                  <span className={cn("font-semibold", groupStatusClass(selectedGroup))}>
                    {groupStatusLabel(selectedGroup)}
                  </span>{" "}
                  · {formatGroupTimestamp(selectedGroup)} ·{" "}
                  {selectedGroup.entries.length} 条记录
                </span>
              )}
            </div>
            <div className="text-muted-foreground min-w-0 truncate text-[11px]">
              {variant === "side" && open
                ? selectedGroup.title
                : open
                  ? "按提问切换 bridge 过程"
                  : `已记录 ${groups.length} 轮，默认隐藏`}
            </div>
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-1.5">
          {open && variant === "side" && (
            <>
              <button
                type="button"
                onClick={() => selectGroupByIndex(selectedGroupIndex - 1)}
                disabled={!canSelectPreviousGroup}
                className="text-muted-foreground hover:text-foreground disabled:text-muted-foreground/30 inline-flex size-7 items-center justify-center rounded-md border transition disabled:cursor-not-allowed disabled:hover:text-muted-foreground/30"
                title="上一轮"
              >
                <IconChevronLeft className="size-3.5" />
              </button>
              <button
                type="button"
                onClick={() => setSelectedGroupId(null)}
                disabled={selectedGroupIsLatest}
                className="text-muted-foreground hover:text-foreground disabled:text-muted-foreground/30 inline-flex size-7 items-center justify-center rounded-md border transition disabled:cursor-not-allowed disabled:hover:text-muted-foreground/30"
                title="回到最新黑板"
              >
                <IconRefresh className="size-3.5" />
              </button>
              <button
                type="button"
                onClick={() => selectGroupByIndex(selectedGroupIndex + 1)}
                disabled={!canSelectNextGroup}
                className="text-muted-foreground hover:text-foreground disabled:text-muted-foreground/30 inline-flex size-7 items-center justify-center rounded-md border transition disabled:cursor-not-allowed disabled:hover:text-muted-foreground/30"
                title="下一轮"
              >
                <IconChevronRight className="size-3.5" />
              </button>
            </>
          )}
          {open && variant !== "side" && (
            <button
              type="button"
              onClick={() => setSelectedGroupId(null)}
              className="text-muted-foreground hover:text-foreground hidden items-center gap-1.5 rounded-md border px-2 py-1 text-[11px] transition sm:inline-flex"
              title="回到最新黑板"
            >
              <IconRefresh className="size-3.5" />
              最新
            </button>
          )}
          {!lockedOpen && (
            <button
              type="button"
              onClick={() => setIsOpen((current) => !current)}
              className="text-muted-foreground hover:text-foreground inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1.5 text-xs font-medium transition"
            >
              {open ? (
                <>
                  <IconX className="size-3.5" />
                  隐藏
                </>
              ) : (
                <>
                  <IconBook2 className="size-3.5" />
                  打开黑板
                </>
              )}
            </button>
          )}
        </div>
      </div>

      {open && variant === "side" && (
        <BlackboardSideContent
          groups={groups}
          selectedGroup={selectedGroup}
          latestGroup={latestGroup}
          isTyping={isTyping}
          onSelectGroup={setSelectedGroupId}
        />
      )}

      {open && variant !== "side" && (
        <div className="border-border/60 border-t">
          <div className="px-3 pt-3">
            <div className="flex items-center gap-2 border-b border-violet-500/70">
              <IconGitBranch className="mb-2 size-4 shrink-0 text-violet-500" />
              <div className="flex min-w-0 flex-1 gap-1 overflow-x-auto pr-2">
                {groups.map((group, index) => (
                  <button
                    key={group.id}
                    onClick={() => setSelectedGroupId(group.id)}
                    className={cn(
                      "relative -mb-px max-w-45 shrink-0 rounded-t-md border px-3 py-2 text-left text-xs transition",
                      selectedGroup.id === group.id
                        ? "border-violet-500 border-b-background bg-background text-foreground"
                        : "border-border/60 bg-muted/30 text-muted-foreground hover:bg-muted/50 hover:text-foreground",
                    )}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <span className="font-semibold">#{index + 1}</span>
                      <span
                        className={cn(
                          "text-[10px] font-semibold",
                          groupStatusClass(group),
                        )}
                      >
                        {groupStatusLabel(group)}
                      </span>
                    </div>
                    <div className="mt-1 line-clamp-1 leading-snug">
                      {group.title}
                    </div>
                  </button>
                ))}
              </div>
            </div>
          </div>

          <div className="px-3 py-3">
            <div className="bg-muted/20 border-border/60 rounded-md border">
              <div className="border-border/60 flex items-start gap-2 border-b px-3 py-2.5">
                <IconBook2 className="mt-0.5 size-4 shrink-0 text-violet-500" />
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2 text-xs font-semibold">
                    本轮黑板
                    <span
                      className={cn(
                        "ml-auto text-[11px]",
                        groupStatusClass(selectedGroup),
                      )}
                    >
                      {groupStatusLabel(selectedGroup)}
                    </span>
                  </div>
                  <div className="text-foreground mt-1 line-clamp-2 text-sm font-semibold leading-snug">
                    {selectedGroup.title}
                  </div>
                </div>
              </div>

              <div className="max-h-80 overflow-y-auto px-3 py-3">
                <div className="flex flex-col gap-3">
                  {selectedGroup.entries.map((entry) => (
                    <BlackboardBubble key={entry.id} entry={entry} />
                  ))}
                  {isTyping && selectedGroup.id === latestGroup?.id && (
                    <div className="text-muted-foreground flex justify-center text-xs">
                      bridge 正在补充本轮事件...
                    </div>
                  )}
                </div>
              </div>
            </div>
          </div>
        </div>
      )}
    </section>
  )
}
