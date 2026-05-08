package agent

import (
	"strings"
	"testing"

	"github.com/sipeed/picoclaw/pkg/config"
)

func TestAssessBlackboardComplexityModes(t *testing.T) {
	cfg := config.DefaultConfig()
	al := &AgentLoop{cfg: cfg}

	simple := al.assessBlackboardComplexity(nil, processOptions{
		Dispatch:  DispatchRequest{UserMessage: "hello"},
		NoHistory: true,
	}, nil)
	if simple.Mode != BlackboardModeDirect {
		t.Fatalf("simple mode = %q, want %q (score=%v reasons=%v)", simple.Mode, BlackboardModeDirect, simple.Score, simple.Reasons)
	}

	gray := al.assessBlackboardComplexity(nil, processOptions{
		Dispatch:  DispatchRequest{UserMessage: "Please propose a plan and review the tradeoffs. " + strings.Repeat("word ", 260)},
		NoHistory: true,
	}, nil)
	if gray.Mode != BlackboardModeDirectWithWatch {
		t.Fatalf("gray mode = %q, want %q (score=%v reasons=%v)", gray.Mode, BlackboardModeDirectWithWatch, gray.Score, gray.Reasons)
	}

	complex := al.assessBlackboardComplexity(nil, processOptions{
		Dispatch:  DispatchRequest{UserMessage: "Implement the fix and run tests to verify it."},
		NoHistory: true,
	}, nil)
	if complex.Mode != BlackboardModeBlackboard {
		t.Fatalf("complex mode = %q, want %q (score=%v reasons=%v)", complex.Mode, BlackboardModeBlackboard, complex.Score, complex.Reasons)
	}
	if !complex.HardGate {
		t.Fatalf("complex.HardGate = false, want true")
	}
}

func TestAssessBlackboardComplexityRiskIntentOnlyDoesNotHardGate(t *testing.T) {
	cfg := config.DefaultConfig()
	al := &AgentLoop{cfg: cfg}

	got := al.assessBlackboardComplexity(nil, processOptions{
		Dispatch:  DispatchRequest{UserMessage: "Can you explain what this shell command means?"},
		NoHistory: true,
	}, nil)
	if got.HardGate {
		t.Fatalf("risk intent alone should not hard gate: score=%v reasons=%v", got.Score, got.Reasons)
	}
	if got.Mode == BlackboardModeBlackboard {
		t.Fatalf("risk intent alone selected blackboard: score=%v reasons=%v", got.Score, got.Reasons)
	}
}

func TestDirectWithWatchEscalationMonitor(t *testing.T) {
	ts := &turnState{
		opts: processOptions{BlackboardMode: BlackboardModeDirectWithWatch},
	}
	ts.noteToolExecution("read_file", false)
	ts.noteToolExecution("grep", false)
	if reason := ts.blackboardEscalationReason(); reason != "" {
		t.Fatalf("reason after two tools = %q, want empty", reason)
	}
	ts.noteToolExecution("run_tests", false)
	if reason := ts.blackboardEscalationReason(); reason != "tool_count_threshold" {
		t.Fatalf("reason after third tool = %q, want tool_count_threshold", reason)
	}
}

func TestDirectWithWatchRepeatedToolFailureEscalation(t *testing.T) {
	ts := &turnState{
		opts: processOptions{BlackboardMode: BlackboardModeDirectWithWatch},
	}
	ts.noteToolExecution("read_file", true)
	ts.noteToolExecution("read_file", true)
	if reason := ts.blackboardEscalationReason(); reason != "repeated_tool_failure" {
		t.Fatalf("reason = %q, want repeated_tool_failure", reason)
	}
}
