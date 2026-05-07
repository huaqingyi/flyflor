package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/sipeed/picoclaw/cmd/flyflor/internal"
	"github.com/sipeed/picoclaw/pkg/skills"
)

type deps struct {
	workspace    string
	skillsLoader *skills.SkillsLoader
}

func defaultSkillsLoader() (*skills.SkillsLoader, error) {
	if _, err := os.Stat(internal.GetConfigPath()); err != nil {
		return nil, err
	}
	cfg, err := internal.LoadConfig()
	if err != nil {
		return nil, err
	}
	globalDir := filepath.Dir(internal.GetConfigPath())
	globalSkillsDir := filepath.Join(globalDir, "skills")
	builtinSkillsDir := filepath.Join(globalDir, "picoclaw", "skills")
	return skills.NewSkillsLoader(cfg.WorkspacePath(), globalSkillsDir, builtinSkillsDir), nil
}

func completeInstalledSkillNames(
	loaderFn func() (*skills.SkillsLoader, error),
) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		if loaderFn == nil {
			loaderFn = defaultSkillsLoader
		}
		loader, err := loaderFn()
		if err != nil || loader == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		infos := loader.ListSkills()
		names := make([]string, 0, len(infos))
		for _, info := range infos {
			if info.Name != "" {
				names = append(names, info.Name)
			}
		}
		sort.Strings(names)
		return names, cobra.ShellCompDirectiveNoFileComp
	}
}

func NewSkillsCommand() *cobra.Command {
	var d deps

	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Manage skills",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := internal.LoadConfig()
			if err != nil {
				return fmt.Errorf("error loading config: %w", err)
			}

			d.workspace = cfg.WorkspacePath()

			// get global config directory and builtin skills directory
			globalDir := filepath.Dir(internal.GetConfigPath())
			globalSkillsDir := filepath.Join(globalDir, "skills")
			builtinSkillsDir := filepath.Join(globalDir, "picoclaw", "skills")
			d.skillsLoader = skills.NewSkillsLoader(d.workspace, globalSkillsDir, builtinSkillsDir)

			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	loaderFn := func() (*skills.SkillsLoader, error) {
		if d.skillsLoader == nil {
			return nil, fmt.Errorf("skills loader is not initialized")
		}
		return d.skillsLoader, nil
	}

	workspaceFn := func() (string, error) {
		if d.workspace == "" {
			return "", fmt.Errorf("workspace is not initialized")
		}
		return d.workspace, nil
	}

	cmd.AddCommand(
		newListCommand(loaderFn),
		newInstallCommand(),
		newInstallBuiltinCommand(workspaceFn),
		newListBuiltinCommand(),
		newRemoveCommand(),
		newSearchCommand(),
		newShowCommand(loaderFn),
	)

	return cmd
}
