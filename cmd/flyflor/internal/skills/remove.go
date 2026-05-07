package skills

import (
	"github.com/spf13/cobra"

	"github.com/sipeed/picoclaw/cmd/flyflor/internal"
)

func newRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "remove",
		Aliases:           []string{"rm", "uninstall"},
		Short:             "Remove installed skill",
		Args:              cobra.ExactArgs(1),
		Example:           `picoclaw skills remove weather`,
		ValidArgsFunction: completeInstalledSkillNames(nil),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := internal.LoadConfig()
			if err != nil {
				return err
			}
			return skillsRemoveFromWorkspace(cfg.WorkspacePath(), cfg.Tools.Skills, args[0])
		},
	}

	return cmd
}
