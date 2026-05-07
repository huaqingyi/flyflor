package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/semanticmemory"
)

type memoryLayerStatus struct {
	Ready   bool   `json:"ready"`
	Detail  string `json:"detail"`
	Path    string `json:"path,omitempty"`
	Count   int    `json:"count,omitempty"`
	Error   string `json:"error,omitempty"`
	Backend string `json:"backend,omitempty"`
}

type memoryStatusResponse struct {
	Markdown memoryLayerStatus     `json:"markdown"`
	SQLite   memoryLayerStatus     `json:"sqlite"`
	Qdrant   semanticmemory.Status `json:"qdrant"`
}

func (h *Handler) registerMemoryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/system/memory", h.handleGetMemoryStatus)
}

func (h *Handler) handleGetMemoryStatus(w http.ResponseWriter, r *http.Request) {
	status := h.resolveMemoryStatus(r.Context())
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) resolveMemoryStatus(ctx context.Context) memoryStatusResponse {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		return memoryStatusResponse{
			Markdown: memoryLayerStatus{Ready: false, Detail: "config unavailable", Error: err.Error()},
			SQLite:   memoryLayerStatus{Ready: false, Detail: "config unavailable", Error: err.Error()},
			Qdrant:   semanticmemory.Status{Enabled: false, Error: err.Error()},
		}
	}

	workspace := cfg.Agents.Defaults.Workspace
	markdown := markdownMemoryStatus(workspace)
	sqlite := sqliteMemoryStatus(workspace, cfg.Agents.Defaults.ContextManager)
	qdrant := qdrantMemoryStatus(ctx, cfg.Agents.Defaults.ContextManagerConfig)
	return memoryStatusResponse{
		Markdown: markdown,
		SQLite:   sqlite,
		Qdrant:   qdrant,
	}
}

func markdownMemoryStatus(workspace string) memoryLayerStatus {
	files := []string{
		filepath.Join(workspace, "AGENT.md"),
		filepath.Join(workspace, "SOUL.md"),
		filepath.Join(workspace, "USER.md"),
		filepath.Join(workspace, "memory", "MEMORY.md"),
	}
	count := 0
	for _, file := range files {
		if st, err := os.Stat(file); err == nil && !st.IsDir() {
			count++
		}
	}
	return memoryLayerStatus{
		Ready:  count == len(files),
		Detail: "identity + profile markdown",
		Path:   workspace,
		Count:  count,
	}
}

func sqliteMemoryStatus(workspace, manager string) memoryLayerStatus {
	if manager == "" {
		manager = "legacy"
	}
	dbPath := filepath.Join(workspace, "sessions", "seahorse.db")
	_, err := os.Stat(dbPath)
	status := memoryLayerStatus{
		Ready:   err == nil || manager == "seahorse",
		Detail:  "session timeline + Seahorse summaries",
		Path:    dbPath,
		Backend: manager,
	}
	if err != nil && !os.IsNotExist(err) {
		status.Ready = false
		status.Error = err.Error()
	}
	return status
}

func qdrantMemoryStatus(ctx context.Context, raw json.RawMessage) semanticmemory.Status {
	cfg := semanticmemory.ConfigFromRaw(raw)
	if !cfg.Enabled {
		return semanticmemory.Status{
			Enabled:    false,
			URL:        cfg.QdrantURL,
			Collection: cfg.Collection,
			Provider:   cfg.Embedding.Provider,
			Dimensions: cfg.Dimensions,
		}
	}
	statusCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	manager, err := semanticmemory.New(statusCtx, cfg)
	if err != nil {
		return semanticmemory.Status{
			Enabled:    true,
			Ready:      false,
			URL:        cfg.QdrantURL,
			Collection: cfg.Collection,
			Provider:   cfg.Embedding.Provider,
			Dimensions: cfg.Dimensions,
			Error:      err.Error(),
		}
	}
	return manager.Status(statusCtx)
}
