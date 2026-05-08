package armsmemory

import (
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

func embedText(text string, dimensions int) []float32 {
	if dimensions <= 0 {
		dimensions = defaultDimensions
	}
	vec := make([]float32, dimensions)
	for _, token := range armsTokens(text) {
		h := fnv.New64a()
		_, _ = h.Write([]byte(strings.ToLower(token)))
		sum := h.Sum64()
		idx := int(sum % uint64(dimensions))
		sign := float32(1)
		if (sum>>63)&1 == 1 {
			sign = -1
		}
		vec[idx] += sign
	}
	normalizeVector(vec)
	return vec
}

func armsTokens(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsNumber(r))
	})
	tokens := make([]string, 0, len(words)+len([]rune(text))/3)
	for _, word := range words {
		word = strings.TrimSpace(word)
		if word != "" {
			tokens = append(tokens, word)
		}
	}
	runes := []rune(text)
	for i := 0; i+2 < len(runes); i++ {
		if unicode.IsSpace(runes[i]) || unicode.IsSpace(runes[i+1]) || unicode.IsSpace(runes[i+2]) {
			continue
		}
		tokens = append(tokens, string(runes[i:i+3]))
	}
	return tokens
}

func normalizeVector(vec []float32) {
	var sum float64
	for _, v := range vec {
		sum += float64(v * v)
	}
	if sum == 0 {
		return
	}
	scale := float32(1 / math.Sqrt(sum))
	for i := range vec {
		vec[i] *= scale
	}
}

func palaceCoords(methodology Methodology) []float64 {
	text := strings.ToLower(strings.Join([]string{
		methodology.Title,
		methodology.Text,
		methodology.Situation,
		methodology.Method,
		methodology.Avoid,
		methodology.NextTimeHint,
		strings.Join(methodology.Tags, " "),
	}, "\n"))
	return []float64{
		axisScore(text, "code", "implement", "refactor", "代码", "实现", "重构", "修复"),
		axisScore(text, "docs", "readme", "documentation", "文档"),
		axisScore(text, "architecture", "design", "routing", "scheduler", "架构", "设计", "调度", "路由"),
		axisScore(text, "deploy", "release", "install", "docker", "部署", "发布", "安装"),
		axisScore(text, "test", "verify", "validate", "验证", "测试", "回归"),
		axisScore(text, "blackboard", "decision form", "planner", "reviewer", "黑板", "决策", "复核"),
		axisScore(text, "risk", "avoid", "rollback", "safe", "风险", "避免", "回滚", "稳妥"),
		axisScore(text, "reflection", "methodology", "memory", "skill", "反思", "方法论", "记忆"),
	}
}

func queryCoords(query string) []float64 {
	return palaceCoords(Methodology{Text: query})
}

func axisScore(text string, terms ...string) float64 {
	if strings.TrimSpace(text) == "" {
		return 0.05
	}
	score := 0.05
	for _, term := range terms {
		if strings.Contains(text, strings.ToLower(term)) {
			score += 0.18
		}
	}
	if score > 1 {
		return 1
	}
	return score
}
