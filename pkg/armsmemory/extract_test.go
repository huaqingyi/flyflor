package armsmemory

import (
	"context"
	"strings"
	"testing"
)

func TestExtractMethodologiesOnlyAssistantReflectionDraft(t *testing.T) {
	user := ExtractMethodologies(MessageInput{
		Role:    "user",
		Content: "## Methodology Reflection Draft\n- Situation: user text should not index\n- Method: ignore",
	}, 0)
	if len(user) != 0 {
		t.Fatalf("user reflection extraction = %d, want 0", len(user))
	}

	plain := ExtractMethodologies(MessageInput{
		Role:    "assistant",
		Content: "普通交付结果，不含方法论草稿。",
	}, 0)
	if len(plain) != 0 {
		t.Fatalf("plain assistant extraction = %d, want 0", len(plain))
	}

	got := ExtractMethodologies(MessageInput{
		Role: "assistant",
		Content: `完成。

## Methodology Reflection Draft

- Situation: When blackboard work reaches a repeated blocker.
- Method: Stop internal debate, summarize options, and ask the user with a decision form.
- Avoid: Continuing hidden discussion after the hard round budget.
- Next-time hint: blackboard deadlock decision form

## Other Section

This should not be indexed.`,
	}, 0)
	if len(got) != 1 {
		t.Fatalf("reflection extraction = %d, want 1: %#v", len(got), got)
	}
	if got[0].Situation != "When blackboard work reaches a repeated blocker." {
		t.Fatalf("Situation = %q", got[0].Situation)
	}
	if !strings.Contains(got[0].Method, "decision form") {
		t.Fatalf("Method = %q, want decision form content", got[0].Method)
	}
	if strings.Contains(got[0].Text, "Other Section") {
		t.Fatalf("methodology text leaked next section: %q", got[0].Text)
	}
}

func TestManagerIndexesOnlyMethodologyDrafts(t *testing.T) {
	store := &fakeARMSStore{}
	mgr := NewWithStore(Config{Enabled: true, SpaceID: "test-space"}, store)

	count, err := mgr.IndexMessage(context.Background(), "session-1", MessageInput{
		Role:    "assistant",
		Content: "已完成普通任务，没有反思草稿。",
	})
	if err != nil {
		t.Fatalf("IndexMessage plain: %v", err)
	}
	if count != 0 || len(store.upserts) != 0 {
		t.Fatalf("plain message indexed count=%d upserts=%d, want 0", count, len(store.upserts))
	}

	count, err = mgr.IndexMessage(context.Background(), "session-1", MessageInput{
		Role: "assistant",
		Content: `## Methodology Reflection Draft

- Situation: When a feature needs stable rollout.
- Method: Put the convention in code and document the small config surface.
- Avoid: Exposing every weight as runtime config.
- Next-time hint: convention over configuration`,
	})
	if err != nil {
		t.Fatalf("IndexMessage reflection: %v", err)
	}
	if count != 1 {
		t.Fatalf("IndexMessage reflection count = %d, want 1", count)
	}
	if len(store.upserts) != 1 {
		t.Fatalf("upsert batches = %d, want 1", len(store.upserts))
	}
	doc := store.upserts[0][0]
	if doc.Namespace != defaultMethodologyNamespace {
		t.Fatalf("Namespace = %q", doc.Namespace)
	}
	if doc.SessionKey != "session-1" {
		t.Fatalf("SessionKey = %q", doc.SessionKey)
	}
	if doc.ID == "" {
		t.Fatal("ID should be stable and non-empty")
	}
}

func TestFormatHitsIdentifiesARMSIsolation(t *testing.T) {
	formatted := FormatHits([]SearchHit{{
		Title:        "decision form",
		Text:         "- Method: Ask the user when blackboard deadlocks.",
		NextTimeHint: "blackboard deadlock",
		Score:        0.82,
	}})
	for _, want := range []string{"METHODOLOGY_MEMORY", "ARMS", "self-growth methods only", "blackboard deadlock"} {
		if !strings.Contains(formatted, want) {
			t.Fatalf("FormatHits missing %q:\n%s", want, formatted)
		}
	}
}

type fakeARMSStore struct {
	upserts [][]Methodology
}

func (s *fakeARMSStore) UpsertMethodologies(_ context.Context, methodologies []Methodology) error {
	s.upserts = append(s.upserts, append([]Methodology(nil), methodologies...))
	return nil
}

func (s *fakeARMSStore) SearchMethodologies(_ context.Context, _ string, _ int, _ float64) ([]SearchHit, error) {
	return nil, nil
}

func (s *fakeARMSStore) Status(context.Context) (Status, error) {
	return Status{Enabled: true, Ready: true}, nil
}
