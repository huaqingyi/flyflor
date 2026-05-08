package agent

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/sipeed/picoclaw/pkg/armsmemory"
	"github.com/sipeed/picoclaw/pkg/logger"
)

type armsContextManager struct {
	base ContextManager
	arms *armsmemory.Manager
}

func newARMSContextManager(base ContextManager, raw json.RawMessage, al *AgentLoop) ContextManager {
	if base == nil {
		return nil
	}
	armsCfg := armsmemory.ConfigFromRaw(raw)
	if !armsCfg.Enabled {
		return base
	}
	armsMgr, err := armsmemory.New(armsCfg)
	if err != nil {
		logger.WarnCF("memory", "ARMS methodology memory is configured but unavailable", map[string]any{
			"url":      armsCfg.APIBase,
			"space_id": armsCfg.SpaceID,
			"error":    err.Error(),
		})
		return base
	}
	logger.InfoCF("memory", "ARMS methodology memory enabled", map[string]any{
		"url":       armsCfg.APIBase,
		"space_id":  armsCfg.SpaceID,
		"namespace": armsCfg.MethodologyNamespace,
	})
	mgr := &armsContextManager{base: base, arms: armsMgr}
	mgr.bootstrap(al)
	return mgr
}

func (m *armsContextManager) Assemble(ctx context.Context, req *AssembleRequest) (*AssembleResponse, error) {
	resp, err := m.base.Assemble(ctx, req)
	if err != nil || resp == nil || m.arms == nil || req == nil || strings.TrimSpace(req.Query) == "" {
		return resp, err
	}
	hits, searchErr := m.arms.Search(ctx, req.Query)
	if searchErr != nil {
		logger.WarnCF("memory", "ARMS methodology recall failed", map[string]any{
			"session": req.SessionKey,
			"error":   searchErr.Error(),
		})
		return resp, nil
	}
	formatted := armsmemory.FormatHits(hits)
	if formatted == "" {
		return resp, nil
	}
	if strings.TrimSpace(resp.Summary) != "" {
		resp.Summary += "\n\n" + formatted
	} else {
		resp.Summary = formatted
	}
	logger.InfoCF("memory", "ARMS methodology recall injected", map[string]any{
		"session": req.SessionKey,
		"hits":    len(hits),
	})
	return resp, nil
}

func (m *armsContextManager) Compact(ctx context.Context, req *CompactRequest) error {
	return m.base.Compact(ctx, req)
}

func (m *armsContextManager) Ingest(ctx context.Context, req *IngestRequest) error {
	err := m.base.Ingest(ctx, req)
	if m.arms == nil || req == nil {
		return err
	}
	count, armsErr := m.arms.IndexMessage(ctx, req.SessionKey, armsmemory.MessageInput{
		Role:    req.Message.Role,
		Content: req.Message.Content,
	})
	if armsErr != nil {
		logger.WarnCF("memory", "ARMS methodology indexing failed", map[string]any{
			"session": req.SessionKey,
			"role":    req.Message.Role,
			"error":   armsErr.Error(),
		})
		return err
	}
	if count > 0 {
		logger.InfoCF("memory", "ARMS methodology indexed", map[string]any{
			"session": req.SessionKey,
			"role":    req.Message.Role,
			"count":   count,
		})
	}
	return err
}

func (m *armsContextManager) Clear(ctx context.Context, sessionKey string) error {
	// ARMS is self-growth memory, not session memory. Clearing a session must not
	// delete reusable methodology notes.
	return m.base.Clear(ctx, sessionKey)
}

func (m *armsContextManager) bootstrap(al *AgentLoop) {
	if m == nil || m.arms == nil || al == nil || al.registry == nil {
		return
	}
	agent := al.registry.GetDefaultAgent()
	if agent == nil || agent.Sessions == nil {
		return
	}
	indexed := 0
	ctx := context.Background()
	for _, sessionKey := range agent.Sessions.ListSessions() {
		for _, msg := range agent.Sessions.GetHistory(sessionKey) {
			count, err := m.arms.IndexMessage(ctx, sessionKey, armsmemory.MessageInput{
				Role:    msg.Role,
				Content: msg.Content,
			})
			if err != nil {
				logger.WarnCF("memory", "ARMS methodology bootstrap failed", map[string]any{
					"session": sessionKey,
					"role":    msg.Role,
					"error":   err.Error(),
				})
				continue
			}
			indexed += count
		}
	}
	if indexed > 0 {
		logger.InfoCF("memory", "ARMS methodology bootstrap indexed", map[string]any{
			"count": indexed,
		})
	}
}
