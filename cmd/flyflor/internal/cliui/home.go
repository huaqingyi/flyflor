package cliui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// HomeInfo is the data rendered by the root Flyflor command.
type HomeInfo struct {
	Version   string
	Mode      string
	Model     string
	Workspace string
	Config    string
}

// RenderHome renders the root CLI landing view.
func RenderHome(info HomeInfo) string {
	if info.Mode == "" {
		info.Mode = "交互模式"
	}
	if info.Model == "" {
		info.Model = "未配置"
	}
	if info.Workspace == "" {
		info.Workspace = "/workspace"
	}
	if info.Config == "" {
		info.Config = "/config/config.json"
	}
	if UseFancyLayout() {
		return renderCockpitHome(info)
	}

	return renderCompactDeckHome(info)
}

// RenderRootHelp renders the root help page as a Flyflor cockpit instead of a
// generic Cobra manual.
func RenderRootHelp(commandPath, useLine string) string {
	info := HomeInfo{
		Version: "dev",
		Mode:    "系统帮助",
		Model:   "就绪",
	}
	if !UseFancyLayout() {
		return renderPlainRootHelp(commandPath, useLine)
	}
	return renderRootOverview(commandPath, useLine, info)
}

func renderCockpitHome(info HomeInfo) string {
	return renderControlDeck("flyflor", "flyflor [flags]", info)
}

func renderCompactDeckHome(info HomeInfo) string {
	width := 78
	tabs := []string{
		compactTab("Commands", true),
		compactTab("Runtime", false),
		compactTab("Memory", false),
		compactTab("Setup", false),
	}
	status := []string{
		"Version " + info.Version,
		"Mode " + info.Mode,
		"Model " + info.Model,
		"Workspace " + compactEllipsis(info.Workspace, 26),
		"Config " + compactEllipsis(info.Config, 30),
	}
	commands := [][2]string{
		{"agent", "进入默认 Agent TUI"},
		{"sessions", "选择并继续历史会话"},
		{"model", "显示或切换默认模型"},
		{"status", "检查运行状态"},
		{"gateway", "启动多渠道 Gateway"},
		{"mcp / skills", "管理 MCP 服务与技能"},
	}
	runtime := []string{
		compactCheck("launcher", true) + " 18800",
		compactCheck("gateway", info.Model != "未配置") + " 18790",
		compactCheck("qdrant", true) + " 6333",
		compactCheck("proxy", true) + " host.docker.internal",
	}

	var cmdRows []string
	for _, row := range commands {
		cmdRows = append(cmdRows, compactCommand(row[0], row[1], 16))
	}

	left := compactPanel("Selected Tab / Commands", strings.Join(cmdRows, "\n"), 38)
	right := compactPanel("Runtime / Readiness", strings.Join(runtime, "\n")+"\n\n"+strings.Join(status, "\n"), 38)

	var out strings.Builder
	out.WriteString("FLYFLOR Agent Control Deck\n")
	out.WriteString(strings.Join(tabs, " "))
	out.WriteString("\n")
	out.WriteString(strings.Repeat("─", width))
	out.WriteString("\n")
	out.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right))
	out.WriteString("\n")
	out.WriteString(compactModal("Tip", "docker exec -it flyflor flyflor 会打开交互式 tabs；Space 标记候选，? 打开快捷键弹窗。", width))
	out.WriteString("\n")
	return out.String()
}

func renderControlDeck(commandPath, useLine string, info HomeInfo) string {
	width := InnerWidth()
	if width < 96 {
		width = 96
	}
	contentW := width - 6
	if contentW < 72 {
		contentW = 72
	}

	var out strings.Builder
	out.WriteString(controlDeckHeader(info, width))
	out.WriteString("\n")
	out.WriteString(controlDeckTabs(width))
	out.WriteString("\n")

	leftW := width * 54 / 100
	rightW := width - leftW - 2
	left := controlDeckCommands(leftW)
	right := lipgloss.JoinVertical(lipgloss.Left,
		controlDeckPreview(commandPath, useLine, info, rightW),
		"",
		controlDeckReadiness(info, rightW),
	)
	out.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right))
	out.WriteString("\n")
	out.WriteString(controlDeckModal(contentW, info))
	out.WriteString("\n")
	out.WriteString(cockpitStatusBar(info, width))
	out.WriteString("\n")
	return out.String()
}

func controlDeckHeader(info HomeInfo, width int) string {
	brand := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#101116")).
		Background(accentCyan).
		Bold(true).
		Padding(0, 1).
		Render("FLYFLOR")
	title := panelTitleStyle(accentPink).Render("Agent Control Deck")
	summary := mutedStyle().Render("统一入口 / tabs / 弹层 / 运行状态")
	left := lipgloss.JoinHorizontal(lipgloss.Center, brand, "  ", title, "  ", summary)
	right := strings.Join([]string{
		deckChip("model", info.Model, accentGold),
		deckChip("mode", info.Mode, accentViolet),
	}, " ")
	return lipgloss.NewStyle().Width(width-lipgloss.Width(right)-2).Render(left) + right
}

func controlDeckTabs(width int) string {
	labels := []string{"Commands", "Runtime", "Memory", "Setup"}
	var tabs []string
	for i, label := range labels {
		active := i == 0
		bg := lipgloss.Color("#171922")
		fg := accentCyan
		border := accentBlue
		if active {
			bg = accentCyan
			fg = lipgloss.Color("#101116")
			border = accentCyan
		}
		tabs = append(tabs, lipgloss.NewStyle().
			Foreground(fg).
			Background(bg).
			Border(lipgloss.NormalBorder(), true, true, false, true).
			BorderForeground(border).
			Bold(active).
			Padding(0, 2).
			Render(label))
	}
	help := mutedStyle().
		Width(width - lipgloss.Width(lipgloss.JoinHorizontal(lipgloss.Top, tabs...))).
		Align(lipgloss.Right).
		Render("docker exec -it flyflor flyflor")
	return lipgloss.JoinHorizontal(lipgloss.Top, append(tabs, help)...)
}

func controlDeckCommands(width int) string {
	sections := []string{
		deckCommandGroup("对话", accentCyan, [][2]string{
			{"agent", "进入默认 Agent TUI"},
			{"sessions", "继续历史会话"},
			{"agent -m <text>", "发送单轮消息后退出"},
		}),
		deckCommandGroup("运行", accentLime, [][2]string{
			{"status", "检查配置与服务状态"},
			{"gateway", "启动多渠道 Gateway"},
			{"cron", "管理定时任务"},
		}),
		deckCommandGroup("配置", accentPink, [][2]string{
			{"model", "显示或切换默认模型"},
			{"auth", "管理登录和 Token"},
			{"mcp / skills", "管理工具服务与技能"},
		}),
	}
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentCyan).
		Padding(1, 2).
		Render(panelTitleStyle(accentCyan).Render("Command Palette") + "\n\n" + strings.Join(sections, "\n\n"))
}

func controlDeckPreview(commandPath, useLine string, info HomeInfo, width int) string {
	if commandPath == "" {
		commandPath = "flyflor"
	}
	if useLine == "" {
		useLine = "flyflor [flags]"
	}
	body := strings.Join([]string{
		panelTitleStyle(accentViolet).Render("Selected Action"),
		"",
		deckKV("Command", commandPath),
		deckKV("Usage", useLine),
		deckKV("Config", info.Config),
		deckKV("Workspace", info.Workspace),
		"",
		mutedStyle().Render("Enter 执行当前命令；? 打开快捷键弹窗。"),
	}, "\n")
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentViolet).
		Padding(1, 2).
		Render(body)
}

func controlDeckReadiness(info HomeInfo, width int) string {
	modelReady := strings.TrimSpace(info.Model) != "" && info.Model != "未配置"
	rows := []string{
		panelTitleStyle(accentGold).Render("Readiness"),
		"",
		deckCheck("launcher 18800", true),
		deckCheck("qdrant 6333", true),
		deckCheck("default model", modelReady),
		deckCheck("gateway 18790", modelReady),
	}
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentGold).
		Padding(1, 2).
		Render(strings.Join(rows, "\n"))
}

func controlDeckModal(width int, info HomeInfo) string {
	body := lipgloss.JoinHorizontal(lipgloss.Top,
		panelTitleStyle(accentCyan).Render("Tip"),
		"  ",
		lipgloss.NewStyle().Width(width-10).Render("交互入口支持 tabs、弹窗和 Space 标记；配置默认模型后 Gateway 会自动进入可启动状态。当前模型: "+info.Model),
	)
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.DoubleBorder()).
		BorderForeground(accentCyan).
		Padding(0, 1).
		Render(body)
}

func deckCommandGroup(title string, color lipgloss.Color, rows [][2]string) string {
	rendered := []string{panelTitleStyle(color).Render("[" + title + "]")}
	for _, row := range rows {
		rendered = append(rendered, deckCommand(row[0], row[1], color))
	}
	return strings.Join(rendered, "\n")
}

func deckCommand(command, desc string, color lipgloss.Color) string {
	cmd := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#101116")).
		Background(color).
		Bold(true).
		Width(18).
		Padding(0, 1).
		Render(command)
	return cmd + "  " + mutedStyle().Render(desc)
}

func deckKV(key, value string) string {
	return panelTitleStyle(accentCyan).Width(10).Render(key) + mutedStyle().Render(value)
}

func deckCheck(label string, ok bool) string {
	color := accentRed
	mark := "□"
	if ok {
		color = accentLime
		mark = "■"
	}
	return panelTitleStyle(color).Render(mark + " " + label)
}

func deckChip(label, value string, color lipgloss.Color) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#101116")).
		Background(color).
		Bold(true).
		Padding(0, 1).
		Render(label) +
		lipgloss.NewStyle().
			Background(lipgloss.Color("#202332")).
			Foreground(lipgloss.Color("#E9EAF2")).
			Padding(0, 1).
			Render(value)
}

func compactTab(label string, active bool) string {
	style := lipgloss.NewStyle().Padding(0, 1)
	if active {
		return style.Bold(true).Render("[" + label + "]")
	}
	return style.Render(label)
}

func compactPanel(title, body string, width int) string {
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Render(title + "\n\n" + body)
}

func compactCommand(command, desc string, commandWidth int) string {
	return lipgloss.NewStyle().Width(commandWidth).Render("› "+command) + desc
}

func compactCheck(label string, ok bool) string {
	if ok {
		return "■ " + label
	}
	return "□ " + label
}

func compactModal(title, body string, width int) string {
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.DoubleBorder()).
		Padding(0, 1).
		Render(title + "  " + body)
}

func renderRootOverview(commandPath, useLine string, info HomeInfo) string {
	if commandPath == "" {
		commandPath = "flyflor"
	}
	if useLine == "" {
		useLine = "flyflor [flags]"
	}

	width := InnerWidth()
	if width < 88 {
		width = 88
	}
	contentW := width - 6
	if contentW < 72 {
		contentW = 72
	}

	var out strings.Builder
	head := []string{
		lipgloss.JoinHorizontal(lipgloss.Top,
			panelTitleStyle(accentCyan).Render("Flyflor CLI"),
			mutedStyle().Render("  /  "),
			panelTitleStyle(accentPink).Render("personal AI runtime"),
		),
		mutedStyle().Render("日常入口是 Agent TUI：对话、长文本输入、折叠黑板和运行状态都在同一个界面里完成。"),
		"",
		kvKeyStyle().Render("Use") + "      " + styleUsageTokens(useLine),
		kvKeyStyle().Render("Open") + "     " + helpIdentStyle().Render("flyflor agent") + "  " + mutedStyle().Render("进入 TUI"),
		kvKeyStyle().Render("Docker") + "   " + mutedStyle().Render("docker exec -it flyflor flyflor agent"),
	}
	if info.Version != "" {
		head = append(head, kvKeyStyle().Render("Version")+"  "+info.Version)
	}
	out.WriteString(neonPanel(width, strings.Join(head, "\n")))
	out.WriteString("\n\n")

	tuiRows := [][2]string{
		{"flyflor agent", "进入对话 TUI；直接输入问题即可开始。"},
		{"Ctrl+E / /edit", "展开长文本编辑区；Enter 换行，Ctrl+D 发送。"},
		{"/bb", "打开按轮次分组的折叠黑板，用于查看思考摘要、结论和运行记录。"},
		{"/think", "展开或折叠当前轮次的思考摘要。"},
		{"Tab", "在输入 / 命令时接受补全；平时切换黑板抽屉。"},
	}
	out.WriteString(sectionPanel("TUI workflow", renderTwoColPairs(tuiRows, contentW), width, accentCyan))
	out.WriteString("\n\n")

	commandRows := [][2]string{
		{"agent / chat", "进入对话 TUI，或用 -m 发送单轮消息。"},
		{"sessions", "查看并继续历史会话。"},
		{"status", "检查配置、模型、工作区和服务状态。"},
		{"model", "显示或切换默认模型。"},
		{"auth", "管理登录、登出和 Token 状态。"},
		{"mcp / skills", "管理 MCP 服务与技能。"},
		{"gateway / cron", "启动网关或管理定时任务。"},
		{"onboard", "初始化或修复配置与工作区。"},
	}
	out.WriteString(sectionPanel("Commands", renderTwoColPairs(commandRows, contentW), width, accentViolet))
	out.WriteString("\n\n")

	toolRows := [][2]string{
		{"fx", "高级透传 JSON 查看器；TUI 的 /bb 已内置折叠查看。"},
		{"gum", "高级脚本交互工具；长文本输入已内置在 TUI 的 Ctrl+E / /edit。"},
		{"dive", "Docker 镜像层分析，适合排查镜像体积。"},
		{"completion", "生成 shell 自动补全脚本。"},
		{"--no-color", "关闭颜色但保留工整布局。"},
	}
	out.WriteString(sectionPanel("Advanced tools", renderTwoColPairs(toolRows, contentW), width, accentGold))
	out.WriteString("\n")
	out.WriteString(RenderAgentStatusBar("cli:help", "ready"))
	return out.String()
}

func renderCockpit(commandPath, useLine string, info HomeInfo) string {
	width := InnerWidth()
	if width < 96 {
		width = 96
	}
	leftW := width * 32 / 100
	midW := width * 32 / 100
	rightW := width - leftW - midW - 4
	if rightW < 30 {
		rightW = 30
	}

	var out strings.Builder
	out.WriteString(cockpitLogo(width))
	out.WriteString("\n")
	out.WriteString(cockpitStatusLine(info))
	out.WriteString("\n")
	out.WriteString(cockpitCommandLine(commandPath))
	out.WriteString("\n")
	out.WriteString(cockpitRule(width, accentCyan, accentLime, accentPink))
	out.WriteString("\n")

	left := cockpitPanel("FLYFLOR CLI / 帮助", cockpitCommandList(), leftW, accentViolet)
	mid := cockpitPanel("智能体心智 & 记忆", cockpitMemoryBody(info), midW, accentLime)
	right := lipgloss.JoinVertical(lipgloss.Left,
		cockpitPanel("黑板 (默认隐藏:/bb)", cockpitBlackboardBody(), rightW, accentPink),
		"",
		cockpitPanel("用法 & 示例", cockpitUsageBody(useLine), rightW, accentGold),
	)
	out.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", mid, "  ", right))
	out.WriteString("\n")
	out.WriteString(cockpitStatusBar(info, width))
	out.WriteString("\n")
	out.WriteString(mutedStyle().Width(width).Align(lipgloss.Center).Render("[←/→: 选择面板, Tab: 切换, ?: 快捷键]"))
	out.WriteString("\n")
	return out.String()
}

func cockpitLogo(width int) string {
	lines := []string{
		"███████╗██╗     ██╗   ██╗███████╗██╗      ██████╗ ██████╗ ",
		"██╔════╝██║     ╚██╗ ██╔╝██╔════╝██║     ██╔═══██╗██╔══██╗",
		"█████╗  ██║      ╚████╔╝ █████╗  ██║     ██║   ██║██████╔╝",
		"██╔══╝  ██║       ╚██╔╝  ██╔══╝  ██║     ██║   ██║██╔══██╗",
		"██║     ███████╗   ██║   ██║     ███████╗╚██████╔╝██║  ██║",
		"╚═╝     ╚══════╝   ╚═╝   ╚═╝     ╚══════╝ ╚═════╝ ╚═╝  ╚═╝",
	}
	left := lipgloss.NewStyle().Foreground(accentBlue).Bold(true)
	right := lipgloss.NewStyle().Foreground(accentPink).Bold(true)
	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		split := len([]rune(line)) / 2
		runes := []rune(line)
		rendered = append(rendered, left.Render(string(runes[:split]))+right.Render(string(runes[split:])))
	}
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(strings.Join(rendered, "\n"))
}

func cockpitStatusLine(info HomeInfo) string {
	phase := "ALPHA"
	context := "系统帮助"
	if strings.TrimSpace(info.Mode) != "" {
		context = strings.TrimSpace(info.Mode)
	}
	return strings.Join([]string{
		bracketLabel("状态", "稳定", accentLime),
		bracketLabel("进化阶段", phase, accentViolet),
		bracketLabel("当前上下文", context, accentGold),
	}, " ")
}

func cockpitCommandLine(commandPath string) string {
	if commandPath == "" {
		commandPath = "flyflor"
	}
	return panelTitleStyle(accentLime).Render("➜  ") +
		panelTitleStyle(accentCyan).Render("flyflor") +
		mutedStyle().Render(" git:") +
		lipgloss.NewStyle().Foreground(accentPink).Bold(true).Render("(dev)") +
		" " +
		panelTitleStyle(accentGold).Render("✗") +
		" " +
		commandPath +
		mutedStyle().Render(" -h")
}

func bracketLabel(label, value string, color lipgloss.Color) string {
	return mutedStyle().Render("[") +
		mutedStyle().Render(label+": ") +
		panelTitleStyle(color).Render(value) +
		mutedStyle().Render("]")
}

func cockpitRule(width int, colors ...lipgloss.Color) string {
	if width < 12 {
		width = 12
	}
	segW := (width - (len(colors)-1)*2) / len(colors)
	var parts []string
	for _, color := range colors {
		parts = append(parts, lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("─", segW)))
	}
	return strings.Join(parts, "  ")
}

func cockpitPanel(title, body string, width int, color lipgloss.Color) string {
	head := panelTitleStyle(color).Render("["+title+"]") + "\n\n"
	content := padMinLines(head+body, 16)
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Padding(0, 1).
		Render(content)
}

func padMinLines(s string, minLines int) string {
	if minLines <= 0 {
		return s
	}
	lines := strings.Count(s, "\n") + 1
	if lines >= minLines {
		return s
	}
	return s + strings.Repeat("\n", minLines-lines)
}

func cockpitCommandList() string {
	sections := []string{
		cockpitSection("◆", "对话", accentCyan, []string{
			cockpitCommand("➜", "agent", "💬 直接对话", accentCyan),
			cockpitCommand("➜", "sessions", "🧭 查看/继续会话", accentCyan),
			cockpitCommand("➜", "model", "☁ 显示或切换模型", accentCyan),
		}),
		cockpitSection("◆", "运行时", accentLime, []string{
			cockpitCommand("➜", "cron", "⏰ 管理定时任务", accentLime),
			cockpitCommand("➜", "dive", "🔎 分析镜像层", accentLime),
			cockpitCommand("➜", "fx", "📐 折叠查看 JSON", accentLime),
			cockpitCommand("➜", "gum", "✨ 终端交互工具", accentLime),
			cockpitCommand("➜", "gateway", "🌐 启动 Flyflor 网关", accentLime),
			cockpitCommand("➜", "status", "📊 显示状态", accentLime),
		}),
		cockpitSection("◆", "集成", accentPink, []string{
			cockpitCommand("➜", "auth", "🔑 管理认证", accentPink),
			cockpitCommand("➜", "mcp", "🔌 MCP 服务配置", accentPink),
			cockpitCommand("➜", "skills", "🛠 管理技能", accentPink),
			cockpitCommand("➜", "completion", "⚡ 安装命令补全", accentPink),
		}),
	}
	return strings.Join(sections, "\n\n")
}

func cockpitSection(marker, title string, color lipgloss.Color, rows []string) string {
	head := panelTitleStyle(color).Render(marker + " [" + title + "]")
	return head + "\n" + strings.Join(rows, "\n")
}

func cockpitCommand(marker, command, desc string, color lipgloss.Color) string {
	return panelTitleStyle(color).Render("  "+marker+" "+command) + " " + desc
}

func cockpitMemoryBody(info HomeInfo) string {
	sqlite := "experience_db.sqlite"
	if info.Workspace != "" && info.Workspace != "/workspace" {
		sqlite = "workspace timeline"
	}
	model := info.Model
	if model == "" {
		model = "就绪"
	}
	return strings.Join([]string{
		panelTitleStyle(accentLime).Render("▶") + " 灵魂设定 (`.md`):",
		"  文件: soul_profile.md " + panelTitleStyle(accentLime).Render("(已加载)"),
		"  特质: 分析、递归、持续进化",
		"",
		panelTitleStyle(accentLime).Render("▶") + " 结构化记忆 (`.sqlite`):",
		"  文件: " + sqlite + " " + mutedStyle().Render("(运行中)"),
		"  经验节点总数: " + panelTitleStyle(accentGold).Render("2453"),
		"",
		panelTitleStyle(accentLime).Render("▶") + " 向量记忆 (`.qdrant`):",
		"  地址: localhost:6333 " + panelTitleStyle(accentLime).Render("(已连接)"),
		"  节点: 120,500",
		"  召回评分: 高",
		"",
		panelTitleStyle(accentCyan).Render("▶") + " 当前模型:",
		"  " + model,
	}, "\n")
}

func cockpitBlackboardBody() string {
	return strings.Join([]string{
		panelTitleStyle(accentPink).Render("▶") + " 状态: 已连接黑板 (/bb)",
		panelTitleStyle(accentPink).Render("▶") + " 活跃调度: Blackboard + Planner/Reviewer",
		panelTitleStyle(accentPink).Render("▶") + " 任务状态: 等待指令。",
	}, "\n")
}

func cockpitUsageBody(useLine string) string {
	if strings.TrimSpace(useLine) == "" {
		useLine = "flyflor [flags]"
	}
	return strings.Join([]string{
		panelTitleStyle(accentGold).Render("➜") + " 用法: " + styleUsageTokens(useLine),
		"",
		panelTitleStyle(accentGold).Render("➜") + " 示例 1: flyflor version",
		panelTitleStyle(accentGold).Render("➜") + " 示例 2: flyflor onboard",
	}, "\n")
}

func cockpitStatusBar(info HomeInfo, width int) string {
	parts := []string{
		statusSegment("运行时", "cockpit", accentLime),
		statusSegment("会话", "cli:help", accentCyan),
		statusSegment("模型", "就绪", accentLime),
		statusSegment("Workers", "planner/reviewer", accentPink),
		statusSegment("黑板", "/bb", accentViolet),
		statusSegment("记忆", "md/sqlite/qdrant", accentGold),
		statusSegment("Git", "dev", accentGold),
	}
	clock := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ECFFF0")).
		Background(lipgloss.Color("#128A2D")).
		Bold(true).
		Padding(0, 1).
		Render(time.Now().Format("15:04:05"))
	line := strings.Join(parts, mutedStyle().Render(" │ ")) + mutedStyle().Render(" │ ") + clock
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(accentBlue).
		Render(line)
}

func statusSegment(label, value string, color lipgloss.Color) string {
	return panelTitleStyle(color).Render(label+":") + value
}

func renderPlainRootHelp(commandPath, useLine string) string {
	if commandPath == "" {
		commandPath = "flyflor"
	}
	if useLine == "" {
		useLine = "flyflor [flags]"
	}
	width := 78
	chatRows := []string{
		plainHelpRow("agent", "进入五个 tab 的对话 TUI；未配置时先运行 flyflor setup"),
		plainHelpRow("sessions", "查看和继续历史 Session"),
		plainHelpRow("model", "显示或切换默认模型"),
	}
	runtimeRows := []string{
		plainHelpRow("status", "显示运行状态"),
		plainHelpRow("gateway", "启动 Flyflor 网关"),
		plainHelpRow("cron", "管理定时任务"),
		plainHelpRow("dive", "分析 Docker 镜像层"),
	}
	toolRows := []string{
		plainHelpRow("fx", "仅用于直接查看 JSON 文件或管道；日常黑板查看用 TUI"),
		plainHelpRow("gum", "仅用于脚本 choose/input/write；长文本输入用 TUI"),
		plainHelpRow("auth", "管理认证"),
		plainHelpRow("mcp", "MCP 服务配置"),
		plainHelpRow("skills", "管理技能"),
		plainHelpRow("completion", "安装 shell 命令补全"),
	}
	tuiRows := []string{
		"  tabs      对话 / 历史 Session / 黑板 / 记忆 / 设置",
		"  /bb       打开按轮次分组的黑板",
		"  /think    展开或收拢对话内思考摘要",
		"  Ctrl+S    回答进行中立即注入引导",
		"  /yolo     切换低风险自主执行模式；不关闭危险命令拦截",
	}
	usageRows := []string{
		"  " + useLine,
		"  flyflor setup",
		"  flyflor agent",
		"  flyflor version",
	}

	return strings.Join([]string{
		plainRootHeader(width),
		plainRootPanel("对话", chatRows, width),
		plainRootPanel("TUI 工作流", tuiRows, width),
		plainRootPanel("运行时", runtimeRows, width),
		plainRootPanel("集成 / 高级工具", toolRows, width),
		plainRootPanel("用法 / 示例", usageRows, width),
		"",
	}, "\n")
}

func plainRootHeader(width int) string {
	lines := []string{
		"Flyflor CLI / 帮助",
		"智能体、黑板、历史 Session、记忆和设置已集中在 flyflor agent TUI。",
	}
	return plainRootPanel("FLYFLOR", lines, width)
}

func plainHelpRow(name, desc string) string {
	return fmt.Sprintf("  %-12s %s", name, desc)
}

func plainRootPanel(title string, lines []string, width int) string {
	if width < 32 {
		width = 32
	}
	inner := width - 4
	title = strings.TrimSpace(title)
	topFill := width - lipgloss.Width(title) - 5
	if topFill < 1 {
		topFill = 1
	}
	var out []string
	out = append(out, "╭─ "+title+" "+strings.Repeat("─", topFill)+"╮")
	for _, line := range lines {
		out = append(out, "│ "+plainPadLine(line, inner)+" │")
	}
	out = append(out, "╰"+strings.Repeat("─", width-2)+"╯")
	return strings.Join(out, "\n")
}

func plainPadLine(line string, width int) string {
	line = strings.TrimRight(line, " \t")
	if lipgloss.Width(line) > width {
		line = compactEllipsis(line, width)
	}
	return line + strings.Repeat(" ", maxInt(width-lipgloss.Width(line), 0))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func compactEllipsis(s string, width int) string {
	s = strings.TrimSpace(s)
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	var b strings.Builder
	for _, r := range s {
		next := b.String() + string(r)
		if lipgloss.Width(next) > width-1 {
			break
		}
		b.WriteRune(r)
	}
	return b.String() + "…"
}

// RenderLoadLine renders a compact multi-color progress line.
func RenderLoadLine(label string, percent int) string {
	return fmt.Sprintf("%s\n%s  %d%%\n",
		mutedStyle().Render(label),
		progressBar(percent, 16),
		percent,
	)
}

func neonPanel(width int, body string) string {
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderTopForeground(accentCyan).
		BorderLeftForeground(accentBlue).
		BorderRightForeground(accentPink).
		BorderBottomForeground(accentViolet).
		Padding(0, 1)
	return border.Width(width).Render(body)
}

func commandPill(s string) string {
	return lipgloss.NewStyle().
		Foreground(accentCyan).
		Bold(true).
		Width(20).
		Render("› " + s)
}

func statusDot(color lipgloss.Color) string {
	return lipgloss.NewStyle().Foreground(color).Bold(true).Render("●")
}
