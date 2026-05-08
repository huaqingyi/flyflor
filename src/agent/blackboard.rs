use serde::Serialize;

pub const DIRECT_THRESHOLD: f32 = 0.35;
pub const BLACKBOARD_THRESHOLD: f32 = 0.55;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[serde(rename_all = "kebab-case")]
pub enum BlackboardMode {
    Direct,
    DirectWithWatch,
    Blackboard,
}

impl BlackboardMode {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Direct => "direct",
            Self::DirectWithWatch => "direct-with-watch",
            Self::Blackboard => "blackboard",
        }
    }
}

#[derive(Debug, Clone, Serialize)]
pub struct ComplexityFeatures {
    pub estimated_tokens: usize,
    pub code_blocks: usize,
    pub diff_markers: usize,
    pub stacktrace_markers: usize,
    pub subtask_markers: usize,
    pub implementation_intent: bool,
    pub verification_intent: bool,
    pub review_intent: bool,
    pub planning_intent: bool,
    pub continuation_reference: bool,
    pub media_count: usize,
    pub session_depth: usize,
    pub recent_tool_uses: usize,
    pub risk_intent: bool,
    pub explicit_blackboard: bool,
    pub cross_file: bool,
}

#[derive(Debug, Clone, Serialize)]
pub struct ComplexityAssessment {
    pub mode: BlackboardMode,
    pub score: f32,
    pub direct_threshold: f32,
    pub threshold: f32,
    pub hard_gate: bool,
    pub reasons: Vec<String>,
    pub features: ComplexityFeatures,
}

pub fn is_intro_mode_request(message: &str) -> bool {
    let text = message.trim().to_lowercase();
    text.starts_with("/intro")
        || text.starts_with("/导论")
        || text.starts_with("导论模式")
        || text.contains("进入导论模式")
        || text.contains("用导论模式")
        || text.contains("以导论模式")
}

pub fn strip_intro_mode_prefix(message: &str) -> String {
    let trimmed = message.trim();
    for prefix in ["/intro", "/导论", "导论模式"] {
        if let Some(rest) = trimmed.strip_prefix(prefix) {
            return rest
                .trim_start_matches([':', '：', '-', ' ', '\n', '\t'])
                .to_string();
        }
    }
    trimmed.to_string()
}

pub fn assess_blackboard_complexity(
    message: &str,
    media_count: usize,
    session_depth: usize,
    recent_tool_uses: usize,
) -> ComplexityAssessment {
    let lower = message.to_lowercase();
    let estimated_tokens = estimate_tokens(message);
    let code_blocks = message.matches("```").count() / 2;
    let diff_markers = count_diff_markers(message);
    let stacktrace_markers = count_any(
        &lower,
        &[
            "traceback",
            "stack trace",
            "exception",
            "panic",
            "thread '",
            "error:",
            "caused by:",
        ],
    );
    let subtask_markers = count_subtask_markers(message);
    let implementation_intent = contains_any(
        &lower,
        &[
            "implement",
            "build",
            "fix",
            "add support",
            "refactor",
            "wire",
            "实现",
            "开发",
            "修复",
            "改代码",
            "接入",
            "重构",
        ],
    );
    let verification_intent = contains_any(
        &lower,
        &[
            "test",
            "verify",
            "reproduce",
            "validate",
            "测试",
            "验证",
            "复现",
            "跑一下",
        ],
    );
    let review_intent = contains_any(
        &lower,
        &[
            "review",
            "audit",
            "check my work",
            "复核",
            "审查",
            "检查",
            "评审",
        ],
    );
    let planning_intent = contains_any(
        &lower,
        &[
            "plan",
            "design",
            "proposal",
            "approach",
            "architecture",
            "方案",
            "设计",
            "规划",
            "拆解",
        ],
    );
    let continuation_reference = contains_any(
        &lower,
        &[
            "continue",
            "follow up",
            "previous",
            "above",
            "继续",
            "接着",
            "上面",
            "前文",
        ],
    );
    let risk_intent = contains_any(
        &lower,
        &[
            "shell",
            "exec",
            "delete",
            "remove",
            "commit",
            "push",
            "deploy",
            "network",
            "database",
            "migration",
            "rm -rf",
            "删除",
            "提交",
            "推送",
            "部署",
            "数据库",
            "迁移",
        ],
    );
    let explicit_blackboard = contains_any(
        &lower,
        &[
            "blackboard",
            "multi-agent",
            "multi agent",
            "planner",
            "reviewer",
            "黑板",
            "多智能体",
            "规划器",
            "复核器",
        ],
    );
    let path_like_refs = message
        .split_whitespace()
        .filter(|part| {
            let trimmed = part.trim_matches(|c: char| {
                matches!(
                    c,
                    ',' | ';' | ':' | ')' | '(' | '[' | ']' | '`' | '"' | '\''
                )
            });
            trimmed.contains('/')
                && (trimmed.contains('.')
                    || trimmed.starts_with("src/")
                    || trimmed.starts_with("./"))
        })
        .count();
    let cross_file = contains_any(
        &lower,
        &[
            "cross-file",
            "multiple files",
            "several files",
            "across files",
            "跨文件",
            "多个文件",
        ],
    ) || path_like_refs >= 2;

    let features = ComplexityFeatures {
        estimated_tokens,
        code_blocks,
        diff_markers,
        stacktrace_markers,
        subtask_markers,
        implementation_intent,
        verification_intent,
        review_intent,
        planning_intent,
        continuation_reference,
        media_count,
        session_depth,
        recent_tool_uses,
        risk_intent,
        explicit_blackboard,
        cross_file,
    };

    let mut score = 0.0f32;
    let mut reasons = Vec::new();
    add_score(
        &mut score,
        &mut reasons,
        estimated_tokens >= 900,
        0.12,
        "long input",
    );
    add_score(
        &mut score,
        &mut reasons,
        code_blocks > 0,
        (code_blocks as f32 * 0.10).min(0.22),
        "code blocks",
    );
    add_score(
        &mut score,
        &mut reasons,
        diff_markers > 0,
        0.16,
        "diff-like input",
    );
    add_score(
        &mut score,
        &mut reasons,
        stacktrace_markers > 0,
        0.12,
        "stacktrace/error markers",
    );
    add_score(
        &mut score,
        &mut reasons,
        subtask_markers >= 2,
        0.12,
        "multiple subtasks",
    );
    add_score(
        &mut score,
        &mut reasons,
        implementation_intent,
        0.13,
        "implementation intent",
    );
    add_score(
        &mut score,
        &mut reasons,
        verification_intent,
        0.12,
        "verification intent",
    );
    add_score(
        &mut score,
        &mut reasons,
        review_intent,
        0.12,
        "review intent",
    );
    add_score(
        &mut score,
        &mut reasons,
        planning_intent,
        0.08,
        "planning intent",
    );
    add_score(
        &mut score,
        &mut reasons,
        continuation_reference,
        0.05,
        "continues prior context",
    );
    add_score(
        &mut score,
        &mut reasons,
        media_count > 0,
        0.08,
        "media input",
    );
    add_score(
        &mut score,
        &mut reasons,
        session_depth >= 6,
        0.05,
        "deep session",
    );
    add_score(
        &mut score,
        &mut reasons,
        recent_tool_uses >= 3,
        0.08,
        "recent tool density",
    );
    add_score(&mut score, &mut reasons, risk_intent, 0.10, "risk intent");
    add_score(
        &mut score,
        &mut reasons,
        explicit_blackboard,
        0.40,
        "explicit blackboard request",
    );
    add_score(
        &mut score,
        &mut reasons,
        cross_file,
        0.14,
        "cross-file workflow",
    );

    let hard_gate = explicit_blackboard
        || estimated_tokens >= 3500
        || code_blocks >= 3
        || (implementation_intent && verification_intent)
        || (implementation_intent && review_intent)
        || (cross_file && (implementation_intent || verification_intent || review_intent));

    if hard_gate {
        reasons.push("hard gate".to_string());
        score = score.max(BLACKBOARD_THRESHOLD);
    }

    score = score.clamp(0.0, 1.0);
    let mode = if hard_gate || score >= BLACKBOARD_THRESHOLD {
        BlackboardMode::Blackboard
    } else if score >= DIRECT_THRESHOLD {
        BlackboardMode::DirectWithWatch
    } else {
        BlackboardMode::Direct
    };

    ComplexityAssessment {
        mode,
        score,
        direct_threshold: DIRECT_THRESHOLD,
        threshold: BLACKBOARD_THRESHOLD,
        hard_gate,
        reasons,
        features,
    }
}

pub fn blackboard_system_prompt(assessment: &ComplexityAssessment) -> String {
    format!(
        "## Blackboard Workbench\n\
This turn is routed through Flyflor blackboard mode. Coordinate internally as two roles:\n\
- flyflor-planner: decompose the user's goal, identify required context, propose the execution path, and define verification points.\n\
- flyflor-reviewer: challenge missing constraints, risk, edge cases, and final readability.\n\n\
Rules:\n\
1. Make the blackboard process visible to the user with a short `## Blackboard Discussion` section before the final answer.\n\
2. In that section, include concise bullets for `Planner` and `Reviewer`; show decisions, risks, and verification points, not private chain-of-thought.\n\
3. Use tools when they are needed to inspect or change real state.\n\
4. Aim to converge in 3 rounds; stop by 5 rounds.\n\
5. If blocked by an irreversible choice or missing user preference, return a fenced `flyflor-decision-form` block with readable options.\n\
6. Put the user-facing result after `## Final`.\n\
7. For substantial technical work, end with `## Methodology Reflection Draft` using Situation, Method, Avoid, and Next-time hint bullets.\n\n\
Routing facts: mode={mode}; score={score:.2}; reasons={reasons}.",
        mode = assessment.mode.as_str(),
        score = assessment.score,
        reasons = if assessment.reasons.is_empty() {
            "none".to_string()
        } else {
            assessment.reasons.join(", ")
        }
    )
}

pub fn intro_system_prompt() -> &'static str {
    "## Introductory Mode / 导论模式\n\
The user requested introductory mode. Start by orienting the work instead of jumping straight to a final implementation.\n\
Provide: current understanding, the smallest useful first step, likely risks or unknowns, and the next concrete action.\n\
If execution is already clearly requested and safe, continue after the orientation; otherwise ask only the minimum blocking question."
}

fn estimate_tokens(text: &str) -> usize {
    let ascii_words = text.split_whitespace().count();
    let cjk_chars = text
        .chars()
        .filter(|c| matches!(*c as u32, 0x4E00..=0x9FFF))
        .count();
    ascii_words + cjk_chars / 2
}

fn count_diff_markers(text: &str) -> usize {
    text.lines()
        .filter(|line| {
            let trimmed = line.trim_start();
            trimmed.starts_with("diff --git")
                || trimmed.starts_with("@@")
                || trimmed.starts_with("+++ ")
                || trimmed.starts_with("--- ")
        })
        .count()
}

fn count_subtask_markers(text: &str) -> usize {
    text.lines()
        .filter(|line| {
            let trimmed = line.trim_start();
            let starts_numbered = trimmed
                .chars()
                .next()
                .map(|c| c.is_ascii_digit())
                .unwrap_or(false)
                && (trimmed.contains(". ") || trimmed.contains(") "));
            trimmed.starts_with("- ")
                || trimmed.starts_with("* ")
                || trimmed.starts_with("- [")
                || starts_numbered
        })
        .count()
}

fn contains_any(haystack: &str, needles: &[&str]) -> bool {
    needles.iter().any(|needle| haystack.contains(needle))
}

fn count_any(haystack: &str, needles: &[&str]) -> usize {
    needles
        .iter()
        .filter(|needle| haystack.contains(**needle))
        .count()
}

fn add_score(score: &mut f32, reasons: &mut Vec<String>, enabled: bool, weight: f32, reason: &str) {
    if enabled {
        *score += weight;
        reasons.push(reason.to_string());
    }
}

#[cfg(test)]
mod tests {
    use super::{
        BlackboardMode, assess_blackboard_complexity, is_intro_mode_request,
        strip_intro_mode_prefix,
    };

    #[test]
    fn simple_chat_stays_direct() {
        let assessed = assess_blackboard_complexity("hello", 0, 0, 0);
        assert_eq!(assessed.mode, BlackboardMode::Direct);
        assert!(!assessed.hard_gate);
    }

    #[test]
    fn implementation_and_verification_enters_blackboard() {
        let assessed = assess_blackboard_complexity("实现登录修复，然后跑测试验证结果", 0, 0, 0);
        assert_eq!(assessed.mode, BlackboardMode::Blackboard);
        assert!(assessed.hard_gate);
    }

    #[test]
    fn medium_planning_task_uses_watch_mode() {
        let assessed = assess_blackboard_complexity(
            "Plan and review a migration approach:\n- step one\n- step two\n- step three\nInclude database risk.",
            0,
            0,
            0,
        );
        assert_eq!(assessed.mode, BlackboardMode::DirectWithWatch);
    }

    #[test]
    fn intro_mode_requires_explicit_intro_phrase() {
        assert!(is_intro_mode_request("/intro explain the repo"));
        assert!(is_intro_mode_request("进入导论模式：解释黑板"));
        assert!(!is_intro_mode_request("实现黑板和导论模式"));
        assert_eq!(strip_intro_mode_prefix("/导论：解释一下"), "解释一下");
    }

    #[test]
    fn blackboard_prompt_makes_discussion_visible() {
        let assessed = assess_blackboard_complexity("hello", 0, 0, 0);
        let prompt = super::blackboard_system_prompt(&assessed);
        assert!(prompt.contains("## Blackboard Discussion"));
        assert!(prompt.contains("Planner"));
        assert!(prompt.contains("Reviewer"));
        assert!(prompt.contains("## Final"));
    }
}
