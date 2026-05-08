package agent

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/sipeed/picoclaw/cmd/flyflor/internal"
	"github.com/sipeed/picoclaw/cmd/flyflor/internal/ui"
	"github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
)

// NewAgentCommand returns the `flyflor agent` command.
//
// With -m/--message it prints a single answer and exits. Otherwise it launches
// the bubbletea TUI: top status bar, scrollable Markdown-rendered chat, multi-
// line input, spinner while the model thinks.
func NewAgentCommand() *cobra.Command {
	var (
		message    string
		sessionKey string
		model      string
		debug      bool
	)

	cmd := &cobra.Command{
		Use:     "agent",
		Aliases: []string{"chat"},
		Short:   "与 flyflor 智能体对话",
		Long:    "启动 flyflor 智能体的精致 TUI 对话界面，或用 -m 进行一次性提问。",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAgent(cmd.OutOrStdout(), message, sessionKey, model, debug)
		},
	}

	cmd.Flags().BoolVarP(&debug, "debug", "d", false, "开启调试日志")
	cmd.Flags().StringVarP(&message, "message", "m", "", "一次性发送一条消息后退出")
	cmd.Flags().StringVarP(&sessionKey, "session", "s", "cli:default", "会话标识")
	cmd.Flags().StringVarP(&model, "model", "", "", "覆盖默认模型")
	_ = cmd.RegisterFlagCompletionFunc("model", completeConfiguredModels)
	_ = cmd.RegisterFlagCompletionFunc("session", completeExistingSessions)

	return cmd
}

func runAgent(out io.Writer, message, sessionKey, model string, debug bool) error {
	if strings.TrimSpace(sessionKey) == "" {
		sessionKey = "cli:default"
	}

	cfg, err := internal.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger.ConfigureFromEnv()
	if debug {
		logger.SetLevel(logger.DEBUG)
	} else {
		logger.SetLevel(logger.WARN)
	}

	if model != "" {
		cfg.Agents.Defaults.ModelName = model
	}

	provider, modelID, err := providers.CreateProvider(cfg)
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}
	if modelID != "" {
		cfg.Agents.Defaults.ModelName = modelID
	}

	msgBus := bus.NewMessageBus()
	defer msgBus.Close()
	loop := agent.NewAgentLoop(cfg, msgBus, provider)
	defer loop.Close()

	modelLabel := cfg.Agents.Defaults.GetModelName()

	if message != "" {
		return askOnce(out, loop, sessionKey, modelLabel, message)
	}

	return runTUI(loop, sessionKey, modelLabel)
}

func askOnce(out io.Writer, loop *agent.AgentLoop, sessionKey, modelLabel, message string) error {
	header := lipgloss.JoinHorizontal(lipgloss.Center,
		ui.Brand.Render(" flyflor "), "  ",
		ui.Chip.Render("model "+modelLabel),
		ui.Chip.Render("session "+sessionKey),
	)
	fmt.Fprintln(out, header)
	fmt.Fprintln(out, ui.UserHeader.Render("● 你"))
	fmt.Fprintln(out, ui.UserBubble.Render(message))

	resp, err := loop.ProcessDirect(context.Background(), message, sessionKey)
	if err != nil {
		fmt.Fprintln(out, ui.Error.Render("⚠ 错误：")+err.Error())
		return err
	}
	rendered := resp
	if r, rerr := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(100)); rerr == nil {
		if md, mderr := r.Render(resp); mderr == nil {
			rendered = strings.TrimRight(md, "\n")
		}
	}
	fmt.Fprintln(out, ui.AssistantHeader.Render("✦ flyflor"))
	fmt.Fprintln(out, rendered)
	return nil
}

func completeConfiguredModels(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	configPath := internal.GetConfigPath()
	if _, err := os.Stat(configPath); err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	candidates := make([]string, 0, len(cfg.ModelList))
	for _, m := range cfg.ModelList {
		if m == nil || !m.Enabled || m.ModelName == "" {
			continue
		}
		candidates = append(candidates, m.ModelName)
	}
	return candidates, cobra.ShellCompDirectiveNoFileComp
}
