package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/agent"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/cliui"
)

type rootMenuAction struct {
	Args []string
}

type rootMenuItem struct {
	Title string
	Desc  string
	Args  []string
	Kind  string
}

type rootMenuSection struct {
	Title string
	Desc  string
	Items []rootMenuItem
}

type rootMenuModel struct {
	width    int
	height   int
	section  int
	cursor   int
	mode     string
	sessions []cliui.SessionRow
	errText  string
	action   *rootMenuAction
}

var (
	rootMenuInk     = lipgloss.Color("#E9EAF2")
	rootMenuMuted   = lipgloss.Color("#9699A8")
	rootMenuDim     = lipgloss.Color("#6D7282")
	rootMenuBase    = lipgloss.Color("#101116")
	rootMenuSurface = lipgloss.Color("#171922")
	rootMenuHi      = lipgloss.Color("#202332")
	rootMenuBorder  = lipgloss.Color("#34384A")
	rootMenuCyan    = lipgloss.Color("#61E7D6")
	rootMenuViolet  = lipgloss.Color("#8A7DFF")
	rootMenuPink    = lipgloss.Color("#F06292")
	rootMenuAmber   = lipgloss.Color("#F2C36B")
	rootMenuGreen   = lipgloss.Color("#7DDC98")
	rootMenuRed     = lipgloss.Color("#FF6B6B")
)

func shouldShowRootMenu() bool {
	if os.Getenv("FLYFLOR_ROOT_MENU") == "0" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func runRootInteractiveMenu() (*rootMenuAction, error) {
	rows, err := agent.LoadSessionRowsForMenu()
	m := rootMenuModel{
		width:    112,
		height:   34,
		mode:     "main",
		sessions: rows,
	}
	if err != nil {
		m.errText = "会话读取失败: " + err.Error()
	}
	final, runErr := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if runErr != nil {
		return nil, runErr
	}
	if menu, ok := final.(rootMenuModel); ok {
		return menu.action, nil
	}
	return nil, nil
}

func executeRootMenuAction(action *rootMenuAction) error {
	if action == nil || len(action.Args) == 0 {
		return nil
	}
	bin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("定位 flyflor 可执行文件失败: %w", err)
	}
	cmd := exec.Command(bin, action.Args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func rootMenuSections() []rootMenuSection {
	return []rootMenuSection{
		{
			Title: "智能体",
			Desc:  "进入对话、恢复会话、查看黑板与思考过程",
			Items: []rootMenuItem{
				{Title: "进入默认 Agent TUI", Desc: "打开 Flyflor 对话界面，默认会话 cli:default", Args: []string{"agent"}},
				{Title: "选择历史 Agent Session", Desc: "从已有会话中选择并继续对话", Kind: "sessions"},
				{Title: "查看全部 Sessions", Desc: "以列表形式查看可继续的 Flyflor 会话", Args: []string{"sessions"}},
				{Title: "初始化 / Onboard", Desc: "创建或修复配置、工作区和基础记忆文件", Args: []string{"onboard"}},
			},
		},
		{
			Title: "运行",
			Desc:  "控制本地运行时、Gateway 和状态检查",
			Items: []rootMenuItem{
				{Title: "查看运行状态", Desc: "检查配置、模型、工作区和关键服务状态", Args: []string{"status"}},
				{Title: "启动 Gateway", Desc: "启动多渠道 Gateway，适合后台服务模式", Args: []string{"gateway"}},
				{Title: "版本信息", Desc: "查看当前 Flyflor 版本和构建信息", Args: []string{"version"}},
				{Title: "检查更新", Desc: "检查并应用 GitHub Release 更新", Args: []string{"update"}},
			},
		},
		{
			Title: "配置",
			Desc:  "管理模型、认证、MCP 和技能",
			Items: []rootMenuItem{
				{Title: "模型配置", Desc: "显示或切换默认模型", Args: []string{"model"}},
				{Title: "认证管理", Desc: "管理登录、登出和 Token 状态", Args: []string{"auth"}},
				{Title: "MCP 服务", Desc: "管理 MCP 服务配置与测试", Args: []string{"mcp"}},
				{Title: "技能管理", Desc: "搜索、安装、查看和移除技能", Args: []string{"skills"}},
			},
		},
		{
			Title: "工具",
			Desc:  "终端辅助工具与自动补全",
			Items: []rootMenuItem{
				{Title: "Gum 交互工具", Desc: "运行 Charm Gum，用于脚本里的选择、输入和确认", Args: []string{"gum"}},
				{Title: "FX JSON 折叠查看", Desc: "用 fx 查看 JSON、黑板结构或事件流", Args: []string{"fx"}},
				{Title: "Dive 镜像分析", Desc: "分析 Docker 镜像层和体积", Args: []string{"dive"}},
				{Title: "Shell 自动补全", Desc: "生成 bash/zsh/fish/powershell 自动补全脚本", Args: []string{"completion", "zsh"}},
				{Title: "命令帮助", Desc: "打开 Flyflor 帮助首页", Args: []string{"help"}},
			},
		},
	}
}

func (m rootMenuModel) Init() tea.Cmd {
	return nil
}

func (m rootMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	default:
		return m, nil
	}
}

func (m rootMenuModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "backspace":
		if m.mode == "sessions" {
			m.mode = "main"
			m.cursor = 1
			return m, nil
		}
		return m, tea.Quit
	case "left", "h":
		if m.mode == "main" {
			m.section--
			if m.section < 0 {
				m.section = len(rootMenuSections()) - 1
			}
			m.cursor = 0
		}
		return m, nil
	case "right", "l", "tab":
		if m.mode == "main" {
			m.section = (m.section + 1) % len(rootMenuSections())
			m.cursor = 0
		}
		return m, nil
	case "up", "k":
		m.cursor--
		if m.cursor < 0 {
			m.cursor = m.itemCount() - 1
		}
		return m, nil
	case "down", "j":
		m.cursor = (m.cursor + 1) % max(1, m.itemCount())
		return m, nil
	case "enter":
		return m.selectCurrent()
	default:
		return m, nil
	}
}

func (m rootMenuModel) selectCurrent() (tea.Model, tea.Cmd) {
	if m.mode == "sessions" {
		items := m.sessionItems()
		if len(items) == 0 {
			m.mode = "main"
			m.cursor = 0
			return m, nil
		}
		item := items[clamp(m.cursor, 0, len(items)-1)]
		if item.Kind == "back" {
			m.mode = "main"
			m.cursor = 1
			return m, nil
		}
		m.action = &rootMenuAction{Args: item.Args}
		return m, tea.Quit
	}

	sections := rootMenuSections()
	section := sections[clamp(m.section, 0, len(sections)-1)]
	item := section.Items[clamp(m.cursor, 0, len(section.Items)-1)]
	if item.Kind == "sessions" {
		m.mode = "sessions"
		m.cursor = 0
		return m, nil
	}
	m.action = &rootMenuAction{Args: item.Args}
	return m, tea.Quit
}

func (m rootMenuModel) itemCount() int {
	if m.mode == "sessions" {
		return max(1, len(m.sessionItems()))
	}
	sections := rootMenuSections()
	if len(sections) == 0 {
		return 1
	}
	return len(sections[clamp(m.section, 0, len(sections)-1)].Items)
}

func (m rootMenuModel) sessionItems() []rootMenuItem {
	if len(m.sessions) == 0 {
		return []rootMenuItem{
			{Title: "暂无历史会话，进入默认 Agent TUI", Desc: "创建 cli:default 会话并开始对话", Args: []string{"agent"}},
		}
	}
	items := make([]rootMenuItem, 0, len(m.sessions)+1)
	for _, row := range m.sessions {
		desc := fmt.Sprintf("%d 条消息", row.Messages)
		if row.Summary != "" {
			desc += " / " + row.Summary
		} else if row.LastText != "" {
			desc += " / " + row.LastText
		}
		items = append(items, rootMenuItem{
			Title: row.Key,
			Desc:  desc,
			Args:  []string{"agent", "--session", row.Key},
		})
	}
	items = append(items, rootMenuItem{
		Title: "返回主菜单",
		Desc:  "回到命令分组",
		Kind:  "back",
		Args:  nil,
	})
	return items
}

func (m rootMenuModel) View() string {
	width := m.width
	if width < 72 {
		width = 72
	}
	if width > 132 {
		width = 132
	}

	header := rootMenuHeader(width)
	body := m.mainView(width)
	if m.mode == "sessions" {
		body = m.sessionsView(width)
	}
	footer := rootMenuFooter(width, m.mode)

	return lipgloss.NewStyle().
		Width(width).
		Foreground(rootMenuInk).
		Background(rootMenuBase).
		Render(header + "\n" + body + "\n" + footer)
}

func rootMenuHeader(width int) string {
	title := lipgloss.NewStyle().Foreground(rootMenuCyan).Bold(true).Render("Flyflor Command Center")
	sub := lipgloss.NewStyle().Foreground(rootMenuMuted).Render("docker exec -it flyflor flyflor 现在会进入这个命令中心")
	line := lipgloss.NewStyle().Foreground(rootMenuDim).Render(strings.Repeat("─", max(1, width)))
	return lipgloss.NewStyle().Width(width).Render(title+"  "+sub) + "\n" + line
}

func (m rootMenuModel) mainView(width int) string {
	sections := rootMenuSections()
	leftW := 24
	if width < 92 {
		leftW = 19
	}
	rightW := width - leftW - 3
	if rightW < 42 {
		rightW = 42
	}
	left := m.sectionTabs(sections, leftW)
	right := m.commandList(sections[clamp(m.section, 0, len(sections)-1)], rightW)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "   ", right)
}

func (m rootMenuModel) sessionsView(width int) string {
	items := m.sessionItems()
	listW := width
	title := lipgloss.NewStyle().Foreground(rootMenuPink).Bold(true).Render("Agent Sessions")
	desc := lipgloss.NewStyle().Foreground(rootMenuMuted).Render("选择会话后会执行 flyflor agent --session <key>，直接进入对应 TUI。Esc 返回主菜单。")
	rows := []string{title, desc, ""}
	for i, item := range items {
		rows = append(rows, renderRootMenuItem(item, i == m.cursor, listW-4))
	}
	if m.errText != "" {
		rows = append(rows, "", lipgloss.NewStyle().Foreground(rootMenuRed).Render(m.errText))
	}
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(rootMenuBorder).
		Padding(1, 2).
		Render(strings.Join(rows, "\n"))
}

func (m rootMenuModel) sectionTabs(sections []rootMenuSection, width int) string {
	rows := []string{
		lipgloss.NewStyle().Foreground(rootMenuViolet).Bold(true).Render("命令分组"),
		lipgloss.NewStyle().Foreground(rootMenuMuted).Render("← / → 切换"),
		"",
	}
	for i, section := range sections {
		style := lipgloss.NewStyle().
			Width(width-4).
			Padding(0, 1).
			Foreground(rootMenuMuted)
		prefix := "  "
		if i == m.section {
			prefix = "▶ "
			style = style.Foreground(rootMenuInk).Background(rootMenuHi).Bold(true)
		}
		rows = append(rows, style.Render(prefix+section.Title))
	}
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(rootMenuBorder).
		Padding(1, 1).
		Render(strings.Join(rows, "\n"))
}

func (m rootMenuModel) commandList(section rootMenuSection, width int) string {
	rows := []string{
		lipgloss.NewStyle().Foreground(rootMenuCyan).Bold(true).Render(section.Title),
		lipgloss.NewStyle().Foreground(rootMenuMuted).Render(section.Desc),
		"",
	}
	for i, item := range section.Items {
		rows = append(rows, renderRootMenuItem(item, i == m.cursor, width-4))
	}
	if m.errText != "" {
		rows = append(rows, "", lipgloss.NewStyle().Foreground(rootMenuRed).Render(m.errText))
	}
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(rootMenuBorder).
		Padding(1, 2).
		Render(strings.Join(rows, "\n"))
}

func renderRootMenuItem(item rootMenuItem, selected bool, width int) string {
	prefix := "  "
	style := lipgloss.NewStyle().Width(width).Padding(0, 1)
	titleStyle := lipgloss.NewStyle().Foreground(rootMenuInk).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(rootMenuMuted)
	if selected {
		prefix = "▶ "
		style = style.Background(rootMenuHi)
		titleStyle = titleStyle.Foreground(rootMenuCyan)
		descStyle = descStyle.Foreground(rootMenuInk)
	}
	cmdText := strings.Join(item.Args, " ")
	if item.Kind == "sessions" {
		cmdText = "打开会话选择器"
	}
	if cmdText != "" {
		cmdText = "  " + lipgloss.NewStyle().Foreground(rootMenuAmber).Render(cmdText)
	}
	first := prefix + titleStyle.Render(item.Title) + cmdText
	second := "   " + descStyle.Render(truncateDisplay(item.Desc, max(16, width-4)))
	return style.Render(first + "\n" + second)
}

func rootMenuFooter(width int, mode string) string {
	help := "↑↓ 选择  ←→ 分组  Enter 执行  q 退出"
	if mode == "sessions" {
		help = "↑↓ 选择会话  Enter 进入  Esc 返回  q 退出"
	}
	return lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Foreground(rootMenuDim).
		Render(help)
}

func truncateDisplay(s string, width int) string {
	s = strings.TrimSpace(s)
	if lipgloss.Width(s) <= width {
		return s
	}
	suffix := "…"
	limit := width - lipgloss.Width(suffix)
	var b strings.Builder
	for _, r := range s {
		next := b.String() + string(r)
		if lipgloss.Width(next) > limit {
			break
		}
		b.WriteRune(r)
	}
	return b.String() + suffix
}

func clamp(v, low, high int) int {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}
