package agent

import (
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sipeed/picoclaw/pkg/config"
	runtimeevents "github.com/sipeed/picoclaw/pkg/events"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
)

type BlackboardMode string

const (
	BlackboardModeDirect          BlackboardMode = "direct"
	BlackboardModeDirectWithWatch BlackboardMode = "direct-with-watch"
	BlackboardModeBlackboard      BlackboardMode = "blackboard"
)

const (
	defaultBlackboardDirectThreshold = 0.35
	defaultBlackboardThreshold       = 0.55
)

type ComplexityFeatures struct {
	TokenEstimate          int
	CodeBlockCount         int
	HasDiffOrStacktrace    bool
	SubtaskCount           int
	RequestsPlan           bool
	RequestsImplementation bool
	RequestsVerification   bool
	RequestsReview         bool
	ConversationDepth      int
	RecentToolCalls        int
	HasContinuationRef     bool
	HasMedia               bool
	HasRiskIntent          bool
	ExplicitBlackboard     bool
	CrossFileOrMultiStep   bool
}

type ComplexityAssessment struct {
	Mode            BlackboardMode
	Score           float64
	DirectThreshold float64
	Threshold       float64
	Reasons         []string
	HardGate        bool
	Features        ComplexityFeatures
}

type blackboardScoringConfig struct {
	enabled             bool
	directThreshold     float64
	threshold           float64
	allowAutoEscalation bool
}

func (al *AgentLoop) assessBlackboardComplexity(
	agent *AgentInstance,
	opts processOptions,
	history []providers.Message,
) *ComplexityAssessment {
	scoring := blackboardScoringConfigFromAgentDefaults(al.cfg)
	if !scoring.enabled {
		return &ComplexityAssessment{
			Mode:            BlackboardModeDirect,
			DirectThreshold: scoring.directThreshold,
			Threshold:       scoring.threshold,
			Reasons:         []string{"blackboard_routing_disabled"},
		}
	}
	if history == nil && agent != nil && agent.Sessions != nil && !opts.NoHistory {
		history = agent.Sessions.GetHistory(opts.Dispatch.SessionKey)
	}

	features := extractComplexityFeatures(opts.Dispatch.UserMessage, opts.Dispatch.Media, history)
	score, reasons := scoreComplexity(features)
	hardGate, hardGateReason := blackboardHardGate(features)
	if hardGate {
		reasons = append(reasons, hardGateReason)
	}

	mode := BlackboardModeDirect
	switch {
	case hardGate || score >= scoring.threshold:
		mode = BlackboardModeBlackboard
	case score >= scoring.directThreshold && scoring.allowAutoEscalation:
		mode = BlackboardModeDirectWithWatch
	}

	return &ComplexityAssessment{
		Mode:            mode,
		Score:           score,
		DirectThreshold: scoring.directThreshold,
		Threshold:       scoring.threshold,
		Reasons:         stableReasons(reasons),
		HardGate:        hardGate,
		Features:        features,
	}
}

func (al *AgentLoop) emitComplexityAssessed(scope turnEventScope, assessment *ComplexityAssessment) {
	if assessment == nil {
		return
	}
	al.emitEvent(
		runtimeevents.KindAgentComplexityAssessed,
		scope.meta(0, "blackboardRouting", "turn.complexity.assessed"),
		ComplexityAssessedPayload{
			Mode:            string(assessment.Mode),
			Score:           assessment.Score,
			DirectThreshold: assessment.DirectThreshold,
			Threshold:       assessment.Threshold,
			Reasons:         append([]string(nil), assessment.Reasons...),
			HardGate:        assessment.HardGate,
			Features:        assessment.Features,
		},
	)
	loggerFields := map[string]any{
		"agent_id":         scope.agentID,
		"session_key":      scope.sessionKey,
		"mode":             assessment.Mode,
		"score":            assessment.Score,
		"direct_threshold": assessment.DirectThreshold,
		"threshold":        assessment.Threshold,
		"reasons":          strings.Join(assessment.Reasons, ","),
	}
	if assessment.Mode == BlackboardModeBlackboard {
		logger.InfoCF("agent", "Blackboard routing selected workbench", loggerFields)
	} else {
		logger.DebugCF("agent", "Blackboard routing selected direct mode", loggerFields)
	}
}

func blackboardScoringConfigFromAgentDefaults(cfg *config.Config) blackboardScoringConfig {
	out := blackboardScoringConfig{
		enabled:             true,
		directThreshold:     defaultBlackboardDirectThreshold,
		threshold:           defaultBlackboardThreshold,
		allowAutoEscalation: true,
	}
	if cfg == nil {
		return out
	}
	rc := cfg.Agents.Defaults.BlackboardRouting
	if rc == nil {
		return out
	}
	if rc.Enabled != nil {
		out.enabled = *rc.Enabled
	}
	if rc.DirectThreshold > 0 {
		out.directThreshold = clamp01(rc.DirectThreshold)
	}
	if rc.Threshold > 0 {
		out.threshold = clamp01(rc.Threshold)
	}
	if out.threshold <= out.directThreshold {
		out.threshold = math.Min(1, out.directThreshold+0.20)
	}
	if rc.AllowAutoEscalation != nil {
		out.allowAutoEscalation = *rc.AllowAutoEscalation
	}
	return out
}

func defaultBlackboardWeights() map[string]float64 {
	return map[string]float64{
		"length_medium":           0.08,
		"length_long":             0.16,
		"length_very_long":        0.24,
		"code_block":              0.14,
		"diff_or_stacktrace":      0.16,
		"multiple_code_blocks":    0.06,
		"multiple_subtasks":       0.16,
		"plan_requested":          0.12,
		"implementation":          0.12,
		"verification":            0.12,
		"review":                  0.12,
		"deep_conversation":       0.08,
		"recent_tool_calls":       0.08,
		"dense_recent_tool_calls": 0.12,
		"continuation_ref":        0.08,
		"media":                   0.18,
		"risk_intent":             0.08,
		"cross_file_or_multistep": 0.14,
	}
}

func scoreComplexity(features ComplexityFeatures) (float64, []string) {
	var score float64
	var reasons []string
	weights := defaultBlackboardWeights()
	add := func(key string) {
		score += weights[key]
		reasons = append(reasons, key)
	}

	switch {
	case features.TokenEstimate > 600:
		add("length_very_long")
	case features.TokenEstimate > 200:
		add("length_long")
	case features.TokenEstimate > 50:
		add("length_medium")
	}
	if features.CodeBlockCount > 0 {
		add("code_block")
	}
	if features.CodeBlockCount > 1 {
		add("multiple_code_blocks")
	}
	if features.HasDiffOrStacktrace {
		add("diff_or_stacktrace")
	}
	if features.SubtaskCount >= 2 {
		add("multiple_subtasks")
	}
	if features.RequestsPlan {
		add("plan_requested")
	}
	if features.RequestsImplementation {
		add("implementation")
	}
	if features.RequestsVerification {
		add("verification")
	}
	if features.RequestsReview {
		add("review")
	}
	if features.ConversationDepth > 10 {
		add("deep_conversation")
	}
	switch {
	case features.RecentToolCalls > 3:
		add("dense_recent_tool_calls")
	case features.RecentToolCalls > 0:
		add("recent_tool_calls")
	}
	if features.HasContinuationRef {
		add("continuation_ref")
	}
	if features.HasMedia {
		add("media")
	}
	if features.HasRiskIntent {
		add("risk_intent")
	}
	if features.CrossFileOrMultiStep {
		add("cross_file_or_multistep")
	}

	return clamp01(score), reasons
}

func blackboardHardGate(features ComplexityFeatures) (bool, string) {
	if features.ExplicitBlackboard {
		return true, "hard_gate_explicit_blackboard"
	}
	if features.TokenEstimate > 1200 || features.CodeBlockCount >= 3 {
		return true, "hard_gate_large_input"
	}
	if features.RequestsImplementation && features.RequestsVerification {
		return true, "hard_gate_implement_and_verify"
	}
	if features.RequestsImplementation && features.RequestsReview {
		return true, "hard_gate_implement_and_review"
	}
	if features.CrossFileOrMultiStep && (features.RequestsImplementation || features.RequestsVerification || features.RequestsReview) {
		return true, "hard_gate_cross_file_workflow"
	}
	return false, ""
}

func extractComplexityFeatures(msg string, media []string, history []providers.Message) ComplexityFeatures {
	lower := strings.ToLower(msg)
	return ComplexityFeatures{
		TokenEstimate:          estimateComplexityTokens(msg),
		CodeBlockCount:         strings.Count(msg, "```") / 2,
		HasDiffOrStacktrace:    hasDiffOrStacktrace(lower),
		SubtaskCount:           estimateSubtaskCount(msg, lower),
		RequestsPlan:           containsAny(lower, planIntentTerms()),
		RequestsImplementation: containsAny(lower, implementationIntentTerms()),
		RequestsVerification:   containsAny(lower, verificationIntentTerms()),
		RequestsReview:         containsAny(lower, reviewIntentTerms()),
		ConversationDepth:      len(history),
		RecentToolCalls:        countComplexityRecentToolCalls(history),
		HasContinuationRef:     containsAny(lower, continuationTerms()),
		HasMedia:               len(media) > 0 || messageLooksLikeMedia(lower),
		HasRiskIntent:          containsAny(lower, riskIntentTerms()),
		ExplicitBlackboard:     containsAny(lower, explicitBlackboardTerms()),
		CrossFileOrMultiStep:   hasCrossFileOrMultiStep(lower),
	}
}

func estimateComplexityTokens(msg string) int {
	total := utf8.RuneCountInString(msg)
	if total == 0 {
		return 0
	}
	cjk := 0
	for _, r := range msg {
		if r >= 0x2E80 && r <= 0x9FFF || r >= 0xF900 && r <= 0xFAFF || r >= 0xAC00 && r <= 0xD7AF {
			cjk++
		}
	}
	return cjk + (total-cjk)/4
}

func countComplexityRecentToolCalls(history []providers.Message) int {
	start := len(history) - 6
	if start < 0 {
		start = 0
	}
	var count int
	for _, msg := range history[start:] {
		count += len(msg.ToolCalls)
	}
	return count
}

func estimateSubtaskCount(msg, lower string) int {
	count := 0
	for _, line := range strings.Split(msg, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			count++
			continue
		}
		first, _ := utf8.DecodeRuneInString(line)
		if unicode.IsDigit(first) && (strings.Contains(line, ". ") || strings.Contains(line, "、")) {
			count++
		}
	}
	connectors := []string{" and ", " then ", " also ", "并且", "然后", "同时", "以及", "再"}
	for _, connector := range connectors {
		if strings.Contains(lower, connector) {
			count++
		}
	}
	return count
}

func hasDiffOrStacktrace(lower string) bool {
	return strings.Contains(lower, "\n+++") ||
		strings.Contains(lower, "\n---") ||
		strings.Contains(lower, "\n@@") ||
		strings.Contains(lower, "traceback (most recent call last)") ||
		strings.Contains(lower, "stack trace") ||
		strings.Contains(lower, "panic:") ||
		strings.Contains(lower, "exception:")
}

func hasCrossFileOrMultiStep(lower string) bool {
	return containsAny(lower, []string{
		"multiple files", "several files", "across files", "across modules",
		"multi-step", "end to end", "e2e", "全流程", "多步骤", "多个文件", "多个模块", "跨文件", "跨模块",
	})
}

func messageLooksLikeMedia(lower string) bool {
	if strings.Contains(lower, "data:image/") ||
		strings.Contains(lower, "data:audio/") ||
		strings.Contains(lower, "data:video/") {
		return true
	}
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".mp3", ".wav", ".ogg", ".m4a", ".flac", ".mp4", ".avi", ".mov", ".webm"} {
		if strings.Contains(lower, ext) {
			return true
		}
	}
	return false
}

func containsAny(s string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(s, term) {
			return true
		}
	}
	return false
}

func stableReasons(reasons []string) []string {
	if len(reasons) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(reasons))
	out := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		if reason == "" {
			continue
		}
		if _, ok := seen[reason]; ok {
			continue
		}
		seen[reason] = struct{}{}
		out = append(out, reason)
	}
	sort.Strings(out)
	return out
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

func planIntentTerms() []string {
	return []string{"plan", "方案", "规划", "拆解", "设计", "architecture", "roadmap"}
}

func implementationIntentTerms() []string {
	return []string{"implement", "implementation", "fix", "refactor", "change the code", "edit", "write code", "实现", "修复", "重构", "修改", "改代码", "写代码"}
}

func verificationIntentTerms() []string {
	return []string{"test", "verify", "validate", "run tests", "测试", "验证", "校验"}
}

func reviewIntentTerms() []string {
	return []string{"review", "risk", "regression", "audit", "复核", "评审", "风险", "回归"}
}

func continuationTerms() []string {
	return []string{"continue", "follow up", "above", "previous", "刚才", "上面", "继续", "接着", "前面"}
}

func riskIntentTerms() []string {
	return []string{"shell", "command", "terminal", "network", "web", "commit", "pull request", "pr", "delete", "rename", "exec", "命令", "终端", "联网", "提交", "删除", "重命名"}
}

func explicitBlackboardTerms() []string {
	return []string{"blackboard", "multi-agent", "multiple agents", "planner", "reviewer", "黑板", "多 agent", "多智能体", "规划器", "复核器"}
}
