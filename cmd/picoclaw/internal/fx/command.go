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
		Short:              "运行 fx 折叠查看 JSON",
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveDefault
			}
			return []string{"--help", "--version", "--yaml", "--raw", "."}, cobra.ShellCompDirectiveDefault
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			bin, err := exec.LookPath("fx")
			if err != nil {
				return fmt.Errorf("fx 未安装；Docker 镜像内已内置，宿主机可运行: go install github.com/antonmedv/fx@latest")
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
