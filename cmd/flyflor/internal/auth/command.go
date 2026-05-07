package auth

import "github.com/spf13/cobra"

var authProviders = []string{"openai", "anthropic", "google-antigravity", "antigravity"}

func completeAuthProviders(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return authProviders, cobra.ShellCompDirectiveNoFileComp
}

func NewAuthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication (login, logout, status)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newLoginCommand(),
		newLogoutCommand(),
		newStatusCommand(),
		newModelsCommand(),
		newWeixinCommand(),
		newWeComCommand(),
	)

	return cmd
}
