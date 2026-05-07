package setup

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sipeed/picoclaw/pkg/config"
)

func TestRunCreatesReadyConfigNonInteractive(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PICOCLAW_HOME", home)
	var out bytes.Buffer

	err := Run(options{
		provider:       "openrouter",
		apiKey:         "sk-test",
		modelID:        "openai/gpt-5.4",
		alias:          "openrouter-gpt-5.4",
		yes:            true,
		nonInteractive: true,
		stdout:         &out,
		stdin:          strings.NewReader(""),
	})
	if err != nil {
		t.Fatalf("Run() error = %v\noutput:\n%s", err, out.String())
	}

	configPath := filepath.Join(home, "config.json")
	st := Check(configPath)
	if !st.ModelReady || !st.RuntimeReady {
		t.Fatalf("setup status not ready: %#v", st)
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Agents.Defaults.ModelName != "openrouter-gpt-5.4" {
		t.Fatalf("default model = %q", cfg.Agents.Defaults.ModelName)
	}
	if cfg.Agents.Defaults.ContextManager != "seahorse" {
		t.Fatalf("context manager = %q", cfg.Agents.Defaults.ContextManager)
	}
	if !cfg.Tools.Subagent.Enabled || !cfg.Tools.Spawn.Enabled || !cfg.Tools.ReadFile.Enabled {
		t.Fatalf("guard tools not enabled: subagent=%v spawn=%v read=%v", cfg.Tools.Subagent.Enabled, cfg.Tools.Spawn.Enabled, cfg.Tools.ReadFile.Enabled)
	}
}

func TestNeedsSetupForMissingConfig(t *testing.T) {
	if !NeedsSetup(filepath.Join(t.TempDir(), "config.json")) {
		t.Fatal("missing config should require setup")
	}
}

func TestRepairRuntimeDefaults(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.ContextManager = ""
	cfg.Tools.Subagent.Enabled = false
	cfg.Tools.Spawn.Enabled = false

	if missing := missingRuntimeTools(cfg); len(missing) == 0 {
		t.Fatal("expected missing runtime tools before repair")
	}
	repairRuntimeDefaults(cfg)
	if missing := missingRuntimeTools(cfg); len(missing) != 0 {
		t.Fatalf("missing after repair: %v", missing)
	}
}
