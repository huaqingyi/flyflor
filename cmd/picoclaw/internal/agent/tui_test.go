package agent

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/cliui"
)

func TestAgentTUIViewShowsSlashCommandMenu(t *testing.T) {
	for _, width := range []int{90, 110, 160} {
		m := agentTUIModel{
			width:       width,
			height:      32,
			session:     "cli:test",
			model:       "test-model",
			input:       "/",
			cursor:      1,
			menuVisible: true,
			status:      "就绪",
			events:      []string{"三层记忆: md + sqlite + qdrant"},
		}

		view := m.View()
		for _, want := range []string{"智能体运行台", "实时对话", "记忆 / 运行状态", "命令扩展菜单", "/bb", "/think", "/memory"} {
			if !strings.Contains(view, want) {
				t.Fatalf("TUI view missing %q:\n%s", want, view)
			}
		}
		if got := lipgloss.Width(view); got > m.width {
			t.Fatalf("TUI rendered width = %d, want <= %d:\n%s", got, m.width, view)
		}
	}
}

func TestAgentTUIBlackboardDrawerIsSeparateFromRightPanel(t *testing.T) {
	m := agentTUIModel{
		width:             110,
		height:            34,
		session:           "cli:test",
		model:             "test-model",
		status:            "就绪",
		events:            []string{"三层记忆: md + sqlite + qdrant"},
		blackboardVisible: true,
		selectedTurn:      0,
		turns: []cliui.BlackboardTurn{
			{Index: 1, User: "解释 bridge 黑板", Assistant: "本轮黑板按提问分组展示。"},
		},
	}

	view := m.View()
	for _, want := range []string{"记忆 / 运行状态", "折叠黑板 / 思考过程", "第1轮", "第1轮摘要", "Bridge 互检"} {
		if !strings.Contains(view, want) {
			t.Fatalf("TUI view missing %q:\n%s", want, view)
		}
	}
	if got := lipgloss.Width(view); got > m.width {
		t.Fatalf("TUI rendered width = %d, want <= %d:\n%s", got, m.width, view)
	}
}

func TestAgentTUIBlackboardFoldNavigation(t *testing.T) {
	m := agentTUIModel{
		width:             110,
		height:            36,
		session:           "cli:test",
		model:             "test-model",
		status:            "就绪",
		blackboardVisible: true,
		selectedTurn:      0,
		expandedNodes:     map[string]bool{},
		turns: []cliui.BlackboardTurn{
			{Index: 1, User: "解释 bridge 黑板", Assistant: "本轮黑板按提问分组展示。"},
		},
	}

	if strings.Contains(m.View(), "阅读方式: 默认只看摘要") {
		t.Fatal("blackboard detail should be collapsed by default")
	}

	next, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	expanded := next.(agentTUIModel)
	expandedView := stripANSIForTest(expanded.View())
	if !strings.Contains(expandedView, "阅读方式: 默认只看摘要") {
		t.Fatalf("expected selected node detail after expand:\n%s", expanded.View())
	}
}

func TestAgentTUIViewShowsInlineThinkingSummary(t *testing.T) {
	m := agentTUIModel{
		width:   112,
		height:  34,
		session: "cli:test",
		model:   "test-model",
		status:  "就绪",
		turns: []cliui.BlackboardTurn{
			{Index: 1, User: "黑板为什么看不懂？", Assistant: "我会按每轮提问分组，并给出可读摘要。"},
		},
	}

	view := stripANSIForTest(m.View())
	for _, want := range []string{"黑板为什么看不懂？", "思考摘要", "Ctrl+T 或 /think 1 展开/折叠", "Flyflor"} {
		if !strings.Contains(view, want) {
			t.Fatalf("inline thinking view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Codex Bridge: 拆解目标") {
		t.Fatalf("inline thinking details should be collapsed by default:\n%s", view)
	}
}

func TestAgentTUIThinkCommandExpandsInlineDetails(t *testing.T) {
	m := agentTUIModel{
		width:   112,
		height:  38,
		session: "cli:test",
		model:   "test-model",
		status:  "就绪",
		turns: []cliui.BlackboardTurn{
			{Index: 1, User: "解释三层记忆", Assistant: "Markdown、SQLite、Qdrant 分别承担身份、时间线和语义召回。"},
		},
	}

	next, _ := m.handleSlashCommand("/think 1")
	expanded := next.(agentTUIModel)
	view := stripANSIForTest(expanded.View())
	for _, want := range []string{"本轮黑板", "Codex Bridge", "拆解目标", "Qdrant", "向量索引"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expanded inline thinking missing %q:\n%s", want, view)
		}
	}
}

func TestAgentTUIHelpAndStatusUseDialogs(t *testing.T) {
	m := agentTUIModel{
		width:   112,
		height:  36,
		session: "cli:test",
		model:   "test-model",
		status:  "就绪",
	}

	next, _ := m.handleSlashCommand("/help")
	help := next.(agentTUIModel)
	if help.dialog == nil || help.dialog.Kind != "help" {
		t.Fatalf("/help should open help dialog, got %#v", help.dialog)
	}
	if view := stripANSIForTest(help.View()); !strings.Contains(view, "命令扩展菜单") || !strings.Contains(view, "/think") {
		t.Fatalf("help dialog missing command content:\n%s", view)
	}

	next, _ = m.handleSlashCommand("/status")
	status := next.(agentTUIModel)
	if status.dialog == nil || status.dialog.Kind != "status" {
		t.Fatalf("/status should open status dialog, got %#v", status.dialog)
	}
	if view := stripANSIForTest(status.View()); !strings.Contains(view, "运行状态") || !strings.Contains(view, "Qdrant") {
		t.Fatalf("status dialog missing memory content:\n%s", view)
	}
}

func TestAgentTUIMemoryMetersExplainPercentMeaning(t *testing.T) {
	t.Setenv("QDRANT_URL", "http://qdrant:6333")
	m := agentTUIModel{
		width:   112,
		height:  34,
		session: "cli:test",
		model:   "test-model",
		status:  "就绪",
	}

	view := stripANSIForTest(m.View())
	for _, want := range []string{"Markdown", "100%", "SQLite", "0%", "Qdrant", "向量索引/相似召回已配置", "百分比表示该记忆层当前接入状态"} {
		if !strings.Contains(view, want) {
			t.Fatalf("memory meter view missing %q:\n%s", want, view)
		}
	}
}

func TestAgentTUIBlackboardScrollAndExit(t *testing.T) {
	m := agentTUIModel{
		width:             110,
		height:            34,
		session:           "cli:test",
		model:             "test-model",
		status:            "就绪",
		blackboardVisible: true,
		selectedTurn:      0,
		blackboardCursor:  4,
		expandedNodes: map[string]bool{
			"turn:1:result": true,
		},
		turns: []cliui.BlackboardTurn{
			{Index: 1, User: "解释 bridge 黑板", Assistant: strings.Repeat("这是一段很长的黑板内容。\n", 40)},
		},
	}

	next, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyPgDown})
	scrolled := next.(agentTUIModel)
	if scrolled.blackboardScroll <= 0 {
		t.Fatalf("expected blackboard scroll to advance, got %d", scrolled.blackboardScroll)
	}

	next, _ = scrolled.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	closed := next.(agentTUIModel)
	if closed.blackboardVisible {
		t.Fatal("q should exit blackboard")
	}
}

func TestAgentTUICtrlCIsTwoStep(t *testing.T) {
	m := agentTUIModel{
		width:   110,
		height:  34,
		session: "cli:test",
		model:   "test-model",
		status:  "就绪",
	}

	next, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	armed := next.(agentTUIModel)
	if cmd != nil {
		t.Fatal("first Ctrl+C should not quit")
	}
	if !armed.exitArmed {
		t.Fatal("first Ctrl+C should arm exit")
	}
	if armed.dialog == nil || armed.dialog.Kind != "exit" {
		t.Fatalf("first Ctrl+C should open exit dialog, got %#v", armed.dialog)
	}
	_, cmd = armed.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("second Ctrl+C should quit")
	}
}

func TestAgentTUIGlowMarkdownRendering(t *testing.T) {
	rendered := tuiGlowMarkdown("# 标题\n\n- **重点**\n- `code`", 64)
	plain := stripANSIForTest(rendered)
	for _, want := range []string{"标题", "重点", "code"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("rendered markdown missing %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(plain, "**重点**") {
		t.Fatalf("markdown emphasis was not rendered:\n%s", rendered)
	}
}

func TestAgentTUIBlackboardArgumentSuggestions(t *testing.T) {
	m := agentTUIModel{
		input: "/bb ",
		turns: []cliui.BlackboardTurn{
			{Index: 1},
			{Index: 2},
		},
	}

	got := m.filteredCommands()
	var names []string
	for _, cmd := range got {
		names = append(names, cmd.Name)
	}
	for _, want := range []string{"/bb latest", "/bb hide", "/bb 1", "/bb 2"} {
		if !strings.Contains(strings.Join(names, "\n"), want) {
			t.Fatalf("suggestions missing %q: %#v", want, names)
		}
	}
}

func TestAgentTUIThinkArgumentSuggestions(t *testing.T) {
	m := agentTUIModel{
		input: "/think ",
		turns: []cliui.BlackboardTurn{
			{Index: 1},
			{Index: 2},
		},
	}

	got := m.filteredCommands()
	var names []string
	for _, cmd := range got {
		names = append(names, cmd.Name)
	}
	for _, want := range []string{"/think latest", "/think hide", "/think 1", "/think 2"} {
		if !strings.Contains(strings.Join(names, "\n"), want) {
			t.Fatalf("think suggestions missing %q: %#v", want, names)
		}
	}
}

func TestAgentTUISlashCommandFuzzySuggestions(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  string
	}{
		{input: "/mem", want: "/memory"},
		{input: "/stat", want: "/status"},
		{input: "/mry", want: "/memory"},
		{input: "/bb", want: "/bb"},
	} {
		m := agentTUIModel{input: tt.input}
		got := m.filteredCommands()
		if len(got) == 0 {
			t.Fatalf("input %q returned no suggestions", tt.input)
		}
		if got[0].Name != tt.want {
			t.Fatalf("input %q first suggestion = %q, want %q; all=%#v", tt.input, got[0].Name, tt.want, got)
		}
	}
}

func TestAgentTUIMenuSubmitDecision(t *testing.T) {
	if shouldAcceptMenuBeforeSubmit("/clear", []slashCommand{{Name: "/clear"}}) {
		t.Fatal("exact slash commands should execute without a second enter")
	}
	if !shouldAcceptMenuBeforeSubmit("/bb ", []slashCommand{{Name: "/bb latest"}}) {
		t.Fatal("argument menus should accept the selected candidate first")
	}
	if !shouldAcceptMenuBeforeSubmit("/he", []slashCommand{{Name: "/help"}}) {
		t.Fatal("partial slash commands should accept the selected candidate first")
	}
}

func stripANSIForTest(s string) string {
	return tuiANSIRe.ReplaceAllString(s, "")
}
