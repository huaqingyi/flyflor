package dive

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func NewDiveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "dive [image] [dive flags...]",
		Short:              "用 dive 分析 Docker 镜像层",
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		ValidArgsFunction: func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return []string{"flyflor:local", "qdrant/qdrant:latest"}, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			bin, err := exec.LookPath("dive")
			if err != nil {
				return fmt.Errorf("dive 未安装；Docker 镜像内已内置，宿主机可运行: go install github.com/wagoodman/dive@latest")
			}
			if len(args) == 0 {
				args = []string{"flyflor:local"}
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
