FROM node:24-bookworm-slim AS builder

ARG TARGETARCH
ARG GO_VERSION=1.25.9

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl git make && \
    rm -rf /var/lib/apt/lists/* && \
    case "${TARGETARCH:-amd64}" in \
      amd64) go_arch=amd64 ;; \
      arm64) go_arch=arm64 ;; \
      *) echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac && \
    curl -fsSL "https://dl.google.com/go/go${GO_VERSION}.linux-${go_arch}.tar.gz" | tar -C /usr/local -xz

ENV PATH=/usr/local/go/bin:$PATH \
    GOTOOLCHAIN=local \
    GOPROXY=https://goproxy.cn,direct

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/go/pkg/mod \
    go mod download
RUN --mount=type=cache,target=/root/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOSUMDB=off go install github.com/wagoodman/dive@v0.13.1 && \
    GOSUMDB=off go install github.com/antonmedv/fx@latest && \
    GOSUMDB=off go install github.com/charmbracelet/glow/v2@v2.1.2 && \
    GOSUMDB=off go install github.com/charmbracelet/gum@v0.17.0

COPY . .
RUN --mount=type=cache,target=/root/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    make build
RUN --mount=type=cache,target=/root/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/.local/share/pnpm/store \
    corepack enable && \
    make -C web build OUTPUT=/src/build/flyflor-web PICOCLAW_BINARY=/src/build/picoclaw

FROM node:24-bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates tzdata curl git openssh-client wget && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir -p /config /data/flyflor /runs /source /workspace

COPY --from=builder /src/build/picoclaw /usr/local/bin/flyflor
COPY --from=builder /src/build/flyflor-web /usr/local/bin/flyflor-web
COPY --from=builder /root/go/bin/dive /usr/local/bin/dive
COPY --from=builder /root/go/bin/fx /usr/local/bin/fx
COPY --from=builder /root/go/bin/glow /usr/local/bin/glow
COPY --from=builder /root/go/bin/gum /usr/local/bin/gum

RUN ln -s /usr/local/bin/flyflor /usr/local/bin/picoclaw && \
    cat > /usr/local/bin/flyflor-entrypoint <<'EOF' && \
    chmod +x /usr/local/bin/flyflor-entrypoint
#!/bin/sh
set -e

export PICOCLAW_HOME="${PICOCLAW_HOME:-/data/flyflor}"
export PICOCLAW_CONFIG="${PICOCLAW_CONFIG:-/config/config.json}"
export PICOCLAW_AGENTS_DEFAULTS_WORKSPACE="${PICOCLAW_AGENTS_DEFAULTS_WORKSPACE:-/workspace}"

mkdir -p "$PICOCLAW_HOME" /runs /workspace

if [ ! -f "$PICOCLAW_CONFIG" ]; then
  echo "flyflor: missing config file at $PICOCLAW_CONFIG" >&2
  echo "Mount ./config/config.json or create one from ./config/config.example.json." >&2
  exit 1
fi

rm -f "$PICOCLAW_HOME/.picoclaw.pid"

if [ "$#" -eq 0 ]; then
  set -- web -console -no-browser -host 0.0.0.0 "$PICOCLAW_CONFIG"
fi

if [ "$1" = "web" ] || [ "$1" = "launcher" ] || [ "$1" = "webui" ]; then
  shift
  exec flyflor-web "$@"
fi

exec flyflor "$@"
EOF

ENV HOME=/root \
    PICOCLAW_HOME=/data/flyflor \
    PICOCLAW_CONFIG=/config/config.json \
    PICOCLAW_BINARY=/usr/local/bin/flyflor \
    PICOCLAW_AGENTS_DEFAULTS_WORKSPACE=/workspace \
    FLYFLOR_RUNS_DIR=/runs \
    FLYFLOR_SEMANTIC_MEMORY_ENABLED=true \
    FLYFLOR_QDRANT_COLLECTION=flyflor_memories \
    QDRANT_URL=http://qdrant:6333

WORKDIR /workspace

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q --spider http://localhost:18790/health || exit 1

ENTRYPOINT ["flyflor-entrypoint"]
CMD ["web", "-console", "-no-browser", "-host", "0.0.0.0", "/config/config.json"]
