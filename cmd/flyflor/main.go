// Flyflor - personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 Flyflor contributors

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/fang"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/sipeed/picoclaw/cmd/flyflor/internal"
	"github.com/sipeed/picoclaw/cmd/flyflor/internal/agent"
	"github.com/sipeed/picoclaw/cmd/flyflor/internal/auth"
	"github.com/sipeed/picoclaw/cmd/flyflor/internal/cliui"
	"github.com/sipeed/picoclaw/cmd/flyflor/internal/cron"
	"github.com/sipeed/picoclaw/cmd/flyflor/internal/gateway"
	"github.com/sipeed/picoclaw/cmd/flyflor/internal/mcp"
	"github.com/sipeed/picoclaw/cmd/flyflor/internal/migrate"
	"github.com/sipeed/picoclaw/cmd/flyflor/internal/model"
	"github.com/sipeed/picoclaw/cmd/flyflor/internal/onboard"
	setupcmd "github.com/sipeed/picoclaw/cmd/flyflor/internal/setup"
	"github.com/sipeed/picoclaw/cmd/flyflor/internal/skills"
	"github.com/sipeed/picoclaw/cmd/flyflor/internal/status"
	"github.com/sipeed/picoclaw/cmd/flyflor/internal/version"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/updater"
)

var rootNoColor bool

func syncCliUIColor(root *cobra.Command) {
	no, _ := root.PersistentFlags().GetBool("no-color")
	cliui.Init(no || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb")
}

// earlyColorDisabled matches lipgloss/banner behavior from env and argv before Cobra parses flags.
func earlyColorDisabled() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return true
	}
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "--no-color" || arg == "--no-color=true" || arg == "--no-color=1" {
			return true
		}
	}
	return false
}

func NewPicoclawCommand() *cobra.Command {
	long := fmt.Sprintf(`%s Flyflor is a personal AI assistant.

Version: %s`, internal.Logo, config.FormatVersion())

	cmd := &cobra.Command{
		Use:   "flyflor",
		Short: "Flyflor - 个人 AI 智能体",
		Long:  long,
		Example: `flyflor version
flyflor onboard
flyflor --no-color status`,
		SilenceErrors: true,
		// Avoid plain UsageString() on stderr/stdout when a command fails; cliui
		// renders matching panels on stderr instead.
		SilenceUsage: true,
		Run: func(c *cobra.Command, _ []string) {
			syncCliUIColor(c.Root())
			runRootLauncher(c)
		},
		PersistentPreRun: func(c *cobra.Command, _ []string) {
			syncCliUIColor(c.Root())
		},
	}
	cmd.CompletionOptions.DisableDefaultCmd = true

	cmd.PersistentFlags().BoolVar(&rootNoColor, "no-color", false,
		"禁用颜色（保留盒式布局）")

	cmd.AddCommand(
		onboard.NewOnboardCommand(),
		agent.NewAgentCommand(),
		agent.NewSessionsCommand(),
		auth.NewAuthCommand(),
		gateway.NewGatewayCommand(),
		status.NewStatusCommand(),
		cron.NewCronCommand(),
		mcp.NewMCPCommand(),
		migrate.NewMigrateCommand(),
		setupcmd.NewSetupCommand(),
		skills.NewSkillsCommand(),
		model.NewModelCommand(),
		newCompletionCommand(cmd),
		updater.NewUpdateCommand("flyflor"),
		version.NewVersionCommand(),
	)
	localizeRootCommandSummaries(cmd)

	return cmd
}

func newHelpCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "help [command]",
		Short:  "显示命令帮助",
		Hidden: true,
		Args:   cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			target := cmd.Root()
			if len(args) > 0 {
				if found, _, err := cmd.Root().Find(args); err == nil && found != nil {
					target = found
				}
			}
			_ = target.Help()
		},
	}
}

// runRootLauncher shows a huh-powered command picker when the user runs
// `flyflor` with no arguments. Selecting an entry executes that subcommand.
func runRootLauncher(c *cobra.Command) {
	type entry struct {
		label string
		desc  string
		args  []string
	}
	entries := []entry{
		{"agent", "进入精致 TUI 与智能体对话", []string{"agent"}},
		{"sessions", "查看可继续的会话", []string{"sessions"}},
		{"status", "查看 flyflor 当前状态", []string{"status"}},
		{"model", "查看或切换默认模型", []string{"model"}},
		{"setup", "初始化模型与工作区", []string{"setup"}},
		{"skills", "管理技能", []string{"skills"}},
		{"mcp", "管理 MCP 服务", []string{"mcp"}},
		{"help", "查看命令总览", []string{"--help"}},
	}

	options := make([]huh.Option[int], 0, len(entries))
	for i, e := range entries {
		options = append(options, huh.NewOption(
			fmt.Sprintf("%-10s  %s", e.label, e.desc), i,
		))
	}

	choice := -1
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title("flyflor · 选择要执行的命令").
				Description("↑↓ 移动 · Enter 确认 · Ctrl+C 退出").
				Options(options...).
				Value(&choice),
		),
	).WithTheme(huh.ThemeCharm())

	if err := form.Run(); err != nil {
		return
	}
	if choice < 0 || choice >= len(entries) {
		return
	}
	selected := entries[choice]
	root := c.Root()
	root.SetArgs(selected.args)
	_ = root.Execute()
}

func localizeRootCommandSummaries(root *cobra.Command) {
	labels := map[string]string{
		"agent":      "直接与 Flyflor 对话",
		"auth":       "管理认证、登录与登出",
		"completion": "生成 shell 自动补全脚本",
		"cron":       "管理定时任务",
		"gateway":    "启动 Flyflor 网关",
		"mcp":        "管理 MCP 服务配置",
		"migrate":    "从 OpenClaw 风格目录迁移到 Flyflor",
		"model":      "显示或切换默认模型",
		"onboard":    "初始化 Flyflor 配置与工作区",
		"sessions":   "查看可继续的 Flyflor 会话",
		"setup":      "初始化模型与 Agent TUI 黑板环境",
		"skills":     "管理技能",
		"status":     "显示 Flyflor 状态",
		"update":     "检查并应用 GitHub Release 更新",
		"version":    "显示版本信息",
	}
	for _, sub := range root.Commands() {
		if label, ok := labels[sub.Name()]; ok {
			sub.Short = label
		}
	}
}

func newCompletionCommand(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:       "completion [bash|zsh|fish|powershell]",
		Short:     "生成 shell 自动补全脚本",
		Long:      "生成 Flyflor 的 shell 自动补全脚本。安装后，flyflor 的所有子命令和 flags 都可以用 Tab 补全。",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(out)
			case "zsh":
				return root.GenZshCompletion(out)
			case "fish":
				return root.GenFishCompletion(out, true)
			case "powershell":
				return root.GenPowerShellCompletion(out)
			default:
				return fmt.Errorf("unsupported shell %q", args[0])
			}
		},
	}
	return cmd
}

func main() {
	cliui.Init(earlyColorDisabled())

	if tzEnv := os.Getenv("TZ"); tzEnv != "" {
		if loc, err := time.LoadLocation(tzEnv); err == nil {
			time.Local = loc //nolint:gosmopolitan // honour TZ env
		}
	}

	cmd := NewPicoclawCommand()
	if err := fang.Execute(
		context.Background(), cmd,
		fang.WithVersion(config.FormatVersion()),
		fang.WithNotifySignal(os.Interrupt),
	); err != nil {
		os.Exit(1)
	}
}
