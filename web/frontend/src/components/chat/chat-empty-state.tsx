import {
  IconGitBranch,
  IconPlugConnectedX,
  IconRobotOff,
  IconStar,
} from "@tabler/icons-react"
import { Link } from "@tanstack/react-router"
import { useTranslation } from "react-i18next"

import flyflorIp from "@/assets/flyflor-ip.jpg"
import { Button } from "@/components/ui/button"

interface ChatEmptyStateProps {
  hasAvailableModels: boolean
  defaultModelName: string
  isConnected: boolean
}

export function ChatEmptyState({
  hasAvailableModels,
  defaultModelName,
  isConnected,
}: ChatEmptyStateProps) {
  const { t } = useTranslation()

  if (!hasAvailableModels) {
    return (
      <div className="flex flex-col items-center justify-center py-20 opacity-70">
        <div className="mb-6 flex h-16 w-16 items-center justify-center rounded-2xl bg-amber-500/10 text-amber-500">
          <IconRobotOff className="h-8 w-8" />
        </div>
        <h3 className="mb-2 text-xl font-medium">
          {t("chat.empty.noConfiguredModel")}
        </h3>
        <p className="text-muted-foreground mb-4 text-center text-sm">
          {t("chat.empty.noConfiguredModelDescription")}
        </p>
        <Button asChild variant="outline" size="sm" className="px-4">
          <Link to="/models">{t("chat.empty.goToModels")}</Link>
        </Button>
      </div>
    )
  }

  if (!defaultModelName) {
    return (
      <div className="flex flex-col items-center justify-center py-20 opacity-70">
        <div className="mb-6 flex h-16 w-16 items-center justify-center rounded-2xl bg-amber-500/10 text-amber-500">
          <IconStar className="h-8 w-8" />
        </div>
        <h3 className="mb-2 text-xl font-medium">
          {t("chat.empty.noSelectedModel")}
        </h3>
        <p className="text-muted-foreground mb-4 text-center text-sm">
          {t("chat.empty.noSelectedModelDescription")}
        </p>
      </div>
    )
  }

  if (!isConnected) {
    return (
      <div className="flex flex-col items-center justify-center py-20 opacity-70">
        <div className="mb-6 flex h-16 w-16 items-center justify-center rounded-2xl bg-amber-500/10 text-amber-500">
          <IconPlugConnectedX className="h-8 w-8" />
        </div>
        <h3 className="mb-2 text-xl font-medium">
          {t("chat.empty.notRunning")}
        </h3>
        <p className="text-muted-foreground mb-4 text-center text-sm">
          {t("chat.empty.notRunningDescription")}
        </p>
      </div>
    )
  }

  return (
    <div className="flex flex-col items-center justify-center py-14">
      <div className="mb-6 flex items-center gap-5">
        <div className="border-border/60 bg-card h-34 w-24 overflow-hidden rounded-lg border shadow-sm">
          <img
            src={flyflorIp}
            alt="Flyflor"
            className="h-full w-full object-cover object-[50%_28%]"
          />
        </div>
        <div className="hidden max-w-xs text-left sm:block">
          <div className="text-muted-foreground text-xs font-medium tracking-[0.18em] uppercase">
            AI Runtime Cockpit
          </div>
          <h3 className="mt-2 text-2xl font-semibold tracking-tight">
            Flyflor
          </h3>
          <p className="text-muted-foreground mt-2 text-sm leading-relaxed">
            把多智能体协商、记忆检索和工具执行放到台前。你发任务，
            Flyflor 调度黑板，Codex 与 Copilot 在 bridge 中互相检查。
          </p>
        </div>
      </div>
      <h3 className="mb-2 text-xl font-medium sm:hidden">
        {t("chat.welcome")}
      </h3>
      <p className="text-muted-foreground max-w-lg text-center text-sm">
        {t("chat.welcomeDesc")}
      </p>
      <div className="text-muted-foreground/80 mt-5 flex flex-wrap justify-center gap-2 text-xs">
        <span className="rounded-md border px-2.5 py-1">Blackboard</span>
        <span className="rounded-md border px-2.5 py-1">Codex Bridge</span>
        <span className="rounded-md border px-2.5 py-1">Copilot Guard</span>
        <span className="inline-flex items-center gap-1 rounded-md border px-2.5 py-1">
          <IconGitBranch className="size-3.5" />
          SQLite + Qdrant
        </span>
      </div>
    </div>
  )
}
