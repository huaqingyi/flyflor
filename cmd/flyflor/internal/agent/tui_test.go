package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sipeed/picoclaw/cmd/flyflor/internal/cliui"
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
		for _, want := range []string{"智能体运行台", "1 对话", "2 历史 Session", "3 黑板", "4 记忆", "5 设置", "实时对话", "命令扩展菜单", "/edit", "/bb"} {
			if !strings.Contains(view, want) {
				t.Fatalf("TUI view missing %q:\n%s", want, view)
			}
		}
		if got := lipgloss.Width(view); got > m.width {
			t.Fatalf("TUI rendered width = %d, want <= %d:\n%s", got, m.width, view)
		}
	}
}

func TestAgentTUIBlackboardTabShowsFoldTree(t *testing.T) {
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
	for _, want := range []string{"3 黑板", "折叠黑板 / 思考过程", "第1轮", "第1轮摘要", "Worker 讨论"} {
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

func TestAgentTUIViewShowsTranscriptWithMarkdownAnswer(t *testing.T) {
	m := agentTUIModel{
		width:   112,
		height:  34,
		session: "cli:test",
		model:   "test-model",
		status:  "就绪",
		turns: []cliui.BlackboardTurn{
			{Index: 1, User: "黑板为什么看不懂？", Assistant: "## 摘要\n\n- **按轮次分组**\n- 可读黑板"},
		},
	}

	view := stripANSIForTest(m.View())
	for _, want := range []string{"实时对话 · 固定布局", "黑板为什么看不懂？", "😁 完成 > 思考过程", "摘要", "按轮次分组", "可读黑板"} {
		if !strings.Contains(view, want) {
			t.Fatalf("chat transcript missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "| 我：") || strings.Contains(view, "| 回答：") {
		t.Fatalf("chat transcript should use color blocks, not pipe prefixes:\n%s", view)
	}
	if strings.Contains(view, "**按轮次分组**") {
		t.Fatalf("assistant markdown should be rendered, not shown raw:\n%s", view)
	}
	if strings.Contains(view, "Flyflor Planner") {
		t.Fatalf("chat transcript should keep thinking collapsed by default:\n%s", view)
	}
}

func TestAgentTUIChatKeepsHeaderFixedAndScrollsInsideTUI(t *testing.T) {
	longMarkdown := "# 长回答\n\n"
	for i := 1; i <= 28; i++ {
		longMarkdown += fmt.Sprintf("- **段落 %02d**：这是一段用于测试滚动阅读的 Markdown 内容。\n", i)
	}
	m := agentTUIModel{
		width:      112,
		height:     28,
		session:    "cli:test",
		model:      "test-model",
		status:     "就绪",
		chatScroll: tuiScrollBottom,
		turns: []cliui.BlackboardTurn{
			{Index: 1, User: "给我一个很长的 Markdown 回答", Assistant: longMarkdown},
		},
	}

	view := stripANSIForTest(m.View())
	if strings.Contains(view, "对话位置") {
		t.Fatalf("chat should not show virtual scroll controls:\n%s", view)
	}
	if !strings.Contains(view, "Header/Tabs/Input 固定") {
		t.Fatalf("chat should explain fixed TUI layout:\n%s", view)
	}
	if !strings.Contains(view, "段落 28") {
		t.Fatalf("chat should start at bottom of long history:\n%s", view)
	}
	if strings.Contains(view, "**段落") {
		t.Fatalf("assistant markdown should be rendered, not raw markdown:\n%s", view)
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
	if !expanded.expandedTurns[1] {
		t.Fatal("/think 1 should mark the turn as expanded for compatibility")
	}
	view := stripANSIForTest(expanded.View())
	for _, want := range []string{"解释三层记忆", "😁 完成 v 思考过程", "Flyflor Planner", "/bb 1"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expanded thinking missing %q:\n%s", want, view)
		}
	}
}

func TestAgentTUIShowsLiveThinkingSummaryAndElapsed(t *testing.T) {
	m := agentTUIModel{
		width:         112,
		height:        34,
		session:       "cli:test",
		model:         "test-model",
		status:        "组织上下文",
		thinking:      true,
		spinner:       2,
		currentInput:  "解释为什么 TUI 卡顿",
		thinkingStart: time.Now().Add(-3 * time.Second),
		events:        []string{"收到问题: 解释为什么 TUI 卡顿", "召回记忆: TUI 偏好"},
	}

	view := stripANSIForTest(m.View())
	for _, want := range []string{"解释为什么 TUI 卡顿", "😅 攻坚 > 思考过程", "摘要: 召回记忆", "用时 3s", "/bb latest"} {
		if !strings.Contains(view, want) {
			t.Fatalf("live thinking view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "| 回答") || strings.Contains(view, "| 我：") {
		t.Fatalf("live thinking should use color blocks, not pipe prefixes:\n%s", view)
	}
	if strings.Contains(view, "回答：") {
		t.Fatalf("live thinking should not show final answer block yet:\n%s", view)
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
	if view := stripANSIForTest(help.View()); !strings.Contains(view, "命令扩展菜单") || !strings.Contains(view, "/edit") || !strings.Contains(view, "/think") {
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

func TestAgentTUILongTextEditorMode(t *testing.T) {
	m := agentTUIModel{
		width:     112,
		height:    34,
		session:   "cli:test",
		model:     "test-model",
		status:    "就绪",
		activeTab: agentTabMemory,
	}

	next, _ := m.handleSlashCommand("/edit")
	editing := next.(agentTUIModel)
	if !editing.inputExpanded {
		t.Fatal("/edit should expand the long text input area")
	}

	next, _ = editing.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("第一行")})
	editing = next.(agentTUIModel)
	next, _ = editing.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	editing = next.(agentTUIModel)
	next, _ = editing.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("第二行")})
	editing = next.(agentTUIModel)

	if editing.input != "第一行\n第二行" {
		t.Fatalf("expanded editor should keep newlines, got %q", editing.input)
	}
	view := stripANSIForTest(editing.View())
	for _, want := range []string{"长文本", "第一行", "第二行", "Ctrl+D 发送"} {
		if !strings.Contains(view, want) {
			t.Fatalf("long text editor view missing %q:\n%s", want, view)
		}
	}
}

func TestAgentTUIYoloModeCommandAndPromptHint(t *testing.T) {
	m := agentTUIModel{width: 100, height: 30, status: "就绪"}

	next, _ := m.handleSlashCommand("/yolo")
	yolo := next.(agentTUIModel)
	if !yolo.yoloMode {
		t.Fatal("/yolo should enable yolo mode")
	}
	if got := yolo.promptForAgent("修复测试"); !strings.Contains(got, "/yolo is ON") || !strings.Contains(got, "修复测试") {
		t.Fatalf("yolo prompt missing autonomy hint: %q", got)
	}
	if view := stripANSIForTest(yolo.View()); !strings.Contains(view, "YOLO") || !strings.Contains(view, "/yolo") {
		t.Fatalf("yolo view should expose mode and command:\n%s", view)
	}

	next, _ = yolo.handleSlashCommand("/yolo off")
	standard := next.(agentTUIModel)
	if standard.yoloMode {
		t.Fatal("/yolo off should disable yolo mode")
	}
	if got := standard.promptForAgent("修复测试"); strings.Contains(got, "/yolo is ON") {
		t.Fatalf("standard prompt should not include yolo hint: %q", got)
	}
}

func TestAgentTUICtrlSWhileThinkingDoesNotQueueAsNormalSubmit(t *testing.T) {
	m := agentTUIModel{
		width:    100,
		height:   30,
		status:   "调用模型",
		thinking: true,
		input:    "/help",
		cursor:   len([]rune("/help")),
	}

	next, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlS})
	steering := next.(agentTUIModel)
	if cmd != nil {
		t.Fatal("ctrl+s steering path should not start a normal submit command while thinking")
	}
	if steering.input != "/help" {
		t.Fatalf("slash input should be preserved while a turn is running, got %q", steering.input)
	}
	if !strings.Contains(steering.status, "回答进行中") {
		t.Fatalf("status should explain running-turn behavior, got %q", steering.status)
	}
}

func TestAgentTUITabSwitchingAndSessionPanel(t *testing.T) {
	m := agentTUIModel{
		width:   112,
		height:  34,
		session: "cli:current",
		model:   "test-model",
		status:  "就绪",
		sessionRows: []cliui.SessionRow{
			{Key: "cli:current", Messages: 2, Summary: "当前会话"},
			{Key: "cli:other", Messages: 4, Summary: "历史会话"},
		},
	}

	next, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	sessions := next.(agentTUIModel)
	if sessions.currentTab() != agentTabSessions {
		t.Fatalf("Tab should switch to sessions tab, got %v", sessions.currentTab())
	}
	view := stripANSIForTest(sessions.View())
	for _, want := range []string{"历史 Session", "cli:current", "cli:other", "Enter 切换当前 TUI 会话"} {
		if !strings.Contains(view, want) {
			t.Fatalf("sessions tab missing %q:\n%s", want, view)
		}
	}

	next, _ = sessions.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	settings := next.(agentTUIModel)
	if settings.currentTab() != agentTabSettings {
		t.Fatalf("number key should switch to settings tab, got %v", settings.currentTab())
	}
	if view := stripANSIForTest(settings.View()); !strings.Contains(view, "flyflor setup") || !strings.Contains(view, "fx") || !strings.Contains(view, "gum") {
		t.Fatalf("settings tab missing setup/tool guidance:\n%s", view)
	}
}

func TestAgentTUITabsFitNarrowTTYWidths(t *testing.T) {
	for _, width := range []int{48, 60, 80, 112} {
		for _, tab := range []agentTUITab{agentTabChat, agentTabSessions, agentTabBlackboard, agentTabMemory, agentTabSettings} {
			m := agentTUIModel{
				width:     width,
				height:    32,
				session:   "cli:test",
				model:     "test-model",
				status:    "就绪",
				activeTab: tab,
				sessionRows: []cliui.SessionRow{
					{Key: "cli:test", Messages: 2, Summary: "当前会话"},
				},
				selectedTurn: 0,
				turns: []cliui.BlackboardTurn{
					{Index: 1, User: "测试窄屏换行", Assistant: "回答内容"},
				},
			}
			if tab == agentTabBlackboard {
				m.blackboardVisible = true
			}
			for _, line := range strings.Split(m.View(), "\n") {
				if got := lipgloss.Width(line); got > width {
					t.Fatalf("tab=%v width=%d line width=%d:\n%s", tab, width, got, line)
				}
			}
		}
	}
}

func TestAgentTUIMemoryTabShowsEditableMarkdownFiles(t *testing.T) {
	t.Setenv("QDRANT_URL", "http://qdrant:6333")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "SOUL.md"), []byte("# 人格记忆\n\n- **直接**、清晰。"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "USER.md"), []byte("# 特征记忆\n\n- 喜欢紫粉配色。"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "memory", "MEMORY.md"), []byte("# 长期记忆\n\n- Docker compose dev。"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := agentTUIModel{
		width:     112,
		height:    34,
		session:   "cli:test",
		model:     "test-model",
		status:    "就绪",
		activeTab: agentTabMemory,
		workspace: workspace,
	}

	view := stripANSIForTest(m.View())
	for _, want := range []string{"记忆 / Markdown 可编辑", "人格记忆", "特征记忆", "长期记忆", "SOUL.md", "USER.md", "memory/MEMORY.md", "Enter/e 编辑", "直接"} {
		if !strings.Contains(view, want) {
			t.Fatalf("memory markdown view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "**直接**") {
		t.Fatalf("memory preview should render markdown, not raw emphasis:\n%s", view)
	}
}

func TestAgentTUIMemoryEditorSavesSelectedMarkdownFile(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "USER.md"), []byte("# USER\n\n- 初始偏好。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := agentTUIModel{
		width:        112,
		height:       34,
		session:      "cli:test",
		model:        "test-model",
		status:       "就绪",
		activeTab:    agentTabMemory,
		workspace:    workspace,
		memoryCursor: 1,
	}

	next, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	editing := next.(agentTUIModel)
	if !editing.memoryEditing {
		t.Fatal("Enter on memory tab should open the selected markdown memory editor")
	}
	next, _ = editing.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("\n- 新增特征记忆。")})
	editing = next.(agentTUIModel)
	next, _ = editing.handleKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	saved := next.(agentTUIModel)
	if saved.memoryEditing {
		t.Fatal("Ctrl+D should save and leave memory edit mode")
	}
	data, err := os.ReadFile(filepath.Join(workspace, "USER.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "新增特征记忆") {
		t.Fatalf("saved USER.md missing edited memory:\n%s", string(data))
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
