package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sipeed/picoclaw/cmd/picoclaw/internal"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/cliui"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/memory"
	"github.com/sipeed/picoclaw/pkg/providers"
	legacySession "github.com/sipeed/picoclaw/pkg/session"
)

type sessionSummary struct {
	Key      string
	Count    int
	Summary  string
	LastRole string
	LastText string
}

func NewSessionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sessions",
		Aliases: []string{"session"},
		Short:   "查看可继续的 Flyflor 会话",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sessions, err := loadSessionSummaries()
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), cliui.RenderSessionList(sessionsToRows(sessions)))
			return nil
		},
	}
	return cmd
}

// LoadSessionRowsForMenu returns session rows for the root interactive command
// menu without rendering the sessions command output.
func LoadSessionRowsForMenu() ([]cliui.SessionRow, error) {
	sessions, err := loadSessionSummaries()
	if err != nil {
		return nil, err
	}
	return sessionsToRows(sessions), nil
}

func completeExistingSessions(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	keys, err := listExistingSessionKeys()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return keys, cobra.ShellCompDirectiveNoFileComp
}

func listExistingSessionKeys() ([]string, error) {
	sessions, err := loadSessionSummaries()
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(sessions))
	for _, session := range sessions {
		keys = append(keys, session.Key)
	}
	return keys, nil
}

func loadSessionSummaries() ([]sessionSummary, error) {
	configPath := internal.GetConfigPath()
	cfg := config.DefaultConfig()
	if _, statErr := os.Stat(configPath); statErr == nil {
		loaded, err := config.LoadConfig(configPath)
		if err != nil {
			return nil, fmt.Errorf("加载配置失败: %w", err)
		}
		cfg = loaded
	}
	dir := filepath.Join(cfg.WorkspacePath(), "sessions")
	store, err := memory.NewJSONLStore(dir)
	if err == nil {
		defer store.Close()
		if _, migrateErr := memory.MigrateFromJSON(context.Background(), dir, store); migrateErr != nil {
			legacy := legacySession.NewSessionManager(dir)
			return sessionSummariesFromLegacy(legacy), nil
		}
		return sessionSummariesFromStore(store), nil
	}
	legacy := legacySession.NewSessionManager(dir)
	return sessionSummariesFromLegacy(legacy), nil
}

func sessionSummariesFromStore(store *memory.JSONLStore) []sessionSummary {
	ctx := context.Background()
	keys := store.ListSessions()
	sort.Strings(keys)
	rows := make([]sessionSummary, 0, len(keys))
	for _, key := range keys {
		history, _ := store.GetHistory(ctx, key)
		summary, _ := store.GetSummary(ctx, key)
		rows = append(rows, makeSessionSummary(key, history, summary))
	}
	return rows
}

func sessionSummariesFromLegacy(store *legacySession.SessionManager) []sessionSummary {
	keys := store.ListSessions()
	sort.Strings(keys)
	rows := make([]sessionSummary, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, makeSessionSummary(key, store.GetHistory(key), store.GetSummary(key)))
	}
	return rows
}

func makeSessionSummary(key string, history []providers.Message, summary string) sessionSummary {
	row := sessionSummary{
		Key:     key,
		Count:   len(history),
		Summary: strings.TrimSpace(summary),
	}
	for i := len(history) - 1; i >= 0; i-- {
		content := strings.TrimSpace(history[i].Content)
		if content == "" {
			continue
		}
		row.LastRole = strings.TrimSpace(history[i].Role)
		row.LastText = content
		break
	}
	return row
}

func sessionsToRows(sessions []sessionSummary) []cliui.SessionRow {
	rows := make([]cliui.SessionRow, 0, len(sessions))
	for _, session := range sessions {
		rows = append(rows, cliui.SessionRow{
			Key:      session.Key,
			Messages: session.Count,
			Summary:  session.Summary,
			LastRole: session.LastRole,
			LastText: session.LastText,
		})
	}
	return rows
}
