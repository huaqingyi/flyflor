package agent

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	glowui "github.com/charmbracelet/glow/v2/ui"
	"github.com/charmbracelet/lipgloss"

	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/cliui"
	coreagent "github.com/sipeed/picoclaw/pkg/agent"
)

var tuiANSIRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

var (
	tuiInk       = lipgloss.Color("#E9EAF2")
	tuiSubtle    = lipgloss.Color("#9699A8")
	tuiDim       = lipgloss.Color("#666A78")
	tuiBase      = lipgloss.Color("#101116")
	tuiSurface   = lipgloss.Color("#171922")
	tuiSurfaceHi = lipgloss.Color("#202332")
	tuiBorder    = lipgloss.Color("#34384A")
	tuiViolet    = lipgloss.Color("#8A7DFF")
	tuiCyan      = lipgloss.Color("#61E7D6")
	tuiPink      = lipgloss.Color("#F06292")
	tuiAmber     = lipgloss.Color("#F2C36B")
	tuiGreen     = lipgloss.Color("#7DDC98")
	tuiRed       = lipgloss.Color("#FF6B6B")
)

var tuiMarkdownRenderers = map[int]*glamour.TermRenderer{}

var tuiGlowConfig = glowui.Config{
	GlamourStyle:     "dark",
	GlamourEnabled:   true,
	PreserveNewLines: true,
}

type slashCommand struct {
	Name string
	Args string
	Desc string
}

var agentSlashCommands = []slashCommand{
	{Name: "/help", Desc: "弹窗显示 / 命令扩展菜单"},
	{Name: "/bb", Args: "[latest|编号|hide]", Desc: "打开或切换本轮黑板"},
	{Name: "/blackboard", Args: "[latest|编号|hide]", Desc: "同 /bb，显示黑板详情"},
	{Name: "/think", Args: "[latest|编号|hide]", Desc: "展开或折叠对话流里的思考过程"},
	{Name: "/memory", Desc: "聚焦右侧记忆面板并隐藏黑板"},
	{Name: "/status", Desc: "弹窗显示当前会话、模型和记忆状态"},
	{Name: "/clear", Desc: "清空当前 TUI 对话窗口"},
	{Name: "/prev", Desc: "切换到上一轮黑板分组"},
	{Name: "/next", Desc: "切换到下一轮黑板分组"},
	{Name: "/exit", Desc: "退出 Flyflor TUI"},
	{Name: "/quit", Desc: "退出 Flyflor TUI"},
}

type agentTUIModel struct {
	agentLoop *coreagent.AgentLoop
	session   string
	model     string

	width  int
	height int

	input        string
	cursor       int
	turns        []cliui.BlackboardTurn
	events       []string
	status       string
	errText      string
	thinking     bool
	spinner      int
	currentInput string
	activePanel  string

	menuVisible bool
	menuIndex   int
	exitArmed   bool
	dialog      *tuiDialog

	blackboardVisible bool
	selectedTurn      int
	blackboardCursor  int
	blackboardScroll  int
	expandedNodes     map[string]bool
	expandedTurns     map[int]bool
}

type blackboardFoldNode struct {
	ID         string
	Title      string
	Summary    string
	Detail     string
	Depth      int
	Expandable bool
}

type tuiDialog struct {
	Kind   string
	Title  string
	Body   string
	Footer string
	Color  lipgloss.Color
}

type agentResponseMsg struct {
	input    string
	response string
	err      error
}

type agentTickMsg time.Time

func runAgentTUI(agentLoop *coreagent.AgentLoop, sessionKey, modelLabel string) error {
	m := agentTUIModel{
		agentLoop:         agentLoop,
		session:           sessionKey,
		model:             modelLabel,
		width:             110,
		height:            32,
		turns:             loadSessionTurns(agentLoop, sessionKey),
		status:            "就绪",
		activePanel:       "chat",
		selectedTurn:      -1,
		expandedNodes:     map[string]bool{},
		expandedTurns:     map[int]bool{},
		blackboardVisible: false,
		events: []string{
			"三层记忆: md + sqlite + qdrant",
			"Bridge: Codex + Claude/OpenCode",
			"Markdown: Glow 风格渲染已启用",
			"每轮提问会在对话流内显示可折叠思考摘要",
		},
	}
	if len(m.turns) > 0 {
		m.selectedTurn = len(m.turns) - 1
		m.events = appendRecent(m.events, fmt.Sprintf("已载入历史对话: %d 轮", len(m.turns)), 7)
	}
	_, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}

func (m agentTUIModel) Init() tea.Cmd {
	return tickAgentTUI()
}

func tickAgentTUI() tea.Cmd {
	return tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg {
		return agentTickMsg(t)
	})
}

func (m agentTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case agentTickMsg:
		if m.thinking {
			m.spinner++
			m.status = m.dynamicThinkingStatus()
		}
		return m, tickAgentTUI()
	case agentResponseMsg:
		m.thinking = false
		m.currentInput = ""
		if msg.err != nil {
			m.errText = msg.err.Error()
			m.status = "请求失败"
			m.events = appendRecent(m.events, "模型返回错误: "+msg.err.Error(), 7)
			return m, nil
		}
		m.status = "回答完成，已写入本轮黑板"
		m.turns = append(m.turns, cliui.BlackboardTurn{
			Index:     len(m.turns) + 1,
			User:      msg.input,
			Assistant: msg.response,
			StartedAt: time.Now(),
		})
		m.selectedTurn = len(m.turns) - 1
		m.blackboardCursor = 0
		m.events = appendRecent(m.events, "完成: "+shortLocal(msg.input, 42), 7)
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	default:
		return m, nil
	}
}

func (m agentTUIModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type != tea.KeyCtrlC {
		m.exitArmed = false
	}
	if m.dialog != nil {
		return m.handleDialogKey(msg)
	}
	switch msg.Type {
	case tea.KeyCtrlC:
		if m.menuVisible {
			m.menuVisible = false
			m.status = "已关闭命令菜单"
			return m, nil
		}
		if m.blackboardVisible {
			m.blackboardVisible = false
			m.activePanel = "memory"
			m.status = "已退出黑板；再次 Ctrl+C 才会退出会话"
			return m, nil
		}
		if strings.TrimSpace(m.input) != "" {
			m.input = ""
			m.cursor = 0
			m.status = "已清空输入；再次 Ctrl+C 才会退出会话"
			return m, nil
		}
		if !m.exitArmed {
			m.exitArmed = true
			m.dialog = exitConfirmDialog()
			m.status = "已打开退出确认弹窗"
			return m, nil
		}
		return m, tea.Quit
	case tea.KeyEsc:
		if m.menuVisible {
			m.menuVisible = false
			return m, nil
		}
		if m.blackboardVisible {
			m.blackboardVisible = false
			m.activePanel = "memory"
			m.status = "黑板已隐藏"
			return m, nil
		}
		return m, nil
	case tea.KeyPgUp, tea.KeyCtrlU:
		if m.canNavigateBlackboard() {
			m.scrollBlackboard(-m.blackboardPageSize())
			return m, nil
		}
		return m, nil
	case tea.KeyPgDown, tea.KeyCtrlD:
		if m.canNavigateBlackboard() {
			m.scrollBlackboard(m.blackboardPageSize())
			return m, nil
		}
		return m, nil
	case tea.KeyTab:
		if m.menuVisible {
			m.acceptMenuSelection()
			return m, nil
		}
		m.togglePanel()
		return m, nil
	case tea.KeyCtrlT:
		m.toggleLatestThinking()
		return m, nil
	case tea.KeyUp:
		if m.menuVisible {
			m.menuIndex--
			if m.menuIndex < 0 {
				m.menuIndex = len(m.filteredCommands()) - 1
			}
		} else if m.canNavigateBlackboard() {
			m.moveBlackboardCursor(-1)
		}
		return m, nil
	case tea.KeyDown:
		if m.menuVisible {
			m.menuIndex++
			if count := len(m.filteredCommands()); count > 0 {
				m.menuIndex %= count
			}
		} else if m.canNavigateBlackboard() {
			m.moveBlackboardCursor(1)
		}
		return m, nil
	case tea.KeyLeft:
		if m.canNavigateBlackboard() {
			m.setSelectedBlackboardExpanded(false)
			return m, nil
		}
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case tea.KeyRight:
		if m.canNavigateBlackboard() {
			m.setSelectedBlackboardExpanded(true)
			return m, nil
		}
		if m.cursor < len([]rune(m.input)) {
			m.cursor++
		}
		return m, nil
	case tea.KeyBackspace, tea.KeyCtrlH:
		m.backspace()
		return m, nil
	case tea.KeyEnter:
		if m.canNavigateBlackboard() && strings.TrimSpace(m.input) == "" {
			m.toggleSelectedBlackboardNode()
			return m, nil
		}
		return m.submit()
	case tea.KeyRunes:
		if len(msg.Runes) == 1 && msg.Runes[0] == '?' && strings.TrimSpace(m.input) == "" {
			m.dialog = m.helpDialog()
			m.status = "已打开帮助弹窗"
			return m, nil
		}
		if m.canNavigateBlackboard() && strings.TrimSpace(m.input) == "" && len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'q', 'Q':
				m.blackboardVisible = false
				m.activePanel = "memory"
				m.status = "已退出黑板"
				return m, nil
			case 'j', 'J':
				m.moveBlackboardCursor(1)
				return m, nil
			case 'k', 'K':
				m.moveBlackboardCursor(-1)
				return m, nil
			case 'h', 'H':
				m.setSelectedBlackboardExpanded(false)
				return m, nil
			case 'l', 'L':
				m.setSelectedBlackboardExpanded(true)
				return m, nil
			case 'd', 'D':
				m.scrollBlackboard(m.blackboardPageSize())
				return m, nil
			case 'u', 'U':
				m.scrollBlackboard(-m.blackboardPageSize())
				return m, nil
			}
		}
		if m.canNavigateBlackboard() && len(msg.Runes) == 1 && msg.Runes[0] == ' ' {
			m.toggleSelectedBlackboardNode()
			return m, nil
		}
		for _, r := range msg.Runes {
			m.insertRune(r)
		}
		m.syncMenu()
		return m, nil
	default:
		return m, nil
	}
}

func (m agentTUIModel) handleDialogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		if m.dialog != nil && m.dialog.Kind == "exit" {
			return m, tea.Quit
		}
		m.dialog = nil
		m.status = "弹窗已关闭"
		return m, nil
	case tea.KeyEsc:
		m.dialog = nil
		m.exitArmed = false
		m.status = "弹窗已关闭"
		return m, nil
	case tea.KeyEnter:
		if m.dialog != nil && m.dialog.Kind == "exit" {
			return m, tea.Quit
		}
		m.dialog = nil
		m.status = "弹窗已关闭"
		return m, nil
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'q', 'Q':
				m.dialog = nil
				m.exitArmed = false
				m.status = "弹窗已关闭"
				return m, nil
			case 'y', 'Y':
				if m.dialog != nil && m.dialog.Kind == "exit" {
					return m, tea.Quit
				}
			case 'n', 'N':
				if m.dialog != nil && m.dialog.Kind == "exit" {
					m.dialog = nil
					m.exitArmed = false
					m.status = "已取消退出"
					return m, nil
				}
			}
		}
		return m, nil
	default:
		return m, nil
	}
}

func (m agentTUIModel) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if !m.canNavigateBlackboard() {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.scrollBlackboard(-3)
	case tea.MouseButtonWheelDown:
		m.scrollBlackboard(3)
	}
	return m, nil
}

func (m *agentTUIModel) insertRune(r rune) {
	rs := []rune(m.input)
	if m.cursor < 0 || m.cursor > len(rs) {
		m.cursor = len(rs)
	}
	rs = append(rs[:m.cursor], append([]rune{r}, rs[m.cursor:]...)...)
	m.input = string(rs)
	m.cursor++
}

func (m *agentTUIModel) backspace() {
	rs := []rune(m.input)
	if m.cursor <= 0 || len(rs) == 0 {
		return
	}
	rs = append(rs[:m.cursor-1], rs[m.cursor:]...)
	m.input = string(rs)
	m.cursor--
	m.syncMenu()
}

func (m *agentTUIModel) syncMenu() {
	trimmed := strings.TrimSpace(m.input)
	m.menuVisible = strings.HasPrefix(trimmed, "/")
	if !m.menuVisible {
		m.menuIndex = 0
		return
	}
	if count := len(m.filteredCommands()); count == 0 {
		m.menuIndex = 0
	} else if m.menuIndex >= count {
		m.menuIndex = count - 1
	}
}

func (m *agentTUIModel) acceptMenuSelection() {
	filtered := m.filteredCommands()
	if len(filtered) == 0 {
		return
	}
	if m.menuIndex < 0 || m.menuIndex >= len(filtered) {
		m.menuIndex = 0
	}
	selected := filtered[m.menuIndex]
	if selected.Args != "" {
		m.input = selected.Name + " "
	} else {
		m.input = selected.Name
	}
	m.cursor = len([]rune(m.input))
	m.menuVisible = false
}

func (m agentTUIModel) submit() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.input)
	if input == "" || m.thinking {
		return m, nil
	}
	if m.menuVisible && shouldAcceptMenuBeforeSubmit(input, m.filteredCommands()) {
		m.acceptMenuSelection()
		return m, nil
	}
	m.input = ""
	m.cursor = 0
	m.menuVisible = false

	if strings.HasPrefix(input, "/") {
		return m.handleSlashCommand(input)
	}

	m.errText = ""
	m.thinking = true
	m.currentInput = input
	m.status = "正在理解问题"
	if m.blackboardVisible {
		m.selectedTurn = m.blackboardTurnCount() - 1
		m.blackboardCursor = 0
		m.blackboardScroll = 0
	}
	m.events = appendRecent(m.events, "收到问题: "+shortLocal(input, 42), 7)
	return m, m.sendToAgent(input)
}

func shouldAcceptMenuBeforeSubmit(input string, filtered []slashCommand) bool {
	trimmed := strings.TrimSpace(input)
	if len(filtered) == 0 {
		return false
	}
	if strings.HasSuffix(input, " ") {
		return true
	}
	if !strings.Contains(trimmed, " ") {
		for _, cmd := range agentSlashCommands {
			if cmd.Name == trimmed {
				return false
			}
		}
		return true
	}
	for _, cmd := range filtered {
		if cmd.Name == trimmed {
			return false
		}
	}
	return true
}

func (m agentTUIModel) handleSlashCommand(input string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return m, nil
	}
	switch fields[0] {
	case "/exit", "/quit":
		m.dialog = exitConfirmDialog()
		m.status = "已打开退出确认弹窗"
	case "/help":
		m.dialog = m.helpDialog()
		m.menuVisible = false
		m.status = "已打开 / 命令帮助弹窗"
	case "/memory":
		m.blackboardVisible = false
		m.activePanel = "memory"
		m.status = "右侧记忆面板已聚焦"
	case "/clear":
		m.turns = nil
		m.selectedTurn = -1
		m.blackboardVisible = false
		m.blackboardCursor = 0
		m.expandedNodes = map[string]bool{}
		m.events = []string{"已清空当前 TUI 对话窗口"}
		m.status = "已清空"
	case "/status":
		m.dialog = m.statusDialog()
		m.events = appendRecent(m.events, fmt.Sprintf("状态: session=%s model=%s turns=%d", m.session, m.model, len(m.turns)), 7)
		m.status = "已打开状态弹窗"
	case "/bb", "/blackboard":
		m.applyBlackboardCommand(fields[1:])
	case "/think":
		m.applyThinkingCommand(fields[1:])
	case "/prev":
		m.shiftBlackboard(-1)
	case "/next":
		m.shiftBlackboard(1)
	default:
		m.errText = "未知命令: " + fields[0]
		m.menuVisible = true
	}
	return m, nil
}

func (m *agentTUIModel) applyBlackboardCommand(args []string) {
	if len(args) > 0 && (args[0] == "hide" || args[0] == "off") {
		m.blackboardVisible = false
		m.activePanel = "memory"
		m.status = "黑板已隐藏"
		return
	}
	count := m.blackboardTurnCount()
	if count == 0 {
		m.blackboardVisible = true
		m.activePanel = "memory"
		m.status = "暂无黑板记录"
		return
	}
	selected := count - 1
	if len(args) > 0 && args[0] != "latest" {
		if n, err := strconv.Atoi(args[0]); err == nil {
			selected = n - 1
		}
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= count {
		selected = count - 1
	}
	m.selectedTurn = selected
	m.blackboardCursor = 0
	m.blackboardScroll = 0
	m.blackboardVisible = true
	m.activePanel = "memory"
	m.status = fmt.Sprintf("已打开第 %d 轮折叠黑板", selected+1)
}

func (m *agentTUIModel) shiftBlackboard(delta int) {
	count := m.blackboardTurnCount()
	if count == 0 {
		m.blackboardVisible = true
		m.status = "暂无黑板记录"
		return
	}
	if m.selectedTurn < 0 || m.selectedTurn >= count {
		m.selectedTurn = count - 1
	}
	m.selectedTurn += delta
	if m.selectedTurn < 0 {
		m.selectedTurn = count - 1
	}
	if m.selectedTurn >= count {
		m.selectedTurn = 0
	}
	m.blackboardCursor = 0
	m.blackboardScroll = 0
	m.blackboardVisible = true
	m.activePanel = "memory"
	m.status = fmt.Sprintf("已切换到第 %d 轮黑板分组", m.selectedTurn+1)
}

func (m agentTUIModel) sendToAgent(input string) tea.Cmd {
	return func() tea.Msg {
		response, err := m.agentLoop.ProcessDirect(context.Background(), input, m.session)
		return agentResponseMsg{input: input, response: response, err: err}
	}
}

func (m agentTUIModel) View() string {
	width := m.width
	if width < 82 {
		width = 82
	}
	height := m.height
	if height < 24 {
		height = 24
	}
	frameW := width - 2
	contentW := frameW - 2
	gapW := 2
	leftW := (contentW - gapW) * 58 / 100
	if leftW < 40 {
		leftW = 40
	}
	rightW := contentW - gapW - leftW
	if rightW < 28 {
		rightW = 28
		leftW = contentW - gapW - rightW
	}

	menuH := 0
	if m.menuVisible {
		menuH = len(m.filteredCommands()) + 2
		if menuH < 5 {
			menuH = 5
		}
		if menuH > 11 {
			menuH = 11
		}
	}
	dialogH := 0
	if m.dialog != nil {
		dialogH = 14
	}
	drawerH := 0
	blackboard := ""
	if m.blackboardVisible {
		drawerH = 12
		if height >= 34 {
			drawerH = 14
		}
		if height >= 40 {
			drawerH = 16
		}
		blackboard = "\n" + m.blackboardFoldTree(contentW, drawerH)
	}
	bodyH := height - 9 - drawerH - menuH - dialogH
	if bodyH < 9 {
		bodyH = 9
	}

	header := m.header(contentW)
	left := m.chatPanel(leftW, bodyH)
	right := m.contextPanel(rightW, bodyH)
	input := m.inputPanel(contentW)
	menu := ""
	if m.menuVisible {
		menu = "\n" + m.commandMenu(contentW)
	}
	dialog := ""
	if m.dialog != nil {
		dialog = "\n" + m.dialogBox(contentW)
	}
	footer := m.footer(contentW)

	frame := lipgloss.NewStyle().
		Width(frameW).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(tuiViolet).
		Background(tuiBase).
		Padding(0, 1).
		Render(
			header + "\n" +
				lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right) + "\n" +
				blackboard + "\n" +
				dialog +
				input + menu + "\n" +
				footer,
		)
	return frame
}

func (m agentTUIModel) header(width int) string {
	statusColor := tuiGreen
	if m.thinking {
		statusColor = tuiAmber
	}
	if m.errText != "" {
		statusColor = tuiRed
	}
	left := lipgloss.NewStyle().
		Foreground(tuiBase).
		Background(tuiCyan).
		Bold(true).
		Padding(0, 1).
		Render("FLYFLOR")
	title := lipgloss.NewStyle().
		Foreground(tuiInk).
		Background(tuiSurfaceHi).
		Bold(true).
		Padding(0, 1).
		Render("智能体运行台")
	status := tuiPill("状态", m.status, statusColor)
	bridge := tuiPill("互检", "Codex + Claude/OpenCode", tuiViolet)
	model := tuiPill("模型", shortLocal(m.model, 24), tuiCyan)
	line := lipgloss.JoinHorizontal(lipgloss.Center, left, " ", title, " ", status, " ", bridge, " ", model)
	if lipgloss.Width(line) > width {
		line = lipgloss.JoinHorizontal(lipgloss.Center, left, " ", title, " ", status)
	}
	return lipgloss.NewStyle().
		Width(width).
		Foreground(tuiInk).
		Background(tuiSurfaceHi).
		Render(line)
}

func (m agentTUIModel) footer(width int) string {
	items := []string{
		tuiKeyHint("ctrl+c", "取消/二次退出"),
		tuiKeyHint("ctrl+t", "思考展开"),
		tuiKeyHint("enter", "发送/执行"),
		tuiKeyHint("tab", "黑板/补全"),
		tuiKeyHint("↑↓", "选择"),
		tuiKeyHint("pgup/pgdn", "滚动"),
		tuiKeyHint("esc/q", "退出黑板"),
		tuiKeyHint("?/help", "弹窗"),
	}
	return lipgloss.NewStyle().
		Width(width).
		Foreground(tuiSubtle).
		Background(tuiBase).
		Render(strings.Join(items, tuiMuted().Render("  │  ")))
}

func (m agentTUIModel) chatPanel(width, height int) string {
	var lines []string
	if len(m.turns) == 0 && !m.thinking {
		lines = append(lines,
			tuiWelcomeBlock(width-4),
		)
	}
	start := len(m.turns) - 3
	if start < 0 {
		start = 0
	}
	for _, turn := range m.turns[start:] {
		lines = append(lines,
			tuiMessageBlock("你", turn.User, width-4, tuiCyan, false),
			m.inlineThinkingBlock(turn, false, width-4),
			tuiMessageBlock("Flyflor", turn.Assistant, width-4, tuiPink, true),
		)
	}
	if m.thinking {
		liveTurn := cliui.BlackboardTurn{
			Index:     len(m.turns) + 1,
			User:      m.currentInput,
			StartedAt: time.Now(),
		}
		lines = append(lines,
			tuiMessageBlock("你", m.currentInput, width-4, tuiCyan, false),
			m.inlineThinkingBlock(liveTurn, true, width-4),
		)
	}
	if m.errText != "" {
		lines = append(lines, tuiErrorStyle().Render("错误: "+m.errText))
	}
	body := clampLines(strings.Join(lines, "\n"), height-2)
	return tuiPanel("实时对话", body, width, height, tuiViolet)
}

func (m agentTUIModel) contextPanel(width, height int) string {
	var lines []string
	lines = append(lines, tuiSectionLabel("运行状态", tuiCyan))
	lines = append(lines,
		tuiKV("当前任务", m.currentTask(), width-4),
		tuiKV("上下文锁", "已启动", width-4),
		tuiKV("思考块", "对话内摘要 /think 展开", width-4),
		tuiKV("黑板", "默认隐藏 /bb 打开详情", width-4),
		tuiKV("Markdown", "Glow 风格渲染", width-4),
	)
	lines = append(lines, "", tuiSectionLabel("三层记忆", tuiViolet))
	for _, layer := range m.memoryLayerStates() {
		lines = append(lines, tuiMemoryMeter(layer.label, layer.detail, layer.percent, layer.color, width-4))
	}
	lines = append(lines,
		tuiMuted().Render(shortLocal("含义: 百分比表示该记忆层当前接入状态，不是容量或 token 使用率。", width-4)),
	)
	lines = append(lines, "", tuiSectionLabel("最近事件", tuiAmber))
	for _, event := range m.events {
		lines = append(lines, tuiEventLine(event, width-4))
	}
	body := clampLines(strings.Join(lines, "\n"), height-2)
	title := "记忆 / 运行状态"
	return tuiPanel(title, body, width, height, tuiBorder)
}

type tuiMemoryLayerState struct {
	label   string
	detail  string
	percent int
	color   lipgloss.Color
}

func (m agentTUIModel) memoryLayerStates() []tuiMemoryLayerState {
	sqlitePercent := 0
	sqliteDetail := "会话库不可用"
	if defaultSessionStore(m.agentLoop) != nil {
		sqlitePercent = 100
		sqliteDetail = "会话/摘要/审计可用"
	}

	qdrantPercent := 0
	qdrantDetail := "未配置向量召回"
	if semanticMemoryConfigured() {
		qdrantPercent = 100
		qdrantDetail = "向量索引/相似召回已配置"
	}

	return []tuiMemoryLayerState{
		{label: "Markdown", detail: "身份/规则热加载", percent: 100, color: tuiCyan},
		{label: "SQLite", detail: sqliteDetail, percent: sqlitePercent, color: tuiAmber},
		{label: "Qdrant", detail: qdrantDetail, percent: qdrantPercent, color: tuiPink},
	}
}

func semanticMemoryConfigured() bool {
	if parseTUIBool(os.Getenv("FLYFLOR_SEMANTIC_MEMORY_ENABLED")) {
		return true
	}
	return strings.TrimSpace(os.Getenv("FLYFLOR_QDRANT_URL")) != "" ||
		strings.TrimSpace(os.Getenv("QDRANT_URL")) != ""
}

func parseTUIBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "t", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func (m agentTUIModel) blackboardFoldTree(width, height int) string {
	if m.blackboardTurnCount() == 0 {
		return tuiPanel("折叠黑板", tuiEmptyState("暂无黑板记录。发送一轮问题后，Flyflor 会按提问分组保存本轮黑板。", width-4), width, height, tuiPink)
	}
	rows := m.blackboardRenderableRows(width)
	if len(rows) == 0 {
		return tuiPanel("折叠黑板", tuiEmptyState("暂无可展示的黑板节点", width-4), width, height, tuiPink)
	}
	bodyLines := height - 3
	if bodyLines < 2 {
		bodyLines = 2
	}
	visible, indicator := visibleBlackboardLines(rows, m.blackboardScroll, bodyLines, width-4)
	if indicator != "" {
		visible = append(visible, indicator)
	}
	return tuiPanel("折叠黑板 / 思考过程", strings.Join(visible, "\n"), width, height, tuiPink)
}

func (m agentTUIModel) blackboardTurnSelector(width int) string {
	count := m.blackboardTurnCount()
	selected := m.normalizedBlackboardTurn()
	start := selected - 2
	if start < 0 {
		start = 0
	}
	end := start + 5
	if end > count {
		end = count
	}
	if end-start < 5 && start > 0 {
		start = maxLocal(end-5, 0)
	}
	var chips []string
	for i := start; i < end; i++ {
		label := fmt.Sprintf("第%d轮", i+1)
		if m.isLiveBlackboardTurn(i) {
			label += " · 进行中"
		}
		style := tuiMuted()
		if i == selected {
			style = tuiSelectedStyle()
		}
		chips = append(chips, style.Render(" "+label+" "))
	}
	line := strings.Join(chips, " ")
	if lipgloss.Width(line) > width {
		return shortLocal(tuiANSIRe.ReplaceAllString(line, ""), width)
	}
	return line
}

func (m agentTUIModel) blackboardNodes(width int) []blackboardFoldNode {
	selected := m.normalizedBlackboardTurn()
	turn, live := m.blackboardTurnAt(selected)
	if turn.Index == 0 {
		return nil
	}
	state := "已完成"
	if live {
		state = "进行中"
	}
	question := shortLocal(turn.User, maxLocal(width-16, 24))
	nodes := []blackboardFoldNode{
		{
			ID:         fmt.Sprintf("turn:%d:summary", turn.Index),
			Title:      fmt.Sprintf("第%d轮摘要", turn.Index),
			Summary:    fmt.Sprintf("%s · %s", state, question),
			Detail:     m.blackboardSummaryDetail(turn, live),
			Depth:      0,
			Expandable: true,
		},
		{
			ID:         fmt.Sprintf("turn:%d:thinking", turn.Index),
			Title:      "思考过程",
			Summary:    m.blackboardThinkingSummary(live),
			Detail:     m.blackboardThinkingDetail(turn, live),
			Depth:      1,
			Expandable: true,
		},
		{
			ID:         fmt.Sprintf("turn:%d:bridge", turn.Index),
			Title:      "Bridge 互检",
			Summary:    "Codex 拆解任务，Claude/OpenCode 复核风险",
			Detail:     "Codex Bridge: 明确目标、边界、执行路径和验证点。\nClaude/OpenCode Guard: 复核误解、遗漏、风险、可读性和回滚点。\nFlyflor: 只把对用户有帮助的过程压缩成可读黑板。",
			Depth:      1,
			Expandable: true,
		},
		{
			ID:         fmt.Sprintf("turn:%d:memory", turn.Index),
			Title:      "三层记忆",
			Summary:    "md 身份约束 · SQLite 时间线 · Qdrant 向量召回",
			Detail:     "### Markdown\n固定身份、语气、用户偏好和长期规则。\n\n### SQLite\n保存会话时间线、审计记录、工具检查点和可复盘事件。\n\n### Qdrant\n对稳定偏好、经验片段和相似任务摘要做向量索引，后续按语义召回。",
			Depth:      1,
			Expandable: true,
		},
	}
	resultSummary := "等待最终回答"
	resultDetail := "本轮仍在进行，最终回答完成后会写入这里。"
	if strings.TrimSpace(turn.Assistant) != "" {
		resultSummary = shortLocal(turn.Assistant, maxLocal(width-18, 28))
		resultDetail = strings.TrimSpace(turn.Assistant)
	}
	nodes = append(nodes, blackboardFoldNode{
		ID:         fmt.Sprintf("turn:%d:result", turn.Index),
		Title:      "结论 / 回答",
		Summary:    resultSummary,
		Detail:     resultDetail,
		Depth:      1,
		Expandable: true,
	})
	return nodes
}

func (m agentTUIModel) blackboardSummaryDetail(turn cliui.BlackboardTurn, live bool) string {
	lines := []string{
		"本轮问题: " + strings.TrimSpace(turn.User),
	}
	if !turn.StartedAt.IsZero() {
		lines = append(lines, "开始时间: "+turn.StartedAt.Format("15:04:05"))
	}
	if live {
		lines = append(lines, "当前状态: "+m.dynamicThinkingSummary())
	} else {
		lines = append(lines, "当前状态: 回答完成，黑板已归档。")
	}
	lines = append(lines, "阅读方式: 默认只看摘要；需要确认细节时展开思考、互检、记忆或结论节点。")
	return strings.Join(lines, "\n")
}

func (m agentTUIModel) blackboardThinkingSummary(live bool) string {
	if live {
		return m.dynamicThinkingSummary()
	}
	return "已压缩为最终回答和黑板结论"
}

func (m agentTUIModel) blackboardThinkingDetail(turn cliui.BlackboardTurn, live bool) string {
	if live {
		return strings.Join([]string{
			"1. **" + m.status + "**。",
			"2. 正在把本轮问题压缩成可执行上下文。",
			"3. Bridge 会优先保留用户可读的判断，而不是堆叠原始日志。",
			"4. 摘要会随状态刷新，展开细节只用于临时检查。",
		}, "\n")
	}
	return "本轮思考已经结束。\n保留摘要: " + shortLocal(turn.Assistant, 160)
}

func (m agentTUIModel) inlineThinkingBlock(turn cliui.BlackboardTurn, live bool, width int) string {
	if width < 28 {
		width = 28
	}
	index := turn.Index
	if index <= 0 {
		index = len(m.turns) + 1
	}
	expanded := m.expandedTurns != nil && m.expandedTurns[index]
	marker := "▸"
	if expanded {
		marker = "▾"
	}
	state := "思考摘要"
	color := tuiGreen
	summary := "已完成 · 黑板已压缩，Bridge 互检和记忆写入可展开阅读"
	if live {
		state = "思考中"
		color = tuiAmber
		summary = m.thinkingLine()
	}
	head := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().
			Foreground(tuiBase).
			Background(color).
			Bold(true).
			Padding(0, 1).
			Render(fmt.Sprintf("%s 第%d轮", state, index)),
		" ",
		lipgloss.NewStyle().Foreground(color).Bold(true).Render(marker),
		" ",
		tuiMuted().Render(shortLocal(summary, maxLocal(width-18, 24))),
	)
	hint := tuiMuted().Render(fmt.Sprintf("Ctrl+T 或 /think %d 展开/折叠 · /bb %d 查看完整黑板", index, index))
	lines := []string{head, hint}
	if expanded {
		lines = append(lines, m.inlineThinkingDetail(turn, live, width-6))
	}
	return lipgloss.NewStyle().
		Width(width-2).
		Foreground(tuiInk).
		Background(tuiSurface).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(color).
		Padding(0, 1).
		Render(strings.Join(lines, "\n"))
}

func (m agentTUIModel) inlineThinkingDetail(turn cliui.BlackboardTurn, live bool, width int) string {
	if width < 24 {
		width = 24
	}
	status := "已完成"
	assistant := strings.TrimSpace(turn.Assistant)
	if assistant == "" {
		assistant = "等待最终回答写入。"
	}
	if live {
		status = m.dynamicThinkingStatus()
		assistant = "正在生成最终回答，摘要会随状态动态刷新。"
	}
	detail := strings.Join([]string{
		"### 本轮黑板",
		"- 用户问题: " + strings.TrimSpace(turn.User),
		"- 当前状态: " + status,
		"- Flyflor 调度: 将问题分给 Bridge，保留对用户有帮助的过程，而不是只显示“达成共识”。",
		"- Codex Bridge: 拆解目标、边界、执行路径与验证点。",
		"- Claude/OpenCode Guard: 复核误解、遗漏、风险与可读性。",
		"- 三层记忆: Markdown 固定身份；SQLite 保存会话时间线；Qdrant 对可复用经验做向量索引与相似召回。",
		"",
		"### 当前结论",
		assistant,
	}, "\n")
	return lipgloss.NewStyle().
		Width(width).
		PaddingTop(1).
		Render(tuiGlowMarkdown(detail, width))
}

func (m agentTUIModel) renderBlackboardNode(node blackboardFoldNode, selected bool, width int) string {
	indent := strings.Repeat("  ", node.Depth)
	marker := "•"
	if node.Expandable {
		if m.expandedNodes != nil && m.expandedNodes[node.ID] {
			marker = "▾"
		} else {
			marker = "▸"
		}
	}
	title := tuiAccent().Render(node.Title)
	summaryWidth := maxLocal(width-lipgloss.Width(indent)-lipgloss.Width(node.Title)-8, 18)
	summary := tuiMuted().Render(shortLocal(node.Summary, summaryWidth))
	line := fmt.Sprintf("%s%s %s %s", indent, marker, title, summary)
	if selected {
		return tuiSelectedStyle().Width(width).Render(shortLocal(tuiANSIRe.ReplaceAllString(line, ""), width))
	}
	return lipgloss.NewStyle().Width(width).Render(line)
}

func (m agentTUIModel) renderBlackboardDetail(node blackboardFoldNode, width int) string {
	detailW := width - node.Depth*2 - 4
	if detailW < 24 {
		detailW = 24
	}
	body := tuiGlowMarkdown(node.Detail, detailW)
	return lipgloss.NewStyle().
		Foreground(tuiSubtle).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(tuiBorder).
		PaddingLeft(1).
		MarginLeft(node.Depth*2 + 2).
		Width(detailW).
		Render(body)
}

func exitConfirmDialog() *tuiDialog {
	return &tuiDialog{
		Kind:   "exit",
		Title:  "确认退出 Flyflor TUI",
		Body:   "Ctrl+C 不会直接结束会话。当前只是进入确认状态。\n\n按 Enter / y / 再次 Ctrl+C 退出；按 Esc / n / q 留在当前会话。",
		Footer: "你的 session 会保留，可用 flyflor sessions 查看，并用 flyflor agent -s <session> 继续。",
		Color:  tuiAmber,
	}
}

func (m agentTUIModel) helpDialog() *tuiDialog {
	var rows []string
	for _, cmd := range agentSlashCommands {
		name := cmd.Name
		if cmd.Args != "" {
			name += " " + cmd.Args
		}
		rows = append(rows, lipgloss.NewStyle().Foreground(tuiCyan).Bold(true).Width(24).Render(name)+cmd.Desc)
	}
	return &tuiDialog{
		Kind:   "help",
		Title:  "命令扩展菜单",
		Body:   strings.Join(rows, "\n"),
		Footer: "输入 / 可实时补全；↑↓ 选择，Tab 接受。按 Esc / q 关闭弹窗。",
		Color:  tuiPink,
	}
}

func (m agentTUIModel) statusDialog() *tuiDialog {
	body := strings.Join([]string{
		tuiKV("会话", m.session, 68),
		tuiKV("模型", m.model, 68),
		tuiKV("轮次", fmt.Sprintf("%d", len(m.turns)), 68),
		tuiKV("Bridge", "Codex 拆解 ↔ Claude/OpenCode 复核", 68),
		tuiKV("Markdown", "身份、语气、长期规则", 68),
		tuiKV("SQLite", "会话时间线、摘要、审计", 68),
		tuiKV("Qdrant", "语义向量索引、相似经验召回", 68),
	}, "\n")
	return &tuiDialog{
		Kind:   "status",
		Title:  "运行状态",
		Body:   body,
		Footer: "按 Esc / q / Enter 关闭。",
		Color:  tuiCyan,
	}
}

func (m agentTUIModel) dialogBox(width int) string {
	if m.dialog == nil {
		return ""
	}
	dialogW := minLocal(width-6, 78)
	if dialogW < 44 {
		dialogW = width - 2
	}
	if dialogW < 28 {
		dialogW = 28
	}
	bodyW := dialogW - 4
	body := clampLines(m.dialog.Body, 10)
	title := lipgloss.NewStyle().
		Foreground(tuiBase).
		Background(m.dialog.Color).
		Bold(true).
		Padding(0, 1).
		Render(" " + m.dialog.Title + " ")
	content := title + "\n\n" +
		lipgloss.NewStyle().Width(bodyW).Foreground(tuiInk).Render(body)
	if strings.TrimSpace(m.dialog.Footer) != "" {
		content += "\n\n" + lipgloss.NewStyle().Width(bodyW).Foreground(tuiSubtle).Render(m.dialog.Footer)
	}
	box := lipgloss.NewStyle().
		Width(dialogW).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.dialog.Color).
		Background(tuiSurfaceHi).
		Padding(0, 1).
		Render(content)
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, box)
}

func (m agentTUIModel) inputPanel(width int) string {
	if width < 12 {
		width = 12
	}
	rs := []rune(m.input)
	cursor := m.cursor
	if cursor < 0 || cursor > len(rs) {
		cursor = len(rs)
	}
	rendered := string(rs[:cursor]) + tuiCursorStyle().Render(" ") + string(rs[cursor:])
	if strings.TrimSpace(m.input) == "" {
		rendered = tuiMuted().Render("输入指令给智能体...  试试 /help 或 /bb")
	}
	return lipgloss.NewStyle().
		Width(width-2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(tuiViolet).
		Background(tuiSurface).
		Padding(0, 1).
		Render(tuiPromptStyle().Render("› ") + rendered)
}

func (m agentTUIModel) commandMenu(width int) string {
	filtered := m.filteredCommands()
	if len(filtered) == 0 {
		return tuiPanel("扩展菜单", tuiEmptyState("没有匹配的 / 命令", width-4), width, 5, tuiPink)
	}
	var rows []string
	for i, cmd := range filtered {
		prefix := "  "
		nameStyle := tuiCommandNameStyle()
		descStyle := tuiMuted()
		if i == m.menuIndex {
			prefix = "➜ "
			nameStyle = tuiSelectedStyle()
			descStyle = lipgloss.NewStyle().Foreground(tuiInk)
		}
		name := cmd.Name
		if cmd.Args != "" {
			name += " " + cmd.Args
		}
		rows = append(rows,
			nameStyle.Width(24).Render(prefix+name)+
				descStyle.Render("  "+cmd.Desc),
		)
	}
	return tuiPanel("命令扩展菜单", strings.Join(rows, "\n"), width, len(rows)+2, tuiPink)
}

func (m agentTUIModel) filteredCommands() []slashCommand {
	raw := strings.TrimLeft(m.input, " \t")
	if strings.HasPrefix(raw, "/bb ") || strings.HasPrefix(raw, "/blackboard ") || strings.HasPrefix(raw, "/think ") {
		fields := strings.Fields(raw)
		base := "/bb"
		if strings.HasPrefix(raw, "/blackboard ") {
			base = "/blackboard"
		} else if strings.HasPrefix(raw, "/think ") {
			base = "/think"
		}
		argPrefix := ""
		if len(fields) > 1 {
			argPrefix = fields[len(fields)-1]
		}
		return m.blackboardArgCommands(base, argPrefix)
	}
	prefix := strings.TrimSpace(raw)
	if idx := strings.Index(prefix, " "); idx >= 0 {
		prefix = prefix[:idx]
	}
	var out []slashCommand
	seen := map[string]bool{}
	for _, cmd := range agentSlashCommands {
		if prefix == "" || prefix == "/" || strings.HasPrefix(cmd.Name, prefix) {
			out = append(out, cmd)
			seen[cmd.Name] = true
		}
	}
	if prefix == "" || prefix == "/" {
		return out
	}
	for _, cmd := range agentSlashCommands {
		if seen[cmd.Name] {
			continue
		}
		if slashCommandMatches(cmd, prefix) {
			out = append(out, cmd)
		}
	}
	return out
}

func (m *agentTUIModel) applyThinkingCommand(args []string) {
	count := m.blackboardTurnCount()
	if len(args) > 0 && (args[0] == "hide" || args[0] == "off") {
		m.expandedTurns = map[int]bool{}
		m.status = "对话流思考过程已全部折叠"
		return
	}
	if count == 0 {
		m.status = "暂无可展开的思考过程"
		return
	}
	selected := count
	if len(args) > 0 && args[0] != "latest" {
		if n, err := strconv.Atoi(args[0]); err == nil {
			selected = n
		}
	}
	if selected < 1 {
		selected = 1
	}
	if selected > count {
		selected = count
	}
	if m.expandedTurns == nil {
		m.expandedTurns = map[int]bool{}
	}
	m.expandedTurns[selected] = !m.expandedTurns[selected]
	if m.expandedTurns[selected] {
		m.status = fmt.Sprintf("已展开第 %d 轮思考过程", selected)
	} else {
		m.status = fmt.Sprintf("已折叠第 %d 轮思考过程", selected)
	}
}

func (m *agentTUIModel) toggleLatestThinking() {
	m.applyThinkingCommand([]string{"latest"})
}

func slashCommandMatches(cmd slashCommand, query string) bool {
	query = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(query), "/"))
	if query == "" {
		return true
	}
	name := strings.TrimPrefix(strings.ToLower(cmd.Name), "/")
	if strings.Contains(name, query) {
		return true
	}
	candidate := strings.Join([]string{
		name,
		strings.ToLower(cmd.Args),
		strings.ToLower(cmd.Desc),
	}, " ")
	return fuzzyMatchLocal(candidate, query)
}

func fuzzyMatchLocal(candidate, query string) bool {
	candidate = strings.ToLower(candidate)
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	needle := []rune(query)
	index := 0
	for _, r := range candidate {
		if index < len(needle) && r == needle[index] {
			index++
		}
	}
	return index == len(needle)
}

func (m agentTUIModel) blackboardArgCommands(base, argPrefix string) []slashCommand {
	latestDesc := "打开最新一轮黑板"
	hideDesc := "隐藏折叠黑板"
	itemDesc := "打开第 %s 轮黑板"
	liveDesc := "打开第 %s 轮进行中黑板"
	if base == "/think" {
		latestDesc = "展开或折叠最新一轮思考过程"
		hideDesc = "折叠所有对话流思考过程"
		itemDesc = "展开或折叠第 %s 轮思考过程"
		liveDesc = "展开或折叠第 %s 轮进行中思考"
	}
	candidates := []slashCommand{
		{Name: base + " latest", Desc: latestDesc},
		{Name: base + " hide", Desc: hideDesc},
	}
	for _, turn := range m.turns {
		candidates = append(candidates, slashCommand{
			Name: base + " " + strconv.Itoa(turn.Index),
			Desc: fmt.Sprintf(itemDesc, strconv.Itoa(turn.Index)),
		})
	}
	if m.thinking && strings.TrimSpace(m.currentInput) != "" {
		index := len(m.turns) + 1
		candidates = append(candidates, slashCommand{
			Name: base + " " + strconv.Itoa(index),
			Desc: fmt.Sprintf(liveDesc, strconv.Itoa(index)),
		})
	}
	if argPrefix == "" {
		return candidates
	}
	var out []slashCommand
	for _, cmd := range candidates {
		fields := strings.Fields(cmd.Name)
		if len(fields) > 1 && strings.HasPrefix(fields[1], argPrefix) {
			out = append(out, cmd)
		}
	}
	return out
}

func (m *agentTUIModel) togglePanel() {
	m.blackboardVisible = !m.blackboardVisible
	if m.blackboardTurnCount() > 0 && m.selectedTurn < 0 {
		m.selectedTurn = m.blackboardTurnCount() - 1
	}
	if m.blackboardVisible {
		m.status = "折叠黑板已展开"
	} else {
		m.status = "黑板已隐藏"
	}
}

func (m agentTUIModel) canNavigateBlackboard() bool {
	return m.blackboardVisible && !m.menuVisible && strings.TrimSpace(m.input) == "" && m.blackboardTurnCount() > 0
}

func (m agentTUIModel) blackboardTurnCount() int {
	count := len(m.turns)
	if m.thinking && strings.TrimSpace(m.currentInput) != "" {
		count++
	}
	return count
}

func (m agentTUIModel) normalizedBlackboardTurn() int {
	count := m.blackboardTurnCount()
	if count <= 0 {
		return -1
	}
	selected := m.selectedTurn
	if selected < 0 || selected >= count {
		selected = count - 1
	}
	return selected
}

func (m agentTUIModel) isLiveBlackboardTurn(index int) bool {
	return m.thinking && strings.TrimSpace(m.currentInput) != "" && index == len(m.turns)
}

func (m agentTUIModel) blackboardTurnAt(index int) (cliui.BlackboardTurn, bool) {
	if m.isLiveBlackboardTurn(index) {
		return cliui.BlackboardTurn{
			Index:     len(m.turns) + 1,
			User:      m.currentInput,
			StartedAt: time.Now(),
		}, true
	}
	if index >= 0 && index < len(m.turns) {
		return m.turns[index], false
	}
	return cliui.BlackboardTurn{}, false
}

func (m agentTUIModel) normalizedBlackboardCursor(nodes []blackboardFoldNode) int {
	if len(nodes) == 0 {
		return 0
	}
	cursor := m.blackboardCursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(nodes) {
		cursor = len(nodes) - 1
	}
	return cursor
}

func (m *agentTUIModel) moveBlackboardCursor(delta int) {
	nodes := m.blackboardNodes(m.width)
	if len(nodes) == 0 {
		m.blackboardCursor = 0
		return
	}
	m.blackboardCursor += delta
	if m.blackboardCursor < 0 {
		m.blackboardCursor = len(nodes) - 1
	}
	if m.blackboardCursor >= len(nodes) {
		m.blackboardCursor = 0
	}
}

func (m agentTUIModel) blackboardPageSize() int {
	page := 6
	if m.height > 0 {
		page = m.height / 4
	}
	if page < 4 {
		return 4
	}
	return page
}

func (m *agentTUIModel) scrollBlackboard(delta int) {
	maxScroll := m.blackboardMaxScroll()
	m.blackboardScroll += delta
	if m.blackboardScroll < 0 {
		m.blackboardScroll = 0
	}
	if m.blackboardScroll > maxScroll {
		m.blackboardScroll = maxScroll
	}
	if maxScroll == 0 {
		m.status = "黑板内容已完整显示"
		return
	}
	m.status = fmt.Sprintf("黑板滚动 %d/%d", m.blackboardScroll, maxScroll)
}

func (m agentTUIModel) blackboardMaxScroll() int {
	width := m.width - 6
	if width < 32 {
		width = 32
	}
	height := 12
	if m.height >= 34 {
		height = 14
	}
	if m.height >= 40 {
		height = 16
	}
	bodyLines := height - 3
	if bodyLines < 2 {
		bodyLines = 2
	}
	lines := flattenBlackboardRows(m.blackboardRenderableRows(width))
	if len(lines) <= bodyLines {
		return 0
	}
	return len(lines) - bodyLines
}

func (m agentTUIModel) blackboardRenderableRows(width int) []string {
	nodes := m.blackboardNodes(width - 8)
	if len(nodes) == 0 {
		return nil
	}
	rows := []string{
		m.blackboardTurnSelector(width - 4),
		tuiMuted().Render("fx 模式: ↑↓/j/k 选择 · Enter/Space/→ 展开 · PgUp/PgDn/滚轮 滚动 · Esc/q 退出"),
	}
	for i, node := range nodes {
		selected := i == m.normalizedBlackboardCursor(nodes)
		rows = append(rows, m.renderBlackboardNode(node, selected, width-4))
		if node.Expandable && m.expandedNodes != nil && m.expandedNodes[node.ID] {
			rows = append(rows, m.renderBlackboardDetail(node, width-4))
		}
	}
	return rows
}

func visibleBlackboardLines(rows []string, scroll, height, width int) ([]string, string) {
	lines := flattenBlackboardRows(rows)
	if height < 1 {
		height = 1
	}
	if len(lines) <= height {
		return lines, ""
	}
	maxScroll := len(lines) - height
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	end := scroll + height
	if end > len(lines) {
		end = len(lines)
	}
	indicator := tuiMuted().Render(fmt.Sprintf("位置 %d-%d/%d · PgUp/PgDn 或鼠标滚轮继续阅读", scroll+1, end, len(lines)))
	if lipgloss.Width(indicator) > width {
		indicator = tuiMuted().Render(shortLocal(tuiANSIRe.ReplaceAllString(indicator, ""), width))
	}
	return lines[scroll:end], indicator
}

func flattenBlackboardRows(rows []string) []string {
	var lines []string
	for _, row := range rows {
		parts := strings.Split(row, "\n")
		lines = append(lines, parts...)
	}
	return lines
}

func (m *agentTUIModel) setSelectedBlackboardExpanded(expanded bool) {
	nodes := m.blackboardNodes(m.width)
	if len(nodes) == 0 {
		return
	}
	cursor := m.normalizedBlackboardCursor(nodes)
	node := nodes[cursor]
	if !node.Expandable {
		return
	}
	if m.expandedNodes == nil {
		m.expandedNodes = map[string]bool{}
	}
	m.expandedNodes[node.ID] = expanded
	if expanded {
		m.status = "已展开: " + node.Title
	} else {
		m.status = "已折叠: " + node.Title
	}
}

func (m *agentTUIModel) toggleSelectedBlackboardNode() {
	nodes := m.blackboardNodes(m.width)
	if len(nodes) == 0 {
		return
	}
	cursor := m.normalizedBlackboardCursor(nodes)
	node := nodes[cursor]
	if !node.Expandable {
		return
	}
	if m.expandedNodes == nil {
		m.expandedNodes = map[string]bool{}
	}
	m.expandedNodes[node.ID] = !m.expandedNodes[node.ID]
	if m.expandedNodes[node.ID] {
		m.status = "已展开: " + node.Title
	} else {
		m.status = "已折叠: " + node.Title
	}
}

func (m agentTUIModel) currentTask() string {
	if strings.TrimSpace(m.currentInput) != "" {
		return shortLocal(m.currentInput, 36)
	}
	if len(m.turns) == 0 {
		return "等待指令"
	}
	return shortLocal(m.turns[len(m.turns)-1].User, 36)
}

func (m agentTUIModel) dynamicThinkingStatus() string {
	stages := []string{"解析意图", "召回记忆", "组织上下文", "调用模型", "压缩黑板", "准备回答"}
	return stages[m.spinner%len(stages)]
}

func (m agentTUIModel) dynamicThinkingSummary() string {
	stage := m.dynamicThinkingStatus()
	task := shortLocal(m.currentInput, 34)
	if task == "" {
		task = m.currentTask()
	}
	return fmt.Sprintf("%s · %s", stage, task)
}

func (m agentTUIModel) thinkingLine() string {
	dots := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	return dots[m.spinner%len(dots)] + " " + m.dynamicThinkingSummary() + "，思考过程会在这里动态刷新。"
}

func tuiPanel(title, body string, width, height int, color lipgloss.Color) string {
	if width < 12 {
		width = 12
	}
	if height < 4 {
		height = 4
	}
	contentW := width - 2
	contentH := height - 2
	if contentW < 8 {
		contentW = 8
	}
	if contentH < 2 {
		contentH = 2
	}
	body = clampLines(body, contentH-1)
	titleBar := lipgloss.NewStyle().
		Foreground(tuiBase).
		Background(color).
		Bold(true).
		Padding(0, 1).
		Render(" " + title + " ")
	return lipgloss.NewStyle().
		Width(contentW).
		Height(contentH).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Background(tuiSurface).
		Padding(0, 1).
		Render(titleBar + "\n" + body)
}

func tuiAIStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(tuiPink).Bold(true)
}

func tuiYouStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(tuiCyan).Bold(true)
}

func tuiMuted() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(tuiSubtle)
}

func tuiAccent() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(tuiInk).Bold(true)
}

func tuiSelectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(tuiBase).Background(tuiCyan).Bold(true)
}

func tuiErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(tuiRed).Bold(true)
}

func tuiCursorStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(tuiBase).Background(tuiCyan)
}

func tuiPromptStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(tuiCyan).Bold(true)
}

func tuiCommandNameStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(tuiCyan).Bold(true)
}

func tuiPill(label, value string, color lipgloss.Color) string {
	if strings.TrimSpace(value) == "" {
		value = "未配置"
	}
	return lipgloss.NewStyle().
		Foreground(tuiBase).
		Background(color).
		Bold(true).
		Padding(0, 1).
		Render(label) +
		lipgloss.NewStyle().
			Foreground(tuiInk).
			Background(tuiSurface).
			Padding(0, 1).
			Render(value)
}

func tuiKeyHint(key, value string) string {
	return lipgloss.NewStyle().Foreground(tuiCyan).Bold(true).Render(key) +
		tuiMuted().Render(": ") +
		lipgloss.NewStyle().Foreground(tuiInk).Render(value)
}

func tuiWelcomeBlock(width int) string {
	if width < 20 {
		width = 20
	}
	lines := []string{
		lipgloss.NewStyle().Foreground(tuiCyan).Bold(true).Render("▌ Flyflor 已就绪"),
		lipgloss.NewStyle().Foreground(tuiInk).Render("直接输入问题开始对话，或输入 / 打开命令扩展菜单。"),
		tuiMuted().Render("黑板默认隐藏，可用 /bb 按每轮提问展开阅读。"),
	}
	return lipgloss.NewStyle().
		Width(width).
		Foreground(tuiInk).
		Background(tuiSurfaceHi).
		Padding(1, 2).
		Render(strings.Join(lines, "\n"))
}

func tuiMessageBlock(role, content string, width int, color lipgloss.Color, assistant bool) string {
	if width < 24 {
		width = 24
	}
	bubbleW := width - 2
	if bubbleW < 20 {
		bubbleW = 20
	}
	roleLabel := lipgloss.NewStyle().
		Foreground(tuiBase).
		Background(color).
		Bold(true).
		Padding(0, 1).
		Render(role)
	bodyColor := tuiInk
	if !assistant {
		bodyColor = lipgloss.Color("#DFFCF8")
	}
	body := lipgloss.NewStyle().
		Width(bubbleW-2).
		Foreground(bodyColor).
		Background(tuiSurfaceHi).
		Padding(0, 1).
		Render(tuiMaybeMarkdown(content, bubbleW-4, assistant))
	return roleLabel + "\n" + body
}

func tuiThinkingBlock(content string, width int) string {
	if width < 24 {
		width = 24
	}
	return lipgloss.NewStyle().
		Width(width-2).
		Foreground(tuiAmber).
		Background(tuiSurfaceHi).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(tuiAmber).
		Padding(0, 1).
		Render(content)
}

func tuiEmptyState(content string, width int) string {
	if width < 20 {
		width = 20
	}
	return lipgloss.NewStyle().
		Width(width).
		Foreground(tuiSubtle).
		Background(tuiSurfaceHi).
		Padding(1, 2).
		Render(content)
}

func tuiSectionLabel(label string, color lipgloss.Color) string {
	return lipgloss.NewStyle().Foreground(color).Bold(true).Render("▌ " + label)
}

func tuiKV(label, value string, width int) string {
	if width < 24 {
		width = 24
	}
	labelStyle := lipgloss.NewStyle().Foreground(tuiSubtle).Width(10)
	valueStyle := lipgloss.NewStyle().Foreground(tuiInk).Width(width - 12)
	return labelStyle.Render(label) + valueStyle.Render(shortLocal(value, width-12))
}

func tuiMemoryMeter(label, value string, percent int, color lipgloss.Color, width int) string {
	if width < 24 {
		width = 24
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	labelW := 10
	percentW := 5
	barW := width - labelW - percentW - 3
	if barW < 6 {
		barW = 6
	}
	if barW > 18 {
		barW = 18
	}
	filled := barW * percent / 100
	bar := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(tuiDim).Render(strings.Repeat("░", maxLocal(barW-filled, 0)))
	head := lipgloss.NewStyle().Foreground(tuiInk).Bold(true).Width(labelW).Render(label) +
		bar + " " +
		lipgloss.NewStyle().Foreground(color).Bold(true).Width(percentW).Render(fmt.Sprintf("%3d%%", percent))
	detail := lipgloss.NewStyle().
		Foreground(tuiSubtle).
		Width(width - 2).
		Render("  " + shortLocal(value, width-4))
	return head + "\n" + detail
}

func tuiEventLine(event string, width int) string {
	if width < 18 {
		width = 18
	}
	return lipgloss.NewStyle().Foreground(tuiDim).Render("• ") +
		lipgloss.NewStyle().Foreground(tuiSubtle).Render(shortLocal(event, width-2))
}

func appendRecent(items []string, value string, max int) []string {
	items = append(items, value)
	if len(items) > max {
		return items[len(items)-max:]
	}
	return items
}

func clampLines(s string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[len(lines)-maxLines:], "\n")
}

func wrapLocal(s string, width int) string {
	if width <= 8 {
		width = 8
	}
	return lipgloss.NewStyle().Width(width).Render(strings.TrimSpace(s))
}

func tuiMaybeMarkdown(content string, width int, enabled bool) string {
	if !enabled || !tuiGlowConfig.GlamourEnabled {
		return strings.TrimSpace(content)
	}
	if !tuiLooksLikeMarkdown(content) {
		return wrapLocal(content, width)
	}
	return tuiGlowMarkdown(content, width)
}

func tuiGlowMarkdown(content string, width int) string {
	if width < 20 {
		width = 20
	}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	renderer, err := tuiMarkdownRenderer(width)
	if err != nil {
		return wrapLocal(trimmed, width)
	}
	rendered, err := renderer.Render(trimmed)
	if err != nil {
		return wrapLocal(trimmed, width)
	}
	return strings.TrimSpace(rendered)
}

func tuiLooksLikeMarkdown(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "```") ||
		strings.Contains(trimmed, "`") ||
		strings.Contains(trimmed, "**") ||
		strings.Contains(trimmed, "__") ||
		strings.Contains(trimmed, "](") ||
		strings.Contains(trimmed, "\n|") {
		return true
	}
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") ||
			strings.HasPrefix(line, "## ") ||
			strings.HasPrefix(line, "### ") ||
			strings.HasPrefix(line, "- ") ||
			strings.HasPrefix(line, "* ") ||
			strings.HasPrefix(line, "> ") {
			return true
		}
		if len(line) >= 3 && line[0] >= '0' && line[0] <= '9' {
			if dot := strings.Index(line, ". "); dot > 0 && dot <= 2 {
				return true
			}
		}
	}
	return false
}

func tuiMarkdownRenderer(width int) (*glamour.TermRenderer, error) {
	if width < 20 {
		width = 20
	}
	if renderer := tuiMarkdownRenderers[width]; renderer != nil {
		return renderer, nil
	}
	style := tuiGlowConfig.GlamourStyle
	if strings.TrimSpace(style) == "" {
		style = "dark"
	}
	options := []glamour.TermRendererOption{
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(width),
	}
	if tuiGlowConfig.PreserveNewLines {
		options = append(options, glamour.WithPreservedNewLines())
	}
	renderer, err := glamour.NewTermRenderer(options...)
	if err != nil {
		return nil, err
	}
	tuiMarkdownRenderers[width] = renderer
	return renderer, nil
}

func shortLocal(s string, max int) string {
	s = tuiANSIRe.ReplaceAllString(s, "")
	rs := []rune(strings.TrimSpace(s))
	if max <= 0 || len(rs) <= max {
		return string(rs)
	}
	if max <= 1 {
		return string(rs[:max])
	}
	return string(rs[:max-1]) + "…"
}

func minLocal(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxLocal(a, b int) int {
	if a > b {
		return a
	}
	return b
}
