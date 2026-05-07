package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/require"

	"github.com/sipeed/picoclaw/cmd/flyflor/internal/cliui"
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
	for _, width := range []int{48, 60, 80, 100} {
		m := rootMenuModel{width: width, height: 24}
		for _, line := range strings.Split(m.View(), "\n") {
			require.LessOrEqual(t, lipgloss.Width(line), width, "width=%d line too wide: %q", width, line)
		}
	}
}

func TestRootMenuSessionsViewKeepsTabs(t *testing.T) {
	m := rootMenuModel{
		width: 80,
		mode:  "sessions",
		sessions: []cliui.SessionRow{
			{Key: "cli:test", Messages: 2, Summary: "测试会话"},
		},
	}

	view := stripANSIForRootMenuTest(m.View())
	require.Contains(t, view, "[智能体]")
	require.Contains(t, view, "运行")
	require.Contains(t, view, "配置")
	require.Contains(t, view, "工具")
	require.Contains(t, view, "Agent Sessions")
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

func stripANSIForRootMenuTest(s string) string {
	replacer := strings.NewReplacer(
		"\x1b[0m", "",
		"\x1b[1m", "",
	)
	return replacer.Replace(s)
}
