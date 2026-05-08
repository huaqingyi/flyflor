package sandbox

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sipeed/picoclaw/pkg/config"
)

type Profile string

const (
	ProfileStandard Profile = "standard"
	ProfileYOLO     Profile = "yolo"
)

type Risk string

const (
	RiskRead    Risk = "read"
	RiskLow     Risk = "low"
	RiskMedium  Risk = "medium"
	RiskHigh    Risk = "high"
	RiskBlocked Risk = "blocked"
)

type Action string

const (
	ActionAllow   Action = "allow"
	ActionConfirm Action = "confirm"
	ActionDeny    Action = "deny"
)

type Call struct {
	Profile   Profile
	Tool      string
	Arguments map[string]any
	Channel   string
	ChatID    string
	Workspace string
}

type Decision struct {
	Profile Profile  `json:"profile"`
	Tool    string   `json:"tool"`
	Risk    Risk     `json:"risk"`
	Action  Action   `json:"action"`
	Reasons []string `json:"reasons,omitempty"`
}

type Box struct {
	enabled bool
}

func NewBox(cfg config.SandboxConfig) *Box {
	return &Box{enabled: cfg.IsEnabled()}
}

func (b *Box) Assess(call Call) Decision {
	profile := normalizeProfile(call.Profile)
	decision := Decision{
		Profile: profile,
		Tool:    strings.TrimSpace(call.Tool),
		Risk:    RiskLow,
		Action:  ActionAllow,
	}
	if b == nil || !b.enabled {
		decision.Reasons = []string{"sandbox box disabled"}
		return decision
	}

	decision.Risk, decision.Reasons = classify(call)
	decision.Action = actionFor(profile, decision.Risk)
	return decision
}

func normalizeProfile(profile Profile) Profile {
	switch profile {
	case ProfileYOLO:
		return ProfileYOLO
	default:
		return ProfileStandard
	}
}

func actionFor(profile Profile, risk Risk) Action {
	switch risk {
	case RiskBlocked:
		return ActionDeny
	case RiskHigh:
		return ActionConfirm
	case RiskMedium:
		if profile == ProfileYOLO {
			return ActionAllow
		}
		return ActionConfirm
	default:
		return ActionAllow
	}
}

func classify(call Call) (Risk, []string) {
	tool := strings.TrimSpace(call.Tool)
	args := call.Arguments
	switch tool {
	case "read_file", "read_file_lines", "list_dir", "load_image", "search_tools":
		return RiskRead, []string{"read-only workspace inspection"}
	case "write_file", "edit_file", "append_file":
		return classifyFileMutation(tool, args, call.Workspace)
	case "send_file", "message":
		return RiskMedium, []string{"external delivery side effect"}
	case "web_search", "search":
		return RiskLow, []string{"network read"}
	case "spawn", "spawn_status", "subagent":
		return RiskLow, []string{"agent-internal coordination"}
	case "exec":
		return classifyExec(args)
	default:
		return RiskMedium, []string{"unknown tool requires sandbox-aware approval"}
	}
}

func classifyFileMutation(tool string, args map[string]any, workspace string) (Risk, []string) {
	reasons := []string{"workspace file mutation"}
	if tool == "write_file" {
		if overwrite, _ := args["overwrite"].(bool); overwrite {
			reasons = append(reasons, "overwrite requested")
		}
	}
	if workspace != "" {
		if path, _ := args["path"].(string); path != "" && filepath.IsAbs(path) {
			if !isInside(path, workspace) {
				return RiskHigh, append(reasons, "absolute path outside workspace")
			}
		}
	}
	return RiskMedium, reasons
}

func classifyExec(args map[string]any) (Risk, []string) {
	action, _ := args["action"].(string)
	action = strings.TrimSpace(action)
	switch action {
	case "list", "poll", "read":
		return RiskLow, []string{"exec session inspection"}
	case "write", "send-keys", "kill":
		return RiskMedium, []string{"exec session control"}
	case "run":
		command, _ := args["command"].(string)
		return classifyCommand(command)
	default:
		return RiskMedium, []string{"unknown exec action"}
	}
}

var (
	blockedCommandPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\brm\s+-[rf]{1,2}\b`),
		regexp.MustCompile(`\b(sudo|su)\b`),
		regexp.MustCompile(`\b(chmod|chown)\b`),
		regexp.MustCompile(`\b(format|mkfs|diskpart)\b`),
		regexp.MustCompile(`\bdd\s+if=`),
		regexp.MustCompile(`\b(shutdown|reboot|poweroff)\b`),
		regexp.MustCompile(`\b(curl|wget)\b.*\|\s*(sh|bash)`),
		regexp.MustCompile(`\b(git\s+push|git\s+force)\b`),
		regexp.MustCompile(`\b(docker\s+run|docker\s+exec)\b`),
		regexp.MustCompile(`\b(eval|source\s+.*\.sh)\b`),
	}
	safeCommandPattern = regexp.MustCompile(`^\s*(rg|grep|cat|sed|awk|ls|find|pwd|git\s+(status|diff|show|log|branch)|go\s+(test|build|vet|list)|npm\s+(test|run|exec)|pnpm\s+(test|run|exec)|yarn\s+(test|run)|cargo\s+(test|build|check)|make\s+[-\w.]+)\b`)
)

func classifyCommand(command string) (Risk, []string) {
	normalized := strings.ToLower(strings.TrimSpace(command))
	if normalized == "" {
		return RiskMedium, []string{"empty exec command"}
	}
	for _, pattern := range blockedCommandPatterns {
		if pattern.MatchString(normalized) {
			return RiskBlocked, []string{"dangerous command pattern"}
		}
	}
	if safeCommandPattern.MatchString(normalized) {
		return RiskLow, []string{"focused read/test/build command"}
	}
	return RiskMedium, []string{"general shell command"}
}

func isInside(path, root string) bool {
	cleanPath := filepath.Clean(path)
	cleanRoot := filepath.Clean(root)
	rel, err := filepath.Rel(cleanRoot, cleanPath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
