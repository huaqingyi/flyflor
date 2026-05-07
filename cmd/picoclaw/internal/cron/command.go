package cron

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sipeed/picoclaw/cmd/picoclaw/internal"
	pkgcron "github.com/sipeed/picoclaw/pkg/cron"
)

func NewCronCommand() *cobra.Command {
	var storePath string

	cmd := &cobra.Command{
		Use:     "cron",
		Aliases: []string{"c"},
		Short:   "Manage scheduled tasks",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
		// Resolve storePath at execution time so it reflects the current config
		// and is shared across all subcommands.
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := internal.LoadConfig()
			if err != nil {
				return fmt.Errorf("error loading config: %w", err)
			}
			storePath = filepath.Join(cfg.WorkspacePath(), "cron", "jobs.json")
			return nil
		},
	}

	cmd.AddCommand(
		newListCommand(func() string { return storePath }),
		newAddCommand(func() string { return storePath }),
		newRemoveCommand(func() string { return storePath }),
		newEnableCommand(func() string { return storePath }),
		newDisableCommand(func() string { return storePath }),
	)

	return cmd
}

func completeJobIDs(storePath func() string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		path := ""
		if storePath != nil {
			path = storePath()
		}
		if path == "" {
			if _, err := os.Stat(internal.GetConfigPath()); err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			cfg, err := internal.LoadConfig()
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			path = filepath.Join(cfg.WorkspacePath(), "cron", "jobs.json")
		}
		service := pkgcron.NewCronService(path, nil)
		jobs := service.ListJobs(true)
		candidates := make([]string, 0, len(jobs))
		for _, job := range jobs {
			if job.ID == "" {
				continue
			}
			label := job.ID
			if job.Name != "" {
				label += "\t" + job.Name
			}
			candidates = append(candidates, label)
		}
		return candidates, cobra.ShellCompDirectiveNoFileComp
	}
}
