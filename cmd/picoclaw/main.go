// Flyflor - personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 Flyflor contributors

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/sipeed/picoclaw/cmd/picoclaw/internal"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/agent"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/auth"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/cliui"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/cron"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/dive"
	fxcmd "github.com/sipeed/picoclaw/cmd/picoclaw/internal/fx"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/gateway"
	gumcmd "github.com/sipeed/picoclaw/cmd/picoclaw/internal/gum"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/mcp"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/migrate"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/model"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/onboard"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/skills"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/status"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/version"
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
			if shouldShowRootMenu() {
				action, err := runRootInteractiveMenu()
				if err != nil {
					fmt.Fprint(c.ErrOrStderr(), cliui.FormatCLIError(err.Error(), c))
					return
				}
				if err := executeRootMenuAction(action); err != nil {
					fmt.Fprint(c.ErrOrStderr(), cliui.FormatCLIError(err.Error(), c))
				}
				return
			}
			fmt.Fprint(c.OutOrStdout(), renderHome())
		},
		PersistentPreRun: func(c *cobra.Command, _ []string) {
			syncCliUIColor(c.Root())
		},
	}
	cmd.CompletionOptions.DisableDefaultCmd = true

	cmd.PersistentFlags().BoolVar(&rootNoColor, "no-color", false,
		"禁用颜色（保留盒式布局）")

	cmd.SetHelpFunc(func(c *cobra.Command, _ []string) {
		syncCliUIColor(c.Root())
		fmt.Fprint(c.OutOrStdout(), cliui.RenderCommandHelp(c))
	})
	cmd.SetHelpCommand(newHelpCommand())

	cmd.AddCommand(
		onboard.NewOnboardCommand(),
		agent.NewAgentCommand(),
		agent.NewSessionsCommand(),
		auth.NewAuthCommand(),
		gateway.NewGatewayCommand(),
		status.NewStatusCommand(),
		cron.NewCronCommand(),
		dive.NewDiveCommand(),
		fxcmd.NewFXCommand(),
		gumcmd.NewGumCommand(),
		mcp.NewMCPCommand(),
		migrate.NewMigrateCommand(),
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
		Use:   "help [command]",
		Short: "显示命令帮助",
		Args:  cobra.ArbitraryArgs,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			root := cmd.Root()
			candidates := make([]string, 0, len(root.Commands()))
			for _, sub := range root.Commands() {
				if sub.Hidden {
					continue
				}
				candidates = append(candidates, sub.Name())
			}
			return candidates, cobra.ShellCompDirectiveNoFileComp
		},
		Run: func(cmd *cobra.Command, args []string) {
			target := cmd.Root()
			if len(args) > 0 {
				if found, _, err := cmd.Root().Find(args); err == nil && found != nil {
					target = found
				}
			}
			syncCliUIColor(target.Root())
			fmt.Fprint(cmd.OutOrStdout(), cliui.RenderCommandHelp(target))
		},
	}
}

func localizeRootCommandSummaries(root *cobra.Command) {
	labels := map[string]string{
		"agent":      "直接与 Flyflor 对话",
		"auth":       "管理认证、登录与登出",
		"completion": "生成 shell 自动补全脚本",
		"cron":       "管理定时任务",
		"dive":       "分析 Docker 镜像层",
		"fx":         "折叠查看 JSON",
		"gum":        "运行 Gum 终端交互工具",
		"gateway":    "启动 Flyflor 网关",
		"mcp":        "管理 MCP 服务配置",
		"migrate":    "从 OpenClaw 风格目录迁移到 Flyflor",
		"model":      "显示或切换默认模型",
		"onboard":    "初始化 Flyflor 配置与工作区",
		"sessions":   "查看可继续的 Flyflor 会话",
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

func renderHome() string {
	configPath := internal.GetConfigPath()
	info := cliui.HomeInfo{
		Version: config.FormatVersion(),
		Mode:    "交互模式",
		Config:  configPath,
	}
	if _, statErr := os.Stat(configPath); statErr == nil {
		cfg, err := config.LoadConfig(configPath)
		if err != nil || cfg == nil {
			return cliui.RenderHome(info)
		}
		info.Model = cfg.Agents.Defaults.GetModelName()
		info.Workspace = cfg.WorkspacePath()
	}
	return cliui.RenderHome(info)
}

const (
	colorBlue = "\033[1;38;2;62;93;185m"
	colorRed  = "\033[1;38;2;213;70;70m"
	banner    = "\r\n" +
		colorBlue + "███████╗██╗     ██╗   ██╗" + colorRed + "███████╗██╗      ██████╗ ██████╗ \n" +
		colorBlue + "██╔════╝██║     ╚██╗ ██╔╝" + colorRed + "██╔════╝██║     ██╔═══██╗██╔══██╗\n" +
		colorBlue + "█████╗  ██║      ╚████╔╝ " + colorRed + "█████╗  ██║     ██║   ██║██████╔╝\n" +
		colorBlue + "██╔══╝  ██║       ╚██╔╝  " + colorRed + "██╔══╝  ██║     ██║   ██║██╔══██╗\n" +
		colorBlue + "██║     ███████╗   ██║   " + colorRed + "██║     ███████╗╚██████╔╝██║  ██║\n" +
		colorBlue + "╚═╝     ╚══════╝   ╚═╝   " + colorRed + "╚═╝     ╚══════╝ ╚═════╝ ╚═╝  ╚═╝\n " +
		"\033[0m\r\n"
	plainBanner = "\r\n" +
		"███████╗██╗     ██╗   ██╗███████╗██╗      ██████╗ ██████╗ \n" +
		"██╔════╝██║     ╚██╗ ██╔╝██╔════╝██║     ██╔═══██╗██╔══██╗\n" +
		"█████╗  ██║      ╚████╔╝ █████╗  ██║     ██║   ██║██████╔╝\n" +
		"██╔══╝  ██║       ╚██╔╝  ██╔══╝  ██║     ██║   ██║██╔══██╗\n" +
		"██║     ███████╗   ██║   ██║     ███████╗╚██████╔╝██║  ██║\n" +
		"╚═╝     ╚══════╝   ╚═╝   ╚═╝     ╚══════╝ ╚═════╝ ╚═╝  ╚═╝\n " +
		"\r\n"
)

func main() {
	cliui.Init(earlyColorDisabled())

	if !suppressStartupBanner() {
		if earlyColorDisabled() {
			fmt.Print(plainBanner)
		} else {
			fmt.Printf("%s", banner)
		}
	}

	tzEnv := os.Getenv("TZ")
	if tzEnv != "" {
		fmt.Println("TZ environment:", tzEnv)
		zoneinfoEnv := os.Getenv("ZONEINFO")
		fmt.Println("ZONEINFO environment:", zoneinfoEnv)
		loc, err := time.LoadLocation(tzEnv)
		if err != nil {
			fmt.Println("Error loading time zone:", err)
		} else {
			fmt.Println("Time zone loaded successfully:", loc)
			time.Local = loc //nolint:gosmopolitan // We intentionally set local timezone from TZ env
		}
	}

	cmd := NewPicoclawCommand()
	last, err := cmd.ExecuteC()
	if err != nil {
		syncCliUIColor(cmd)
		fmt.Fprint(os.Stderr, cliui.FormatCLIError(err.Error(), last))
		os.Exit(1)
	}
}

func suppressStartupBanner() bool {
	return true
}
