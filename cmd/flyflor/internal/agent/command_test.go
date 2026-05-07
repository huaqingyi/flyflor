package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sipeed/picoclaw/pkg/providers"
)

func TestNewAgentCommand(t *testing.T) {
	cmd := NewAgentCommand()

	require.NotNil(t, cmd)

	assert.Equal(t, "agent", cmd.Use)
	assert.Equal(t, "Chat with Flyflor directly", cmd.Short)

	assert.Equal(t, []string{"chat"}, cmd.Aliases)
	assert.False(t, cmd.HasSubCommands())

	assert.Nil(t, cmd.Run)
	assert.NotNil(t, cmd.RunE)

	assert.Nil(t, cmd.PersistentPreRun)
	assert.Nil(t, cmd.PersistentPostRun)

	assert.True(t, cmd.HasFlags())

	assert.NotNil(t, cmd.Flags().Lookup("debug"))
	assert.NotNil(t, cmd.Flags().Lookup("message"))
	assert.NotNil(t, cmd.Flags().Lookup("session"))
	assert.NotNil(t, cmd.Flags().Lookup("model"))
}

func TestNewSessionsCommand(t *testing.T) {
	cmd := NewSessionsCommand()

	require.NotNil(t, cmd)
	assert.Equal(t, "sessions", cmd.Use)
	assert.Equal(t, "查看可继续的 Flyflor 会话", cmd.Short)
	assert.Contains(t, cmd.Aliases, "session")
	assert.NotNil(t, cmd.RunE)
}

func TestHistoryToBlackboardTurnsGroupsUserAssistantPairs(t *testing.T) {
	turns := historyToBlackboardTurns([]providers.Message{
		{Role: "system", Content: "ignore"},
		{Role: "user", Content: "第一问"},
		{Role: "assistant", Content: "第一答"},
		{Role: "assistant", Content: "补充"},
		{Role: "user", Content: "第二问"},
		{Role: "assistant", Content: "第二答"},
	})

	require.Len(t, turns, 2)
	assert.Equal(t, 1, turns[0].Index)
	assert.Equal(t, "第一问", turns[0].User)
	assert.Contains(t, turns[0].Assistant, "第一答")
	assert.Contains(t, turns[0].Assistant, "补充")
	assert.Equal(t, 2, turns[1].Index)
	assert.Equal(t, "第二问", turns[1].User)
	assert.Equal(t, "第二答", turns[1].Assistant)
}

func TestMatchShortSessionKeySupportsCopiedEllipsis(t *testing.T) {
	keys := []string{
		"sk_v1_cb10fea2000000000000000000000000000000000000a00577b35d7cc1",
		"other",
	}

	got := matchShortSessionKey("sk_v1_cb10fea2…a00577b35d7cc1", keys)
	require.Equal(t, []string{keys[0]}, got)
}
