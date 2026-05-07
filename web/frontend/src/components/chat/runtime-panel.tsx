import {
  IconBrain,
  IconDatabase,
  IconGitBranch,
  IconHeartbeat,
  IconHexagonLetterF,
  IconRoute,
  IconShieldCheck,
} from "@tabler/icons-react"

import type { MemoryStatus } from "@/api/system"
import { cn } from "@/lib/utils"
import type {
  BlackboardEntry,
  ConnectionState,
  ContextUsage,
} from "@/store/chat"
import type { GatewayState } from "@/store/gateway"

interface RuntimePanelProps {
  gatewayState: GatewayState
  connectionState: ConnectionState
  defaultModelName: string
  activeSessionId: string
  blackboardEntries: BlackboardEntry[]
  contextUsage?: ContextUsage
  memoryStatus?: MemoryStatus
}

function shortSession(id: string) {
  if (!id) return "new"
  if (id.length <= 12) return id
  return `${id.slice(0, 6)}...${id.slice(-4)}`
}

function statusTone(status: string) {
  if (status === "running" || status === "connected") {
    return "bg-emerald-500"
  }
  if (status === "starting" || status === "connecting") {
    return "bg-amber-500"
  }
  if (status === "error" || status === "disconnected") {
    return "bg-rose-500"
  }
  return "bg-zinc-400"
}

function RuntimeLine({
  label,
  value,
  tone = "text-foreground",
}: {
  label: string
  value: string
  tone?: string
}) {
  return (
    <div className="flex items-center justify-between gap-3 text-xs">
      <span className="text-muted-foreground">{label}</span>
      <span className={cn("truncate font-medium", tone)}>{value}</span>
    </div>
  )
}

function MemoryMeter({
  label,
  percent,
  detail,
  tone,
  barClassName,
}: {
  label: string
  percent: number
  detail: string
  tone: string
  barClassName: string
}) {
  const safePercent = Math.min(Math.max(percent, 0), 100)

  return (
    <div className="border-border/60 bg-muted/20 rounded-md border px-2.5 py-2">
      <div className="flex items-center justify-between gap-3">
        <span className="text-xs font-semibold">{label}</span>
        <span className={cn("font-mono text-xs font-semibold", tone)}>
          {safePercent}%
        </span>
      </div>
      <div className="bg-muted mt-1.5 h-1.5 overflow-hidden rounded-full">
        <div
          className={cn("h-full rounded-full", barClassName)}
          style={{ width: `${safePercent}%` }}
        />
      </div>
      <div className="text-muted-foreground mt-1.5 truncate text-[11px]">
        {detail}
      </div>
    </div>
  )
}

function memoryValue(
  ready: boolean | undefined,
  readyText: string,
  fallback: string,
) {
  if (ready === true) return readyText
  if (ready === false) return fallback
  return "checking"
}

function AgentNode({
  label,
  role,
  side,
  icon: Icon,
}: {
  label: string
  role: string
  side: "left" | "right" | "center"
  icon: typeof IconBrain
}) {
  return (
    <div
      className={cn(
        "border-border/60 bg-background/70 flex items-center gap-2 rounded-md border px-2.5 py-2 shadow-sm",
        side === "left" && "border-cyan-300/60",
        side === "right" && "border-rose-300/60",
        side === "center" && "border-violet-300/60",
      )}
    >
      <div
        className={cn(
          "flex size-7 items-center justify-center rounded-md text-white",
          side === "left" && "bg-cyan-500",
          side === "right" && "bg-rose-500",
          side === "center" && "bg-violet-500",
        )}
      >
        <Icon className="size-4" />
      </div>
      <div className="min-w-0">
        <div className="truncate text-xs font-semibold">{label}</div>
        <div className="text-muted-foreground truncate text-[10px]">{role}</div>
      </div>
    </div>
  )
}

export function RuntimePanel({
  gatewayState,
  connectionState,
  defaultModelName,
  activeSessionId,
  blackboardEntries,
  contextUsage,
  memoryStatus,
}: RuntimePanelProps) {
  const contextPercent = Math.min(contextUsage?.used_percent ?? 0, 100)
  const markdownPercent = memoryStatus?.markdown.ready ? 100 : 0
  const sqlitePercent =
    memoryStatus?.sqlite.ready || memoryStatus?.sqlite.backend ? 100 : 0
  const qdrantPercent = memoryStatus?.qdrant.ready
    ? 100
    : memoryStatus?.qdrant.enabled
      ? 50
      : 0
  const qdrantDetail = memoryStatus?.qdrant.ready
    ? `${memoryStatus.qdrant.count} 条向量索引可召回`
    : memoryStatus?.qdrant.enabled
      ? "已启用，等待向量库连接"
      : memoryStatus
        ? "未启用向量召回"
        : "向量库检查中"
  const lastPhase =
    [...blackboardEntries].reverse().find((entry) => entry.phase)?.phase ??
    "idle"

  return (
    <aside className="border-border/60 bg-background/80 hidden min-h-0 w-[280px] shrink-0 flex-col border-r xl:flex">
      <div className="border-border/60 border-b px-4 py-4">
        <div className="text-muted-foreground text-[11px] font-medium tracking-[0.18em] uppercase">
          Runtime Cockpit
        </div>
        <div className="mt-1 text-lg font-semibold tracking-tight">Flyflor</div>
        <p className="text-muted-foreground mt-2 text-xs leading-relaxed">
          多智能体运行时控制台：自然对话、黑板协商、记忆与工具事件同时可见。
        </p>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4">
        <section className="space-y-3">
          <div className="flex items-center gap-2 text-xs font-semibold">
            <IconHeartbeat className="size-4 text-emerald-500" />
            Runtime Status
          </div>
          <div className="space-y-2">
            <RuntimeLine
              label="Gateway"
              value={gatewayState}
              tone="text-emerald-600 dark:text-emerald-300"
            />
            <RuntimeLine
              label="Bridge socket"
              value={connectionState}
              tone="text-cyan-600 dark:text-cyan-300"
            />
            <RuntimeLine
              label="Session"
              value={shortSession(activeSessionId)}
            />
            <RuntimeLine
              label="Model"
              value={defaultModelName || "not selected"}
            />
          </div>
          <div className="flex gap-1.5">
            <span
              className={cn(
                "h-1.5 flex-1 rounded-full",
                statusTone(gatewayState),
              )}
            />
            <span
              className={cn(
                "h-1.5 flex-1 rounded-full",
                statusTone(connectionState),
              )}
            />
            <span className="h-1.5 flex-1 rounded-full bg-violet-500" />
            <span className="h-1.5 flex-1 rounded-full bg-amber-500" />
          </div>
        </section>

        <div className="bg-border/70 my-5 h-px" />

        <section className="space-y-3">
          <div className="flex items-center gap-2 text-xs font-semibold">
            <IconGitBranch className="size-4 text-cyan-500" />
            Agent Graph
          </div>
          <div className="space-y-2">
            <AgentNode
              label="Flyflor"
              role="dispatch / memory / channel"
              side="center"
              icon={IconHexagonLetterF}
            />
            <div className="text-muted-foreground flex items-center justify-center gap-2 text-[10px]">
              <span className="h-px w-10 bg-cyan-300" />
              bridge blackboard
              <span className="h-px w-10 bg-rose-300" />
            </div>
            <div className="grid grid-cols-2 gap-2">
              <AgentNode
                label="Codex"
                role="plan / execute"
                side="left"
                icon={IconBrain}
              />
              <AgentNode
                label="Copilot"
                role="review / guard"
                side="right"
                icon={IconShieldCheck}
              />
            </div>
          </div>
        </section>

        <div className="bg-border/70 my-5 h-px" />

        <section className="space-y-3">
          <div className="flex items-center gap-2 text-xs font-semibold">
            <IconDatabase className="size-4 text-violet-500" />
            三层记忆
          </div>
          <div className="space-y-2 text-xs">
            <MemoryMeter
              label="Markdown"
              percent={markdownPercent}
              detail={memoryValue(
                memoryStatus?.markdown.ready,
                `${memoryStatus?.markdown.count ?? 4}/4 身份/规则文件可用`,
                "身份/规则文件缺失",
              )}
              tone="text-cyan-600 dark:text-cyan-300"
              barClassName="bg-cyan-500"
            />
            <MemoryMeter
              label="SQLite"
              percent={sqlitePercent}
              detail={
                memoryStatus?.sqlite.backend
                  ? `${memoryStatus.sqlite.backend} 会话/摘要/审计可用`
                  : "会话库检查中"
              }
              tone="text-amber-600 dark:text-amber-300"
              barClassName="bg-amber-500"
            />
            <MemoryMeter
              label="Qdrant"
              percent={qdrantPercent}
              detail={qdrantDetail}
              tone="text-violet-600 dark:text-violet-300"
              barClassName="bg-violet-500"
            />
            <RuntimeLine
              label="Embedding"
              value={memoryStatus?.qdrant.provider || "hash"}
            />
            <div className="border-border/70 rounded-md border border-dashed px-2.5 py-2">
              <div className="flex items-center justify-between gap-3">
                <span className="text-muted-foreground">上下文用量</span>
                <span className="text-foreground font-mono font-semibold">
                  {contextPercent}%
                </span>
              </div>
              <div className="bg-muted/70 mt-1.5 h-1.5 overflow-hidden rounded-full">
                <div
                  className="h-full rounded-full bg-emerald-500"
                  style={{ width: `${contextPercent}%` }}
                />
              </div>
              <p className="text-muted-foreground mt-2 text-[11px] leading-relaxed">
                记忆百分比表示该层当前接入状态，不是容量或 token
                使用率；上下文用量单独显示。
              </p>
            </div>
          </div>
        </section>

        <div className="bg-border/70 my-5 h-px" />

        <section className="space-y-3">
          <div className="flex items-center gap-2 text-xs font-semibold">
            <IconRoute className="size-4 text-amber-500" />
            Event Flow
          </div>
          <div className="space-y-2 text-xs">
            <RuntimeLine
              label="Blackboard"
              value={`${blackboardEntries.length} events`}
            />
            <RuntimeLine label="Current phase" value={lastPhase} />
            <div className="text-muted-foreground rounded-md border border-dashed px-2.5 py-2 leading-relaxed">
              {
                "message -> blackboard -> Codex/Copilot bridge -> memory/tool checkpoint -> Flyflor response"
              }
            </div>
          </div>
        </section>
      </div>
    </aside>
  )
}
