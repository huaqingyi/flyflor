package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	"github.com/sipeed/picoclaw/cmd/flyflor/internal"
	"github.com/sipeed/picoclaw/cmd/flyflor/internal/agent"
	"github.com/sipeed/picoclaw/cmd/flyflor/internal/cliui"
	setupcmd "github.com/sipeed/picoclaw/cmd/flyflor/internal/setup"
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
	helpOpen bool
	marked   map[string]bool
	sessions []cliui.SessionRow
	errText  string
	action   *rootMenuAction
	setup    bool
}

var (
	rootMenuInk     = lipgloss.Color("#F8F2FF")
	rootMenuMuted   = lipgloss.Color("#CEC2DA")
	rootMenuDim     = lipgloss.Color("#8A7A99")
	rootMenuBase    = lipgloss.Color("#140B22")
	rootMenuSurface = lipgloss.Color("#1A1028")
	rootMenuHi      = lipgloss.Color("#2A173E")
	rootMenuBorder  = lipgloss.Color("#6E4A8A")
	rootMenuCyan    = lipgloss.Color("#D946EF")
	rootMenuViolet  = lipgloss.Color("#8B5CF6")
	rootMenuPink    = lipgloss.Color("#F472D0")
	rootMenuAmber   = lipgloss.Color("#F0ABFC")
	rootMenuGreen   = lipgloss.Color("#C084FC")
	rootMenuRed     = lipgloss.Color("#FB7185")
)

func shouldShowRootMenu() bool {
	if os.Getenv("FLYFLOR_ROOT_MENU") == "0" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func runRootInteractiveMenu() (*rootMenuAction, error) {
	rows, err := agent.LoadSessionRowsForMenu()
	needsSetup := setupcmd.NeedsSetup(internal.GetConfigPath())
	width, height := 80, 24
	if w, h, sizeErr := term.GetSize(int(os.Stdout.Fd())); sizeErr == nil {
		width = w
		height = h
	}
	m := rootMenuModel{
		width:    width,
		height:   height,
		cursor:   rootMenuInitialCursor(needsSetup),
		mode:     "main",
		marked:   map[string]bool{},
		sessions: rows,
		setup:    needsSetup,
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

func rootMenuInitialCursor(needsSetup bool) int {
	if needsSetup {
		return 3
	}
	return 0
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
				{Title: "进入默认 Agent TUI", Desc: "打开对话界面；Ctrl+E 编写长文本，/bb 查看折叠黑板", Args: []string{"agent"}},
				{Title: "选择历史 Agent Session", Desc: "从已有会话中选择并继续对话", Kind: "sessions"},
				{Title: "查看全部 Sessions", Desc: "以列表形式查看可继续的 Flyflor 会话", Args: []string{"sessions"}},
				{Title: "首次配置 / Setup", Desc: "配置默认模型并修复 Agent TUI 黑板/Guard 工具", Args: []string{"setup"}},
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
			Desc:  "高级透传工具与自动补全，日常查看/编辑优先用 Agent TUI",
			Items: []rootMenuItem{
				{Title: "TUI 长文本 / 黑板", Desc: "进入 Agent TUI；Ctrl+E 编写长文本，/bb 折叠查看黑板", Args: []string{"agent"}},
				{Title: "Gum 脚本交互工具", Desc: "高级透传: 只在脚本里需要 choose/input/write 时使用", Args: []string{"gum"}},
				{Title: "FX JSON 查看器", Desc: "高级透传: 只在需要直接查看 JSON 文件或管道时使用", Args: []string{"fx"}},
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
	if m.marked == nil {
		m.marked = map[string]bool{}
	}
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "?":
		m.helpOpen = !m.helpOpen
		return m, nil
	case "esc", "backspace":
		if m.helpOpen {
			m.helpOpen = false
			return m, nil
		}
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
	case " ":
		m.toggleMarked()
		return m, nil
	default:
		return m, nil
	}
}

func (m *rootMenuModel) toggleMarked() {
	if m.marked == nil {
		m.marked = map[string]bool{}
	}
	item, ok := m.currentItem()
	if !ok || len(item.Args) == 0 {
		return
	}
	key := rootMenuItemKey(item)
	m.marked[key] = !m.marked[key]
	if !m.marked[key] {
		delete(m.marked, key)
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
	if width < 48 {
		width = 48
	}
	if width > 118 {
		width = 118
	}
	if m.mode == "sessions" {
		return m.simpleSessionsView(width)
	}
	return m.simpleMainView(width)
}

func (m rootMenuModel) simpleMainView(width int) string {
	sections := rootMenuSections()
	section := sections[clamp(m.section, 0, len(sections)-1)]
	item, _ := m.currentItem()
	cmdText := strings.Join(item.Args, " ")
	if item.Kind == "sessions" {
		cmdText = "session selector"
	}
	if cmdText == "" {
		cmdText = "no command"
	}

	lines := []string{
		rootMenuFit(rootMenuBrandLine(m.modeLabel(), len(m.marked)), width),
		rootMenuRule(width),
		rootMenuFit(rootMenuTabs(sections, m.section), width),
		rootMenuRule(width),
	}
	if m.setup {
		lines = append(lines,
			rootMenuFit("Environment: setup required - default model is missing. Choose Setup first.", width),
			rootMenuRule(width),
		)
	}
	lines = append(lines, rootMenuFit(section.Title+" - "+section.Desc, width), "")
	for i, row := range section.Items {
		lines = append(lines, m.simpleMenuItem(row, i == m.cursor, width))
	}
	lines = append(lines,
		"",
		rootMenuRule(width),
		rootMenuFit("Selected: flyflor "+cmdText, width),
		rootMenuFit("Info: "+item.Desc, width),
		rootMenuRule(width),
		rootMenuFit(rootMenuFooterText(m.mode), width),
	)
	if m.helpOpen {
		lines = append(lines, "", rootMenuRule(width))
		lines = append(lines, rootMenuHelpLines(width)...)
	}
	return lipgloss.NewStyle().
		Width(width).
		Render(strings.Join(lines, "\n"))
}

func (m rootMenuModel) simpleSessionsView(width int) string {
	items := m.sessionItems()
	sections := rootMenuSections()
	lines := []string{
		rootMenuFit(rootMenuBrandLine("sessions", len(m.marked)), width),
		rootMenuRule(width),
		rootMenuFit(rootMenuTabs(sections, clamp(m.section, 0, len(sections)-1)), width),
		rootMenuRule(width),
		rootMenuFit("Agent Sessions - Enter 继续会话，Esc 返回主菜单", width),
		"",
	}
	for i, item := range items {
		lines = append(lines, m.simpleMenuItem(item, i == m.cursor, width))
	}
	if m.errText != "" {
		lines = append(lines, "", rootMenuFit("Error: "+m.errText, width))
	}
	lines = append(lines, "", rootMenuRule(width), rootMenuFit(rootMenuFooterText(m.mode), width))
	if m.helpOpen {
		lines = append(lines, "", rootMenuRule(width))
		lines = append(lines, rootMenuHelpLines(width)...)
	}
	return lipgloss.NewStyle().
		Width(width).
		Render(strings.Join(lines, "\n"))
}

func (m rootMenuModel) simpleMenuItem(item rootMenuItem, selected bool, width int) string {
	mark := "[ ]"
	if m.marked[rootMenuItemKey(item)] {
		mark = "[x]"
	}
	prefix := "  " + mark + " "
	if selected {
		prefix = "> " + mark + " "
	}
	cmdText := strings.Join(item.Args, " ")
	if item.Kind == "sessions" {
		cmdText = "sessions"
	}
	left := prefix + item.Title
	if cmdText != "" {
		left += "  (" + cmdText + ")"
	}
	line1 := rootMenuFit(left, width)
	line2 := rootMenuFit("      "+item.Desc, width)
	if selected {
		return rootMenuSelectedLine(line1, width) + "\n" + lipgloss.NewStyle().
			Width(max(12, width-4)).
			MarginLeft(1).
			Foreground(rootMenuInk).
			Background(rootMenuHi).
			Padding(0, 1).
			Render(line2)
	}
	return lipgloss.NewStyle().Width(width).Render(line1) + "\n" +
		lipgloss.NewStyle().Width(width).Foreground(rootMenuMuted).Render(line2)
}

func rootMenuSelectedLine(line string, width int) string {
	contentW := max(12, width-4)
	line = rootMenuFit(line, contentW)
	runes := []rune(line)
	if len(runes) == 0 {
		return lipgloss.NewStyle().Width(contentW).MarginLeft(1).Background(rootMenuViolet).Render("")
	}
	split := max(1, len(runes)*62/100)
	left := string(runes[:split])
	right := string(runes[split:])
	gradient := lipgloss.NewStyle().
		Foreground(rootMenuInk).
		Background(rootMenuViolet).
		Bold(true).
		Render(left) +
		lipgloss.NewStyle().
			Foreground(rootMenuBase).
			Background(rootMenuPink).
			Bold(true).
			Render(right)
	return lipgloss.NewStyle().MarginLeft(1).Render(gradient)
}

func rootMenuBrandLine(mode string, marked int) string {
	return fmt.Sprintf("FLYFLOR  Agent Control  mode=%s  marked=%d", mode, marked)
}

func rootMenuTabs(sections []rootMenuSection, active int) string {
	var parts []string
	for i, section := range sections {
		if i == active {
			parts = append(parts, "["+section.Title+"]")
		} else {
			parts = append(parts, section.Title)
		}
	}
	return strings.Join(parts, "  ")
}

func rootMenuRule(width int) string {
	return strings.Repeat("-", max(1, width))
}

func rootMenuFooterText(mode string) string {
	if mode == "sessions" {
		return "Up/Down select  Enter open  Esc back  ? help  q quit"
	}
	return "Up/Down select  Left/Right tabs  Space mark  Enter run  ? help  q quit"
}

func rootMenuHelpLines(width int) []string {
	raw := []string{
		"Help",
		"  Up/Down or j/k    move selection",
		"  Left/Right or h/l switch section",
		"  Space             mark candidate",
		"  Enter             run selected command",
		"  Esc               back / close help",
		"  q or Ctrl+C       quit",
	}
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		out = append(out, rootMenuFit(line, width))
	}
	return out
}

func rootMenuFit(s string, width int) string {
	s = strings.ReplaceAll(strings.TrimRight(s, " \t\r\n"), "\n", " ")
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	return truncateDisplay(s, width)
}

func (m rootMenuModel) rootMenuHeader(width int) string {
	title := lipgloss.NewStyle().
		Foreground(rootMenuBase).
		Background(rootMenuCyan).
		Bold(true).
		Padding(0, 1).
		Render("FLYFLOR")
	sub := lipgloss.NewStyle().Foreground(rootMenuInk).Bold(true).Render("Agent Control Deck")
	meta := strings.Join([]string{
		rootMenuChip("docker", "exec -it", rootMenuViolet),
		rootMenuChip("mode", m.modeLabel(), rootMenuGreen),
		rootMenuChip("marked", fmt.Sprintf("%d", len(m.marked)), rootMenuAmber),
	}, " ")
	line := lipgloss.NewStyle().Foreground(rootMenuDim).Render(strings.Repeat("─", max(1, width)))
	left := lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", sub)
	head := lipgloss.NewStyle().Width(width - lipgloss.Width(meta) - 2).Render(left)
	return lipgloss.JoinHorizontal(lipgloss.Center, head, meta) + "\n" + line
}

func (m rootMenuModel) mainView(width int) string {
	sections := rootMenuSections()
	tabs := m.sectionTabs(sections, width)
	section := sections[clamp(m.section, 0, len(sections)-1)]
	if width < 100 {
		list := m.commandList(section, width)
		preview := m.commandPreview(section, width)
		return tabs + "\n" + list + "\n" + preview
	}
	leftW := width * 56 / 100
	rightW := width - leftW - 3
	list := m.commandList(section, leftW)
	preview := m.commandPreview(section, rightW)
	return tabs + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, list, "   ", preview)
}

func (m rootMenuModel) sessionsView(width int) string {
	items := m.sessionItems()
	listW := width
	title := lipgloss.NewStyle().
		Foreground(rootMenuBase).
		Background(rootMenuPink).
		Bold(true).
		Padding(0, 1).
		Render("Agent Sessions")
	desc := lipgloss.NewStyle().Foreground(rootMenuMuted).Render("选择会话后进入对应 TUI。Esc 返回主菜单，Space 标记候选。")
	rows := []string{lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", desc), ""}
	for i, item := range items {
		rows = append(rows, m.renderRootMenuItem(item, i == m.cursor, listW-4))
	}
	if m.errText != "" {
		rows = append(rows, "", lipgloss.NewStyle().Foreground(rootMenuRed).Render(m.errText))
	}
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.ThickBorder()).
		BorderForeground(rootMenuPink).
		Padding(1, 2).
		Render(strings.Join(rows, "\n"))
}

func (m rootMenuModel) sectionTabs(sections []rootMenuSection, width int) string {
	inactiveBorder := lipgloss.Border{
		Top: "─", Bottom: "─", Left: "│", Right: "│",
		TopLeft: "╭", TopRight: "╮", BottomLeft: "┘", BottomRight: "└",
	}
	activeBorder := lipgloss.Border{
		Top: "─", Bottom: " ", Left: "│", Right: "│",
		TopLeft: "╭", TopRight: "╮", BottomLeft: "┘", BottomRight: "└",
	}
	var tabs []string
	for i, section := range sections {
		style := lipgloss.NewStyle().
			Border(inactiveBorder, true).
			BorderForeground(rootMenuBorder).
			Padding(0, 2).
			Foreground(rootMenuMuted).
			Background(rootMenuSurface)
		if i == m.section {
			style = style.
				Border(activeBorder, true).
				BorderForeground(rootMenuCyan).
				Foreground(rootMenuBase).
				Background(rootMenuCyan).
				Bold(true)
		}
		tabs = append(tabs, style.Render(section.Title))
	}
	help := lipgloss.NewStyle().
		Foreground(rootMenuDim).
		Width(max(1, width-lipgloss.Width(lipgloss.JoinHorizontal(lipgloss.Top, tabs...)))).
		Align(lipgloss.Right).
		Render("←/→ tabs  ? help")
	return lipgloss.JoinHorizontal(lipgloss.Top, append(tabs, help)...)
}

func (m rootMenuModel) commandList(section rootMenuSection, width int) string {
	rows := []string{
		rootMenuPanelTitle(section.Title, section.Desc, rootMenuCyan),
		"",
	}
	for i, item := range section.Items {
		rows = append(rows, m.renderRootMenuItem(item, i == m.cursor, width-4))
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

func (m rootMenuModel) commandPreview(section rootMenuSection, width int) string {
	item, _ := m.currentItem()
	cmdText := strings.Join(item.Args, " ")
	if item.Kind == "sessions" {
		cmdText = "session selector"
	}
	if cmdText == "" {
		cmdText = "no command"
	}
	bodyW := max(20, width-6)
	checks := []string{
		rootMenuCheck("配置", true),
		rootMenuCheck("工作区", true),
		rootMenuCheck("默认模型", section.Title != "智能体" || item.Title != "进入默认 Agent TUI"),
		rootMenuCheck("网关", section.Title == "运行" && item.Title == "启动 Gateway"),
	}
	rows := []string{
		rootMenuPanelTitle("Command preview", "当前选中项会在 Enter 后执行", rootMenuViolet),
		"",
		rootMenuActionCard(item.Title, item.Desc, "flyflor "+cmdText, bodyW),
		rootMenuMarkedQueue(m, bodyW),
		rootMenuReadinessCard(checks, bodyW),
		"",
		rootMenuMiniModal(bodyW),
	}
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(rootMenuViolet).
		Padding(1, 2).
		Render(strings.Join(rows, "\n"))
}

func rootMenuActionCard(title, desc, command string, width int) string {
	rows := []string{
		lipgloss.NewStyle().Foreground(rootMenuCyan).Bold(true).Render(truncateDisplay(title, max(12, width-4))),
		lipgloss.NewStyle().Foreground(rootMenuMuted).Width(max(12, width-4)).Render(truncateDisplay(desc, max(12, width-4))),
		"",
		lipgloss.NewStyle().
			Foreground(rootMenuBase).
			Background(rootMenuAmber).
			Bold(true).
			Padding(0, 1).
			Render(truncateDisplay(command, max(12, width-4))),
	}
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.NormalBorder()).
		BorderForeground(rootMenuCyan).
		Padding(0, 1).
		Render(strings.Join(rows, "\n"))
}

func rootMenuMarkedQueue(m rootMenuModel, width int) string {
	items := m.markedItems()
	rows := []string{
		lipgloss.NewStyle().Foreground(rootMenuPink).Bold(true).Render("Marked queue"),
	}
	if len(items) == 0 {
		rows = append(rows, lipgloss.NewStyle().Foreground(rootMenuDim).Render("Space 标记多个候选后会在这里集中显示。"))
	} else {
		limit := min(3, len(items))
		for i := 0; i < limit; i++ {
			label := "■ " + items[i].Title
			rows = append(rows, lipgloss.NewStyle().Foreground(rootMenuInk).Render(truncateDisplay(label, max(12, width-4))))
		}
		if len(items) > limit {
			rows = append(rows, lipgloss.NewStyle().Foreground(rootMenuDim).Render(fmt.Sprintf("+%d more", len(items)-limit)))
		}
	}
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.NormalBorder()).
		BorderForeground(rootMenuPink).
		Padding(0, 1).
		Render(strings.Join(rows, "\n"))
}

func (m rootMenuModel) markedItems() []rootMenuItem {
	if len(m.marked) == 0 {
		return nil
	}
	var items []rootMenuItem
	for _, section := range rootMenuSections() {
		for _, item := range section.Items {
			if m.marked[rootMenuItemKey(item)] {
				items = append(items, item)
			}
		}
	}
	for _, item := range m.sessionItems() {
		if m.marked[rootMenuItemKey(item)] {
			items = append(items, item)
		}
	}
	return items
}

func rootMenuReadinessCard(checks []string, width int) string {
	body := lipgloss.NewStyle().
		Width(max(12, width-4)).
		Render(strings.Join(checks, "  "))
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.NormalBorder()).
		BorderForeground(rootMenuAmber).
		Padding(0, 1).
		Render(lipgloss.NewStyle().Foreground(rootMenuAmber).Bold(true).Render("Readiness") + "\n" + body)
}

func (m rootMenuModel) renderRootMenuItem(item rootMenuItem, selected bool, width int) string {
	mark := "☐"
	if m.marked[rootMenuItemKey(item)] {
		mark = "☑"
	}
	prefix := "  " + mark + " "
	style := lipgloss.NewStyle().Width(width).Padding(0, 1)
	titleStyle := lipgloss.NewStyle().Foreground(rootMenuInk).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(rootMenuMuted)
	if selected {
		prefix = "▸ " + mark + " "
		style = style.Background(rootMenuHi).Border(lipgloss.NormalBorder(), false, false, false, true).BorderForeground(rootMenuCyan)
		titleStyle = titleStyle.Foreground(rootMenuCyan)
		descStyle = descStyle.Foreground(rootMenuInk)
	}
	cmdText := strings.Join(item.Args, " ")
	if item.Kind == "sessions" {
		cmdText = "打开会话选择器"
	}
	if cmdText != "" {
		cmdText = "  " + lipgloss.NewStyle().
			Foreground(rootMenuBase).
			Background(rootMenuAmber).
			Padding(0, 1).
			Render(cmdText)
	}
	first := prefix + titleStyle.Render(item.Title) + cmdText
	second := "   " + descStyle.Render(truncateDisplay(item.Desc, max(16, width-4)))
	return style.Render(first + "\n" + second)
}

func rootMenuFooter(width int, mode string) string {
	help := "↑↓ 选择  ←→/Tab 切换 tabs  Space 标记  Enter 执行  ? 弹窗  q 退出"
	if mode == "sessions" {
		help = "↑↓ 选择会话  Space 标记  Enter 进入  Esc 返回  ? 弹窗  q 退出"
	}
	return lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Foreground(rootMenuDim).
		Render(help)
}

func (m rootMenuModel) currentItem() (rootMenuItem, bool) {
	if m.mode == "sessions" {
		items := m.sessionItems()
		if len(items) == 0 {
			return rootMenuItem{}, false
		}
		return items[clamp(m.cursor, 0, len(items)-1)], true
	}
	sections := rootMenuSections()
	if len(sections) == 0 {
		return rootMenuItem{}, false
	}
	section := sections[clamp(m.section, 0, len(sections)-1)]
	if len(section.Items) == 0 {
		return rootMenuItem{}, false
	}
	return section.Items[clamp(m.cursor, 0, len(section.Items)-1)], true
}

func (m rootMenuModel) modeLabel() string {
	if m.mode == "sessions" {
		return "sessions"
	}
	return "commands"
}

func rootMenuItemKey(item rootMenuItem) string {
	if len(item.Args) > 0 {
		return strings.Join(item.Args, "\x00")
	}
	return item.Kind + "\x00" + item.Title
}

func rootMenuChip(label, value string, color lipgloss.Color) string {
	return lipgloss.NewStyle().
		Foreground(rootMenuBase).
		Background(color).
		Bold(true).
		Padding(0, 1).
		Render(label) +
		lipgloss.NewStyle().
			Foreground(rootMenuInk).
			Background(rootMenuHi).
			Padding(0, 1).
			Render(value)
}

func rootMenuPanelTitle(title, desc string, color lipgloss.Color) string {
	label := lipgloss.NewStyle().
		Foreground(rootMenuBase).
		Background(color).
		Bold(true).
		Padding(0, 1).
		Render(title)
	return lipgloss.JoinHorizontal(lipgloss.Center, label, "  ", lipgloss.NewStyle().Foreground(rootMenuMuted).Render(desc))
}

func rootMenuCheck(label string, ok bool) string {
	box := "□"
	color := rootMenuMuted
	if ok {
		box = "■"
		color = rootMenuGreen
	}
	return lipgloss.NewStyle().Foreground(color).Render(box + " " + label)
}

func rootMenuMiniModal(width int) string {
	body := strings.Join([]string{
		lipgloss.NewStyle().Foreground(rootMenuCyan).Bold(true).Render("弹层提示"),
		lipgloss.NewStyle().Foreground(rootMenuInk).Width(max(18, width-4)).Render("Enter 会退出控制台并执行命令；需要批量规划时先用 Space 标记多个候选。"),
	}, "\n")
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.DoubleBorder()).
		BorderForeground(rootMenuCyan).
		Padding(0, 1).
		Render(body)
}

func rootMenuHelpModal(width int) string {
	bodyW := min(58, max(38, width-12))
	rows := []string{
		lipgloss.NewStyle().Foreground(rootMenuBase).Background(rootMenuCyan).Bold(true).Padding(0, 1).Render("Keyboard"),
		"",
		"↑/↓ 或 j/k     移动选择",
		"←/→ 或 h/l     切换顶部 tabs",
		"Space          标记/取消标记候选",
		"Enter          执行当前命令",
		"Esc            返回上层或关闭弹窗",
		"q / Ctrl+C     退出",
	}
	return lipgloss.NewStyle().
		Width(bodyW).
		Border(lipgloss.DoubleBorder()).
		BorderForeground(rootMenuCyan).
		Background(rootMenuSurface).
		Padding(1, 2).
		Render(strings.Join(rows, "\n"))
}

func rootMenuOverlay(base, modal string, width int) string {
	lines := strings.Split(base, "\n")
	modalLines := strings.Split(modal, "\n")
	if len(lines) < len(modalLines)+2 {
		return modal
	}
	top := min(max(1, len(lines)/3), len(lines)-len(modalLines))
	left := max(0, (width-lipgloss.Width(modalLines[0]))/2)
	for i, ml := range modalLines {
		idx := top + i
		back := lipgloss.NewStyle().
			Foreground(rootMenuDim).
			Render(truncateDisplay(lines[idx], max(1, width)))
		lines[idx] = lipgloss.PlaceHorizontal(width, lipgloss.Left, strings.Repeat(" ", left)+ml)
		if strings.TrimSpace(lines[idx]) == "" {
			lines[idx] = back
		}
	}
	return strings.Join(lines, "\n")
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
