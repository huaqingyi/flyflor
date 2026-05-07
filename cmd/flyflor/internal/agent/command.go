package agent

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/sipeed/picoclaw/cmd/flyflor/internal"
	"github.com/sipeed/picoclaw/pkg/config"
)

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
		Short:   "Chat with Flyflor directly",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return agentCmd(message, sessionKey, model, debug)
		},
	}

	cmd.Flags().BoolVarP(&debug, "debug", "d", false, "Enable debug logging")
	cmd.Flags().StringVarP(&message, "message", "m", "", "Send a single message (non-interactive mode)")
	cmd.Flags().StringVarP(&sessionKey, "session", "s", "cli:default", "Session key")
	cmd.Flags().StringVarP(&model, "model", "", "", "Model to use")
	_ = cmd.RegisterFlagCompletionFunc("model", completeConfiguredModels)
	_ = cmd.RegisterFlagCompletionFunc("session", completeExistingSessions)

	return cmd
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
	for _, model := range cfg.ModelList {
		if model == nil || !model.Enabled || model.ModelName == "" {
			continue
		}
		candidates = append(candidates, model.ModelName)
	}
	return candidates, cobra.ShellCompDirectiveNoFileComp
}
