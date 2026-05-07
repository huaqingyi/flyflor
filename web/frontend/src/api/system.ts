import { launcherFetch } from "@/api/http"

export interface AutoStartStatus {
  enabled: boolean
  supported: boolean
  platform: string
  message?: string
}

export interface LauncherConfig {
  port: number
  public: boolean
  allowed_cidrs: string[]
}

export interface SystemVersionInfo {
  version: string
  git_commit?: string
  build_time?: string
  go_version: string
}

export interface MemoryLayerStatus {
  ready: boolean
  detail: string
  path?: string
  count?: number
  error?: string
  backend?: string
}

export interface QdrantMemoryStatus {
  enabled: boolean
  ready: boolean
  url: string
  collection: string
  provider: string
  dimensions: number
  count: number
  error?: string
}

export interface MemoryStatus {
  markdown: MemoryLayerStatus
  sqlite: MemoryLayerStatus
  qdrant: QdrantMemoryStatus
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await launcherFetch(path, options)
  if (!res.ok) {
    let message = `API error: ${res.status} ${res.statusText}`
    try {
      const body = (await res.json()) as {
        error?: string
        errors?: string[]
      }
      if (Array.isArray(body.errors) && body.errors.length > 0) {
        message = body.errors.join("; ")
      } else if (typeof body.error === "string" && body.error.trim() !== "") {
        message = body.error
      }
    } catch {
      // Keep fallback error message when response body is not JSON.
    }
    throw new Error(message)
  }
  return res.json() as Promise<T>
}

export async function getAutoStartStatus(): Promise<AutoStartStatus> {
  return request<AutoStartStatus>("/api/system/autostart")
}

export async function setAutoStartEnabled(
  enabled: boolean,
): Promise<AutoStartStatus> {
  return request<AutoStartStatus>("/api/system/autostart", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ enabled }),
  })
}

export async function getLauncherConfig(): Promise<LauncherConfig> {
  return request<LauncherConfig>("/api/system/launcher-config")
}

export async function setLauncherConfig(
  payload: LauncherConfig,
): Promise<LauncherConfig> {
  return request<LauncherConfig>("/api/system/launcher-config", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  })
}

export async function getSystemVersionInfo(): Promise<SystemVersionInfo> {
  return request<SystemVersionInfo>("/api/system/version")
}

export async function getMemoryStatus(): Promise<MemoryStatus> {
  return request<MemoryStatus>("/api/system/memory")
}
