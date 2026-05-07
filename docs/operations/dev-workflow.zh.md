# Flyflor 开发调试流程

> 返回 [Operations](README.md)

本文说明 Flyflor 本地开发时应该如何调试 Web UI、TUI 和容器运行环境。核心原则是：日常 UI 迭代不要反复执行 `docker compose up -d --build flyflor`。完整 Docker 构建只用于验证 Dockerfile、发布镜像或依赖层变化。

## 总览

| 场景 | 推荐命令 | 是否重建 Docker 镜像 |
| --- | --- | --- |
| Web UI 高频调试，想要热更新 | `make dev-webui` | 否 |
| 只生成 Web 静态资源 | `make build-web-dist` | 否 |
| 更新容器内 `flyflor` CLI 二进制 | `make dev-build-cli` | 否 |
| 更新容器内 `flyflor-web` 二进制 | `make dev-build-web-backend` | 否 |
| 验证最终镜像或 Dockerfile | `docker compose up -d --build flyflor` | 是 |

## 方式一：Web UI 热更新

这是前端界面调试的默认方式。

```bash
make dev-webui
```

它会：

- 保持 Docker 后端和 Qdrant 正常运行。
- 启动 Vite dev server。
- 把 `/api`、`/pico/ws`、`/pico/media` 代理到 `http://127.0.0.1:18800`。
- 通过 HMR 实时刷新 Web UI。

访问：

```text
http://127.0.0.1:5173
```

适合修改：

- `web/frontend/src/**`
- Web 聊天布局
- 右侧黑板
- 登录页、配置页、模型页等 React 组件
- CSS / Tailwind / 图标 / favicon

这条路径不更新 `http://127.0.0.1:18800` 上的嵌入式静态资源。它直接使用 Vite，所以改 UI 时速度最快。

## 方式二：只构建 Web 静态资源

当你想生成 `web/backend/dist`，但不想重建镜像时，使用：

```bash
make build-web-dist
```

等价于：

```bash
pnpm -C web/frontend build:backend
```

它会执行 TypeScript 检查和 Vite 生产构建，并把结果写入：

```text
web/backend/dist
```

适合修改：

- favicon / PWA manifest
- 生产构建下才暴露的问题
- 需要检查 `web/backend/dist` 产物的场景

## 方式三：更新容器内的 `flyflor-web`

如果你修改了 Web 后端 Go 代码，或者想让 `http://127.0.0.1:18800` 使用最新的嵌入式 Web 后端，不要重建完整镜像。使用：

```bash
make dev-build-web-backend
```

它会：

1. 启动一个长期复用的 builder 容器 `flyflor-builder`。
2. 使用 Docker volume 缓存 Go 模块、Go build cache 和 pnpm store。
3. 在 builder 容器内增量构建 `flyflor-web`。
4. 把新二进制复制进正在运行的 `flyflor` 容器。
5. 重启 `flyflor` 容器。

相关缓存卷：

```text
flyflor_flyflor-go-mod-cache
flyflor_flyflor-go-build-cache
flyflor_flyflor-pnpm-store
```

这比 `docker compose build` 稳定得多，因为它不会反复执行 Dockerfile 的所有构建层，也不会因为无关的 Go 命令编译错误阻塞 Web UI 迭代。

## 方式三点五：更新容器内的 `flyflor` CLI

如果你修改了 CLI / TUI 入口，例如 `docker exec -it flyflor flyflor` 的交互菜单，不需要重建完整镜像。使用：

```bash
make dev-build-cli
```

它会复用同一个 builder 容器和 Go 缓存，增量构建 `build/dev/flyflor`，然后复制到正在运行容器的 `/usr/local/bin/flyflor`。

更新后直接测试：

```bash
docker exec -it flyflor flyflor
```

## 方式四：让 18800 直接读取挂载 dist

Web 后端支持通过环境变量从外部目录读取静态资源：

```bash
FLYFLOR_WEB_DIST_DIR=/source/web/backend/dist
```

项目提供了 compose 覆盖文件：

```bash
docker compose -f docker-compose.yml -f docker-compose.webdev.yml up -d flyflor
```

这会让容器中的 Web 后端优先读取挂载进来的：

```text
/source/web/backend/dist
```

也就是宿主机上的：

```text
web/backend/dist
```

之后每次修改前端，只需要：

```bash
make build-web-dist
```

然后刷新：

```text
http://127.0.0.1:18800
```

注意：这个能力要求容器里的 `flyflor-web` 二进制已经包含 `FLYFLOR_WEB_DIST_DIR` 支持。如果当前容器来自较旧镜像，先运行一次：

```bash
make dev-build-web-backend
```

## 什么时候才需要完整 Docker build

只在这些场景使用：

```bash
docker compose up -d --build flyflor
```

- 修改了 `Dockerfile` 或运行时镜像依赖。
- 修改了容器入口脚本或系统包。
- 需要验证发布镜像是否能从零构建。
- CI / release 前的最终验证。

不要在以下场景使用完整 Docker build：

- 微调 Web UI 布局。
- 修改黑板样式。
- 调整 TUI 文案或颜色。
- 替换 favicon。
- 修改前端 TypeScript 组件。

## 推荐日常循环

Web UI：

```bash
make dev-webui
# 打开 http://127.0.0.1:5173
# 修改 web/frontend/src/**，浏览器自动刷新
```

Web 生产构建检查：

```bash
make build-web-dist
```

Web 后端 Go 代码：

```bash
go test -tags goolm,stdjson ./web/backend
make dev-build-web-backend
```

完整镜像验证：

```bash
docker compose up -d --build flyflor
docker ps --filter name=flyflor --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
```

## 常见问题

### Vite 页面能打开，但 18800 没更新

这是正常现象。`5173` 是 Vite 热更新页面，`18800` 是容器里的 Web 后端页面。

如果只调 UI，用 `5173`。如果要更新 `18800`，用：

```bash
make build-web-dist
make dev-build-web-backend
```

或者启用 `docker-compose.webdev.yml` 后只执行：

```bash
make build-web-dist
```

### 完整 Docker build 因无关 Go 编译错误失败

不要让 Web UI 迭代依赖完整镜像构建。先用：

```bash
make dev-webui
```

继续前端调试，再单独修复 Go 编译错误。

### 想清理 builder 缓存

一般不需要清理。如果依赖缓存损坏，可以删除相关 volume：

```bash
docker volume rm \
  flyflor_flyflor-go-mod-cache \
  flyflor_flyflor-go-build-cache \
  flyflor_flyflor-pnpm-store
```

下次 `make dev-build-web-backend` 会重新创建缓存，但第一次会慢。
