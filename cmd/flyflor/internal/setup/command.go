package setup

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/sipeed/picoclaw/cmd/flyflor/internal"
	"github.com/sipeed/picoclaw/pkg"
	"github.com/sipeed/picoclaw/pkg/config"
)

const (
	defaultProvider = "openai"
	defaultAPIBase  = "https://api.openai.com/v1"
	defaultModelID  = "gpt-5.4"
	defaultAlias    = "gpt-5.4"
)

type Status struct {
	ConfigPath       string
	ConfigExists     bool
	ModelReady       bool
	RuntimeReady     bool
	DefaultModelName string
	Missing          []string
	LoadErr          error
}

type options struct {
	provider       string
	apiBase        string
	apiKey         string
	modelID        string
	alias          string
	yes            bool
	nonInteractive bool
	stdin          io.Reader
	stdout         io.Writer
}

// NewSetupCommand creates the first-run setup wizard.
func NewSetupCommand() *cobra.Command {
	var opt options
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "初始化模型与 Agent TUI 黑板运行环境",
		Long: `Run the Flyflor first-run setup wizard.

The wizard checks whether Flyflor has a usable default model and the local
runtime tools needed by the Agent TUI blackboard / bridge workflow. It then
writes config.json and .security.yml as needed.`,
		Example: `flyflor setup
flyflor setup --provider openrouter --api-key sk-... --model openai/gpt-5.4 --name openrouter-gpt-5.4 --yes
flyflor setup --provider local --api-base http://localhost:8000/v1 --model my-model --yes`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opt.stdin = cmd.InOrStdin()
			opt.stdout = cmd.OutOrStdout()
			return Run(opt)
		},
	}
	cmd.Flags().StringVar(&opt.provider, "provider", "", "model provider: openai, openrouter, anthropic, local, openai-compatible")
	cmd.Flags().StringVar(&opt.apiBase, "api-base", "", "OpenAI-compatible API base URL")
	cmd.Flags().StringVar(&opt.apiKey, "api-key", "", "API key; optional for --provider local")
	cmd.Flags().StringVar(&opt.modelID, "model", "", "provider model id")
	cmd.Flags().StringVar(&opt.alias, "name", "", "local model alias written to model_list")
	cmd.Flags().BoolVarP(&opt.yes, "yes", "y", false, "accept defaults for omitted non-secret values")
	cmd.Flags().BoolVar(&opt.nonInteractive, "non-interactive", false, "do not prompt; fail if required values are missing")
	return cmd
}

func Run(opt options) error {
	if opt.stdin == nil {
		opt.stdin = os.Stdin
	}
	if opt.stdout == nil {
		opt.stdout = os.Stdout
	}

	configPath := internal.GetConfigPath()
	status := Check(configPath)
	printSetupIntro(opt.stdout, status)

	cfg, created, err := loadOrCreateConfig(configPath)
	if err != nil {
		return err
	}
	repairRuntimeDefaults(cfg)
	if err := os.MkdirAll(cfg.WorkspacePath(), 0o755); err != nil {
		return fmt.Errorf("create workspace %s: %w", cfg.WorkspacePath(), err)
	}

	if modelConfigured(cfg) && !opt.yes && !opt.nonInteractive {
		fmt.Fprintf(opt.stdout, "✓ 默认模型已配置: %s\n", cfg.Agents.Defaults.ModelName)
	} else if !modelConfigured(cfg) || opt.provider != "" || opt.modelID != "" || opt.apiKey != "" {
		if err := configureModel(cfg, &opt); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return fmt.Errorf("create config directory %s: %w", filepath.Dir(configPath), err)
	}
	if err := config.SaveConfig(configPath, cfg); err != nil {
		return fmt.Errorf("write config %s: %w", configPath, err)
	}

	status = Check(configPath)
	if !status.ModelReady || !status.RuntimeReady {
		return fmt.Errorf("setup incomplete: %s", strings.Join(status.Missing, ", "))
	}
	if created {
		fmt.Fprintf(opt.stdout, "\n✓ Created %s\n", configPath)
	} else {
		fmt.Fprintf(opt.stdout, "\n✓ Updated %s\n", configPath)
	}
	fmt.Fprintln(opt.stdout, "✓ Agent TUI 黑板、长文本输入、Bridge/Guard 基础工具已就绪")
	fmt.Fprintln(opt.stdout, "\n下一步: flyflor agent")
	return nil
}

func Check(configPath string) Status {
	st := Status{ConfigPath: configPath}
	if _, err := os.Stat(configPath); err == nil {
		st.ConfigExists = true
	} else if err != nil && !os.IsNotExist(err) {
		st.LoadErr = err
		st.Missing = append(st.Missing, "配置文件不可读")
		return st
	}
	if !st.ConfigExists {
		st.Missing = append(st.Missing, "缺少 config.json")
		return st
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		st.LoadErr = err
		st.Missing = append(st.Missing, "配置文件无法解析")
		return st
	}
	st.DefaultModelName = strings.TrimSpace(cfg.Agents.Defaults.ModelName)
	st.ModelReady = modelConfigured(cfg)
	if !st.ModelReady {
		st.Missing = append(st.Missing, "缺少可用默认模型")
	}
	missingTools := missingRuntimeTools(cfg)
	st.RuntimeReady = len(missingTools) == 0
	st.Missing = append(st.Missing, missingTools...)
	return st
}

func NeedsSetup(configPath string) bool {
	st := Check(configPath)
	return !st.ConfigExists || st.LoadErr != nil || !st.ModelReady || !st.RuntimeReady
}

func RenderRequired(st Status) string {
	var b strings.Builder
	fmt.Fprintln(&b, "Flyflor 需要先完成初始配置。")
	fmt.Fprintln(&b)
	if len(st.Missing) > 0 {
		fmt.Fprintln(&b, "缺失项:")
		for _, item := range st.Missing {
			fmt.Fprintf(&b, "  - %s\n", item)
		}
		fmt.Fprintln(&b)
	}
	fmt.Fprintln(&b, "运行:")
	fmt.Fprintln(&b, "  flyflor setup")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "setup 会配置默认模型，并修复 Agent TUI 黑板/Bridge/Guard 所需的本地工具。")
	return b.String()
}

func loadOrCreateConfig(configPath string) (*config.Config, bool, error) {
	cfg, err := config.LoadConfig(configPath)
	if err == nil {
		return cfg, false, nil
	}
	if _, statErr := os.Stat(configPath); statErr == nil {
		return nil, false, fmt.Errorf("load config %s: %w", configPath, err)
	}
	cfg = config.DefaultConfig()
	if workspace := os.Getenv("PICOCLAW_AGENTS_DEFAULTS_WORKSPACE"); workspace != "" {
		cfg.Agents.Defaults.Workspace = workspace
	}
	if cfg.Agents.Defaults.Workspace == "" {
		cfg.Agents.Defaults.Workspace = filepath.Join(config.GetHome(), pkg.WorkspaceName)
	}
	return cfg, true, nil
}

func configureModel(cfg *config.Config, opt *options) error {
	profile, err := resolveProfile(opt)
	if err != nil {
		return err
	}

	if profile.Provider != "local" && strings.TrimSpace(profile.APIKey) == "" {
		if opt.nonInteractive {
			return fmt.Errorf("--api-key is required for provider %s", profile.Provider)
		}
		key, readErr := promptSecret(opt.stdin, opt.stdout, "API key")
		if readErr != nil {
			return readErr
		}
		profile.APIKey = strings.TrimSpace(key)
	}
	if profile.Provider != "local" && strings.TrimSpace(profile.APIKey) == "" {
		return fmt.Errorf("API key is required")
	}

	model := &config.ModelConfig{
		ModelName: profile.Alias,
		Provider:  profile.ConfigProvider,
		Model:     profile.ModelID,
		APIBase:   profile.APIBase,
		Enabled:   true,
	}
	if profile.Provider == "local" {
		if profile.APIKey == "" {
			profile.APIKey = "dummy"
		}
	}
	if profile.APIKey != "" {
		model.APIKeys = config.SimpleSecureStrings(profile.APIKey)
	}
	upsertModel(cfg, model)
	cfg.Agents.Defaults.ModelName = model.ModelName
	return nil
}

type providerProfile struct {
	Provider       string
	ConfigProvider string
	APIBase        string
	APIKey         string
	ModelID        string
	Alias          string
}

func resolveProfile(opt *options) (providerProfile, error) {
	reader := bufio.NewReader(opt.stdin)
	provider := strings.TrimSpace(opt.provider)
	if provider == "" {
		if opt.nonInteractive || opt.yes {
			provider = defaultProvider
		} else {
			answer, err := promptLine(reader, opt.stdout, "Provider [openai/openrouter/anthropic/local]", defaultProvider)
			if err != nil {
				return providerProfile{}, err
			}
			provider = answer
		}
	}
	provider = strings.ToLower(strings.TrimSpace(provider))

	p := defaultsForProvider(provider)
	if p.Provider == "" {
		return providerProfile{}, fmt.Errorf("unsupported provider %q", provider)
	}
	p.APIKey = strings.TrimSpace(opt.apiKey)
	p.APIBase = firstNonEmpty(opt.apiBase, p.APIBase)
	p.ModelID = firstNonEmpty(opt.modelID, p.ModelID)
	p.Alias = firstNonEmpty(opt.alias, p.Alias)

	if !opt.nonInteractive && !opt.yes {
		var err error
		p.APIBase, err = promptLine(reader, opt.stdout, "API base", p.APIBase)
		if err != nil {
			return providerProfile{}, err
		}
		p.ModelID, err = promptLine(reader, opt.stdout, "Model id", p.ModelID)
		if err != nil {
			return providerProfile{}, err
		}
		p.Alias, err = promptLine(reader, opt.stdout, "Local model name", p.Alias)
		if err != nil {
			return providerProfile{}, err
		}
	}

	if strings.TrimSpace(p.ModelID) == "" {
		return providerProfile{}, fmt.Errorf("model id is required")
	}
	if strings.TrimSpace(p.Alias) == "" {
		return providerProfile{}, fmt.Errorf("local model name is required")
	}
	return p, nil
}

func defaultsForProvider(provider string) providerProfile {
	switch provider {
	case "", "openai", "openai-compatible":
		return providerProfile{Provider: "openai", ConfigProvider: "openai", APIBase: defaultAPIBase, ModelID: defaultModelID, Alias: defaultAlias}
	case "openrouter":
		return providerProfile{Provider: "openrouter", ConfigProvider: "openrouter", APIBase: "https://openrouter.ai/api/v1", ModelID: "openai/gpt-5.4", Alias: "openrouter-gpt-5.4"}
	case "anthropic", "claude":
		return providerProfile{Provider: "anthropic", ConfigProvider: "anthropic", APIBase: "https://api.anthropic.com/v1", ModelID: "claude-sonnet-4.6", Alias: "claude-sonnet-4.6"}
	case "local", "lmstudio", "vllm", "ollama":
		return providerProfile{Provider: "local", ConfigProvider: "openai", APIBase: "http://localhost:8000/v1", ModelID: "custom-model", Alias: "local-model", APIKey: "dummy"}
	default:
		return providerProfile{}
	}
}

func upsertModel(cfg *config.Config, model *config.ModelConfig) {
	for i, existing := range cfg.ModelList {
		if existing != nil && existing.ModelName == model.ModelName {
			cfg.ModelList[i] = model
			return
		}
	}
	cfg.ModelList = append(cfg.ModelList, model)
}

func modelConfigured(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	name := strings.TrimSpace(cfg.Agents.Defaults.ModelName)
	if name == "" {
		return false
	}
	for _, m := range cfg.ModelList {
		if m == nil || !m.Enabled || m.ModelName != name || strings.TrimSpace(m.Model) == "" {
			continue
		}
		if providerNeedsKey(m.Provider) && strings.TrimSpace(m.APIKey()) == "" {
			continue
		}
		return true
	}
	return false
}

func providerNeedsKey(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "ollama", "vllm":
		return false
	default:
		return true
	}
}

func repairRuntimeDefaults(cfg *config.Config) {
	if cfg.Agents.Defaults.Workspace == "" {
		cfg.Agents.Defaults.Workspace = filepath.Join(config.GetHome(), pkg.WorkspaceName)
	}
	cfg.Agents.Defaults.RestrictToWorkspace = true
	cfg.Agents.Defaults.ContextManager = "seahorse"
	cfg.Tools.ReadFile.Enabled = true
	cfg.Tools.WriteFile.Enabled = true
	cfg.Tools.EditFile.Enabled = true
	cfg.Tools.ListDir.Enabled = true
	cfg.Tools.Spawn.Enabled = true
	cfg.Tools.Subagent.Enabled = true
	cfg.Tools.WebFetch.Enabled = true
	cfg.Tools.Skills.Enabled = true
	cfg.Tools.FindSkills.Enabled = true
	cfg.Tools.InstallSkill.Enabled = true
}

func missingRuntimeTools(cfg *config.Config) []string {
	if cfg == nil {
		return []string{"配置为空"}
	}
	var missing []string
	if strings.TrimSpace(cfg.Agents.Defaults.ContextManager) != "seahorse" {
		missing = append(missing, "context_manager 不是 seahorse")
	}
	if !cfg.Tools.ReadFile.Enabled {
		missing = append(missing, "read_file 工具未启用")
	}
	if !cfg.Tools.WriteFile.Enabled {
		missing = append(missing, "write_file 工具未启用")
	}
	if !cfg.Tools.EditFile.Enabled {
		missing = append(missing, "edit_file 工具未启用")
	}
	if !cfg.Tools.ListDir.Enabled {
		missing = append(missing, "list_dir 工具未启用")
	}
	if !cfg.Tools.Spawn.Enabled {
		missing = append(missing, "spawn 工具未启用")
	}
	if !cfg.Tools.Subagent.Enabled {
		missing = append(missing, "subagent/Guard 工具未启用")
	}
	return missing
}

func printSetupIntro(w io.Writer, st Status) {
	fmt.Fprintln(w, "Flyflor setup")
	fmt.Fprintln(w, "1. 检查配置文件")
	fmt.Fprintln(w, "2. 配置默认模型")
	fmt.Fprintln(w, "3. 修复 Agent TUI 黑板 / Bridge / Guard 工具")
	fmt.Fprintln(w, "4. 写入配置并提示下一步")
	fmt.Fprintln(w)
	if len(st.Missing) > 0 {
		fmt.Fprintln(w, "当前缺失:")
		for _, item := range st.Missing {
			fmt.Fprintf(w, "  - %s\n", item)
		}
		fmt.Fprintln(w)
	}
}

func promptLine(reader *bufio.Reader, w io.Writer, label, def string) (string, error) {
	if def != "" {
		fmt.Fprintf(w, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(w, "%s: ", label)
	}
	text, err := reader.ReadString('\n')
	if err != nil && len(text) == 0 {
		return "", fmt.Errorf("read %s: %w", label, err)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return def, nil
	}
	return text, nil
}

func promptSecret(stdin io.Reader, stdout io.Writer, label string) (string, error) {
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		fmt.Fprintf(stdout, "%s: ", label)
		data, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(stdout)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", label, err)
		}
		return string(data), nil
	}
	reader := bufio.NewReader(stdin)
	return promptLine(reader, stdout, label, "")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
