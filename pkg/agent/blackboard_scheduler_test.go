package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestBlackboardSchedulerSeparatesSessionsAndLimitsWorkers(t *testing.T) {
	scheduler := NewBlackboardScheduler(nil)

	sessionA, err := scheduler.BeginTurn("session-a")
	if err != nil {
		t.Fatalf("BeginTurn(session-a) error = %v", err)
	}
	if _, err := scheduler.BeginTurn("session-a"); !errors.Is(err, ErrBlackboardSessionBusy) {
		t.Fatalf("second BeginTurn(session-a) error = %v, want ErrBlackboardSessionBusy", err)
	}
	sessionB, err := scheduler.BeginTurn("session-b")
	if err != nil {
		t.Fatalf("BeginTurn(session-b) should be isolated from session-a, got %v", err)
	}
	sessionB.Done()
	sessionA.Done()
	sessionA2, err := scheduler.BeginTurn("session-a")
	if err != nil {
		t.Fatalf("BeginTurn(session-a) after Done() error = %v", err)
	}
	sessionA2.Done()

	worker, err := scheduler.AcquireWorker(context.Background(), "flyflor-planner")
	if err != nil {
		t.Fatalf("AcquireWorker(flyflor-planner) error = %v", err)
	}
	if _, err := scheduler.AcquireWorker(context.Background(), "flyflor-planner"); !errors.Is(err, ErrBlackboardWorkerBusy) {
		t.Fatalf("second AcquireWorker(flyflor-planner) error = %v, want ErrBlackboardWorkerBusy", err)
	}
	worker.Done()
	worker2, err := scheduler.AcquireWorker(context.Background(), "flyflor-planner")
	if err != nil {
		t.Fatalf("AcquireWorker(flyflor-planner) after Done() error = %v", err)
	}
	worker2.Done()
}

func TestBlackboardPromptContributorInjectsDefaultInternalWorkers(t *testing.T) {
	t.Setenv("PICOCLAW_BUILTIN_SKILLS", t.TempDir())
	scheduler := NewBlackboardScheduler(nil)
	builder := NewContextBuilder(t.TempDir())
	if err := builder.RegisterPromptContributor(blackboardPromptContributor{scheduler: scheduler}); err != nil {
		t.Fatalf("RegisterPromptContributor() error = %v", err)
	}

	messages := builder.BuildMessagesFromPrompt(PromptBuildRequest{
		SessionKey:     "session-a",
		CurrentMessage: "hello",
	})
	if len(messages) == 0 {
		t.Fatal("BuildMessagesFromPrompt returned no messages")
	}
	system := messages[0].Content
	for _, want := range []string{
		"Blackboard Workbench",
		"session-a",
		"flyflor-planner",
		"flyflor-reviewer",
		"Do not require external Codex, Copilot, Claude CLI, or OpenCode installations",
	} {
		if !strings.Contains(system, want) {
			t.Fatalf("system prompt missing %q:\n%s", want, system)
		}
	}
}
