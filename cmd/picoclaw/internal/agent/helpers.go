package agent

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ergochat/readline"
	"golang.org/x/term"

	"github.com/sipeed/picoclaw/cmd/picoclaw/internal"
	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/cliui"
	coreagent "github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/bus"
	runtimeevents "github.com/sipeed/picoclaw/pkg/events"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
)

func agentCmd(message, sessionKey, model string, debug bool) error {
	if sessionKey == "" {
		sessionKey = "cli:default"
	}

	cfg, err := internal.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	logger.ConfigureFromEnv()

	if debug {
		logger.SetLevel(logger.DEBUG)
		fmt.Print(cliui.RenderAgentNotice("debug logging enabled"))
	} else {
		logger.SetLevel(logger.WARN)
	}

	if model != "" {
		cfg.Agents.Defaults.ModelName = model
	}

	provider, modelID, err := providers.CreateProvider(cfg)
	if err != nil {
		return fmt.Errorf("error creating provider: %w", err)
	}

	// Use the resolved model ID from provider creation
	if modelID != "" {
		cfg.Agents.Defaults.ModelName = modelID
	}

	msgBus := bus.NewMessageBus()
	defer msgBus.Close()
	agentLoop := coreagent.NewAgentLoop(cfg, msgBus, provider)
	defer agentLoop.Close()
	resolvedSessionKey, resolveErr := resolveRequestedSessionKey(agentLoop, sessionKey)
	if resolveErr != nil {
		return resolveErr
	}
	sessionKey = resolvedSessionKey

	// Print agent startup info (only for interactive mode)
	startupInfo := agentLoop.GetStartupInfo()
	logger.InfoCF("agent", "Agent initialized",
		map[string]any{
			"tools_count":      startupInfo["tools"].(map[string]any)["count"],
			"skills_total":     startupInfo["skills"].(map[string]any)["total"],
			"skills_available": startupInfo["skills"].(map[string]any)["available"],
		})

	if message != "" {
		ctx := context.Background()
		fmt.Print(cliui.RenderUserMessage(message))
		response, err := processDirectWithProgress(ctx, agentLoop, message, sessionKey)
		if err != nil {
			return fmt.Errorf("error processing message: %w", err)
		}
		fmt.Println(cliui.RenderAssistantMessage(response))
		fmt.Print(cliui.RenderAgentStatusBar(sessionKey, cfg.Agents.Defaults.GetModelName()))
		return nil
	}

	modelLabel := cfg.Agents.Defaults.GetModelName()
	if useBubbleTUI() {
		return runAgentTUI(agentLoop, sessionKey, modelLabel)
	}
	fmt.Println(cliui.RenderAgentIntro(modelLabel, sessionKey))
	interactiveMode(agentLoop, sessionKey)

	return nil
}

func useBubbleTUI() bool {
	if os.Getenv("FLYFLOR_TUI_LEGACY") == "1" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func resolveRequestedSessionKey(agentLoop *coreagent.AgentLoop, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return "cli:default", nil
	}
	store := defaultSessionStore(agentLoop)
	if store == nil {
		return requested, nil
	}
	if len(store.GetHistory(requested)) > 0 || strings.TrimSpace(store.GetSummary(requested)) != "" {
		return requested, nil
	}
	for _, key := range store.ListSessions() {
		if key == requested {
			return requested, nil
		}
	}
	if !strings.Contains(requested, "…") && !strings.Contains(requested, "...") {
		return requested, nil
	}
	matches := matchShortSessionKey(requested, store.ListSessions())
	switch len(matches) {
	case 0:
		return requested, nil
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("session 缩略 key %q 匹配到多个会话，请运行 flyflor sessions 后用 Tab 补全完整 session key", requested)
	}
}

func matchShortSessionKey(shortKey string, keys []string) []string {
	shortKey = strings.TrimSpace(shortKey)
	separators := []string{"…", "..."}
	var prefix, suffix string
	for _, sep := range separators {
		parts := strings.Split(shortKey, sep)
		if len(parts) == 2 {
			prefix = parts[0]
			suffix = parts[1]
			break
		}
	}
	if prefix == "" && suffix == "" {
		return nil
	}
	var matches []string
	for _, key := range keys {
		if strings.HasPrefix(key, prefix) && strings.HasSuffix(key, suffix) {
			matches = append(matches, key)
		}
	}
	return matches
}

func defaultSessionStore(agentLoop *coreagent.AgentLoop) interface {
	GetHistory(string) []providers.Message
	GetSummary(string) string
	ListSessions() []string
} {
	if agentLoop == nil || agentLoop.GetRegistry() == nil {
		return nil
	}
	agent := agentLoop.GetRegistry().GetDefaultAgent()
	if agent == nil || agent.Sessions == nil {
		return nil
	}
	return agent.Sessions
}

func loadSessionTurns(agentLoop *coreagent.AgentLoop, sessionKey string) []cliui.BlackboardTurn {
	store := defaultSessionStore(agentLoop)
	if store == nil {
		return nil
	}
	return historyToBlackboardTurns(store.GetHistory(sessionKey))
}

func historyToBlackboardTurns(history []providers.Message) []cliui.BlackboardTurn {
	var turns []cliui.BlackboardTurn
	var current *cliui.BlackboardTurn
	for _, msg := range history {
		role := strings.TrimSpace(strings.ToLower(msg.Role))
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		switch role {
		case "user":
			if current != nil {
				turns = append(turns, *current)
			}
			current = &cliui.BlackboardTurn{
				Index: len(turns) + 1,
				User:  content,
			}
		case "assistant":
			if current == nil {
				current = &cliui.BlackboardTurn{
					Index: len(turns) + 1,
				}
			}
			if strings.TrimSpace(current.Assistant) == "" {
				current.Assistant = content
			} else {
				current.Assistant += "\n\n" + content
			}
		}
	}
	if current != nil {
		turns = append(turns, *current)
	}
	return turns
}

func interactiveMode(agentLoop *coreagent.AgentLoop, sessionKey string) {
	prompt := cliui.Prompt()
	turns := loadSessionTurns(agentLoop, sessionKey)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          prompt,
		HistoryFile:     filepath.Join(os.TempDir(), ".flyflor_history"),
		HistoryLimit:    100,
		AutoComplete:    agentReadlineCompleter(func() []cliui.BlackboardTurn { return turns }),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		fmt.Print(cliui.RenderAgentError(fmt.Errorf("readline unavailable: %w", err)))
		fmt.Println("Falling back to simple input mode.")
		simpleInteractiveMode(agentLoop, sessionKey)
		return
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt || err == io.EOF {
				fmt.Println("\nGoodbye.")
				return
			}
			fmt.Print(cliui.RenderAgentError(fmt.Errorf("read input: %w", err)))
			continue
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye.")
			return
		}
		if handled := handleAgentLocalCommand(input, turns); handled {
			continue
		}

		ctx := context.Background()
		fmt.Println()
		fmt.Print(cliui.RenderUserMessage(input))
		response, err := processDirectWithProgress(ctx, agentLoop, input, sessionKey)
		if err != nil {
			fmt.Print(cliui.RenderAgentError(err))
			continue
		}

		turns = append(turns, cliui.BlackboardTurn{
			Index:     len(turns) + 1,
			User:      input,
			Assistant: response,
			StartedAt: time.Now(),
		})
		fmt.Println(cliui.RenderAssistantMessage(response))
		fmt.Print(cliui.RenderAgentStatusBar(sessionKey, currentAgentModel(agentLoop)))
	}
}

func simpleInteractiveMode(agentLoop *coreagent.AgentLoop, sessionKey string) {
	reader := bufio.NewReader(os.Stdin)
	turns := loadSessionTurns(agentLoop, sessionKey)
	for {
		fmt.Print(cliui.Prompt())
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println("\nGoodbye.")
				return
			}
			fmt.Print(cliui.RenderAgentError(fmt.Errorf("read input: %w", err)))
			continue
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye.")
			return
		}
		if handled := handleAgentLocalCommand(input, turns); handled {
			continue
		}

		ctx := context.Background()
		fmt.Println()
		fmt.Print(cliui.RenderUserMessage(input))
		response, err := processDirectWithProgress(ctx, agentLoop, input, sessionKey)
		if err != nil {
			fmt.Print(cliui.RenderAgentError(err))
			continue
		}

		turns = append(turns, cliui.BlackboardTurn{
			Index:     len(turns) + 1,
			User:      input,
			Assistant: response,
			StartedAt: time.Now(),
		})
		fmt.Println(cliui.RenderAssistantMessage(response))
		fmt.Print(cliui.RenderAgentStatusBar(sessionKey, currentAgentModel(agentLoop)))
	}
}

func handleAgentLocalCommand(input string, turns []cliui.BlackboardTurn) bool {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return false
	}
	if fields[0] == "/help" || fields[0] == "?" {
		fmt.Print(cliui.RenderAgentNotice("内置命令：/bb 打开本会话黑板，/bb <编号> 切换回合，/bb hide 保持隐藏，exit/quit 退出。Tab 可补全这些命令。"))
		return true
	}
	if fields[0] != "/bb" && fields[0] != "/blackboard" {
		return false
	}
	if len(fields) > 1 && (fields[1] == "hide" || fields[1] == "off") {
		fmt.Print(cliui.RenderAgentNotice("黑板在普通对话中默认隐藏；需要查看时再次输入 /bb。"))
		return true
	}
	selected := len(turns) - 1
	if len(fields) > 1 {
		if n, err := strconv.Atoi(fields[1]); err == nil {
			selected = n - 1
		}
	}
	fmt.Print(cliui.RenderBlackboardTabs(turns, selected))
	return true
}

func agentReadlineCompleter(turns func() []cliui.BlackboardTurn) readline.AutoCompleter {
	turnIDs := func(string) []string {
		current := []cliui.BlackboardTurn(nil)
		if turns != nil {
			current = turns()
		}
		candidates := []string{"hide", "off", "latest"}
		for _, turn := range current {
			if turn.Index > 0 {
				candidates = append(candidates, strconv.Itoa(turn.Index))
			}
		}
		return candidates
	}
	return readline.NewPrefixCompleter(
		readline.PcItem("/bb",
			readline.PcItemDynamic(turnIDs),
		),
		readline.PcItem("/blackboard",
			readline.PcItemDynamic(turnIDs),
		),
		readline.PcItem("/help"),
		readline.PcItem("exit"),
		readline.PcItem("quit"),
	)
}

func currentAgentModel(agentLoop *coreagent.AgentLoop) string {
	if agentLoop == nil || agentLoop.GetRegistry() == nil {
		return "unknown"
	}
	if a := agentLoop.GetRegistry().GetDefaultAgent(); a != nil && a.Model != "" {
		return a.Model
	}
	return "unknown"
}

func processDirectWithProgress(
	ctx context.Context,
	agentLoop *coreagent.AgentLoop,
	message string,
	sessionKey string,
) (string, error) {
	progress := cliui.NewAgentProgress(os.Stdout)
	done := make(chan struct{})
	sub, eventsCh := subscribeAgentProgress(ctx, agentLoop, progress, done)
	if sub != nil {
		defer sub.Close()
	}

	response, err := agentLoop.ProcessDirect(ctx, message, sessionKey)

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		if err != nil {
			progress.Error("turn failed")
		} else {
			progress.Done("completed")
		}
	}
	if eventsCh == nil {
		if err != nil {
			progress.Error("turn failed")
		} else {
			progress.Done("completed")
		}
	}
	return response, err
}

func subscribeAgentProgress(
	ctx context.Context,
	agentLoop *coreagent.AgentLoop,
	progress *cliui.AgentProgress,
	done chan<- struct{},
) (runtimeevents.Subscription, <-chan runtimeevents.Event) {
	if agentLoop == nil || agentLoop.RuntimeEvents() == nil {
		progress.Start("preparing turn")
		close(done)
		return nil, nil
	}
	sub, ch, err := agentLoop.RuntimeEvents().
		KindPrefix("agent.").
		SubscribeChan(ctx, runtimeevents.SubscribeOptions{Name: "cli-progress", Buffer: 64})
	if err != nil {
		progress.Start("preparing turn")
		close(done)
		return nil, nil
	}

	go func() {
		defer close(done)
		for evt := range ch {
			if renderAgentProgressEvent(progress, evt) {
				return
			}
		}
	}()

	return sub, ch
}

func renderAgentProgressEvent(progress *cliui.AgentProgress, evt runtimeevents.Event) bool {
	switch evt.Kind {
	case runtimeevents.KindAgentTurnStart:
		progress.Start("thinking through the request")
	case runtimeevents.KindAgentLLMRequest:
		payload, _ := evt.Payload.(coreagent.LLMRequestPayload)
		label := "calling model"
		if payload.Model != "" {
			label = "calling " + payload.Model
		}
		if payload.MessagesCount > 0 || payload.ToolsCount > 0 {
			label = fmt.Sprintf("%s · %d messages · %d tools", label, payload.MessagesCount, payload.ToolsCount)
		}
		progress.Step(label)
	case runtimeevents.KindAgentLLMRetry:
		payload, _ := evt.Payload.(coreagent.LLMRetryPayload)
		label := "retrying model request"
		if payload.Reason != "" {
			label += " · " + payload.Reason
		}
		if payload.Attempt > 0 && payload.MaxRetries > 0 {
			label += fmt.Sprintf(" · %d/%d", payload.Attempt, payload.MaxRetries)
		}
		progress.Warn(label)
	case runtimeevents.KindAgentContextCompress:
		payload, _ := evt.Payload.(coreagent.ContextCompressPayload)
		label := "compressing context"
		if payload.Reason != "" {
			label += " · " + string(payload.Reason)
		}
		progress.Warn(label)
	case runtimeevents.KindAgentLLMResponse:
		payload, _ := evt.Payload.(coreagent.LLMResponsePayload)
		if payload.ToolCalls > 0 {
			progress.Step(fmt.Sprintf("model requested %d tool call(s)", payload.ToolCalls))
		} else {
			progress.Step("drafting final response")
		}
	case runtimeevents.KindAgentToolExecStart:
		payload, _ := evt.Payload.(coreagent.ToolExecStartPayload)
		progress.ToolStart(payload.Tool)
	case runtimeevents.KindAgentToolExecEnd:
		payload, _ := evt.Payload.(coreagent.ToolExecEndPayload)
		var details []string
		if payload.Duration > 0 {
			details = append(details, payload.Duration.Round(time.Millisecond).String())
		}
		if payload.Async {
			details = append(details, "async")
		}
		progress.ToolDone(payload.Tool, strings.Join(details, " · "), payload.IsError)
	case runtimeevents.KindAgentToolExecSkipped:
		payload, _ := evt.Payload.(coreagent.ToolExecSkippedPayload)
		label := "tool skipped"
		if payload.Tool != "" {
			label = payload.Tool + " skipped"
		}
		if payload.Reason != "" {
			label += " · " + payload.Reason
		}
		progress.Warn(label)
	case runtimeevents.KindAgentSubTurnSpawn:
		payload, _ := evt.Payload.(coreagent.SubTurnSpawnPayload)
		label := "delegating"
		if payload.Label != "" {
			label += " · " + payload.Label
		}
		if payload.AgentID != "" {
			label += " → " + payload.AgentID
		}
		progress.Step(label)
	case runtimeevents.KindAgentError:
		payload, _ := evt.Payload.(coreagent.ErrorPayload)
		label := "agent error"
		if payload.Stage != "" {
			label += " · " + payload.Stage
		}
		progress.Error(label)
		return true
	case runtimeevents.KindAgentTurnEnd:
		payload, _ := evt.Payload.(coreagent.TurnEndPayload)
		label := string(payload.Status)
		if label == "" {
			label = "completed"
		}
		if payload.Iterations > 0 {
			label += fmt.Sprintf(" · %d iteration(s)", payload.Iterations)
		}
		if payload.Duration > 0 {
			label += " · " + payload.Duration.Round(time.Millisecond).String()
		}
		if payload.Status == coreagent.TurnEndStatusError || payload.Status == coreagent.TurnEndStatusAborted {
			progress.Error(label)
		} else {
			progress.Done(label)
		}
		return true
	}
	return false
}
