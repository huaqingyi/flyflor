package gum

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func NewGumCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "gum [gum command] [gum flags...]",
		Short:              "高级: 透传 Gum 脚本交互工具",
		Long:               "高级透传命令。日常长文本输入请优先进入 flyflor agent 后按 Ctrl+E 或输入 /edit；这里仅在你明确需要调用外部 Gum 时保留。",
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveDefault
			}
			return []string{
				"choose", "confirm", "file", "filter", "format", "input",
				"join", "log", "pager", "spin", "style", "table", "write",
			}, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || isHelpArg(args[0]) {
				fmt.Fprint(cmd.OutOrStdout(), gumUsageText())
				return nil
			}
			bin, err := exec.LookPath("gum")
			if err != nil {
				return fmt.Errorf("gum 未安装；Docker 镜像内已内置，宿主机可运行: go install github.com/charmbracelet/gum@latest")
			}
			process := exec.CommandContext(cmd.Context(), bin, args...)
			process.Stdin = cmd.InOrStdin()
			process.Stdout = cmd.OutOrStdout()
			process.Stderr = cmd.ErrOrStderr()
			process.Env = os.Environ()
			return process.Run()
		},
	}
	return cmd
}

func isHelpArg(arg string) bool {
	return arg == "-h" || arg == "--help" || arg == "help"
}

func gumUsageText() string {
	return `Flyflor 已把常用交互融合进 Agent TUI:

  flyflor agent
  Ctrl+E    展开长文本编辑区
  /edit     同 Ctrl+E；Enter 换行，Ctrl+D 发送
  /help     查看 TUI 内置命令

gum 仍作为高级透传命令保留，用于脚本里的选择、输入、确认和表格:

  flyflor gum choose red green blue
  flyflor gum input --placeholder "title"
  flyflor gum write

`
}
