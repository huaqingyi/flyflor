package agent

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/sipeed/picoclaw/cmd/flyflor/internal/ui"
	"github.com/sipeed/picoclaw/pkg/agent"
)

type turnRole int

const (
	roleUser turnRole = iota
	roleAssistant
	roleError
	roleSystem
)

type chatTurn struct {
	role    turnRole
	content string
	at      time.Time
}

type tuiModel struct {
	loop       *agent.AgentLoop
	sessionKey string
	model      string

	turns      []chatTurn
	viewport   viewport.Model
	textarea   textarea.Model
	spinner    spinner.Model
	renderer   *glamour.TermRenderer
	rendererW  int
	width      int
	height     int
	busy       bool
	statusText string
	err        error
	ready      bool
}

type assistantMsg struct {
	text string
}

type assistantErrMsg struct {
	err error
}

func newTUIModel(loop *agent.AgentLoop, sessionKey, model string) tuiModel {
	ta := textarea.New()
	ta.Placeholder = "向 flyflor 发送消息（Enter 发送 · Shift+Enter 换行 · Ctrl+C 退出）"
	ta.Prompt = "│ "
	ta.CharLimit = 0
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.Focus()
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(ui.ColorPrimary)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.Prompt = lipgloss.NewStyle().Foreground(ui.ColorMuted)
	ta.KeyMap.InsertNewline.SetEnabled(true)

	vp := viewport.New(0, 0)
	vp.SetContent("")

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ui.ColorAccent)

	return tuiModel{
		loop:       loop,
		sessionKey: sessionKey,
		model:      model,
		viewport:   vp,
		textarea:   ta,
		spinner:    sp,
		statusText: "ready",
	}
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick)
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		m.ready = true

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			if m.textarea.Value() == "" {
				return m, tea.Quit
			}
		case tea.KeyEnter:
			// Plain Enter sends; Alt+Enter / Shift+Enter inserts a newline.
			if !msg.Alt && msg.String() == "enter" && !m.busy {
				input := strings.TrimSpace(m.textarea.Value())
				if input != "" {
					m.textarea.Reset()
					return m, m.sendUserInput(input)
				}
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case assistantMsg:
		m.busy = false
		m.statusText = "ready"
		m.appendTurn(roleAssistant, msg.text)

	case assistantErrMsg:
		m.busy = false
		m.statusText = "error"
		m.err = msg.err
		m.appendTurn(roleError, msg.err.Error())
	}

	if !m.busy {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

	{
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *tuiModel) layout() {
	headerH := lipgloss.Height(m.renderHeader())
	footerH := lipgloss.Height(m.renderFooter())
	taH := m.textarea.Height() + 2
	vpH := m.height - headerH - footerH - taH - 2
	if vpH < 3 {
		vpH = 3
	}
	m.viewport.Width = m.width
	m.viewport.Height = vpH
	m.textarea.SetWidth(m.width - 2)

	if m.rendererW != m.width {
		r, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(max(20, m.width-4)),
		)
		if err == nil {
			m.renderer = r
			m.rendererW = m.width
			m.rerenderAll()
		}
	}
}

func (m *tuiModel) appendTurn(role turnRole, content string) {
	m.turns = append(m.turns, chatTurn{role: role, content: content, at: time.Now()})
	m.rerenderAll()
	m.viewport.GotoBottom()
}

func (m *tuiModel) rerenderAll() {
	var b strings.Builder
	for i, t := range m.turns {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(m.renderTurn(t))
	}
	m.viewport.SetContent(b.String())
}

func (m tuiModel) renderTurn(t chatTurn) string {
	switch t.role {
	case roleUser:
		head := ui.UserHeader.Render("● 你")
		body := ui.UserBubble.Width(m.width - 4).Render(t.content)
		return head + "\n" + body
	case roleAssistant:
		head := ui.AssistantHeader.Render("✦ flyflor")
		body := t.content
		if m.renderer != nil {
			if rendered, err := m.renderer.Render(t.content); err == nil {
				body = strings.TrimRight(rendered, "\n")
			}
		}
		return head + "\n" + body
	case roleError:
		return ui.Error.Render("⚠ 错误：") + t.content
	case roleSystem:
		return ui.Hint.Render("· " + t.content)
	}
	return t.content
}

func (m tuiModel) renderHeader() string {
	brand := ui.Brand.Render(" flyflor ")
	chips := lipgloss.JoinHorizontal(lipgloss.Left,
		ui.Chip.Render("model "+m.model),
		ui.Chip.Render("session "+truncate(m.sessionKey, 22)),
		statusChip(m.busy, m.statusText),
	)
	left := lipgloss.JoinHorizontal(lipgloss.Center, brand, "  ", chips)

	rule := lipgloss.NewStyle().
		Foreground(ui.ColorBorder).
		Render(strings.Repeat("─", max(0, m.width)))
	return left + "\n" + rule
}

func (m tuiModel) renderFooter() string {
	hint := "Enter 发送 · Shift+Enter 换行 · Esc/Ctrl+C 退出"
	if m.busy {
		hint = m.spinner.View() + " 正在思考…  " + ui.Hint.Render("Ctrl+C 中断")
	}
	rule := lipgloss.NewStyle().
		Foreground(ui.ColorBorder).
		Render(strings.Repeat("─", max(0, m.width)))
	return rule + "\n" + ui.Hint.Render(hint)
}

func (m tuiModel) View() string {
	if !m.ready {
		return "正在初始化…"
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(),
		m.viewport.View(),
		m.textarea.View(),
		m.renderFooter(),
	)
}

func (m *tuiModel) sendUserInput(input string) tea.Cmd {
	m.appendTurn(roleUser, input)
	m.busy = true
	m.statusText = "thinking"
	loop := m.loop
	session := m.sessionKey
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			resp, err := loop.ProcessDirect(ctx, input, session)
			if err != nil {
				return assistantErrMsg{err: err}
			}
			return assistantMsg{text: strings.TrimSpace(resp)}
		},
	)
}

func statusChip(busy bool, text string) string {
	switch {
	case busy:
		return ui.ChipWarn.Render("● " + text)
	case text == "error":
		return ui.ChipError.Render("● error")
	default:
		return ui.ChipOk.Render("● " + text)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

func runTUI(loop *agent.AgentLoop, sessionKey, model string) error {
	p := tea.NewProgram(
		newTUIModel(loop, sessionKey, model),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	return err
}
