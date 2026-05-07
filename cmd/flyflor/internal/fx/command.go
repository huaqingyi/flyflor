package fx

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func NewFXCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "fx [fx flags...] [file]",
		Short:              "高级: 透传 fx JSON 查看器",
		Long:               "高级透传命令。日常查看黑板和思考过程请优先进入 flyflor agent 后使用 /bb；这里仅在你明确需要调用外部 fx 时保留。",
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveDefault
			}
			return []string{"--help", "--version", "--yaml", "--raw", "."}, cobra.ShellCompDirectiveDefault
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || isHelpArg(args[0]) {
				fmt.Fprint(cmd.OutOrStdout(), fxUsageText())
				return nil
			}
			bin, err := exec.LookPath("fx")
			if err != nil {
				return fmt.Errorf("fx 未安装；Docker 镜像内已内置，宿主机可运行: go install github.com/antonmedv/fx@latest")
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

func fxUsageText() string {
	return `Flyflor 已把折叠查看融合进 Agent TUI:

  flyflor agent
  /bb       打开按轮次分组的折叠黑板
  /think    展开或折叠对话流里的思考摘要

fx 仍作为高级透传命令保留，用于查看 JSON 文件或管道:

  flyflor fx data.json
  cat data.json | flyflor fx .

`
}
