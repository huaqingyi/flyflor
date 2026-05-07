package cliui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// AgentProgress renders compact, Copilot-like turn progress for the terminal.
type AgentProgress struct {
	out         io.Writer
	mu          sync.Mutex
	started     time.Time
	lines       int
	step        int
	stage       int
	section     string
	closed      bool
	toolsHeader bool
}

// BlackboardTurn is the compact terminal representation of one user turn.
// The WebUI owns the clickable blackboard timeline; the CLI keeps it hidden
// until the user asks for a specific turn.
type BlackboardTurn struct {
	Index     int
	User      string
	Assistant string
	StartedAt time.Time
}

// SessionRow is the compact CLI representation of one persistent conversation.
type SessionRow struct {
	Key      string
	Messages int
	Summary  string
	LastRole string
	LastText string
}

// NewAgentProgress creates a progress renderer for one agent turn.
func NewAgentProgress(out io.Writer) *AgentProgress {
	if out == nil {
		out = io.Discard
	}
	return &AgentProgress{out: out}
}

// Start prints the initial thinking line.
func (p *AgentProgress) Start(label string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.started = time.Now()
	p.step = 1
	p.writeSection("思考", accentCyan)
	p.writeProgress("receive", label, accentCyan, 12)
}

// Step prints an intermediate progress line.
func (p *AgentProgress) Step(label string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.step++
	p.writeProgress("step", label, accentBlue, p.percent())
}

// Warn prints a recoverable progress warning.
func (p *AgentProgress) Warn(label string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.writeProgress("notice", label, accentGold, p.percent())
}

// Error prints a failed progress line and closes the renderer.
func (p *AgentProgress) Error(label string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.writeSection("错误", accentRed)
	p.writeProgress("failed", label, accentRed, 100)
	p.closed = true
}

// Done prints the terminal progress line.
func (p *AgentProgress) Done(label string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	if label == "" {
		label = "completed"
	}
	p.writeSection("完成", colorOK)
	p.writeProgress("complete", label, colorOK, 100)
	p.closed = true
}

// ToolStart prints a tool execution line.
func (p *AgentProgress) ToolStart(tool string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	if !p.toolsHeader {
		p.writeSection("工具", accentViolet)
		p.toolsHeader = true
	}
	if tool == "" {
		tool = "tool"
	}
	p.writeProgress("running", tool, accentViolet, p.percent())
}

// ToolDone prints a completed tool execution line.
func (p *AgentProgress) ToolDone(tool, detail string, failed bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	if !p.toolsHeader {
		p.writeSection("工具", accentViolet)
		p.toolsHeader = true
	}
	if tool == "" {
		tool = "tool"
	}
	label := tool
	if detail != "" {
		label += " · " + detail
	}
	if failed {
		p.writeProgress("warn", label, accentGold, p.percent())
		return
	}
	p.writeProgress("done", label, colorOK, p.percent())
}

func (p *AgentProgress) percent() int {
	if p.step < 1 {
		return 12
	}
	if p.step > 4 {
		return 88
	}
	return 12 + p.step*18
}

func (p *AgentProgress) writeSection(title string, color lipgloss.Color) {
	p.stage++
	p.section = title
}

func (p *AgentProgress) writeProgress(kind, label string, color lipgloss.Color, percent int) {
	if strings.TrimSpace(label) == "" {
		return
	}
	if p.started.IsZero() {
		p.started = time.Now()
	}
	elapsed := time.Since(p.started).Round(100 * time.Millisecond)
	section := p.section
	if section == "" {
		section = "思考"
	}
	prefix := lipgloss.NewStyle().Foreground(color).Bold(true).Render(progressGlyph(kind) + " " + section)
	bar := progressBar(percent, 12)
	line := fmt.Sprintf("%s %s %3d%%  %s", prefix, bar, percent, label)
	if elapsed > 0 {
		line += " " + mutedStyle().Render("("+elapsed.String()+")")
	}
	line = truncateRunes(line, progressLineWidth())
	fmt.Fprint(p.out, "\r\x1b[2K"+line)
	if kind == "complete" || kind == "failed" {
		fmt.Fprint(p.out, "\n")
	}
	p.lines++
}

func progressGlyph(kind string) string {
	switch kind {
	case "complete", "done":
		return "✓"
	case "failed":
		return "✕"
	case "warn", "notice":
		return "!"
	case "running":
		return "◆"
	default:
		return "●"
	}
}

// RenderAgentIntro renders the interactive-mode header.
func RenderAgentIntro(model, session string) string {
	title := lipgloss.JoinHorizontal(lipgloss.Top,
		panelTitleStyle(accentCyan).Render("Flyflor"),
		mutedStyle().Render("  /  "),
		panelTitleStyle(accentViolet).Render("智能体运行台"),
	)
	var runtime []string
	if model != "" {
		runtime = append(runtime, kvKeyStyle().Render("模型")+"     "+model)
	}
	if session != "" {
		runtime = append(runtime, kvKeyStyle().Render("会话")+"     "+mutedStyle().Render(shortenMiddle(session, 18)))
	}
	runtime = append(runtime,
		kvKeyStyle().Render("运行时")+"   "+mutedStyle().Render("对话 + 黑板 + 记忆"),
		kvKeyStyle().Render("流程")+"     "+progressBar(38, 10)+" "+mutedStyle().Render("bridge 就绪"),
	)

	graph := []string{
		panelTitleStyle(accentViolet).Render("Flyflor") + mutedStyle().Render(" 分发"),
		mutedStyle().Render("  │"),
		panelTitleStyle(accentCyan).Render("Codex Bridge") + mutedStyle().Render(" 拆解/执行") + mutedStyle().Render("  ⇄  ") + panelTitleStyle(accentPink).Render("Copilot Guard") + mutedStyle().Render(" 复核"),
	}

	memory := []string{
		panelTitleStyle(accentGold).Render("记忆检查器"),
		mutedStyle().Render("markdown    身份/规则"),
		mutedStyle().Render("sqlite      会话/事件"),
		mutedStyle().Render("qdrant      语义召回"),
	}

	body := title + "\n\n"
	if UseColumnLayout() {
		colWidth := (InnerWidth() - 8) / 3
		body += lipgloss.JoinHorizontal(lipgloss.Top,
			runtimeIntroBlock("运行时", runtime, accentCyan, colWidth),
			"  ",
			runtimeIntroBlock("智能体图", graph, accentViolet, colWidth),
			"  ",
			runtimeIntroBlock("记忆", memory, accentGold, colWidth),
		)
	} else {
		body += strings.Join(append(runtime, append(graph, memory...)...), "\n")
	}
	body += "\n\n" + mutedStyle().Render("Enter 发送 · /bb 打开隐藏黑板 tabs · /help 命令 · sessions 查看会话 · exit/quit 离开 · Ctrl+C 停止")
	if UseFancyLayout() {
		return runtimeCockpitPanel(InnerWidth(), body) + "\n"
	}
	return body + "\n"
}

// RenderUserMessage renders a user turn in transcript form.
func RenderUserMessage(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	return renderTranscriptPanel("你", content, accentBlue, conversationPanelWidth()) + "\n"
}

// RenderAssistantMessage renders a final assistant answer.
func RenderAssistantMessage(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	return renderTranscriptPanel("Flyflor", content, accentPink, conversationPanelWidth()) + "\n"
}

// RenderAgentError renders an in-chat error without dumping Cobra usage.
func RenderAgentError(err error) string {
	if err == nil {
		return ""
	}
	head := lipgloss.NewStyle().Foreground(accentRed).Bold(true).Render("错误")
	body := subtleBorderStyle().Render(err.Error())
	return head + "\n" + body + "\n"
}

// RenderAgentNotice renders a low-volume chat notice.
func RenderAgentNotice(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	head := lipgloss.NewStyle().Foreground(accentGold).Bold(true).Render("提示")
	body := subtleBorderStyle().Render(content)
	return head + "\n" + body + "\n"
}

// Prompt returns the styled readline prompt.
func Prompt() string {
	return helpIdentStyle().Render("flyflor") + mutedStyle().Render(" › ")
}

// RenderAgentStatusBar renders the compact bottom status strip for chat turns.
func RenderAgentStatusBar(session, model string) string {
	parts := []string{
		panelTitleStyle(accentCyan).Render("运行台"),
		panelTitleStyle(accentViolet).Render("会话") + " " + mutedStyle().Render(shortenMiddle(session, 14)),
		panelTitleStyle(accentCyan).Render("模型") + " " + model,
		panelTitleStyle(accentPink).Render("互检") + " codex/copilot",
		panelTitleStyle(accentViolet).Render("黑板") + " 默认隐藏:/bb",
		panelTitleStyle(accentGold).Render("记忆") + " md+sqlite+qdrant",
		time.Now().Format("15:04:05"),
	}
	line := strings.Join(parts, mutedStyle().Render("  │  "))
	if UseFancyLayout() {
		return borderStyle().Width(InnerWidth()).Render(line) + "\n"
	}
	return line + "\n"
}

// RenderSessionList renders persistent sessions and the exact resume command.
func RenderSessionList(rows []SessionRow) string {
	width := InnerWidth()
	title := lipgloss.JoinHorizontal(lipgloss.Center,
		panelTitleStyle(accentCyan).Render("Flyflor 会话"),
		mutedStyle().Render(" / 继续同一个上下文"),
	)
	if len(rows) == 0 {
		body := title + "\n\n" +
			RenderAgentNotice("还没有可继续的 CLI session。先运行 flyflor agent 开始默认会话，或 flyflor agent -s <name> 创建命名会话。")
		return runtimeCockpitPanel(width, body) + "\n"
	}

	keyW := 30
	if width < 96 {
		keyW = 22
	}
	countW := 8
	lastW := width - keyW - countW - 12
	if lastW < 28 {
		lastW = 28
	}

	header := lipgloss.JoinHorizontal(lipgloss.Top,
		tuiLikeHeaderCell("会话", keyW, accentCyan),
		tuiLikeHeaderCell("消息", countW, accentViolet),
		tuiLikeHeaderCell("最近内容", lastW, accentGold),
	)
	var rendered []string
	rendered = append(rendered, header)
	for i, row := range rows {
		keyStyle := lipgloss.NewStyle().Foreground(accentCyan).Width(keyW)
		if i == 0 {
			keyStyle = keyStyle.Bold(true)
		}
		last := row.LastText
		if row.Summary != "" {
			last = row.Summary
		}
		if row.LastRole != "" && row.Summary == "" {
			last = row.LastRole + ": " + last
		}
		last = truncateDisplay(strings.ReplaceAll(last, "\n", " "), lastW-1)
		rendered = append(rendered, lipgloss.JoinHorizontal(lipgloss.Top,
			keyStyle.Render(shortenMiddle(row.Key, keyW-1)),
			lipgloss.NewStyle().Foreground(accentViolet).Width(countW).Render(fmt.Sprintf("%d", row.Messages)),
			lipgloss.NewStyle().Foreground(colorMuted).Render(last),
		))
	}

	latest := rows[0].Key
	body := title + "\n\n" +
		strings.Join(rendered, "\n") +
		"\n\n" +
		panelTitleStyle(accentGold).Render("继续方式") + "\n" +
		"  flyflor agent -s <Tab>  " + mutedStyle().Render("选择已有会话") + "\n" +
		"  flyflor agent -s " + latest + "\n" +
		mutedStyle().Render("提示: 列表为阅读会缩略显示；继续命令必须使用完整 key。agent --session/-s 已接入 Tab 自动补全。")
	return runtimeCockpitPanel(width, body) + "\n"
}

func tuiLikeHeaderCell(label string, width int, color lipgloss.Color) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#101116")).
		Background(color).
		Bold(true).
		Width(width).
		Padding(0, 1).
		Render(label)
}

func conversationPanelWidth() int {
	return InnerWidth()
}

func blackboardPanelWidth() int {
	return InnerWidth()
}

// RenderBlackboardTabs renders the hidden-by-default terminal blackboard when
// the user explicitly requests it with /bb or /blackboard.
func RenderBlackboardTabs(turns []BlackboardTurn, selected int) string {
	if len(turns) == 0 {
		return RenderAgentNotice("当前 CLI 会话还没有黑板记录")
	}
	if selected < 0 || selected >= len(turns) {
		selected = len(turns) - 1
	}
	width := blackboardPanelWidth()
	var tabs []string
	for i, turn := range turns {
		label := fmt.Sprintf("#%d", turn.Index)
		if strings.TrimSpace(turn.User) != "" {
			label += " " + truncateRunes(strings.ReplaceAll(turn.User, "\n", " "), 20)
		}
		style := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, true, false, true).
			BorderForeground(accentViolet).
			Padding(0, 1)
		if i != selected {
			style = style.
				Foreground(colorMuted).
				BorderForeground(colorMuted)
		} else {
			style = style.
				Foreground(accentViolet).
				Bold(true)
		}
		tabs = append(tabs, style.Render(label))
	}

	turn := turns[selected]
	head := lipgloss.JoinHorizontal(lipgloss.Top,
		panelTitleStyle(accentViolet).Render("黑板"),
		mutedStyle().Render(" / 默认隐藏的分组 tabs"),
	)
	meta := fmt.Sprintf("当前第 %d 轮", turn.Index)
	if !turn.StartedAt.IsZero() {
		meta += " · " + turn.StartedAt.Format("15:04:05")
	}
	rows := []string{
		blackboardCenter("Flyflor", "本轮问题 · "+truncateRunes(strings.TrimSpace(turn.User), 96), width),
		blackboardLeft("A", "Codex Bridge", "拆解任务边界、执行路径和需要验证的点。", width),
		blackboardRight("B", "Copilot Guard", "复核风险、遗漏和用户是否能读懂本轮结果。", width),
		blackboardCenter("Memory", "三层记忆 · md 身份约束 + SQLite 时间线 + Qdrant 语义索引。", width),
	}
	if strings.TrimSpace(turn.Assistant) != "" {
		rows = append(rows,
			blackboardCenter("Consensus", "最终回答 · "+truncateRunes(strings.TrimSpace(turn.Assistant), 120), width),
		)
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, tabs...) + "\n\n" +
		head + "\n" + mutedStyle().Render(meta+" · /bb <编号> 切换轮次 · 普通聊天默认隐藏完整黑板") + "\n\n" +
		strings.Join(rows, "\n")
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentViolet).
		Padding(0, 1).
		Render(body) + "\n"
}

func renderTranscriptPanel(title, content string, color lipgloss.Color, width int) string {
	head := lipgloss.JoinHorizontal(lipgloss.Top,
		panelTitleStyle(color).Render(title),
		mutedStyle().Render(" / dialogue"),
	)
	body := lipgloss.NewStyle().
		Width(width - 4).
		Render(content)
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Padding(0, 1).
		Render(head + "\n\n" + body)
}

func runtimeIntroBlock(title string, rows []string, color lipgloss.Color, width int) string {
	body := panelTitleStyle(color).Render(title) + "\n\n" + strings.Join(rows, "\n")
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(color).
		PaddingTop(1).
		Render(body)
}

func runtimeCockpitPanel(width int, body string) string {
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentCyan).
		Padding(0, 1)
	return border.Width(width).Render(body)
}

func renderBlackboardPanel(content string, consensus bool, width int) string {
	title := lipgloss.JoinHorizontal(lipgloss.Top,
		panelTitleStyle(accentCyan).Render("黑板"),
		mutedStyle().Render(" / 最新轮次"),
	)
	var rows []string
	if consensus {
		rows = append(rows,
			blackboardCenter("Flyflor", "本轮结论 · 最终回答已经写入自然对话区。", width),
			blackboardLeft("A", "Codex Bridge", "我确认交付内容已经覆盖本轮问题；如果有工具结果，它们应能支撑最终回答。", width),
			blackboardRight("B", "Copilot Guard", "我完成复核：黑板保留本轮关键判断，而不是只显示一句“达成共识”。", width),
			blackboardCenter("记忆", "可回写 · 若本轮形成稳定偏好或经验，进入 SQLite/Qdrant 记忆队列。", width),
		)
	} else {
		task := content
		if len([]rune(task)) > 96 {
			runes := []rune(task)
			task = string(runes[:96]) + "..."
		}
		rows = append(rows,
			blackboardCenter("Flyflor", "本轮问题 · "+task, width),
			blackboardLeft("A", "Codex Bridge", "我先拆任务：明确用户想完成什么，再判断需要设计、改代码、跑工具还是直接回答。", width),
			blackboardRight("B", "Copilot Guard", "我负责复核：检查 Codex 是否答偏、是否遗漏限制、最终结果是否能被用户读懂。", width),
			blackboardCenter("记忆", "上下文 · 读取 Flyflor 身份、用户偏好、最近任务和可复用经验。", width),
			blackboardLeft("A", "Codex Bridge", "执行计划：先处理最影响体验的问题，再做配套调整，最后用测试或截图验证。", width),
			blackboardRight("B", "Copilot Guard", "阅读标准：每条黑板记录都说明“为什么出现、谁负责、下一步是什么”。", width),
			blackboardCenter("黑板", "分组 · 当前 TUI 展示最新提问回合；WebUI 可点击历史回合回看本轮黑板。", width),
		)
	}
	body := title + "\n\n" + strings.Join(rows, "\n")
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentCyan).
		Padding(0, 1).
		Render(body)
}

func blackboardLeft(avatar, name, text string, width int) string {
	label := lipgloss.NewStyle().
		Foreground(accentCyan).
		Bold(true).
		Render("[" + avatar + "] " + name)
	bubble := lipgloss.NewStyle().
		Width(width-12).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(accentCyan).
		PaddingLeft(1).
		Render(text)
	return label + "\n" + bubble
}

func blackboardRight(avatar, name, text string, width int) string {
	label := lipgloss.NewStyle().
		Width(width - 4).
		Align(lipgloss.Right).
		Foreground(accentPink).
		Bold(true).
		Render(name + " [" + avatar + "]")
	bubble := lipgloss.NewStyle().
		Width(width-12).
		Align(lipgloss.Right).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(accentPink).
		PaddingRight(1).
		Render(text)
	return label + "\n" + lipgloss.NewStyle().Width(width-4).Align(lipgloss.Right).Render(bubble)
}

func blackboardCenter(name, text string, width int) string {
	msg := panelTitleStyle(accentGold).Render(name) + mutedStyle().Render(" · ") + text
	return lipgloss.NewStyle().
		Width(width - 4).
		Align(lipgloss.Center).
		Foreground(colorMuted).
		Render(msg)
}

func progressLineWidth() int {
	width := InnerWidth()
	if width < 60 {
		return width
	}
	return min(width, 118)
}

func truncateRunes(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

func shortenMiddle(s string, maxLen int) string {
	if maxLen < 5 || len(s) <= maxLen {
		return s
	}
	keep := (maxLen - 1) / 2
	tail := maxLen - keep - 1
	return s[:keep] + "…" + s[len(s)-tail:]
}

func truncateDisplay(s string, maxWidth int) string {
	if maxWidth <= 0 || lipgloss.Width(s) <= maxWidth {
		return s
	}
	suffix := "..."
	limit := maxWidth - lipgloss.Width(suffix)
	if limit <= 0 {
		return suffix
	}
	var out []rune
	for _, r := range s {
		next := string(append(out, r))
		if lipgloss.Width(next) > limit {
			break
		}
		out = append(out, r)
	}
	return string(out) + suffix
}
