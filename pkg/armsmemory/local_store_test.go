//go:build !mipsle && !netbsd && !(freebsd && arm)

package armsmemory

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStorePersistsAndSearchesMethodologies(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "arms.db")
	cfg := Config{
		Enabled:              true,
		Driver:               "local",
		Path:                 dbPath,
		SpaceID:              "test-methodologies",
		MethodologyNamespace: "methodology",
		Dimensions:           64,
		TopK:                 3,
		MinScore:             0.01,
	}
	store, err := NewLocalStore(cfg)
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	err = store.UpsertMethodologies(ctx, []Methodology{
		{
			ID:           "blackboard-deadlock",
			Title:        "blackboard deadlock",
			Text:         "- Situation: Blackboard discussion repeats the same blocker.\n- Method: Return a decision form for human choice.\n- Avoid: Continuing internal debate.",
			Situation:    "Blackboard discussion repeats the same blocker.",
			Method:       "Return a decision form for human choice.",
			Avoid:        "Continuing internal debate.",
			NextTimeHint: "blackboard decision form",
			Tags:         []string{"blackboard"},
			Source:       "flyflor.reflection",
		},
		{
			ID:           "docs-deploy",
			Title:        "deployment docs",
			Text:         "- Situation: Installer docs need validation.\n- Method: Check command examples and run script syntax tests.",
			Situation:    "Installer docs need validation.",
			Method:       "Check command examples and run script syntax tests.",
			NextTimeHint: "install docs validation",
			Tags:         []string{"docs"},
			Source:       "flyflor.reflection",
		},
	})
	if err != nil {
		t.Fatalf("UpsertMethodologies: %v", err)
	}

	hits, err := store.SearchMethodologies(ctx, "黑板讨论卡住，需要让用户做选择", 2, 0.01)
	if err != nil {
		t.Fatalf("SearchMethodologies: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("expected local ARMS search hit")
	}
	if !strings.Contains(hits[0].Text, "decision form") {
		t.Fatalf("top hit = %#v, want blackboard decision methodology", hits[0])
	}

	reopened, err := NewLocalStore(cfg)
	if err != nil {
		t.Fatalf("reopen NewLocalStore: %v", err)
	}
	hits, err = reopened.SearchMethodologies(ctx, "installer documentation validation", 2, 0.01)
	if err != nil {
		t.Fatalf("reopened SearchMethodologies: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("expected reopened local ARMS search hit")
	}
}

func TestNewManagerUsesLocalDriver(t *testing.T) {
	mgr, err := New(Config{
		Enabled: true,
		Driver:  "local",
		Path:    filepath.Join(t.TempDir(), "arms.db"),
	})
	if err != nil {
		t.Fatalf("New local manager: %v", err)
	}
	status := mgr.Status(context.Background())
	if !status.Ready || status.Driver != "local" {
		t.Fatalf("Status = %#v, want ready local", status)
	}
}
