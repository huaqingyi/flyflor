package onboard

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sipeed/picoclaw/cmd/flyflor/internal"
	"github.com/sipeed/picoclaw/cmd/flyflor/internal/cliui"
	"github.com/sipeed/picoclaw/pkg"
	"github.com/sipeed/picoclaw/pkg/config"
)

// NewOnboardCommand creates the initial configuration and workspace layout.
func NewOnboardCommand() *cobra.Command {
	var force bool
	var encrypt bool

	cmd := &cobra.Command{
		Use:   "onboard",
		Short: "初始化 Flyflor 配置与工作区",
		RunE: func(cmd *cobra.Command, _ []string) error {
			configPath := internal.GetConfigPath()
			if err := runOnboard(configPath, force); err != nil {
				return err
			}
			cliui.PrintOnboardComplete(internal.Logo, encrypt, configPath)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "覆盖已有配置")
	cmd.Flags().BoolVar(&encrypt, "encrypt", false, "显示凭据加密后的下一步提示")

	return cmd
}

func runOnboard(configPath string, force bool) error {
	if configPath == "" {
		return fmt.Errorf("config path is empty")
	}

	if _, err := os.Stat(configPath); err == nil && !force {
		workspace := effectiveWorkspace(config.DefaultConfig())
		if err := os.MkdirAll(workspace, 0o755); err != nil {
			return fmt.Errorf("create workspace %s: %w", workspace, err)
		}
		return nil
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("stat config %s: %w", configPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return fmt.Errorf("create config directory %s: %w", filepath.Dir(configPath), err)
	}

	cfg := config.DefaultConfig()
	if workspace := os.Getenv("PICOCLAW_AGENTS_DEFAULTS_WORKSPACE"); workspace != "" {
		cfg.Agents.Defaults.Workspace = workspace
	}
	if cfg.Agents.Defaults.Workspace == "" {
		cfg.Agents.Defaults.Workspace = filepath.Join(config.GetHome(), pkg.WorkspaceName)
	}

	if err := os.MkdirAll(effectiveWorkspace(cfg), 0o755); err != nil {
		return fmt.Errorf("create workspace %s: %w", effectiveWorkspace(cfg), err)
	}
	if err := config.SaveConfig(configPath, cfg); err != nil {
		return fmt.Errorf("write config %s: %w", configPath, err)
	}
	return nil
}

func effectiveWorkspace(cfg *config.Config) string {
	if workspace := os.Getenv("PICOCLAW_AGENTS_DEFAULTS_WORKSPACE"); workspace != "" {
		return workspace
	}
	return cfg.WorkspacePath()
}
