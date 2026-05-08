package semanticmemory

import (
	"context"
	"strings"
	"testing"
)

func TestExtractCandidatesSplitsUserMemoryDesign(t *testing.T) {
	msg := MessageInput{
		Role:    "user",
		Content: "有个严重问题，黑板上的对话我有些看不懂。请让每一个我向 Flyflor 提问回答可以分组。我们现在是三层记忆架构 md 和 sqlite 以及 qdrant，需要向量索引提取压缩。",
	}

	candidates := ExtractCandidates(msg, 200)
	if len(candidates) < 3 {
		t.Fatalf("expected several memory candidates, got %#v", candidates)
	}

	assertCandidateKind(t, candidates, "preference")
	assertCandidateKind(t, candidates, "project")
	assertCandidateKind(t, candidates, "memory_design")
	for _, candidate := range candidates {
		if len([]rune(candidate.Text)) > 240 {
			t.Fatalf("candidate was not compressed: %q", candidate.Text)
		}
	}
}

func TestExtractCandidatesAvoidsGenericAssistantSummary(t *testing.T) {
	msg := MessageInput{
		Role:    "assistant",
		Content: "主人好，我是飞花，可以帮你解答问题、整理资料、写代码、制定计划。你可以直接告诉我想做什么。",
	}

	if got := ExtractCandidates(msg, 200); len(got) != 0 {
		t.Fatalf("generic assistant greeting should not be indexed, got %#v", got)
	}
}

func TestExtractCandidatesKeepsAssistantOutcome(t *testing.T) {
	msg := MessageInput{
		Role:    "assistant",
		Content: "已实现 Qdrant 语义记忆写入，并通过 go test 验证。后续建议把召回事件显示在黑板。",
	}

	candidates := ExtractCandidates(msg, 200)
	assertCandidateKind(t, candidates, "outcome")
}

func TestExtractCandidatesSkipsMethodologyReflectionDraft(t *testing.T) {
	msg := MessageInput{
		Role: "assistant",
		Content: `已完成普通交付。

## Methodology Reflection Draft

- Situation: When a blackboard task repeats the same blocker.
- Method: Return a decision form instead of continuing hidden debate.
- Avoid: Indexing this method into ordinary Qdrant memory.
- Next-time hint: blackboard deadlock

## Summary

已验证普通总结仍可进入常规语义记忆。`,
	}

	candidates := ExtractCandidates(msg, 900)
	if len(candidates) == 0 {
		t.Fatal("expected ordinary assistant outcome candidate after reflection draft")
	}
	for _, candidate := range candidates {
		if strings.Contains(candidate.Text, "decision form") ||
			strings.Contains(candidate.Text, "blackboard deadlock") ||
			strings.Contains(candidate.Text, "Methodology Reflection Draft") {
			t.Fatalf("semantic memory leaked methodology draft: %#v", candidate)
		}
	}
}

func TestHashEmbedderDimensionsAndNormalization(t *testing.T) {
	embedder := hashEmbedder{dimensions: 32}
	vec, err := embedder.Embed(context.Background(), "Flyflor 需要用 Qdrant 做向量召回")
	if err != nil {
		t.Fatalf("Embed returned error: %v", err)
	}
	if len(vec) != 32 {
		t.Fatalf("len(vec) = %d, want 32", len(vec))
	}
	var nonZero bool
	for _, v := range vec {
		if v != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Fatal("embedding vector is all zeros")
	}
}

func assertCandidateKind(t *testing.T, candidates []Candidate, kind string) {
	t.Helper()
	for _, candidate := range candidates {
		if candidate.Kind == kind {
			return
		}
	}
	t.Fatalf("missing candidate kind %q in %#v", kind, candidates)
}
