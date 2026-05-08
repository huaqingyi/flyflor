package agent

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	ErrBlackboardSessionBusy    = errors.New("blackboard session is already active")
	ErrBlackboardWorkerBusy     = errors.New("blackboard worker is busy")
	ErrBlackboardWorkerNotFound = errors.New("blackboard worker not found")
)

type BlackboardWorkerKind string

const (
	BlackboardWorkerKindAgent BlackboardWorkerKind = "agent"
	BlackboardWorkerKindTUI   BlackboardWorkerKind = "tui"
)

type BlackboardWorkerSpec struct {
	ID              string
	Label           string
	Kind            BlackboardWorkerKind
	Specialty       string
	MaxConcurrency  int
	MaxContextRunes int
	Default         bool
}

type blackboardSessionState struct {
	active bool
	turns  uint64
}

type blackboardWorkerState struct {
	spec BlackboardWorkerSpec
	sem  chan struct{}
}

type BlackboardScheduler struct {
	mu       sync.Mutex
	sessions map[string]*blackboardSessionState
	workers  map[string]*blackboardWorkerState
	order    []string
}

type BlackboardPolicy struct {
	MaxRounds         int
	HardMaxRounds     int
	LivelockDetection bool
	AskUserOnDeadlock bool
}

const (
	BlackboardDefaultMaxRounds  = 3
	BlackboardHardMaxRounds     = 5
	BlackboardLivelockDetection = true
	BlackboardAskUserOnDeadlock = true
)

type BlackboardTurnLease struct {
	scheduler  *BlackboardScheduler
	sessionKey string
	once       sync.Once
}

type BlackboardWorkerLease struct {
	scheduler *BlackboardScheduler
	workerID  string
	once      sync.Once
}

func NewBlackboardScheduler(workers []BlackboardWorkerSpec) *BlackboardScheduler {
	s := &BlackboardScheduler{
		sessions: make(map[string]*blackboardSessionState),
		workers:  make(map[string]*blackboardWorkerState),
	}
	for _, worker := range workers {
		_ = s.RegisterWorker(worker)
	}
	if len(s.workers) == 0 {
		for _, worker := range DefaultBlackboardWorkers() {
			_ = s.RegisterWorker(worker)
		}
	}
	return s
}

func DefaultBlackboardWorkers() []BlackboardWorkerSpec {
	return []BlackboardWorkerSpec{
		{
			ID:              "flyflor-planner",
			Label:           "Flyflor Planner",
			Kind:            BlackboardWorkerKindAgent,
			Specialty:       "拆解目标、提炼上下文、提出执行路径和验证点",
			MaxConcurrency:  1,
			MaxContextRunes: 12000,
			Default:         true,
		},
		{
			ID:              "flyflor-reviewer",
			Label:           "Flyflor Reviewer",
			Kind:            BlackboardWorkerKindAgent,
			Specialty:       "复核约束、遗漏、风险、边界条件和最终可读性",
			MaxConcurrency:  1,
			MaxContextRunes: 12000,
			Default:         true,
		},
	}
}

func (s *BlackboardScheduler) RegisterWorker(worker BlackboardWorkerSpec) error {
	if s == nil {
		return errors.New("blackboard scheduler is nil")
	}
	worker.ID = strings.TrimSpace(worker.ID)
	if worker.ID == "" {
		return errors.New("blackboard worker id is required")
	}
	if worker.Kind == "" {
		worker.Kind = BlackboardWorkerKindAgent
	}
	if worker.MaxConcurrency <= 0 {
		worker.MaxConcurrency = 1
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.workers[worker.ID]; !exists {
		s.order = append(s.order, worker.ID)
		sort.Strings(s.order)
	}
	s.workers[worker.ID] = &blackboardWorkerState{
		spec: worker,
		sem:  make(chan struct{}, worker.MaxConcurrency),
	}
	return nil
}

func (s *BlackboardScheduler) BeginTurn(sessionKey string) (*BlackboardTurnLease, error) {
	if s == nil {
		return nil, errors.New("blackboard scheduler is nil")
	}
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		sessionKey = "default"
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.sessions[sessionKey]
	if state == nil {
		state = &blackboardSessionState{}
		s.sessions[sessionKey] = state
	}
	if state.active {
		return nil, fmt.Errorf("%w: %s", ErrBlackboardSessionBusy, sessionKey)
	}
	state.active = true
	state.turns++
	return &BlackboardTurnLease{scheduler: s, sessionKey: sessionKey}, nil
}

func (l *BlackboardTurnLease) Done() {
	if l == nil || l.scheduler == nil {
		return
	}
	l.once.Do(func() {
		l.scheduler.mu.Lock()
		defer l.scheduler.mu.Unlock()
		if state := l.scheduler.sessions[l.sessionKey]; state != nil {
			state.active = false
		}
	})
}

func (s *BlackboardScheduler) AcquireWorker(ctx context.Context, workerID string) (*BlackboardWorkerLease, error) {
	if s == nil {
		return nil, errors.New("blackboard scheduler is nil")
	}
	workerID = strings.TrimSpace(workerID)
	s.mu.Lock()
	state := s.workers[workerID]
	s.mu.Unlock()
	if state == nil {
		return nil, fmt.Errorf("%w: %s", ErrBlackboardWorkerNotFound, workerID)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case state.sem <- struct{}{}:
		return &BlackboardWorkerLease{scheduler: s, workerID: workerID}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return nil, fmt.Errorf("%w: %s", ErrBlackboardWorkerBusy, workerID)
	}
}

func (l *BlackboardWorkerLease) Done() {
	if l == nil || l.scheduler == nil {
		return
	}
	l.once.Do(func() {
		l.scheduler.mu.Lock()
		state := l.scheduler.workers[l.workerID]
		l.scheduler.mu.Unlock()
		if state == nil {
			return
		}
		select {
		case <-state.sem:
		default:
		}
	})
}

func (s *BlackboardScheduler) DefaultWorkers() []BlackboardWorkerSpec {
	if s == nil {
		return DefaultBlackboardWorkers()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var workers []BlackboardWorkerSpec
	for _, id := range s.order {
		state := s.workers[id]
		if state != nil && state.spec.Default {
			workers = append(workers, state.spec)
		}
	}
	return workers
}

func (s *BlackboardScheduler) SessionActive(sessionKey string) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.sessions[strings.TrimSpace(sessionKey)]
	return state != nil && state.active
}

func DefaultBlackboardPolicy() BlackboardPolicy {
	return BlackboardPolicy{
		MaxRounds:         BlackboardDefaultMaxRounds,
		HardMaxRounds:     BlackboardHardMaxRounds,
		LivelockDetection: BlackboardLivelockDetection,
		AskUserOnDeadlock: BlackboardAskUserOnDeadlock,
	}
}

func (s *BlackboardScheduler) PromptContent(sessionKey string) string {
	policy := DefaultBlackboardPolicy()
	workers := s.DefaultWorkers()
	var b strings.Builder
	b.WriteString("## Blackboard Workbench\n\n")
	b.WriteString("Flyflor uses an internal blackboard scheduler for this turn. Do not require external Codex, Copilot, Claude CLI, or OpenCode installations for the default discussion.\n\n")
	if strings.TrimSpace(sessionKey) != "" {
		b.WriteString("- Session workbench: `")
		b.WriteString(sessionKey)
		b.WriteString("` is isolated from other sessions.\n")
	}
	b.WriteString("- Scheduler policy: keep worker context compact, avoid copying full history into worker notes, and merge only useful conclusions back into the final answer.\n")
	b.WriteString(fmt.Sprintf("- Convergence budget: complete the worker discussion in at most %d rounds; %d is a hard upper bound and must not be exceeded.\n", policy.MaxRounds, policy.HardMaxRounds))
	if policy.LivelockDetection {
		b.WriteString("- Livelock detection: stop early if two consecutive rounds add no new facts, repeat the same disagreement, keep the same blocker open, or retry the same failing tool path.\n")
	}
	b.WriteString("- Default workflow: Blackboard Workbench coordinates the workers below, lets them challenge each other briefly, then Flyflor produces the final user-facing answer.\n\n")
	b.WriteString("Default workers:\n")
	for _, worker := range workers {
		b.WriteString("- `")
		b.WriteString(worker.ID)
		b.WriteString("` (")
		b.WriteString(worker.Label)
		b.WriteString(", ")
		b.WriteString(string(worker.Kind))
		b.WriteString("): ")
		b.WriteString(worker.Specialty)
		if worker.MaxContextRunes > 0 {
			b.WriteString(fmt.Sprintf("；context budget about %d runes", worker.MaxContextRunes))
		}
		b.WriteString(".\n")
	}
	if policy.AskUserOnDeadlock {
		b.WriteString("\nDeadlock handoff:\n")
		b.WriteString("- If the discussion reaches the round budget without a stable answer, do not continue debating internally.\n")
		b.WriteString("- Return a concise summary of what is known, what is blocked, and what the user must decide.\n")
		b.WriteString("- Include a machine-readable decision form exactly once, using this fenced block format:\n\n")
		b.WriteString("```flyflor-decision-form\n")
		b.WriteString("{\n")
		b.WriteString("  \"version\": 1,\n")
		b.WriteString("  \"title\": \"Decision needed\",\n")
		b.WriteString("  \"summary\": \"One short sentence describing why Flyflor needs user input.\",\n")
		b.WriteString("  \"single_select\": {\"id\": \"path\", \"label\": \"Choose one path\", \"options\": [{\"id\": \"recommended\", \"label\": \"Recommended\", \"description\": \"Why this is safest.\"}]},\n")
		b.WriteString("  \"multi_select\": {\"id\": \"constraints\", \"label\": \"Optional constraints\", \"options\": [{\"id\": \"add_tests\", \"label\": \"Add tests\", \"description\": \"Include verification before final delivery.\"}]},\n")
		b.WriteString("  \"custom_input\": {\"id\": \"notes\", \"label\": \"Additional context\", \"placeholder\": \"Tell Flyflor any missing constraint.\"}\n")
		b.WriteString("}\n")
		b.WriteString("```\n")
	}
	b.WriteString("\nReflection handoff:\n")
	b.WriteString("- When this turn reveals a reusable method, add a short `Methodology Reflection Draft` section in Markdown: situation, method, avoid, next-time hint.\n")
	b.WriteString("- Keep it concise; later Flyflor memory will store these drafts like skill-style methodology notes.\n")
	b.WriteString("\nWhen answering, synthesize the workers' discussion without exposing noisy raw logs unless the user opens the blackboard.")
	return b.String()
}

type blackboardPromptContributor struct {
	scheduler *BlackboardScheduler
}

func (c blackboardPromptContributor) PromptSource() PromptSourceDescriptor {
	return PromptSourceDescriptor{
		ID:              PromptSourceBlackboard,
		Owner:           "blackboard",
		Description:     "Internal blackboard scheduler and default worker roles",
		Allowed:         []PromptPlacement{{Layer: PromptLayerInstruction, Slot: PromptSlotWorkspace}},
		StableByDefault: true,
	}
}

func (c blackboardPromptContributor) ContributePrompt(_ context.Context, req PromptBuildRequest) ([]PromptPart, error) {
	if req.BlackboardMode != BlackboardModeBlackboard {
		return nil, nil
	}
	scheduler := c.scheduler
	if scheduler == nil {
		scheduler = NewBlackboardScheduler(nil)
	}
	return []PromptPart{
		{
			ID:      "instruction.blackboard.workers",
			Layer:   PromptLayerInstruction,
			Slot:    PromptSlotWorkspace,
			Source:  PromptSource{ID: PromptSourceBlackboard, Name: "blackboard.scheduler"},
			Title:   "Blackboard worker scheduler",
			Content: scheduler.PromptContent(req.SessionKey),
			Stable:  true,
			Cache:   PromptCacheEphemeral,
		},
	}, nil
}
