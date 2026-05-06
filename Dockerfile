ARG QDRANT_IMAGE=qdrant/qdrant:latest
ARG PYTHON_IMAGE=python:3.12-slim
ARG NODE_MAJOR=22

FROM ${QDRANT_IMAGE} AS qdrant

FROM ${PYTHON_IMAGE} AS app

ARG NODE_MAJOR

ENV PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    LANG=C.UTF-8 \
    LC_ALL=C.UTF-8 \
    FLYFLOR_HOME=/data/flyflor \
    FLYFLOR_DATABASE=/data/flyflor/flyflor.db \
    QDRANT_URL=http://127.0.0.1:6333 \
    QDRANT_COLLECTION=flyflor_memory \
    FLYFLOR_CORE_URL=http://127.0.0.1:8080 \
    CODEX_HOME=/data/flyflor/agents/codex \
    CLAUDE_CONFIG_DIR=/data/flyflor/agents/claude \
    COPILOT_CONFIG_DIR=/data/flyflor/agents/copilot \
    XDG_CONFIG_HOME=/data/flyflor/xdg/config \
    XDG_CACHE_HOME=/data/flyflor/xdg/cache \
    XDG_DATA_HOME=/data/flyflor/xdg/data \
    TMPDIR=/data/flyflor/tmp \
    QDRANT__SERVICE__HTTP_PORT=6333 \
    QDRANT__SERVICE__GRPC_PORT=6334 \
    QDRANT__STORAGE__STORAGE_PATH=/data/qdrant/storage \
    NANOBOT_WORKSPACE=/workspace \
    DISABLE_AUTOUPDATER=1

COPY --from=qdrant /qdrant /qdrant

WORKDIR /app

COPY pyproject.toml README.md ./
COPY flyflor_core ./flyflor_core
COPY vendor/nanobot ./vendor/nanobot

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        bash \
        ca-certificates \
        curl \
        git \
        gnupg \
        libunwind8 \
        procps \
        ripgrep \
    && curl -fsSL "https://deb.nodesource.com/setup_${NODE_MAJOR}.x" | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && npm install -g @openai/codex @anthropic-ai/claude-code @github/copilot \
    && pip install --upgrade pip \
    && pip install -e . \
    && pip install -e ./vendor/nanobot \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

COPY configs ./configs
COPY docker/entrypoint.sh /usr/local/bin/flyflor-entrypoint

RUN chmod +x /usr/local/bin/flyflor-entrypoint

EXPOSE 8080 8765

ENTRYPOINT ["flyflor-entrypoint"]
CMD ["python", "-m", "flyflor_core"]
