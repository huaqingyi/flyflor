package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	glowui "github.com/charmbracelet/glow/v2/ui"
	"github.com/charmbracelet/lipgloss"

	"github.com/sipeed/picoclaw/cmd/flyflor/internal/cliui"
	coreagent "github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/fileutil"
	"github.com/sipeed/picoclaw/pkg/providers"
)

var tuiANSIRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

var (
	tuiInk       = lipgloss.Color("#F8F2FF")
	tuiSubtle    = lipgloss.Color("#CEC2DA")
	tuiDim       = lipgloss.Color("#8A7A99")
	tuiBase      = lipgloss.Color("#140B22")
	tuiSurface   = lipgloss.Color("#1A1028")
	tuiSurfaceHi = lipgloss.Color("#2A173E")
	tuiBorder    = lipgloss.Color("#6E4A8A")
	tuiViolet    = lipgloss.Color("#A855F7")
	tuiCyan      = lipgloss.Color("#D946EF")
	tuiPink      = lipgloss.Color("#F472D0")
	tuiAmber     = lipgloss.Color("#F0ABFC")
	tuiGreen     = lipgloss.Color("#C084FC")
	tuiRed       = lipgloss.Color("#FB7185")
)

var (
	tuiMarkdownRenderers = map[int]*glamour.TermRenderer{}
	tuiMarkdownCache     = map[string]string{}
)

const tuiScrollBottom = 1 << 30

var tuiGlowConfig = glowui.Config{
	GlamourStyle:     "dark",
	GlamourEnabled:   true,
	PreserveNewLines: true,
}

type agentTUITab int

const (
	agentTabChat agentTUITab = iota
	agentTabSessions
	agentTabBlackboard
	agentTabMemory
	agentTabSettings
)

type agentTUITabSpec struct {
	ID    agentTUITab
	Label string
	Desc  string
	Color lipgloss.Color
}

var agentTUITabs = []agentTUITabSpec{
	{ID: agentTabChat, Label: "对话", Desc: "实时问答与长文本输入", Color: tuiViolet},
	{ID: agentTabSessions, Label: "历史 Session", Desc: "查看并切换可继续会话", Color: tuiViolet},
	{ID: agentTabBlackboard, Label: "黑板", Desc: "按轮次展开思考与互检", Color: tuiPink},
	{ID: agentTabMemory, Label: "记忆", Desc: "核心 Markdown 记忆预览与编辑", Color: tuiViolet},
	{ID: agentTabSettings, Label: "设置", Desc: "模型、setup 与工具状态", Color: tuiPink},
}

type slashCommand struct {
	Name string
	Args string
	Desc string
}

var agentSlashCommands = []slashCommand{
	{Name: "/help", Desc: "弹窗显示 / 命令扩展菜单"},
	{Name: "/edit", Desc: "展开长文本编辑区，Enter 换行，Ctrl+D 发送"},
	{Name: "/chat", Desc: "切换到对话 tab"},
	{Name: "/sessions", Desc: "切换到历史 Session tab"},
	{Name: "/bb", Args: "[latest|编号|hide]", Desc: "打开或切换本轮黑板"},
	{Name: "/blackboard", Args: "[latest|编号|hide]", Desc: "同 /bb，显示黑板详情"},
	{Name: "/think", Args: "[latest|编号|hide]", Desc: "展开或折叠对话流里的思考过程"},
	{Name: "/memory", Desc: "切换到记忆 tab"},
	{Name: "/settings", Desc: "切换到设置 tab"},
	{Name: "/yolo", Args: "[on|off]", Desc: "切换自主执行模式，减少低风险确认"},
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
	workspace string

	width  int
	height int

	input         string
	cursor        int
	inputExpanded bool
	yoloMode      bool
	chatScroll    int
	turns         []cliui.BlackboardTurn
	events        []string
	status        string
	errText       string
	thinking      bool
	spinner       int
	currentInput  string
	thinkingStart time.Time
	activePanel   string
	activeTab     agentTUITab

	menuVisible bool
	menuIndex   int
	exitArmed   bool
	dialog      *tuiDialog

	sessionRows    []cliui.SessionRow
	sessionCursor  int
	sessionLoadErr string

	blackboardVisible bool
	selectedTurn      int
	blackboardCursor  int
	blackboardScroll  int
	expandedNodes     map[string]bool
	expandedTurns     map[int]bool

	memoryCursor     int
	memoryScroll     int
	memoryEditing    bool
	memoryEditBuffer string
	memoryEditCursor int
	memoryMessage    string
}

type tuiMemoryFile struct {
	Label       string
	Kind        string
	RelPath     string
	Description string
	Placeholder string
	Color       lipgloss.Color
}

var tuiMemoryFiles = []tuiMemoryFile{
	{
		Label:       "人格记忆",
		Kind:        "SOUL",
		RelPath:     "SOUL.md",
		Description: "主要人格、语气、行为边界",
		Placeholder: "# SOUL\n\n- 记录 Flyflor 的核心人格、表达方式和长期行为准则。\n",
		Color:       tuiPink,
	},
	{
		Label:       "特征记忆",
		Kind:        "USER",
		RelPath:     "USER.md",
		Description: "用户偏好、习惯、关键特征",
		Placeholder: "# USER\n\n- 记录用户偏好、沟通习惯、工作方式和需要长期记住的特征。\n",
		Color:       tuiViolet,
	},
	{
		Label:       "长期记忆",
		Kind:        "MEMORY",
		RelPath:     "memory/MEMORY.md",
		Description: "持续积累的事实与项目记忆",
		Placeholder: "# MEMORY\n\n- 记录对后续任务有价值的长期事实、约定和项目上下文。\n",
		Color:       tuiCyan,
	},
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
	rows, rowsErr := LoadSessionRowsForMenu()
	m := agentTUIModel{
		agentLoop:         agentLoop,
		session:           sessionKey,
		model:             modelLabel,
		workspace:         defaultAgentWorkspace(agentLoop),
		width:             110,
		height:            32,
		turns:             loadSessionTurns(agentLoop, sessionKey),
		status:            "就绪",
		activePanel:       "chat",
		activeTab:         agentTabChat,
		sessionRows:       rows,
		selectedTurn:      -1,
		expandedNodes:     map[string]bool{},
		expandedTurns:     map[int]bool{},
		blackboardVisible: false,
		events: []string{
			"三层记忆: md + sqlite + qdrant",
			"Blackboard Scheduler: Flyflor Planner + Flyflor Reviewer",
			"Markdown: Glow 风格渲染已启用",
			"每轮提问会在对话流内显示可折叠思考摘要",
		},
	}
	if rowsErr != nil {
		m.sessionLoadErr = rowsErr.Error()
		m.events = appendRecent(m.events, "历史 Session 读取失败: "+rowsErr.Error(), 7)
	}
	for i, row := range rows {
		if row.Key == sessionKey {
			m.sessionCursor = i
			break
		}
	}
	if len(m.turns) > 0 {
		m.selectedTurn = len(m.turns) - 1
		m.chatScroll = tuiScrollBottom
		m.events = appendRecent(m.events, fmt.Sprintf("已载入历史对话: %d 轮", len(m.turns)), 7)
	}
	_, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}

func (m agentTUIModel) Init() tea.Cmd {
	return nil
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
			return m, tickAgentTUI()
		}
		return m, nil
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
			StartedAt: m.thinkingStart,
		})
		m.selectedTurn = len(m.turns) - 1
		m.blackboardCursor = 0
		m.events = appendRecent(m.events, "完成: "+shortLocal(msg.input, 42), 7)
		m.chatScroll = tuiScrollBottom
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
	if m.memoryEditing {
		return m.handleMemoryEditorKey(msg)
	}
	if msg.String() == "ctrl+e" {
		m.inputExpanded = !m.inputExpanded
		if m.inputExpanded {
			m.status = "已展开长文本编辑区；Enter 换行，Ctrl+D 发送"
		} else {
			m.status = "已收起长文本编辑区"
		}
		return m, nil
	}
	if m.inputExpanded {
		switch msg.Type {
		case tea.KeyCtrlS:
			return m.submitOrSteer()
		case tea.KeyEnter:
			m.insertRune('\n')
			m.syncMenu()
			return m, nil
		case tea.KeyCtrlD:
			return m.submit()
		case tea.KeyEsc:
			m.inputExpanded = false
			m.status = "已收起长文本编辑区"
			return m, nil
		}
	}
	switch msg.Type {
	case tea.KeyCtrlC:
		if m.menuVisible {
			m.menuVisible = false
			m.status = "已关闭命令菜单"
			return m, nil
		}
		if m.currentTab() == agentTabBlackboard {
			m.switchTab(agentTabChat)
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
	case tea.KeyCtrlS:
		return m.submitOrSteer()
	case tea.KeyEsc:
		if m.menuVisible {
			m.menuVisible = false
			return m, nil
		}
		if m.currentTab() == agentTabBlackboard {
			m.switchTab(agentTabChat)
			m.status = "已回到对话 tab"
			return m, nil
		}
		return m, nil
	case tea.KeyPgUp, tea.KeyCtrlU:
		if m.canNavigateBlackboard() {
			m.scrollBlackboard(-m.blackboardPageSize())
			return m, nil
		}
		if m.canNavigateMemory() {
			m.scrollMemory(-m.memoryPageSize())
			return m, nil
		}
		return m, nil
	case tea.KeyPgDown, tea.KeyCtrlD:
		if m.canNavigateBlackboard() {
			m.scrollBlackboard(m.blackboardPageSize())
			return m, nil
		}
		if m.canNavigateMemory() {
			m.scrollMemory(m.memoryPageSize())
			return m, nil
		}
		return m, nil
	case tea.KeyTab:
		if m.menuVisible {
			m.acceptMenuSelection()
			return m, nil
		}
		m.nextTab()
		return m, nil
	case tea.KeyShiftTab:
		m.prevTab()
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
		} else if m.canNavigateSessions() {
			m.moveSessionCursor(-1)
		} else if m.canNavigateMemory() {
			m.moveMemoryCursor(-1)
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
		} else if m.canNavigateSessions() {
			m.moveSessionCursor(1)
		} else if m.canNavigateMemory() {
			m.moveMemoryCursor(1)
		}
		return m, nil
	case tea.KeyLeft:
		if m.canNavigateBlackboard() {
			m.setSelectedBlackboardExpanded(false)
			return m, nil
		}
		if strings.TrimSpace(m.input) == "" {
			m.prevTab()
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
		if strings.TrimSpace(m.input) == "" {
			m.nextTab()
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
		if m.canNavigateMemory() {
			return m.beginMemoryEdit()
		}
		if m.canNavigateSessions() {
			cmd := m.selectSession()
			return m, cmd
		}
		return m.submit()
	case tea.KeyRunes:
		if len(msg.Runes) == 1 && msg.Runes[0] == '?' && strings.TrimSpace(m.input) == "" {
			m.dialog = m.helpDialog()
			m.status = "已打开帮助弹窗"
			return m, nil
		}
		if strings.TrimSpace(m.input) == "" && len(msg.Runes) == 1 && msg.Runes[0] >= '1' && msg.Runes[0] <= '5' {
			m.switchTab(agentTUITab(msg.Runes[0] - '1'))
			return m, nil
		}
		if m.canNavigateBlackboard() && strings.TrimSpace(m.input) == "" && len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'q', 'Q':
				m.switchTab(agentTabChat)
				m.status = "已回到对话 tab"
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
		if m.canNavigateMemory() && strings.TrimSpace(m.input) == "" && len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'j', 'J':
				m.moveMemoryCursor(1)
				return m, nil
			case 'k', 'K':
				m.moveMemoryCursor(-1)
				return m, nil
			case 'd', 'D':
				m.scrollMemory(m.memoryPageSize())
				return m, nil
			case 'u', 'U':
				m.scrollMemory(-m.memoryPageSize())
				return m, nil
			case 'e', 'E':
				return m.beginMemoryEdit()
			}
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
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if m.canNavigateBlackboard() {
			m.scrollBlackboard(-3)
		} else if m.canNavigateMemory() {
			m.scrollMemory(-3)
		}
	case tea.MouseButtonWheelDown:
		if m.canNavigateBlackboard() {
			m.scrollBlackboard(3)
		} else if m.canNavigateMemory() {
			m.scrollMemory(3)
		}
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

func (m agentTUIModel) handleMemoryEditorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		m.memoryEditing = false
		m.memoryEditBuffer = ""
		m.memoryEditCursor = 0
		m.memoryMessage = "已取消编辑"
		m.status = "记忆编辑已取消"
		return m, nil
	case tea.KeyCtrlD:
		return m.saveMemoryEdit()
	case tea.KeyEnter:
		m.insertMemoryRune('\n')
		return m, nil
	case tea.KeyBackspace, tea.KeyCtrlH:
		m.backspaceMemoryEdit()
		return m, nil
	case tea.KeyLeft:
		if m.memoryEditCursor > 0 {
			m.memoryEditCursor--
		}
		return m, nil
	case tea.KeyRight:
		if m.memoryEditCursor < len([]rune(m.memoryEditBuffer)) {
			m.memoryEditCursor++
		}
		return m, nil
	case tea.KeyHome:
		m.memoryEditCursor = 0
		return m, nil
	case tea.KeyEnd:
		m.memoryEditCursor = len([]rune(m.memoryEditBuffer))
		return m, nil
	case tea.KeyPgUp, tea.KeyCtrlU:
		m.scrollMemory(-m.memoryPageSize())
		return m, nil
	case tea.KeyPgDown:
		m.scrollMemory(m.memoryPageSize())
		return m, nil
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			m.insertMemoryRune(r)
		}
		return m, nil
	default:
		return m, nil
	}
}

func (m *agentTUIModel) insertMemoryRune(r rune) {
	rs := []rune(m.memoryEditBuffer)
	if m.memoryEditCursor < 0 || m.memoryEditCursor > len(rs) {
		m.memoryEditCursor = len(rs)
	}
	rs = append(rs[:m.memoryEditCursor], append([]rune{r}, rs[m.memoryEditCursor:]...)...)
	m.memoryEditBuffer = string(rs)
	m.memoryEditCursor++
}

func (m *agentTUIModel) backspaceMemoryEdit() {
	rs := []rune(m.memoryEditBuffer)
	if m.memoryEditCursor <= 0 || len(rs) == 0 {
		return
	}
	rs = append(rs[:m.memoryEditCursor-1], rs[m.memoryEditCursor:]...)
	m.memoryEditBuffer = string(rs)
	m.memoryEditCursor--
}

func (m agentTUIModel) beginMemoryEdit() (tea.Model, tea.Cmd) {
	spec := m.selectedMemoryFile()
	content, exists, err := m.readMemoryFile(spec)
	if err != nil {
		m.memoryMessage = err.Error()
		m.errText = err.Error()
		m.status = "记忆读取失败"
		return m, nil
	}
	if !exists || strings.TrimSpace(content) == "" {
		content = spec.Placeholder
	}
	m.memoryEditing = true
	m.memoryEditBuffer = content
	m.memoryEditCursor = len([]rune(content))
	m.memoryScroll = 0
	m.memoryMessage = "编辑 " + spec.Label + " · Ctrl+D 保存 · Esc 取消"
	m.status = "正在编辑 " + spec.Label
	return m, nil
}

func (m agentTUIModel) saveMemoryEdit() (tea.Model, tea.Cmd) {
	spec := m.selectedMemoryFile()
	path := m.memoryFilePath(spec)
	perm := os.FileMode(0o644)
	if strings.Contains(filepath.ToSlash(spec.RelPath), "/") {
		perm = 0o600
	}
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}
	content := strings.TrimRight(m.memoryEditBuffer, " \t\r\n") + "\n"
	if err := fileutil.WriteFileAtomic(path, []byte(content), perm); err != nil {
		m.memoryMessage = err.Error()
		m.errText = err.Error()
		m.status = "记忆保存失败"
		return m, nil
	}
	m.memoryEditing = false
	m.memoryEditBuffer = ""
	m.memoryEditCursor = 0
	m.memoryScroll = 0
	m.errText = ""
	m.memoryMessage = "已保存 " + spec.Label + " -> " + spec.RelPath
	m.status = "记忆已保存: " + spec.Label
	m.events = appendRecent(m.events, "记忆已保存: "+spec.RelPath, 7)
	return m, nil
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
	if input == "" {
		return m, nil
	}
	if m.thinking {
		m.status = "回答进行中；Ctrl+S 可即时发送引导"
		return m, nil
	}
	if m.menuVisible && shouldAcceptMenuBeforeSubmit(input, m.filteredCommands()) {
		m.acceptMenuSelection()
		return m, nil
	}
	m.input = ""
	m.cursor = 0
	m.inputExpanded = false
	m.menuVisible = false

	if strings.HasPrefix(input, "/") {
		return m.handleSlashCommand(input)
	}

	m.errText = ""
	m.thinking = true
	m.currentInput = input
	m.thinkingStart = time.Now()
	m.chatScroll = tuiScrollBottom
	m.status = "正在理解问题"
	if m.blackboardVisible {
		m.selectedTurn = m.blackboardTurnCount() - 1
		m.blackboardCursor = 0
		m.blackboardScroll = 0
	}
	m.events = appendRecent(m.events, "收到问题: "+shortLocal(input, 42), 7)
	return m, tea.Batch(m.sendToAgent(input), tickAgentTUI())
}

func (m agentTUIModel) submitOrSteer() (tea.Model, tea.Cmd) {
	if !m.thinking {
		return m.submit()
	}
	input := strings.TrimSpace(m.input)
	if input == "" {
		m.status = "回答进行中；输入引导后按 Ctrl+S 立即注入"
		return m, nil
	}
	if strings.HasPrefix(input, "/") {
		m.status = "回答进行中；/ 命令请等本轮完成后执行，普通文本可 Ctrl+S 注入"
		return m, nil
	}
	m.input = ""
	m.cursor = 0
	m.menuVisible = false
	if m.agentLoop == nil {
		m.errText = "AgentLoop 未初始化，无法注入引导"
		m.status = "即时引导失败"
		return m, nil
	}
	steering := "[Flyflor TUI Ctrl+S 即时引导]\n" + m.promptForAgent(input)
	if err := m.agentLoop.Steer(providers.Message{Role: "user", Content: steering}); err != nil {
		m.errText = err.Error()
		m.status = "即时引导失败"
		m.events = appendRecent(m.events, "即时引导失败: "+err.Error(), 7)
		return m, nil
	}
	m.errText = ""
	m.status = "已即时注入引导: " + shortLocal(input, 30)
	m.events = appendRecent(m.events, "Ctrl+S 引导: "+shortLocal(input, 42), 7)
	return m, nil
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
	case "/edit":
		m.inputExpanded = true
		m.menuVisible = false
		m.status = "已展开长文本编辑区；Enter 换行，Ctrl+D 发送"
	case "/chat":
		m.switchTab(agentTabChat)
	case "/sessions":
		m.switchTab(agentTabSessions)
	case "/memory":
		m.switchTab(agentTabMemory)
		m.status = "已切换到记忆 tab"
	case "/settings":
		m.switchTab(agentTabSettings)
	case "/yolo":
		m.applyYoloCommand(fields[1:])
	case "/clear":
		m.turns = nil
		m.selectedTurn = -1
		m.switchTab(agentTabChat)
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
		m.switchTab(agentTabChat)
		m.status = "黑板已隐藏"
		return
	}
	count := m.blackboardTurnCount()
	if count == 0 {
		m.switchTab(agentTabBlackboard)
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
	m.switchTab(agentTabBlackboard)
	m.status = fmt.Sprintf("已打开第 %d 轮折叠黑板", selected+1)
}

func (m *agentTUIModel) shiftBlackboard(delta int) {
	count := m.blackboardTurnCount()
	if count == 0 {
		m.switchTab(agentTabBlackboard)
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
	m.switchTab(agentTabBlackboard)
	m.status = fmt.Sprintf("已切换到第 %d 轮黑板分组", m.selectedTurn+1)
}

func (m *agentTUIModel) applyYoloCommand(args []string) {
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "on", "true", "1", "yes":
			m.yoloMode = true
		case "off", "false", "0", "no":
			m.yoloMode = false
		default:
			m.status = "用法: /yolo [on|off]"
			return
		}
	} else {
		m.yoloMode = !m.yoloMode
	}
	if m.yoloMode {
		m.status = "YOLO 模式已开启：低风险任务会更主动执行"
		m.events = appendRecent(m.events, "模式切换: YOLO", 7)
		return
	}
	m.status = "YOLO 模式已关闭：恢复保守确认"
	m.events = appendRecent(m.events, "模式切换: 标准", 7)
}

func (m agentTUIModel) sendToAgent(input string) tea.Cmd {
	return func() tea.Msg {
		response, err := m.agentLoop.ProcessDirect(context.Background(), m.promptForAgent(input), m.session)
		return agentResponseMsg{input: input, response: response, err: err}
	}
}

func (m agentTUIModel) promptForAgent(input string) string {
	input = strings.TrimSpace(input)
	if !m.yoloMode {
		return input
	}
	return input + "\n\n" + strings.Join([]string{
		"[Flyflor TUI mode: /yolo is ON]",
		"Act autonomously for low-risk work: make reasonable assumptions, inspect files, and run focused read/test/build commands when useful.",
		"Do not ask for confirmation unless an action is destructive, irreversible, credential-related, outside the user's requested scope, or blocked by policy/configuration.",
	}, "\n")
}

func (m agentTUIModel) View() string {
	width := m.width
	if width <= 0 {
		width = 110
	}
	if width < 48 {
		width = 48
	}
	height := m.height
	if height <= 0 {
		height = 32
	}
	if height < 20 {
		height = 20
	}
	frameW := width - 2
	contentW := frameW - 2

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
	inputH := 3
	if m.inputExpanded {
		inputH = 8
	}

	header := m.header(contentW)
	tabs := m.tabBar(contentW)
	tabH := strings.Count(tabs, "\n") + 1
	bodyH := height - 9 - (tabH - 1) - menuH - dialogH - (inputH - 3)
	if bodyH < 9 {
		bodyH = 9
	}

	body := m.activeTabPanel(contentW, bodyH)
	input := m.inputPanel(contentW, inputH)
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
		Padding(0, 1).
		Render(
			header + "\n" +
				tabs + "\n" +
				body + "\n" +
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
		Background(tuiPink).
		Bold(true).
		Padding(0, 2).
		Render(" FLYFLOR ")
	title := lipgloss.NewStyle().
		Foreground(tuiInk).
		Bold(true).
		Render("智能体运行台")
	status := tuiPill("状态", m.status, statusColor)
	mode := tuiPill("模式", m.modeLabel(), m.modeColor())
	bridge := tuiPill("调度", "Blackboard + 2 workers", tuiViolet)
	model := tuiPill("模型", shortLocal(m.model, 24), tuiCyan)
	line := lipgloss.JoinHorizontal(lipgloss.Center, left, " ", title, " ", status, " ", mode, " ", bridge, " ", model)
	if lipgloss.Width(line) > width {
		line = lipgloss.JoinHorizontal(lipgloss.Center, left, " ", title, " ", status, " ", mode)
	}
	return lipgloss.NewStyle().
		Width(width).
		Foreground(tuiInk).
		Render(line)
}

func (m agentTUIModel) modeLabel() string {
	if m.yoloMode {
		return "YOLO"
	}
	return "标准"
}

func (m agentTUIModel) modeColor() lipgloss.Color {
	if m.yoloMode {
		return tuiAmber
	}
	return tuiCyan
}

func (m agentTUIModel) tabBar(width int) string {
	active := m.currentTab()
	if width < 56 {
		return m.compactTabBar(width)
	}

	inactiveBorder := lipgloss.Border{
		Top: "─", Bottom: "─", Left: "│", Right: "│",
		TopLeft: "╭", TopRight: "╮", BottomLeft: "┴", BottomRight: "┴",
	}
	activeBorder := lipgloss.Border{
		Top: "─", Bottom: " ", Left: "│", Right: "│",
		TopLeft: "╭", TopRight: "╮", BottomLeft: "┘", BottomRight: "└",
	}
	var parts []string
	for i, tab := range agentTUITabs {
		label := fmt.Sprintf("%d %s", i+1, tab.Label)
		style := lipgloss.NewStyle().
			Border(inactiveBorder, true).
			BorderForeground(tuiBorder).
			Foreground(tuiSubtle).
			Padding(0, 2)
		if tab.ID == active {
			style = style.
				Border(activeBorder, true).
				BorderForeground(tab.Color).
				Foreground(tuiInk).
				Bold(true)
		}
		parts = append(parts, style.Render(label))
	}
	line := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	if lipgloss.Width(line) > width {
		return m.compactTabBar(width)
	}
	ruleW := width - lipgloss.Width(line)
	if ruleW < 0 {
		ruleW = 0
	}
	rule := lipgloss.NewStyle().Foreground(agentTUITabs[int(active)].Color).Render(strings.Repeat("─", ruleW))
	return lipgloss.NewStyle().
		Width(width).
		Foreground(tuiInk).
		Render(lipgloss.JoinHorizontal(lipgloss.Bottom, line, rule))
}

func (m agentTUIModel) compactTabBar(width int) string {
	active := m.currentTab()
	compact := []string{"1对话", "2历史", "3黑板", "4记忆", "5设置"}
	for i, tab := range agentTUITabs {
		if tab.ID == active {
			compact[i] = "[" + compact[i] + "]"
		}
	}
	line := strings.Join(compact, " ")
	if lipgloss.Width(line) > width {
		line = shortLocal(line, width)
	}
	return lipgloss.NewStyle().
		Width(width).
		Foreground(tuiInk).
		Render(line)
}

func (m agentTUIModel) activeTabPanel(width, height int) string {
	switch m.currentTab() {
	case agentTabSessions:
		return m.sessionsPanel(width, height)
	case agentTabBlackboard:
		return m.blackboardFoldTree(width, height)
	case agentTabMemory:
		return m.memoryPanel(width, height)
	case agentTabSettings:
		return m.settingsPanel(width, height)
	default:
		return m.chatPanel(width, height)
	}
}

func (m agentTUIModel) footer(width int) string {
	items := []string{
		tuiKeyHint("ctrl+c", "取消/二次退出"),
		tuiKeyHint("ctrl+s", "即时引导"),
		tuiKeyHint("ctrl+e", "长文本"),
		tuiKeyHint("ctrl+t", "思考展开"),
		tuiKeyHint("enter", "发送/换行"),
		tuiKeyHint("tab", "切换tab/补全"),
		tuiKeyHint("1-5", "直达tab"),
		tuiKeyHint("↑↓", "选择"),
		tuiKeyHint("pgup/pgdn", "滚动"),
		tuiKeyHint("esc/q", "退出黑板"),
		tuiKeyHint("?/help", "弹窗"),
	}
	line := strings.Join(items, tuiMuted().Render("  │  "))
	if lipgloss.Width(line) > width {
		line = "Tab 切换  1-5 直达  Ctrl+S 即时引导  Ctrl+E 长文本  Enter 发送  ? 帮助"
		if lipgloss.Width(line) > width {
			line = shortLocal(line, width)
		}
	}
	return lipgloss.NewStyle().
		Width(width).
		Foreground(tuiSubtle).
		Render(line)
}

func (m agentTUIModel) chatPanel(width, height int) string {
	contentW := maxLocal(width-4, 20)
	allRows := m.chatRenderableRows(contentW)
	visible, indicator := visibleChatRows(allRows, m.chatScroll, maxLocal(height-5, 3), contentW)
	lines := []string{
		tuiMuted().Render("PgUp/PgDn 或鼠标滚轮浏览历史；Header/Tabs/Input 固定在当前 TUI。"),
	}
	if indicator != "" {
		lines = append(lines, indicator)
	}
	lines = append(lines, visible...)
	body := strings.Join(lines, "\n")
	return tuiPanel("实时对话 · 固定布局", body, width, height, tuiViolet)
}

func (m agentTUIModel) chatRenderableRows(width int) []string {
	var lines []string
	if len(m.turns) == 0 && !m.thinking {
		lines = append(lines,
			"Flyflor 已就绪",
			"直接输入问题开始对话。Ctrl+E 编写长文本；Tab/1-5 切换 tab。",
			"对话页在独立 TUI 内显示完整历史；PgUp/PgDn 或鼠标滚轮浏览。",
		)
	}
	for _, turn := range m.turns {
		lines = appendChatUserBlock(lines, turn.User, width)
		lines = m.appendThinkingBlock(lines, turn, false, width)
		lines = appendChatAnswerBlock(lines, turn.Assistant, width)
		lines = append(lines, "")
	}
	if m.thinking {
		liveTurn := cliui.BlackboardTurn{
			Index:     len(m.turns) + 1,
			User:      m.currentInput,
			StartedAt: m.thinkingStart,
		}
		lines = appendChatUserBlock(lines, m.currentInput, width)
		lines = m.appendThinkingBlock(lines, liveTurn, true, width)
	}
	if m.errText != "" {
		lines = append(lines, tuiErrorStyle().Render("错误: "+m.errText))
	}
	return lines
}

func appendChatUserBlock(lines []string, text string, width int) []string {
	return appendChatBlock(lines, "", text, width, false, tuiViolet, tuiSurfaceHi)
}

func appendChatAnswerBlock(lines []string, text string, width int) []string {
	return appendChatBlock(lines, "", text, width, true, tuiPink, tuiSurface)
}

func visibleChatRows(rows []string, scroll, height, width int) ([]string, string) {
	if height < 1 {
		height = 1
	}
	if len(rows) <= height {
		return rows, ""
	}
	maxScroll := len(rows) - height
	if scroll == tuiScrollBottom {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	end := scroll + height
	if end > len(rows) {
		end = len(rows)
	}
	indicator := tuiMuted().Render(fmt.Sprintf("历史 %d-%d/%d", scroll+1, end, len(rows)))
	if lipgloss.Width(indicator) > width {
		indicator = tuiMuted().Render(shortLocal(tuiANSIRe.ReplaceAllString(indicator, ""), width))
	}
	return rows[scroll:end], indicator
}

func (m *agentTUIModel) scrollChat(delta int) {
	width := m.width
	if width <= 0 {
		width = 110
	}
	height := m.height
	if height <= 0 {
		height = 32
	}
	frameW := width - 2
	contentW := maxLocal(frameW-6, 20)
	pageH := m.chatPageSize()
	rows := m.chatRenderableRows(contentW)
	maxScroll := len(rows) - pageH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.chatScroll == tuiScrollBottom {
		m.chatScroll = maxScroll
	}
	m.chatScroll += delta
	if m.chatScroll < 0 {
		m.chatScroll = 0
	}
	if m.chatScroll > maxScroll {
		m.chatScroll = maxScroll
	}
	m.status = fmt.Sprintf("对话历史 %d/%d", minLocal(m.chatScroll+pageH, len(rows)), len(rows))
}

func (m agentTUIModel) chatPageSize() int {
	height := m.height
	if height <= 0 {
		height = 32
	}
	return maxLocal(height-14, 3)
}

func (m agentTUIModel) appendThinkingBlock(lines []string, turn cliui.BlackboardTurn, live bool, width int) []string {
	index := turn.Index
	if index <= 0 {
		index = len(m.turns) + 1
	}
	expanded := m.expandedTurns != nil && m.expandedTurns[index]
	marker := ">"
	if expanded {
		marker = "v"
	}
	label := fmt.Sprintf("%s %s %s 思考过程", m.thinkingIcon(live), m.thinkingMood(live), marker)
	summary := m.thinkingCollapsedSummary(turn, live)
	lines = appendChatBlock(lines, label, summary, width, false, tuiCyan, tuiSurface)
	if expanded {
		detail := "完整黑板: /bb " + strconv.Itoa(index)
		if live {
			detail = "完整黑板: /bb latest"
		}
		lines = appendChatBlock(lines, "", detail+"\n"+m.inlineThinkingDetail(turn, live, maxLocal(width-10, 24)), width, true, tuiCyan, tuiSurface)
	}
	return lines
}

func appendChatBlock(lines []string, label, text string, width int, markdown bool, color, background lipgloss.Color) []string {
	if width < 24 {
		width = 24
	}
	text = strings.TrimSpace(text)
	if text == "" {
		text = "(空)"
	}
	blockW := maxLocal(width-6, 18)
	bodyW := maxLocal(blockW-8, 12)
	var content string
	if markdown {
		content = tuiMaybeMarkdown(text, bodyW, true)
	} else {
		text = tuiANSIRe.ReplaceAllString(text, "")
		var wrappedLines []string
		for _, raw := range strings.Split(text, "\n") {
			wrapped := wrapPlainDisplay(raw, bodyW)
			if len(wrapped) == 0 {
				wrapped = []string{""}
			}
			wrappedLines = append(wrappedLines, wrapped...)
		}
		content = strings.Join(wrappedLines, "\n")
	}
	if strings.TrimSpace(label) != "" {
		labelLine := lipgloss.NewStyle().
			Foreground(color).
			Bold(true).
			Render(label)
		content = labelLine + "\n" + content
	}
	block := lipgloss.NewStyle().
		Width(blockW).
		MarginLeft(1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Background(background).
		Padding(1, 2).
		Render(content)
	return append(lines, strings.Split(block, "\n")...)
}

func wrapPlainDisplay(s string, width int) []string {
	s = strings.TrimRight(s, " \t\r")
	if width <= 0 {
		return []string{s}
	}
	if s == "" {
		return []string{""}
	}
	var lines []string
	var b strings.Builder
	used := 0
	for _, r := range s {
		part := string(r)
		w := lipgloss.Width(part)
		if used > 0 && used+w > width {
			lines = append(lines, b.String())
			b.Reset()
			used = 0
		}
		b.WriteRune(r)
		used += w
	}
	lines = append(lines, b.String())
	return lines
}

func (m agentTUIModel) contextPanel(width, height int) string {
	return m.memoryPanel(width, height)
}

func (m agentTUIModel) sessionsPanel(width, height int) string {
	rows := m.sessionRowsForView()
	if len(rows) == 0 {
		body := tuiEmptyState("暂无历史 Session。发送一轮问题后，这里会显示可继续会话。", width-4)
		return tuiPanel("历史 Session", body, width, height, tuiViolet)
	}
	if m.sessionCursor < 0 {
		m.sessionCursor = 0
	}
	if m.sessionCursor >= len(rows) {
		m.sessionCursor = len(rows) - 1
	}
	visibleRows, offset := visibleSessionRows(rows, m.sessionCursor, maxLocal(height-5, 3))
	lines := []string{
		tuiMuted().Render("↑↓ 选择 · Enter 切换当前 TUI 会话 · Tab/1-5 切换 tab"),
	}
	if m.sessionLoadErr != "" {
		lines = append(lines, tuiErrorStyle().Render("历史读取失败: "+shortLocal(m.sessionLoadErr, width-12)))
	}
	if offset > 0 {
		lines = append(lines, tuiMuted().Render("... 上方还有 Session"))
	}
	for i, row := range visibleRows {
		global := offset + i
		selected := global == m.sessionCursor
		prefix := "  "
		style := lipgloss.NewStyle().Foreground(tuiInk)
		if selected {
			prefix = "> "
			style = tuiSelectedStyle()
		}
		current := " "
		if row.Key == m.session {
			current = "*"
		}
		title := fmt.Sprintf("%s%s %s", prefix, current, shortLocal(row.Key, width-18))
		desc := row.Summary
		if desc == "" {
			desc = row.LastText
		}
		if desc == "" {
			desc = fmt.Sprintf("%d 条消息", row.Messages)
		} else {
			desc = fmt.Sprintf("%d 条消息 / %s", row.Messages, desc)
		}
		lines = append(lines,
			style.Width(width-4).Render(shortLocal(title, width-4)),
			tuiMuted().Render("     "+shortLocal(desc, width-9)),
		)
	}
	if offset+len(visibleRows) < len(rows) {
		lines = append(lines, tuiMuted().Render("... 下方还有 Session"))
	}
	return tuiPanel("历史 Session", strings.Join(lines, "\n"), width, height, tuiViolet)
}

func (m agentTUIModel) sessionRowsForView() []cliui.SessionRow {
	if len(m.sessionRows) > 0 {
		return m.sessionRows
	}
	if strings.TrimSpace(m.session) == "" {
		return nil
	}
	return []cliui.SessionRow{{Key: m.session, Messages: len(m.turns), Summary: "当前会话"}}
}

func visibleSessionRows(rows []cliui.SessionRow, selected, maxRows int) ([]cliui.SessionRow, int) {
	if maxRows <= 0 || len(rows) <= maxRows {
		return rows, 0
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= len(rows) {
		selected = len(rows) - 1
	}
	start := selected - maxRows/2
	if start < 0 {
		start = 0
	}
	if start+maxRows > len(rows) {
		start = len(rows) - maxRows
	}
	return rows[start : start+maxRows], start
}

func (m agentTUIModel) memoryPanel(width, height int) string {
	if m.memoryEditing {
		return m.memoryEditorPanel(width, height)
	}
	bodyH := maxLocal(height-5, 6)
	contentW := maxLocal(width-4, 32)
	lines := []string{
		tuiMuted().Render("↑↓/j/k 选择 · Enter/e 编辑 · PgUp/PgDn 滚动预览 · Tab/1-5 切换 tab"),
	}
	if m.memoryMessage != "" {
		lines = append(lines, tuiAccent().Render(shortLocal(m.memoryMessage, contentW)))
	}
	if width < 78 {
		lines = append(lines, "", tuiSectionLabel("核心 Markdown 记忆", tuiPink))
		lines = append(lines, m.renderMemoryList(contentW, minLocal(7, bodyH))...)
		lines = append(lines, "")
		lines = append(lines, m.renderMemoryPreview(contentW, maxLocal(bodyH-len(lines)-1, 4))...)
		return tuiPanel("记忆 / Markdown 可编辑", strings.Join(clampHeadStringLines(lines, bodyH), "\n"), width, height, tuiPink)
	}

	listW := 30
	previewW := contentW - listW - 3
	if previewW < 32 {
		previewW = 32
		listW = contentW - previewW - 3
	}
	left := strings.Join(m.renderMemoryList(listW, bodyH-1), "\n")
	right := strings.Join(m.renderMemoryPreview(previewW, bodyH-1), "\n")
	columns := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(listW).Render(left),
		tuiMuted().Render(" │ "),
		lipgloss.NewStyle().Width(previewW).Render(right),
	)
	lines = append(lines, columns)
	return tuiPanel("记忆 / Markdown 可编辑", strings.Join(lines, "\n"), width, height, tuiPink)
}

func (m agentTUIModel) memoryEditorPanel(width, height int) string {
	spec := m.selectedMemoryFile()
	contentW := maxLocal(width-4, 32)
	bodyH := maxLocal(height-5, 6)
	lines := []string{
		tuiSectionLabel("编辑 "+spec.Label, spec.Color),
		tuiKV("文件", spec.RelPath, contentW),
		tuiMuted().Render("Ctrl+D 保存 · Esc/Ctrl+C 取消 · Enter 换行 · PgUp/PgDn 滚动"),
		"",
	}
	editLines := m.renderMemoryEditLines(contentW, maxLocal(bodyH-len(lines), 3))
	lines = append(lines, editLines...)
	return tuiPanel("记忆编辑 · "+spec.Kind, strings.Join(clampHeadStringLines(lines, bodyH), "\n"), width, height, spec.Color)
}

func (m agentTUIModel) renderMemoryList(width, maxRows int) []string {
	if maxRows < 1 {
		return nil
	}
	var rows []string
	rows = append(rows, tuiSectionLabel("核心记忆", tuiPink))
	for i, spec := range tuiMemoryFiles {
		if len(rows) >= maxRows {
			break
		}
		content, exists, err := m.readMemoryFile(spec)
		state := "未创建"
		if err != nil {
			state = "读取失败"
		} else if exists {
			state = fmt.Sprintf("%d 行", countDisplayLines(content))
		}
		prefix := "  "
		style := lipgloss.NewStyle().Foreground(tuiInk)
		if i == m.memoryCursor {
			prefix = "> "
			style = tuiSelectedStyle()
		}
		name := fmt.Sprintf("%s%-8s %s", prefix, spec.Label, state)
		rows = append(rows, style.Render(shortLocal(name, width)))
		if len(rows) >= maxRows {
			break
		}
		rows = append(rows, tuiMuted().Render(shortLocal("   "+spec.RelPath+" · "+spec.Description, width)))
	}
	return rows
}

func (m agentTUIModel) renderMemoryPreview(width, maxRows int) []string {
	if maxRows < 1 {
		return nil
	}
	spec := m.selectedMemoryFile()
	content, exists, err := m.readMemoryFile(spec)
	var rows []string
	rows = append(rows, tuiSectionLabel(spec.Label+" · "+spec.RelPath, spec.Color))
	if err != nil {
		rows = append(rows, tuiErrorStyle().Render(shortLocal("读取失败: "+err.Error(), width)))
		return clampHeadStringLines(rows, maxRows)
	}
	if !exists {
		rows = append(rows, tuiMuted().Render(shortLocal("文件尚未创建，Enter/e 会使用模板开始编辑。", width)))
		content = spec.Placeholder
	}
	if strings.TrimSpace(content) == "" {
		content = spec.Placeholder
	}
	rendered := tuiMaybeMarkdown(content, width, true)
	bodyLines := flattenTUIRows(strings.Split(rendered, "\n"))
	available := maxRows - len(rows) - 1
	if available < 1 {
		available = 1
	}
	visible, indicator := visibleMemoryLines(bodyLines, m.memoryScroll, available, width)
	rows = append(rows, visible...)
	if indicator != "" && len(rows) < maxRows {
		rows = append(rows, indicator)
	}
	return clampHeadStringLines(rows, maxRows)
}

func (m agentTUIModel) renderMemoryEditLines(width, maxRows int) []string {
	if maxRows < 1 {
		return nil
	}
	rs := []rune(m.memoryEditBuffer)
	cursor := m.memoryEditCursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(rs) {
		cursor = len(rs)
	}
	withCursor := string(rs[:cursor]) + "▌" + string(rs[cursor:])
	var rows []string
	for _, raw := range strings.Split(withCursor, "\n") {
		wrapped := wrapPlainDisplay(raw, width)
		if len(wrapped) == 0 {
			wrapped = []string{""}
		}
		rows = append(rows, wrapped...)
	}
	visible, indicator := visibleMemoryLines(rows, m.memoryScroll, maxRows, width)
	if indicator != "" && len(visible) < maxRows {
		visible = append(visible, indicator)
	}
	return visible
}

func (m agentTUIModel) readMemoryContent(spec tuiMemoryFile) string {
	content, exists, err := m.readMemoryFile(spec)
	if err != nil {
		return "读取失败: " + err.Error()
	}
	if !exists || strings.TrimSpace(content) == "" {
		return spec.Placeholder
	}
	return content
}

func (m agentTUIModel) readMemoryFile(spec tuiMemoryFile) (string, bool, error) {
	path := m.memoryFilePath(spec)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return string(data), true, nil
}

func (m agentTUIModel) memoryFilePath(spec tuiMemoryFile) string {
	rel := filepath.Clean(filepath.FromSlash(spec.RelPath))
	return filepath.Join(m.memoryWorkspacePath(), rel)
}

func (m agentTUIModel) memoryWorkspacePath() string {
	if strings.TrimSpace(m.workspace) != "" {
		return m.workspace
	}
	if workspace := defaultAgentWorkspace(m.agentLoop); workspace != "" {
		return workspace
	}
	if m.agentLoop != nil && m.agentLoop.GetConfig() != nil {
		if workspace := strings.TrimSpace(m.agentLoop.GetConfig().WorkspacePath()); workspace != "" {
			return workspace
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

func defaultAgentWorkspace(agentLoop *coreagent.AgentLoop) string {
	if agentLoop == nil || agentLoop.GetRegistry() == nil {
		return ""
	}
	agent := agentLoop.GetRegistry().GetDefaultAgent()
	if agent == nil {
		return ""
	}
	return strings.TrimSpace(agent.Workspace)
}

func visibleMemoryLines(rows []string, scroll, height, width int) ([]string, string) {
	if height < 1 {
		height = 1
	}
	if len(rows) <= height {
		return rows, ""
	}
	maxScroll := len(rows) - height
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	end := scroll + height
	if end > len(rows) {
		end = len(rows)
	}
	indicator := tuiMuted().Render(fmt.Sprintf("位置 %d-%d/%d", scroll+1, end, len(rows)))
	if lipgloss.Width(indicator) > width {
		indicator = tuiMuted().Render(shortLocal(tuiANSIRe.ReplaceAllString(indicator, ""), width))
	}
	return rows[scroll:end], indicator
}

func countDisplayLines(content string) int {
	trimmed := strings.TrimRight(content, "\n")
	if trimmed == "" {
		return 0
	}
	return len(strings.Split(trimmed, "\n"))
}

func (m agentTUIModel) settingsPanel(width, height int) string {
	lines := []string{
		tuiSectionLabel("设置", tuiGreen),
		tuiKV("当前模型", m.model, width-4),
		tuiKV("当前会话", m.session, width-4),
		tuiKV("运行模式", m.modeLabel()+"（/yolo 切换；不会关闭危险命令拦截）", width-4),
		tuiKV("初始化", "flyflor setup 会配置默认模型、黑板调度器与必要 TUI 工具", width-4),
		tuiKV("模型配置", "flyflor model 查看或切换默认模型", width-4),
		tuiKV("会话查看", "历史 Session 已内置到第 2 个 tab", width-4),
		tuiKV("黑板查看", "黑板已内置到第 3 个 tab；/bb 仍可直达", width-4),
		tuiKV("长文本", "Ctrl+E 或 /edit 展开编辑区，Enter 换行，Ctrl+D 发送", width-4),
		tuiKV("即时引导", "回答中输入补充要求，Ctrl+S 立即注入当前 turn", width-4),
		"",
		tuiSectionLabel("高级透传", tuiAmber),
		tuiKV("fx", "仅用于直接查看 JSON 文件或管道；日常黑板查看使用 TUI tab", width-4),
		tuiKV("gum", "仅用于脚本 choose/input/write；日常长文本使用 Ctrl+E", width-4),
		"",
		tuiSectionLabel("快捷键", tuiCyan),
		tuiKV("Ctrl+S", "回答进行中立即注入引导；空闲时等同发送", width-4),
		tuiKV("Tab", "切换五个 tab；输入 / 时接受补全", width-4),
		tuiKV("1-5", "直达 对话 / 历史 Session / 黑板 / 记忆 / 设置", width-4),
		tuiKV("?", "打开命令帮助弹窗", width-4),
	}
	return tuiPanel("设置", strings.Join(lines, "\n"), width, height, tuiGreen)
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
	bodyLines := height - 4
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
			Title:      "Worker 讨论",
			Summary:    "Blackboard Scheduler 协调，Flyflor Planner 拆解，Flyflor Reviewer 复核",
			Detail:     "Blackboard Scheduler: 协调主回答、worker 讨论和黑板压缩。\nFlyflor Planner: 明确目标、边界、执行路径和验证点。\nFlyflor Reviewer: 复核误解、遗漏、风险、可读性和回滚点。\nSession Workbench: 按 session 隔离 workbench，只把对用户有帮助的过程压缩成可读黑板。",
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
			"3. Blackboard Scheduler 会优先保留用户可读的判断，而不是堆叠原始日志。",
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
	summary := "已完成 · 黑板已压缩，worker 讨论和记忆写入可展开阅读"
	if live {
		state = "思考中"
		color = tuiAmber
		summary = m.thinkingLine()
	}
	head := lipgloss.JoinHorizontal(lipgloss.Center,
		tuiBlockLabel(fmt.Sprintf("%s 第%d轮", state, index), color),
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
		"- Blackboard Scheduler: 按 session 隔离 workbench，控制多个 worker 的上下文与并发。",
		"- Blackboard Scheduler: 协调主回答、worker 讨论和黑板压缩。",
		"- Flyflor Planner: 拆解目标、边界、执行路径与验证点。",
		"- Flyflor Reviewer: 复核误解、遗漏、风险与可读性。",
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
		tuiKV("模式", m.modeLabel(), 68),
		tuiKV("轮次", fmt.Sprintf("%d", len(m.turns)), 68),
		tuiKV("Workers", "Flyflor Planner ↔ Flyflor Reviewer", 68),
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
	if m.dialog.Kind == "help" {
		body = clampHeadLines(m.dialog.Body, 10)
	}
	title := tuiBlockLabel(m.dialog.Title, m.dialog.Color)
	content := title + "\n\n" +
		lipgloss.NewStyle().Width(bodyW).Foreground(tuiInk).Render(body)
	if strings.TrimSpace(m.dialog.Footer) != "" {
		content += "\n\n" + lipgloss.NewStyle().Width(bodyW).Foreground(tuiSubtle).Render(m.dialog.Footer)
	}
	box := lipgloss.NewStyle().
		Width(dialogW).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.dialog.Color).
		Padding(0, 1).
		Render(content)
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, box)
}

func (m agentTUIModel) inputPanel(width, height int) string {
	if width < 12 {
		width = 12
	}
	if height < 3 {
		height = 3
	}
	rs := []rune(m.input)
	cursor := m.cursor
	if cursor < 0 || cursor > len(rs) {
		cursor = len(rs)
	}
	rendered := string(rs[:cursor]) + tuiCursorStyle().Render(" ") + string(rs[cursor:])
	if strings.TrimSpace(m.input) == "" {
		if m.inputExpanded {
			rendered = tuiMuted().Render("长文本编辑区。Enter 换行，Ctrl+D 发送；回答中 Ctrl+S 即时引导。")
		} else {
			rendered = tuiMuted().Render("向 Flyflor 提问，或输入 /help、/edit、/bb、/yolo")
		}
	}
	body := renderedInputWindow(rendered, string(rs[:cursor]), height-2)
	label := "› "
	border := tuiViolet
	if m.inputExpanded {
		label = "编辑 › "
		border = tuiCyan
	}
	if m.yoloMode {
		border = tuiAmber
	}
	return lipgloss.NewStyle().
		Width(width-2).
		Height(height-2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Background(tuiSurface).
		Padding(0, 2).
		Render(tuiPromptStyle().Render(label) + body)
}

func renderedInputWindow(rendered, beforeCursor string, maxLines int) string {
	if maxLines < 1 {
		maxLines = 1
	}
	lines := strings.Split(rendered, "\n")
	if len(lines) <= maxLines {
		return rendered
	}
	cursorLine := strings.Count(beforeCursor, "\n")
	start := cursorLine - maxLines + 1
	if start < 0 {
		start = 0
	}
	if start+maxLines > len(lines) {
		start = len(lines) - maxLines
	}
	if start < 0 {
		start = 0
	}
	window := append([]string{}, lines[start:start+maxLines]...)
	if start > 0 && len(window) > 0 {
		window[0] = tuiMuted().Render("...") + window[0]
	}
	if start+maxLines < len(lines) && len(window) > 0 {
		last := len(window) - 1
		window[last] = window[last] + tuiMuted().Render(" ...")
	}
	return strings.Join(window, "\n")
}

func (m agentTUIModel) commandMenu(width int) string {
	filtered := m.filteredCommands()
	if len(filtered) == 0 {
		return tuiPanel("扩展菜单", tuiEmptyState("没有匹配的 / 命令", width-4), width, 5, tuiPink)
	}
	visible, selectedOffset, clipped := visibleCommandMenuRows(filtered, m.menuIndex, 6)
	var rows []string
	if clipped.before {
		rows = append(rows, tuiMuted().Render("... 上方还有命令"))
	}
	for i, cmd := range visible {
		prefix := "  "
		nameStyle := tuiCommandNameStyle()
		descStyle := tuiMuted()
		if i == selectedOffset {
			prefix = "➜ "
			nameStyle = tuiSelectedStyle()
			descStyle = lipgloss.NewStyle().Foreground(tuiInk)
		}
		name := cmd.Name
		if cmd.Args != "" {
			name += " " + cmd.Args
		}
		name = shortLocal(prefix+name, 23)
		rows = append(rows,
			nameStyle.Width(24).Render(name)+
				descStyle.Render("  "+cmd.Desc),
		)
	}
	if clipped.after {
		rows = append(rows, tuiMuted().Render("... 下方还有命令"))
	}
	return tuiPanel("命令菜单 · /edit 长文本 · /yolo 模式", strings.Join(rows, "\n"), width, len(rows)+3, tuiPink)
}

type commandMenuClip struct {
	before bool
	after  bool
}

func visibleCommandMenuRows(commands []slashCommand, selected, maxRows int) ([]slashCommand, int, commandMenuClip) {
	if maxRows <= 0 || len(commands) <= maxRows {
		return commands, selected, commandMenuClip{}
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= len(commands) {
		selected = len(commands) - 1
	}
	start := selected - maxRows/2
	if start < 0 {
		start = 0
	}
	if start+maxRows > len(commands) {
		start = len(commands) - maxRows
	}
	end := start + maxRows
	return commands[start:end], selected - start, commandMenuClip{
		before: start > 0,
		after:  end < len(commands),
	}
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
	if m.currentTab() == agentTabBlackboard {
		m.switchTab(agentTabChat)
		return
	}
	m.switchTab(agentTabBlackboard)
	if m.blackboardTurnCount() > 0 && m.selectedTurn < 0 {
		m.selectedTurn = m.blackboardTurnCount() - 1
	}
	m.status = "折叠黑板已展开"
}

func (m agentTUIModel) currentTab() agentTUITab {
	if m.blackboardVisible && m.activeTab == agentTabChat {
		return agentTabBlackboard
	}
	if m.activeTab < agentTabChat || m.activeTab > agentTabSettings {
		return agentTabChat
	}
	return m.activeTab
}

func (m *agentTUIModel) switchTab(tab agentTUITab) {
	if tab < agentTabChat || tab > agentTabSettings {
		tab = agentTabChat
	}
	m.activeTab = tab
	m.blackboardVisible = tab == agentTabBlackboard
	if tab != agentTabMemory {
		m.memoryEditing = false
		m.memoryEditBuffer = ""
		m.memoryEditCursor = 0
	}
	if tab == agentTabBlackboard {
		if m.blackboardTurnCount() > 0 && m.selectedTurn < 0 {
			m.selectedTurn = m.blackboardTurnCount() - 1
		}
		m.blackboardScroll = 0
	}
	m.activePanel = agentTUITabLabel(tab)
	m.status = "已切换到 " + agentTUITabLabel(tab) + " tab"
}

func (m *agentTUIModel) nextTab() {
	m.switchTab(agentTUITab((int(m.currentTab()) + 1) % len(agentTUITabs)))
}

func (m *agentTUIModel) prevTab() {
	next := int(m.currentTab()) - 1
	if next < 0 {
		next = len(agentTUITabs) - 1
	}
	m.switchTab(agentTUITab(next))
}

func agentTUITabLabel(tab agentTUITab) string {
	for _, spec := range agentTUITabs {
		if spec.ID == tab {
			return spec.Label
		}
	}
	return "对话"
}

func (m agentTUIModel) canNavigateSessions() bool {
	return m.currentTab() == agentTabSessions && !m.menuVisible && strings.TrimSpace(m.input) == "" && len(m.sessionRowsForView()) > 0
}

func (m *agentTUIModel) moveSessionCursor(delta int) {
	rows := m.sessionRowsForView()
	if len(rows) == 0 {
		m.sessionCursor = 0
		return
	}
	m.sessionCursor += delta
	if m.sessionCursor < 0 {
		m.sessionCursor = len(rows) - 1
	}
	if m.sessionCursor >= len(rows) {
		m.sessionCursor = 0
	}
	m.status = fmt.Sprintf("已选择 Session %s", rows[m.sessionCursor].Key)
}

func (m *agentTUIModel) selectSession() tea.Cmd {
	rows := m.sessionRowsForView()
	if len(rows) == 0 {
		m.status = "暂无可切换的 Session"
		return nil
	}
	if m.sessionCursor < 0 {
		m.sessionCursor = 0
	}
	if m.sessionCursor >= len(rows) {
		m.sessionCursor = len(rows) - 1
	}
	row := rows[m.sessionCursor]
	if row.Key == "" {
		m.status = "Session key 为空，无法切换"
		return nil
	}
	m.session = row.Key
	if m.agentLoop != nil {
		m.turns = loadSessionTurns(m.agentLoop, row.Key)
	} else {
		m.turns = nil
	}
	m.selectedTurn = len(m.turns) - 1
	m.blackboardCursor = 0
	m.blackboardScroll = 0
	m.chatScroll = tuiScrollBottom
	m.expandedNodes = map[string]bool{}
	m.expandedTurns = map[int]bool{}
	m.switchTab(agentTabChat)
	m.status = "已切换 Session: " + row.Key
	m.events = appendRecent(m.events, "已切换 Session: "+row.Key, 7)
	return nil
}

func (m agentTUIModel) canNavigateMemory() bool {
	return m.currentTab() == agentTabMemory && !m.menuVisible && strings.TrimSpace(m.input) == "" && len(tuiMemoryFiles) > 0
}

func (m *agentTUIModel) moveMemoryCursor(delta int) {
	count := len(tuiMemoryFiles)
	if count == 0 {
		m.memoryCursor = 0
		return
	}
	m.memoryCursor += delta
	if m.memoryCursor < 0 {
		m.memoryCursor = count - 1
	}
	if m.memoryCursor >= count {
		m.memoryCursor = 0
	}
	m.memoryScroll = 0
	spec := tuiMemoryFiles[m.memoryCursor]
	m.status = "已选择记忆: " + spec.Label
}

func (m agentTUIModel) selectedMemoryFile() tuiMemoryFile {
	if len(tuiMemoryFiles) == 0 {
		return tuiMemoryFile{}
	}
	if m.memoryCursor < 0 || m.memoryCursor >= len(tuiMemoryFiles) {
		return tuiMemoryFiles[0]
	}
	return tuiMemoryFiles[m.memoryCursor]
}

func (m agentTUIModel) memoryPageSize() int {
	page := m.height - 16
	if page < 4 {
		page = 4
	}
	return page
}

func (m *agentTUIModel) scrollMemory(delta int) {
	var lines []string
	if m.memoryEditing {
		for _, raw := range strings.Split(m.memoryEditBuffer, "\n") {
			lines = append(lines, wrapPlainDisplay(raw, maxLocal(m.width-4, 24))...)
		}
	} else {
		spec := m.selectedMemoryFile()
		content := m.readMemoryContent(spec)
		lines = flattenTUIRows(strings.Split(tuiMaybeMarkdown(content, maxLocal(m.width-34, 24), true), "\n"))
	}
	maxScroll := maxLocal(len(lines)-m.memoryPageSize(), 0)
	m.memoryScroll += delta
	if m.memoryScroll < 0 {
		m.memoryScroll = 0
	}
	if m.memoryScroll > maxScroll {
		m.memoryScroll = maxScroll
	}
	m.status = fmt.Sprintf("记忆预览滚动 %d/%d", m.memoryScroll, maxScroll)
}

func (m agentTUIModel) canNavigateBlackboard() bool {
	return m.currentTab() == agentTabBlackboard && !m.menuVisible && strings.TrimSpace(m.input) == "" && m.blackboardTurnCount() > 0
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
	bodyLines := height - 4
	if bodyLines < 2 {
		bodyLines = 2
	}
	lines := flattenTUIRows(m.blackboardRenderableRows(width))
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
		tuiMuted().Render("黑板 tab: ↑↓/j/k 选择 · Enter/Space/→ 展开 · PgUp/PgDn/滚轮 滚动 · Esc/q 回到对话"),
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
	lines := flattenTUIRows(rows)
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

func flattenTUIRows(rows []string) []string {
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

type thinkingState struct {
	icon  string
	mood  string
	stage string
}

func (m agentTUIModel) dynamicThinkingState() thinkingState {
	states := []thinkingState{
		{icon: "🤔", mood: "斟酌", stage: "解析意图"},
		{icon: "🙂", mood: "从容", stage: "召回记忆"},
		{icon: "😅", mood: "攻坚", stage: "组织上下文"},
		{icon: "😮‍💨", mood: "缓冲", stage: "调用模型"},
		{icon: "💭", mood: "构思", stage: "压缩黑板"},
		{icon: "😁", mood: "收尾", stage: "准备回答"},
	}
	return states[m.spinner%len(states)]
}

func (m agentTUIModel) dynamicThinkingStatus() string {
	state := m.dynamicThinkingState()
	return state.mood + " · " + state.stage
}

func (m agentTUIModel) thinkingIcon(live bool) string {
	if !live {
		return "😁"
	}
	return m.dynamicThinkingState().icon
}

func (m agentTUIModel) thinkingMood(live bool) string {
	if !live {
		return "完成"
	}
	return m.dynamicThinkingState().mood
}

func (m agentTUIModel) thinkingElapsed() string {
	if m.thinkingStart.IsZero() {
		return "0s"
	}
	elapsed := time.Since(m.thinkingStart)
	if elapsed < time.Second {
		return fmt.Sprintf("%dms", elapsed.Milliseconds())
	}
	return fmt.Sprintf("%ds", int(elapsed.Seconds()))
}

func (m agentTUIModel) latestDiscussionSummary() string {
	for i := len(m.events) - 1; i >= 0; i-- {
		event := strings.TrimSpace(m.events[i])
		if event != "" {
			return shortLocal(event, 42)
		}
	}
	return m.dynamicThinkingSummary()
}

func (m agentTUIModel) thinkingCollapsedSummary(turn cliui.BlackboardTurn, live bool) string {
	index := turn.Index
	if index <= 0 {
		index = len(m.turns) + 1
	}
	if live {
		return fmt.Sprintf("%s · 摘要: %s · 用时 %s · /bb latest 查看黑板", m.dynamicThinkingStatus(), m.latestDiscussionSummary(), m.thinkingElapsed())
	}
	state := "已折叠"
	if m.expandedTurns != nil && m.expandedTurns[index] {
		state = "已展开"
	}
	return fmt.Sprintf("%s · /think %d 展开/收拢 · /bb %d 查看黑板", state, index, index)
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
	return m.thinkingIcon(true) + " " + m.dynamicThinkingSummary() + "，思考过程会在这里动态刷新。"
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
	titleBar := tuiBlockLabel(title, color)
	return lipgloss.NewStyle().
		Width(contentW).
		Height(contentH).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Background(tuiSurface).
		Padding(0, 1).
		Render(titleBar + "\n" + body)
}

func tuiFlowPanel(title, body string, width int, color lipgloss.Color) string {
	if width < 12 {
		width = 12
	}
	contentW := width - 2
	if contentW < 8 {
		contentW = 8
	}
	titleBar := lipgloss.NewStyle().
		Foreground(tuiBase).
		Background(color).
		Bold(true).
		Padding(0, 1).
		Render(" " + title + " ")
	return lipgloss.NewStyle().
		Width(contentW).
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
	return lipgloss.NewStyle().Foreground(tuiInk).Bold(true)
}

func tuiMuted() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(tuiSubtle)
}

func tuiAccent() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(tuiInk).Bold(true)
}

func tuiSelectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(tuiPink).Bold(true).Underline(true)
}

func tuiErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(tuiRed).Bold(true)
}

func tuiCursorStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(tuiBase).Background(tuiPink)
}

func tuiPromptStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(tuiViolet).Bold(true)
}

func tuiCommandNameStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(tuiPink).Bold(true)
}

func tuiBlockLabel(label string, color lipgloss.Color) string {
	return lipgloss.NewStyle().
		Foreground(tuiBase).
		Background(color).
		Bold(true).
		Padding(0, 2).
		Render(" " + strings.TrimSpace(label) + " ")
}

func tuiTranscriptPrefix(prefix string) string {
	color := tuiCyan
	if strings.HasPrefix(prefix, "flyflor") {
		color = tuiPink
	}
	return lipgloss.NewStyle().Foreground(color).Bold(true).Render(prefix)
}

func tuiPill(label, value string, color lipgloss.Color) string {
	if strings.TrimSpace(value) == "" {
		value = "未配置"
	}
	return tuiBlockLabel(label, color) +
		lipgloss.NewStyle().
			Foreground(tuiSubtle).
			Padding(0, 1).
			Render(value)
}

func tuiKeyHint(key, value string) string {
	return lipgloss.NewStyle().Foreground(tuiPink).Bold(true).Render(key) +
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
	roleLabel := tuiBlockLabel(role, color)
	bodyColor := tuiInk
	if !assistant {
		bodyColor = lipgloss.Color("#FBE7FF")
	}
	body := lipgloss.NewStyle().
		Width(bubbleW-2).
		Foreground(bodyColor).
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

func clampHeadLines(s string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[:maxLines], "\n")
}

func clampHeadStringLines(lines []string, maxLines int) []string {
	if maxLines <= 0 {
		return nil
	}
	if len(lines) <= maxLines {
		return lines
	}
	return lines[:maxLines]
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
	cacheKey := fmt.Sprintf("%d\x00%s", width, trimmed)
	if rendered, ok := tuiMarkdownCache[cacheKey]; ok {
		return rendered
	}
	renderer, err := tuiMarkdownRenderer(width)
	if err != nil {
		return wrapLocal(trimmed, width)
	}
	rendered, err := renderer.Render(trimmed)
	if err != nil {
		return wrapLocal(trimmed, width)
	}
	rendered = strings.TrimSpace(rendered)
	if len(tuiMarkdownCache) > 128 {
		tuiMarkdownCache = map[string]string{}
	}
	tuiMarkdownCache[cacheKey] = rendered
	return rendered
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
	s = strings.TrimSpace(s)
	if max <= 0 || lipgloss.Width(s) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	ellipsisW := lipgloss.Width("…")
	var b strings.Builder
	used := 0
	for _, r := range s {
		part := string(r)
		w := lipgloss.Width(part)
		if used+w+ellipsisW > max {
			break
		}
		b.WriteRune(r)
		used += w
	}
	return b.String() + "…"
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
