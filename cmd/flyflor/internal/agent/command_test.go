package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAgentCommand(t *testing.T) {
	cmd := NewAgentCommand()
	require.NotNil(t, cmd)

	assert.Equal(t, "agent", cmd.Use)
	assert.Equal(t, []string{"chat"}, cmd.Aliases)
	assert.False(t, cmd.HasSubCommands())
	assert.NotNil(t, cmd.RunE)

	for _, name := range []string{"debug", "message", "session", "model"} {
		assert.NotNil(t, cmd.Flags().Lookup(name), "flag %s should exist", name)
	}
}

func TestNewSessionsCommand(t *testing.T) {
	cmd := NewSessionsCommand()
	require.NotNil(t, cmd)
	assert.Equal(t, "sessions", cmd.Use)
	assert.Contains(t, cmd.Aliases, "session")
	assert.NotNil(t, cmd.RunE)
}
