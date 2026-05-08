package armsmemory

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

type MessageInput struct {
	Role    string
	Content string
}

type Methodology struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Text          string   `json:"text"`
	Situation     string   `json:"situation,omitempty"`
	Method        string   `json:"method,omitempty"`
	Avoid         string   `json:"avoid,omitempty"`
	NextTimeHint  string   `json:"next_time_hint,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	SessionKey    string   `json:"session_key,omitempty"`
	Source        string   `json:"source"`
	Namespace     string   `json:"namespace"`
	CreatedAt     string   `json:"created_at,omitempty"`
	SchemaVersion int      `json:"schema_version"`
}

const methodologySchemaVersion = 1

var reflectionHeadingRE = regexp.MustCompile(`(?im)^#{2,4}\s*Methodology Reflection Draft\s*$`)
var nextHeadingRE = regexp.MustCompile(`(?m)^#{1,4}\s+\S`)

func ExtractMethodologies(msg MessageInput, maxChars int) []Methodology {
	if strings.ToLower(strings.TrimSpace(msg.Role)) != "assistant" {
		return nil
	}
	content := strings.TrimSpace(msg.Content)
	if content == "" {
		return nil
	}
	sections := reflectionSections(content)
	out := make([]Methodology, 0, len(sections))
	for _, section := range sections {
		methodology := parseMethodologySection(section, maxChars)
		if strings.TrimSpace(methodology.Text) == "" {
			continue
		}
		if strings.TrimSpace(methodology.Method) == "" && strings.TrimSpace(methodology.Situation) == "" {
			continue
		}
		out = append(out, methodology)
	}
	return dedupeMethodologies(out)
}

func reflectionSections(content string) []string {
	matches := reflectionHeadingRE.FindAllStringIndex(content, -1)
	if len(matches) == 0 {
		return nil
	}
	sections := make([]string, 0, len(matches))
	for i, match := range matches {
		start := match[1]
		end := len(content)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		} else if next := nextHeadingRE.FindStringIndex(content[start:]); next != nil {
			end = start + next[0]
		}
		section := strings.TrimSpace(content[start:end])
		if section != "" {
			sections = append(sections, section)
		}
	}
	return sections
}

func parseMethodologySection(section string, maxChars int) Methodology {
	text := normalizeMethodologyText(section, maxChars)
	fields := parseReflectionFields(text)
	title := firstNonEmpty(fields["title"], fields["next-time hint"], fields["next time hint"])
	if title == "" {
		title = "Methodology Reflection Draft"
	}
	return Methodology{
		Title:         truncateRunes(title, 120),
		Text:          text,
		Situation:     fields["situation"],
		Method:        fields["method"],
		Avoid:         fields["avoid"],
		NextTimeHint:  firstNonEmpty(fields["next-time hint"], fields["next time hint"]),
		Tags:          inferTags(text),
		Source:        "flyflor.reflection",
		Namespace:     defaultMethodologyNamespace,
		SchemaVersion: methodologySchemaVersion,
	}
}

func parseReflectionFields(text string) map[string]string {
	fields := map[string]string{}
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		line = strings.TrimPrefix(line, "-")
		line = strings.TrimPrefix(line, "*")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			key, value, ok = strings.Cut(line, "：")
		}
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		switch key {
		case "title", "situation", "method", "avoid", "next-time hint", "next time hint":
			if value != "" {
				fields[key] = value
			}
		}
	}
	return fields
}

func normalizeMethodologyText(section string, maxChars int) string {
	lines := strings.Split(section, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		kept = append(kept, line)
	}
	text := strings.Join(kept, "\n")
	if maxChars <= 0 {
		maxChars = defaultMaxMethodologyChars
	}
	return truncateRunes(text, maxChars)
}

func inferTags(text string) []string {
	lower := strings.ToLower(text)
	var tags []string
	for tag, terms := range map[string][]string{
		"blackboard": {"blackboard", "黑板"},
		"testing":    {"test", "verify", "验证", "测试"},
		"routing":    {"routing", "route", "调度", "路由"},
		"memory":     {"memory", "reflection", "记忆", "反思", "方法论"},
		"docs":       {"readme", "docs", "documentation", "文档"},
	} {
		for _, term := range terms {
			if strings.Contains(lower, term) {
				tags = append(tags, tag)
				break
			}
		}
	}
	return tags
}

func truncateRunes(text string, maxChars int) string {
	text = strings.TrimSpace(text)
	if maxChars <= 0 || utf8.RuneCountInString(text) <= maxChars {
		return text
	}
	runes := []rune(text)
	return string(runes[:maxChars]) + "..."
}

func dedupeMethodologies(in []Methodology) []Methodology {
	seen := map[string]struct{}{}
	out := make([]Methodology, 0, len(in))
	for _, methodology := range in {
		key := methodology.Text
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, methodology)
	}
	return out
}
