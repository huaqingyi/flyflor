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

	identity := []string{
		lipgloss.JoinHorizontal(lipgloss.Top,
			panelTitleStyle(accentCyan).Render("Flyflor"),
			mutedStyle().Render("  /  "),
			panelTitleStyle(accentPink).Render("智能体运行时"),
		),
		mutedStyle().Render("面向本地多智能体协作的 AI Runtime Cockpit"),
		"",
		kvKeyStyle().Render("✦ 版本") + "  " + info.Version,
		kvKeyStyle().Render("◈ 模式") + "  " + info.Mode,
		kvKeyStyle().Render("◉ 模型") + "  " + info.Model,
		kvKeyStyle().Render("⚙ Workspace") + " " + mutedStyle().Render(info.Workspace),
		kvKeyStyle().Render("⛁ Config") + "    " + mutedStyle().Render(info.Config),
	}

	commands := [][]string{
		{"chat", "进入对话模式"},
		{"agent -m <text>", "发送单轮消息后退出"},
		{"sessions", "查看可继续的会话"},
		{"gateway", "启动 Flyflor 网关"},
		{"status", "查看运行状态"},
		{"model", "显示或切换默认模型"},
		{"mcp", "管理 MCP 服务"},
		{"skills", "管理技能"},
	}

	var cmdRows []string
	for _, row := range commands {
		cmdRows = append(cmdRows,
			commandPill(row[0])+
				"  "+
				mutedStyle().Render(row[1]),
		)
	}

	memoryRows := []string{
		statusDot(accentCyan) + " 身份设定        " + mutedStyle().Render("Markdown"),
		statusDot(accentBlue) + " 每日笔记        " + mutedStyle().Render("YYYYMMDD.md"),
		statusDot(accentGold) + " 时间线记忆      " + mutedStyle().Render("SQLite"),
		statusDot(accentViolet) + " 向量记忆        " + mutedStyle().Render("Qdrant"),
		statusDot(accentLime) + " 配置目录        " + mutedStyle().Render("./config"),
		statusDot(accentOrange) + " 工作区数据      " + mutedStyle().Render("./data/workspace"),
		statusDot(accentPink) + " 运行产物        " + mutedStyle().Render("./runs"),
	}

	var out strings.Builder
	if UseFancyLayout() {
		w := InnerWidth()
		out.WriteString(neonPanel(w, strings.Join(identity, "\n")))
		out.WriteString("\n\n")
		if UseColumnLayout() {
			leftW := (w - 2) / 2
			rightW := w - leftW - 2
			left := borderStyle().Width(leftW).Render(
				panelTitleStyle(accentViolet).Render("Commands") + "\n\n" + strings.Join(cmdRows, "\n"),
			)
			right := borderStyle().Width(rightW).Render(
				panelTitleStyle(accentGold).Render("Memory / persistence") + "\n\n" + strings.Join(memoryRows, "\n"),
			)
			out.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right))
		} else {
			out.WriteString(borderStyle().Width(w).Render(
				panelTitleStyle(accentViolet).Render("Commands") + "\n\n" + strings.Join(cmdRows, "\n"),
			))
			out.WriteString("\n\n")
			out.WriteString(borderStyle().Width(w).Render(
				panelTitleStyle(accentGold).Render("Memory / persistence") + "\n\n" + strings.Join(memoryRows, "\n"),
			))
		}
		out.WriteString("\n\n")
		out.WriteString(RenderLoadLine("runtime containers and tools", 100))
		return out.String()
	}

	out.WriteString(strings.Join(identity, "\n"))
	out.WriteString("\n\n命令\n")
	out.WriteString(strings.Join(cmdRows, "\n"))
	out.WriteString("\n\n记忆 / 持久化\n")
	out.WriteString(strings.Join(memoryRows, "\n"))
	out.WriteString("\n\n")
	out.WriteString(RenderLoadLine("runtime containers and tools", 100))
	return out.String()
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
	return renderCockpit(commandPath, useLine, info)
}

func renderCockpitHome(info HomeInfo) string {
	return renderCockpit("flyflor", "flyflor [flags]", info)
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
		panelTitleStyle(accentPink).Render("▶") + " 活跃桥接: Codex + Claude/OpenCode",
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
		statusSegment("Bridge", "codex/claude", accentPink),
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
	return strings.Join([]string{
		"Flyflor CLI / 帮助",
		"",
		"对话:",
		"  agent    直接对话",
		"  sessions 查看/继续会话",
		"  model    显示或切换模型",
		"",
		"运行时:",
		"  cron    管理定时任务",
		"  dive    分析 Docker 镜像层",
		"  fx      折叠查看 JSON",
		"  gum     终端交互工具",
		"  gateway 启动 Flyflor 网关",
		"  status  显示状态",
		"",
		"集成:",
		"  auth       管理认证",
		"  mcp        MCP 服务配置",
		"  skills     管理技能",
		"  completion 安装 shell 命令补全",
		"",
		"黑板: 默认隐藏，在对话中用 /bb 打开。",
		"用法: " + useLine,
		"示例: flyflor version · flyflor onboard",
		"",
	}, "\n")
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
