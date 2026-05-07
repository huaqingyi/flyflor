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
		Short:              "运行 Gum 终端交互工具",
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
			bin, err := exec.LookPath("gum")
			if err != nil {
				return fmt.Errorf("gum 未安装；Docker 镜像内已内置，宿主机可运行: go install github.com/charmbracelet/gum@latest")
			}
			if len(args) == 0 {
				args = []string{"--help"}
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
