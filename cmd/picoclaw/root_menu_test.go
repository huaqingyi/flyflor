package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/require"

	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/cliui"
)

func TestRootMenuSectionsExposePrimaryCommands(t *testing.T) {
	sections := rootMenuSections()
	require.Len(t, sections, 4)

	var commands []string
	for _, section := range sections {
		for _, item := range section.Items {
			if len(item.Args) > 0 {
				commands = append(commands, item.Args[0])
			}
		}
	}

	for _, want := range []string{"agent", "sessions", "status", "gateway", "model", "mcp", "skills", "gum", "fx", "dive"} {
		require.Contains(t, commands, want)
	}
}

func TestRootMenuFitsDefaultDockerTTYWidth(t *testing.T) {
	m := rootMenuModel{width: 80, height: 24}
	for _, line := range strings.Split(m.View(), "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), 80, "line too wide: %q", line)
	}
}

func TestRootMenuSelectSessionBuildsAgentCommand(t *testing.T) {
	m := rootMenuModel{
		mode:   "sessions",
		cursor: 0,
		sessions: []cliui.SessionRow{
			{Key: "cli:test", Messages: 3, Summary: "测试会话"},
		},
	}

	next, cmd := m.selectCurrent()
	require.NotNil(t, cmd)

	selected := next.(rootMenuModel)
	require.NotNil(t, selected.action)
	require.Equal(t, []string{"agent", "--session", "cli:test"}, selected.action.Args)
}

func TestRootMenuBackFromSessionSelector(t *testing.T) {
	m := rootMenuModel{
		mode: "sessions",
		sessions: []cliui.SessionRow{
			{Key: "cli:test"},
		},
		cursor: 1,
	}

	next, cmd := m.selectCurrent()
	require.Nil(t, cmd)

	selected := next.(rootMenuModel)
	require.Equal(t, "main", selected.mode)
	require.Nil(t, selected.action)
}
