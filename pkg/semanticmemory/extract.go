package semanticmemory

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

type MessageInput struct {
	Role    string
	Content string
}

type Candidate struct {
	Kind string
	Text string
}

var codeFenceRE = regexp.MustCompile("(?s)```.*?```")
var sentenceSplitRE = regexp.MustCompile(`[。！？!?；;]\s*|\n+`)

var userPreferenceKeywords = []string{
	"我需要", "我想", "我希望", "必须", "一定要", "不要", "不能", "以后", "记住", "偏好", "喜欢", "请让", "我们需要",
	"默认", "要求", "需要", "更好", "更清晰",
}

var projectKeywords = []string{
	"flyflor", "飞花", "bridge", "qdrant", "sqlite", "markdown", "md", "黑板", "tui", "webui", "ui",
	"智能体", "记忆", "架构", "codex", "claude", "opencode", "seahorse",
}

var memoryKeywords = []string{
	"三层记忆", "长期记忆", "向量", "索引", "提取", "压缩", "召回", "qdrant", "sqlite", "markdown", "md", "越使用越聪明",
}

var assistantOutcomeKeywords = []string{
	"已", "实现", "新增", "修改", "修复", "验证", "通过", "失败", "结论", "建议", "风险", "设计", "测试", "部署",
}

const maxCandidateChars = 360

func ExtractCandidates(msg MessageInput, maxChars int) []Candidate {
	role := strings.ToLower(strings.TrimSpace(msg.Role))
	if role != "user" && role != "assistant" {
		return nil
	}
	content := compressText(msg.Content, maxChars)
	if len([]rune(content)) < 24 {
		return nil
	}

	var candidates []Candidate
	segments := memorableSegments(content, maxChars)
	if role == "user" {
		for _, segment := range segments {
			if containsAny(segment, userPreferenceKeywords...) {
				candidates = append(candidates, Candidate{Kind: "preference", Text: "用户偏好/需求：" + segment})
			}
			if containsAny(segment, projectKeywords...) {
				candidates = append(candidates, Candidate{Kind: "project", Text: "项目事实/约束：" + segment})
			}
			if containsAny(segment, memoryKeywords...) {
				candidates = append(candidates, Candidate{Kind: "memory_design", Text: "记忆系统设计要求：" + segment})
			}
		}
		if len(candidates) == 0 && len([]rune(content)) >= 180 {
			candidates = append(candidates, Candidate{Kind: "user_summary", Text: "用户长文本摘要：" + content})
		}
	} else {
		for _, segment := range segments {
			if containsAny(segment, assistantOutcomeKeywords...) {
				candidates = append(candidates, Candidate{Kind: "outcome", Text: "交付/验证经验：" + segment})
			}
		}
	}

	return dedupeCandidates(candidates)
}

func compressText(text string, maxChars int) string {
	text = codeFenceRE.ReplaceAllString(text, "[code omitted]")
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "|") && strings.HasSuffix(line, "|") {
			continue
		}
		kept = append(kept, line)
	}
	text = strings.Join(kept, " ")
	text = strings.Join(strings.Fields(text), " ")
	if maxChars <= 0 {
		maxChars = 900
	}
	runes := []rune(text)
	if len(runes) > maxChars {
		return string(runes[:maxChars]) + "..."
	}
	return text
}

func memorableSegments(content string, maxChars int) []string {
	if maxChars <= 0 {
		maxChars = 900
	}
	if maxChars > maxCandidateChars {
		maxChars = maxCandidateChars
	}
	parts := sentenceSplitRE.Split(content, -1)
	segments := make([]string, 0, len(parts)+1)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if utf8.RuneCountInString(part) < 12 {
			continue
		}
		segments = append(segments, truncateRunes(part, maxChars))
	}
	if len(segments) == 0 {
		segments = append(segments, truncateRunes(content, maxChars))
	}
	return segments
}

func truncateRunes(text string, maxChars int) string {
	runes := []rune(strings.TrimSpace(text))
	if maxChars <= 0 || len(runes) <= maxChars {
		return string(runes)
	}
	return string(runes[:maxChars]) + "..."
}

func containsAny(text string, needles ...string) bool {
	lower := strings.ToLower(text)
	for _, needle := range needles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func dedupeCandidates(candidates []Candidate) []Candidate {
	seen := map[string]struct{}{}
	out := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		key := candidate.Kind + "\x00" + candidate.Text
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, candidate)
	}
	return out
}
