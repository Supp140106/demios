package core

import (
	"context"
	"demios/llm"
	"demios/tools"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	openai "github.com/openai/openai-go"
)

type AgentEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type Agent struct {
	Name              string
	SystemPrompt      string
	Workspace         string
	client            *Client
	history           []llm.Message
	historyReasonings []string
	tools             map[string]tools.Tool
	toolDefs          []llm.ToolDefinition
	serverManager     *tools.ServerManager

	PermissionMode string // "allow_all", "confirm_write", "confirm_all"

	permissionRequests map[string]chan bool
	permissionMu       sync.Mutex

	humanInputRequests map[string]chan string
	humanInputMu       sync.Mutex

	currentEvents chan<- AgentEvent
}

func NewAgent(name string) *Agent {
	a := &Agent{
		Name:   name,
		client: NewClient(),
		tools: map[string]tools.Tool{
			"Read":             tools.ReadFile,
			"Write":            tools.WriteFile,
			"Edit":             tools.ApplyPatch,
			"Grep":             tools.Grep,
			"Glob":             tools.Glob,
			"Bash":             tools.Bash,
			"ListSkills":       tools.ListSkills,
			"ReadSkill":        tools.ReadSkill,
			"Undo":             tools.UndoLast,
			"ReadRelated":      tools.ReadRelated,
			"ProjectStructure": tools.ProjectStructure,
		},
		serverManager:     tools.NewServerManager(),
		PermissionMode:    "allow_all",
		permissionRequests:  make(map[string]chan bool),
		humanInputRequests:  make(map[string]chan string),
	}
	a.serverManager.SetURLResolver(a.resolveServerURLFromOutput)
	a.tools["Task"] = a.makeTaskTool()
	a.tools["AskUser"] = a.makeAskUserTool()
	a.tools["StartServer"] = tools.MakeStartServerTool(a.serverManager)
	a.tools["StopServer"] = tools.MakeStopServerTool(a.serverManager)
	a.tools["GetServerStatus"] = tools.MakeGetServerStatusTool(a.serverManager)
	a.tools["RestartServer"] = tools.MakeRestartServerTool(a.serverManager)
	a.tools["ListServers"] = tools.MakeListServersTool(a.serverManager)
	a.tools["TestWebsite"] = a.makeTestWebsiteTool()
	a.toolDefs = tools.AllToolDefs(a.tools)

	a.SystemPrompt = fmt.Sprintf(`You are %s, an AI coding agent with access to tools.

You have access to the following tools to help users with software engineering tasks:
- Read: Read file contents.
- Write: Create or overwrite a file.
- Edit: Edit a file using a unified diff patch.
- Grep: Search file contents with regex (ripgrep-powered).
- Glob: Find files matching a glob pattern (ripgrep-powered).
- Bash: Execute a shell command (bash on Linux/macOS, PowerShell on Windows).
- Undo: Undo the most recent file edit.
- ReadRelated: Read multiple files at once into context.
- ProjectStructure: Get the project's file structure.
- Task: Delegate a complex task to a sub-agent with its own isolated context.
- AskUser: Ask the user a question when you need their input or a decision.
- StartServer: Start a local dev server in the background. Detects the port from server output, waits until ready, and returns the URL. The server keeps running after the tool returns.
- StopServer: Stop a running dev server by its ID.
- GetServerStatus: Check the status of a running server, or list all running servers.
- RestartServer: Restart a running dev server.
- ListServers: List all running dev servers.
- TestWebsite: Agent-to-agent delegation to the Browser Agent. Provide 'url' and a detailed 'prompt' describing exactly what to test (click, fill forms, screenshot, verify). The Browser Agent opens a real Chromium browser, runs the full flow autonomously in its own conversation, and returns a detailed report.

IMPORTANT: Use the right tool for the job:
- Use Glob for listing/finding files.
- Use Grep for searching file contents.
- Use Bash only for running tests, builds, git operations, installing packages, and executing scripts. Bash NEVER runs dev servers.
- Use Read to inspect files before editing them.
- Use ReadRelated to load multiple related files at once.
- Use ProjectStructure to understand the codebase layout.
- Use Task to delegate complex multi-step work to a sub-agent.
- When the user asks you to verify/test a website or check its appearance, ALWAYS: (1) StartServer to launch the dev server, then (2) TestWebsite with a detailed 'prompt' for the Browser Agent (what to click, what to fill, what to verify, what colors/theme to check). Do NOT skip steps — give the Browser Agent complete, explicit instructions so it can run the whole flow itself.

STARTING & TESTING WEB APPS (follow these rules strictly):
- ALWAYS start a dev server with StartServer and test with TestWebsite. NEVER run 'npm run dev', 'pnpm dev', 'next dev', 'vite', 'ng serve', 'react-scripts start', 'python -m http.server', or any long-running server command through Bash — Bash BLOCKS them and cannot report a reliable port. If you try, Bash will refuse and tell you to use StartServer.
- StartServer runs the server in the background, streams its real-time output, waits until it is ready, and returns the AUTHORITATIVE URL. TRUST that returned URL — it is the exact port the server actually bound to. Never invent, guess, or reuse a different port, even if another tool's output mentioned one.
- If server output contains "Port X is in use, trying another one..." the real URL is the one printed in the final 'Local:' / '➜ Local:' line — never the port that was already in use.
- For non-JavaScript projects (Go, Rust, Python, etc.) there is no package.json, so pass the exact start command to StartServer, e.g. StartServer(command="go run .", workdir=<project dir>). StartServer detects the port from the output.
- For a Vite/Next.js app with a package.json, StartServer(command="npm run dev", workdir=<project dir>) works, or omit the command and StartServer auto-detects it from package.json.
- After StartServer returns, immediately call TestWebsite(url=<the EXACT URL StartServer returned>, prompt=<detailed steps: what to navigate, wait for, click, fill, verify, screenshot>). Pass the returned URL; do not let TestWebsite re-guess a URL.
- The Browser Agent opens a real Chromium window, navigates to the URL you gave it, fills real form fields, clicks, screenshots, and returns a report. Trust its report but confirm it actually reached your URL.
- If the URL StartServer returned points to an app you did NOT start (e.g. the Demios app itself), STOP and report the discrepancy. Never test the wrong app.

Bash runs bash on Linux/macOS and PowerShell on Windows. Use the appropriate syntax for the platform:
- On Linux/macOS (bash): use && to chain commands, use test -f or [ -f ] for file tests, use date for dates
- On Windows (PowerShell): use ; to chain commands, use Test-Path for file tests, use Get-Date for dates

RULES:
- Always read files before editing them.
- Prefer Glob/Grep over Get-ChildItem/Select-String for file operations.
- When writing files, provide complete file contents.
- When editing files, use Edit with precise diffs.
- You can call multiple independent tools in a single response.
- Be thorough but concise in your responses.
- If no workspace is configured and the user asks you to work with files, ask them to set a workspace directory first.
- Use AskUser when you need clarification, a design decision, or specific input from the user. Do not guess when you're unsure.

SKILLS:
- Skills/ contains skill files with specialized instructions for specific tasks.
- Call ListSkills to see what skills are available.
- When a task matches a skill's description, call ReadSkill to load its instructions.
- Follow the loaded skill instructions closely.`, name)

	return a
}

func (a *Agent) Step(ctx context.Context, input string) (string, error) {
	a.history = append(a.history, llm.UserMessage(input))
	return a.loopStep(ctx)
}

func (a *Agent) StepStream(ctx context.Context, input string, events chan<- AgentEvent) {
	defer close(events)

	log.Printf("[agent] StepStream called with input: %d chars", len(input))
	a.history = append(a.history, llm.UserMessage(input))

	maxIter := 25
	for i := 0; i < maxIter; i++ {
		log.Printf("[agent] iteration %d/%d", i+1, maxIter)

		if err := a.maybePruneContext(ctx); err != nil {
			log.Printf("[agent] context pruning error: %v", err)
		}

		done := a.loopStepStream(ctx, events)
		if done {
			log.Printf("[agent] done at iteration %d", i+1)
			return
		}
	}

	log.Printf("[agent] exceeded max iterations")
	select {
	case events <- AgentEvent{Type: "error", Data: map[string]string{"error": fmt.Sprintf("agent exceeded max iterations (%d)", maxIter)}}:
	case <-ctx.Done():
	}
}

func (a *Agent) Reset() {
	a.history = nil
	a.historyReasonings = nil
	tools.ClearBackups()
}

func (a *Agent) RestoreHistory(history []llm.Message) {
	a.history = history
	a.historyReasonings = make([]string, len(history))
}

func (a *Agent) GetHistory() []llm.Message {
	return a.history
}

func (a *Agent) GetHistoryReasonings() []string {
	return a.historyReasonings
}

func (a *Agent) SetWorkspace(path string) {
	a.Workspace = path
}

func (a *Agent) SetModel(modelID string) error {
	return a.client.SetModel(modelID)
}

func (a *Agent) GetModels() []llm.ModelConfig {
	return a.client.GetModels()
}

func (a *Agent) GetCurrentModel() string {
	return a.client.GetCurrentModel().ID
}

func (a *Agent) Client() *Client {
	return a.client
}

func (a *Agent) ServerManager() *tools.ServerManager {
	return a.serverManager
}

func (a *Agent) SetPermissionMode(mode string) {
	a.PermissionMode = mode
}

// --- Task tool (sub-agent delegation) ---

func (a *Agent) makeTaskTool() tools.Tool {
	return tools.MakeTaskTool(func(ctx context.Context, prompt string, maxIter int) (string, error) {
		sub := &Agent{
			Name:              a.Name + "-sub",
			client:            a.client,
			tools:             a.tools,
			toolDefs:          a.toolDefs,
			SystemPrompt:      a.SystemPrompt,
			Workspace:         a.Workspace,
			PermissionMode:    "allow_all",
			permissionRequests: make(map[string]chan bool),
		}
		return sub.RunTask(ctx, prompt, maxIter)
	})
}

func (a *Agent) RunTask(ctx context.Context, prompt string, maxIter int) (string, error) {
	savedHistory := a.history
	savedReasonings := a.historyReasonings
	a.history = nil
	a.historyReasonings = nil
	defer func() {
		a.history = savedHistory
		a.historyReasonings = savedReasonings
	}()

	a.history = append(a.history, llm.UserMessage(prompt))

	if maxIter <= 0 {
		maxIter = 15
	}
	if maxIter > 30 {
		maxIter = 30
	}

	for i := 0; i < maxIter; i++ {
		resp, err := a.client.Chat(ctx, a.SystemPrompt, a.history, a.toolDefs)
		if err != nil {
			return "", err
		}
		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("no response choices")
		}

		choice := resp.Choices[0]
		msg := choice.Message

		if len(msg.ToolCalls) > 0 {
			tcParams := toToolCallParams(msg.ToolCalls)
			a.history = append(a.history, llm.AssistantMessageWithTools(msg.Content, tcParams))

			results := a.execToolCalls(ctx, tcParams)
			for _, r := range results {
				a.addToolResultToHistory(r)
			}
			continue
		}

		return msg.Content, nil
	}
	return "", fmt.Errorf("sub-agent exceeded max iterations (%d)", maxIter)
}

// --- AskUser tool (human-in-the-loop) ---

func (a *Agent) makeAskUserTool() tools.Tool {
	return tools.MakeAskUserTool(func(ctx context.Context, question string, options []string) (string, error) {
		return a.RequestHumanInput(ctx, question, options)
	})
}

func (a *Agent) makeTestWebsiteTool() tools.Tool {
	return tools.MakeTestWebsiteTool(a.serverManager, func(ctx context.Context, url string, prompt string) (string, error) {
		return a.runBrowserTest(ctx, url, prompt)
	})
}

func (a *Agent) runBrowserTest(ctx context.Context, url string, prompt string) (string, error) {
	ba := NewBrowserAgent("browser-test", a.client, a.serverManager)
	ba.Workspace = a.Workspace
	ba.TargetURL = url

	toolID := tools.ToolCallIDFrom(ctx)

	browserEvents := make(chan AgentEvent)
	if a.currentEvents != nil {
		go func() {
			for evt := range browserEvents {
				payload := map[string]interface{}{
					"agent":      "browser-agent",
					"tool_id":    toolID,
					"inner_type": evt.Type,
					"data":       evt.Data,
				}
				if !a.emit(ctx, a.currentEvents, AgentEvent{Type: "subagent-event", Data: payload}) {
					return
				}
			}
		}()
	} else {
		go func() {
			for range browserEvents {
			}
		}()
	}

	done := make(chan string, 1)
	go func() {
		done <- ba.StepStream(ctx, prompt, browserEvents)
	}()

	select {
	case report := <-done:
		if strings.TrimSpace(report) == "" {
			return "Browser test completed, but the browser agent returned no report. The page may have failed to load or the browser encountered an error.", nil
		}
		return report, nil
	case <-time.After(300 * time.Second):
		ba.StopBrowser()
		return "Browser test timed out after 300s.", nil
	}
}

// TestWebsite runs the Browser Agent against a resolved target URL and returns
// its report. URL resolution follows the shared harness rules (explicit URL,
// then a running server, then an auto-started server), so the port is always
// handed to the Browser Agent.
func (a *Agent) TestWebsite(ctx context.Context, url, prompt string) (string, error) {
	targetURL, err := a.serverManager.ResolveBrowserTarget(ctx, a.Workspace, url)
	if err != nil {
		return "", err
	}
	return a.runBrowserTest(ctx, targetURL, prompt)
}

// StartDevServer starts a dev server and returns its instance (exposed for the
// headless CLI harness).
func (a *Agent) StartDevServer(ctx context.Context, workdir, command string, port int) (*tools.ServerInstance, error) {
	return a.serverManager.StartServer(ctx, workdir, command, port)
}

// StopDevServer stops a dev server by ID (exposed for the headless CLI harness).
func (a *Agent) StopDevServer(id string) error {
	return a.serverManager.StopServer(id)
}

// ListDevServers returns all running dev servers (exposed for the headless
// CLI harness).
func (a *Agent) ListDevServers() []*tools.ServerInstance {
	return a.serverManager.ListServers()
}

var serverURLJSONRegex = regexp.MustCompile(`"url"\s*:\s*"([^"]+)"`)
var serverURLBareRegex = regexp.MustCompile(`(?:https?://)?(?:localhost|127\.0\.0\.1|0\.0\.0\.0|::1|\[::\]|\[::1\]|\[0\.0\.0\.0\]):\d+`)

// resolveServerURLFromOutput asks the currently selected LLM model to extract
// the URL a dev server is actually listening on from its startup output. Used
// by the ServerManager only when output parsing is ambiguous (port conflict or
// no URL detected), so the browser always tests the real port.
func (a *Agent) resolveServerURLFromOutput(ctx context.Context, output string) (string, error) {
	systemPrompt := `You extract the local URL that a dev server is listening on from its startup output.
Rules:
- Prefer the "Local" / "Localhost" URL over any "Network" URL.
- If the output says a port is already in use and the server switched to another port (e.g. "Port 5173 is in use, trying another one..."), use the NEW port the server actually bound to.
- If the server binds to 0.0.0.0 or [::], report it as http://localhost:<port>.
- Answer with ONLY a JSON object: {"url":"http://localhost:<port>"}
- If you cannot determine a URL, answer {"url":""}
- No markdown, no extra text.`

	resp, err := a.client.Chat(ctx, systemPrompt, []llm.Message{llm.UserMessage(output)}, nil)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response choices")
	}
	return extractServerURL(resp.Choices[0].Message.Content), nil
}

// extractServerURL pulls a localhost URL out of an LLM reply, tolerating JSON,
// bare URLs, and code fences.
func extractServerURL(reply string) string {
	reply = strings.TrimSpace(reply)
	if m := serverURLJSONRegex.FindStringSubmatch(reply); len(m) > 1 {
		return m[1]
	}
	if m := serverURLBareRegex.FindString(reply); m != "" {
		if !strings.HasPrefix(m, "http") {
			m = "http://" + m
		}
		return m
	}
	return ""
}

func (a *Agent) RequestHumanInput(ctx context.Context, question string, options []string) (string, error) {
	id := fmt.Sprintf("human-%d", len(a.humanInputRequests)+1)

	ch := make(chan string, 1)
	a.humanInputMu.Lock()
	a.humanInputRequests[id] = ch
	a.humanInputMu.Unlock()

	defer func() {
		a.humanInputMu.Lock()
		delete(a.humanInputRequests, id)
		a.humanInputMu.Unlock()
	}()

	if a.currentEvents != nil {
		data := map[string]interface{}{
			"id":       id,
			"question": question,
		}
		if len(options) > 0 {
			data["options"] = options
		}
		a.emit(ctx, a.currentEvents, AgentEvent{
			Type: "human-input-request",
			Data: data,
		})
	}

	select {
	case answer := <-ch:
		return answer, nil
	case <-time.After(600 * time.Second):
		return "", fmt.Errorf("human input request timed out")
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (a *Agent) RespondHumanInput(id string, input string) bool {
	a.humanInputMu.Lock()
	ch, ok := a.humanInputRequests[id]
	a.humanInputMu.Unlock()
	if !ok {
		return false
	}
	select {
	case ch <- input:
		return true
	default:
		return false
	}
}

// --- Context pruning ---

func (a *Agent) maybePruneContext(ctx context.Context) error {
	const targetBytes = 320000

	total := 0
	for _, msg := range a.history {
		data, _ := json.Marshal(msg)
		total += len(data)
	}

	if total < targetBytes {
		return nil
	}

	keep := 10
	if len(a.history) <= keep+1 {
		return nil
	}

	toSummarize := a.history[:len(a.history)-keep]

	summary, err := a.summarizeMessages(ctx, toSummarize)
	if err != nil {
		return fmt.Errorf("summarization failed: %w", err)
	}

	a.history = append(
		[]llm.Message{llm.SystemMessage("Previous conversation summary:\n" + summary)},
		a.history[len(a.history)-keep:]...,
	)

	log.Printf("[agent] pruned context: summarized %d messages, kept %d recent", len(toSummarize), keep)
	return nil
}

func (a *Agent) summarizeMessages(ctx context.Context, messages []llm.Message) (string, error) {
	var sb strings.Builder
	sb.WriteString("Summarize the following conversation concisely. Focus on:\n")
	sb.WriteString("- Key decisions made\n")
	sb.WriteString("- Files that were read or modified\n")
	sb.WriteString("- Code changes and their purposes\n")
	sb.WriteString("- What was accomplished\n\n")
	sb.WriteString("CONVERSATION:\n")

	for _, msg := range messages {
		switch {
		case msg.OfUser != nil:
			sb.WriteString("USER: ")
			sb.WriteString(llm.ContentString(msg.OfUser.Content))
			sb.WriteString("\n")
		case msg.OfAssistant != nil:
			if c := llm.ContentString(msg.OfAssistant.Content); c != "" {
				sb.WriteString("ASSISTANT: ")
				sb.WriteString(c)
				sb.WriteString("\n")
			}
			for _, tc := range msg.OfAssistant.ToolCalls {
				sb.WriteString(fmt.Sprintf("  -> TOOL: %s(%s)\n", tc.Function.Name, tc.Function.Arguments))
			}
		case msg.OfTool != nil:
			c := llm.ContentString(msg.OfTool.Content)
			if len(c) > 200 {
				c = c[:200] + "..."
			}
			sb.WriteString("  RESULT: ")
			sb.WriteString(c)
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\nSUMMARY:")

	resp, err := a.client.Chat(ctx,
		"You are a precise summarizer. Produce a concise factual summary. Include file paths and key decisions.",
		[]llm.Message{llm.UserMessage(sb.String())},
		nil,
	)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no summary response")
	}
	return resp.Choices[0].Message.Content, nil
}

// --- Permission system ---

func (a *Agent) RequestPermission(ctx context.Context, events chan<- AgentEvent, id, toolName string, args map[string]interface{}) (bool, error) {
	if a.PermissionMode == "allow_all" {
		return true, nil
	}

	needsConfirm := false
	switch a.PermissionMode {
	case "confirm_write":
		needsConfirm = (toolName == "Write" || toolName == "Edit")
	case "confirm_all":
		needsConfirm = true
	}

	if !needsConfirm {
		return true, nil
	}

	ch := make(chan bool, 1)
	a.permissionMu.Lock()
	a.permissionRequests[id] = ch
	a.permissionMu.Unlock()

	defer func() {
		a.permissionMu.Lock()
		delete(a.permissionRequests, id)
		a.permissionMu.Unlock()
	}()

	if !a.emit(ctx, events, AgentEvent{
		Type: "permission-request",
		Data: map[string]interface{}{
			"id":   id,
			"name": toolName,
			"args": args,
		},
	}) {
		return false, ctx.Err()
	}

	select {
	case allowed := <-ch:
		return allowed, nil
	case <-time.After(300 * time.Second):
		return false, fmt.Errorf("permission request timed out")
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

func (a *Agent) RespondPermission(id string, allowed bool) bool {
	a.permissionMu.Lock()
	ch, ok := a.permissionRequests[id]
	a.permissionMu.Unlock()
	if !ok {
		return false
	}
	select {
	case ch <- allowed:
		return true
	default:
		return false
	}
}

// --- loopStep (non-streaming) ---

func (a *Agent) loopStep(ctx context.Context) (string, error) {
	maxIter := 25
	for i := 0; i < maxIter; i++ {
		if err := a.maybePruneContext(ctx); err != nil {
			log.Printf("[agent] context pruning error: %v", err)
		}

		resp, err := a.client.Chat(ctx, a.SystemPrompt, a.history, a.toolDefs)
		if err != nil {
			return "", err
		}
		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("no response choices")
		}

		choice := resp.Choices[0]
		msg := choice.Message

		if len(msg.ToolCalls) > 0 {
			tcParams := toToolCallParams(msg.ToolCalls)
			a.history = append(a.history, llm.AssistantMessageWithTools(msg.Content, tcParams))

			results := a.execToolCalls(ctx, tcParams)
			for _, r := range results {
				a.addToolResultToHistory(r)
			}
			continue
		}

		a.history = append(a.history, llm.AssistantMessageWithTools(msg.Content, nil))
		return msg.Content, nil
	}
	return "", fmt.Errorf("agent exceeded max iterations (%d)", maxIter)
}

// --- loopStepStream (streaming) ---

func (a *Agent) loopStepStream(ctx context.Context, events chan<- AgentEvent) bool {
	a.currentEvents = events
	if !a.emit(ctx, events, AgentEvent{Type: "iteration", Data: map[string]interface{}{}}) {
		return true
	}
	log.Printf("[agent] calling LLM with %d tools", len(a.toolDefs))

	stream, err := a.client.ChatStream(ctx, a.SystemPrompt, a.history, a.toolDefs)
	if err != nil {
		log.Printf("[agent] LLM error: %v", err)
		a.emit(ctx, events, AgentEvent{Type: "error", Data: map[string]string{"error": err.Error()}})
		return true
	}

	var textContent strings.Builder
	var reasoningContent strings.Builder
	var toolCalls []openai.ChatCompletionMessageToolCallParam

	for event := range stream {
		switch event.Type {
		case "text":
			textContent.WriteString(event.Text)
			if !a.emit(ctx, events, AgentEvent{Type: "token", Data: map[string]string{"token": event.Text}}) {
				return true
			}

		case "think":
			reasoningContent.WriteString(event.Thinking)
			if !a.emit(ctx, events, AgentEvent{Type: "think", Data: map[string]string{"token": event.Thinking}}) {
				return true
			}

		case "tool_call":
			if event.ToolCall != nil {
				tc := event.ToolCall
				toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallParam{
					ID: tc.ID,
					Function: openai.ChatCompletionMessageToolCallFunctionParam{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
				})
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
					log.Printf("[agent] tool %s: failed to parse args JSON: %v", tc.Name, err)
					args = map[string]interface{}{}
				}
				log.Printf("[agent] tool: %s args: %s", tc.Name, tc.Arguments)
				if !a.emit(ctx, events, AgentEvent{
					Type: "tool-call",
					Data: map[string]interface{}{
						"id":   tc.ID,
						"name": tc.Name,
						"args": args,
					},
				}) {
					return true
				}
			}

		case "error":
			a.emit(ctx, events, AgentEvent{Type: "error", Data: map[string]string{"error": event.Error}})
			return true

		case "done":
		}
	}

	text := textContent.String()

	if text == "" && len(toolCalls) == 0 {
		text = "I processed your request but have no additional response."
	}

	a.history = append(a.history, llm.AssistantMessageWithTools(text, toolCalls))
	a.historyReasonings = append(a.historyReasonings, reasoningContent.String())

	if len(toolCalls) > 0 {
		log.Printf("[agent] received %d tool calls", len(toolCalls))

		results := a.execToolCalls(ctx, toolCalls, events)
		for _, r := range results {
			a.addToolResultToHistory(r)
			if !a.emit(ctx, events, resultToEvent(r)) {
				return true
			}
		}

		return false
	}

	log.Printf("[agent] final text response: %d chars", textContent.Len())
	a.emit(ctx, events, AgentEvent{Type: "done", Data: map[string]string{}})
	return true
}

// --- shared tool execution ---

type toolExecResult struct {
	index    int
	name     string
	id       string
	args     map[string]interface{}
	title    string
	output   string
	metadata map[string]interface{}
	err      error
}

func (a *Agent) execToolCalls(ctx context.Context, toolCalls []openai.ChatCompletionMessageToolCallParam, eventChans ...chan<- AgentEvent) []toolExecResult {
	ctx = tools.WithWorkspace(ctx, a.Workspace)
	log.Printf("[agent] executing %d tools", len(toolCalls))

	var events chan<- AgentEvent
	if len(eventChans) > 0 {
		events = eventChans[0]
	}

	results := make([]toolExecResult, len(toolCalls))
	var wg sync.WaitGroup

	for i, tc := range toolCalls {
		wg.Add(1)
		i, tc := i, tc
		go func() {
			defer wg.Done()

			tool, ok := a.tools[tc.Function.Name]
			if !ok {
				results[i] = toolExecResult{index: i, name: tc.Function.Name, id: tc.ID, err: fmt.Errorf("unknown tool: %s", tc.Function.Name)}
				return
			}

			rawArgs := json.RawMessage(tc.Function.Arguments)
			if string(rawArgs) == "" || string(rawArgs) == "null" {
				rawArgs = json.RawMessage("{}")
			}

			var parsedArgs map[string]interface{}
			if err := json.Unmarshal(rawArgs, &parsedArgs); err != nil {
				log.Printf("[agent] tool %s: failed to parse args for result event: %v", tc.Function.Name, err)
			}

			if events != nil {
				allowed, err := a.RequestPermission(ctx, events, tc.ID, tc.Function.Name, parsedArgs)
				if err != nil {
					results[i] = toolExecResult{index: i, name: tc.Function.Name, id: tc.ID, args: parsedArgs, err: fmt.Errorf("permission denied: %w", err)}
					return
				}
				if !allowed {
					results[i] = toolExecResult{index: i, name: tc.Function.Name, id: tc.ID, args: parsedArgs, err: fmt.Errorf("permission denied by user")}
					return
				}
			}

			// Per-tool context: carry the tool-call ID so streamed events can be
			// correlated in the UI, and an event emitter so long-running tools
			// (StartServer) can stream output in real time while still executing.
			toolCtx := tools.WithToolCallID(ctx, tc.ID)
			if events != nil {
				toolCtx = tools.WithEventEmitter(toolCtx, func(evtType string, data map[string]any) {
					a.emit(ctx, events, AgentEvent{Type: evtType, Data: data})
				})
			}

			execResult, err := tool.Execute(toolCtx, rawArgs)
			if err != nil {
				results[i] = toolExecResult{index: i, name: tc.Function.Name, id: tc.ID, args: parsedArgs, err: err}
				return
			}
			results[i] = toolExecResult{index: i, name: tc.Function.Name, id: tc.ID, args: parsedArgs, title: execResult.Title, output: execResult.Output, metadata: execResult.Metadata}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	<-done

	return results
}

func (a *Agent) addToolResultToHistory(r toolExecResult) {
	var content string
	if r.err != nil {
		content = fmt.Sprintf("Tool %q failed: %v", r.name, r.err)
	} else {
		content = r.output
	}
	if content == "" {
		content = fmt.Sprintf("[Tool %q returned empty output]", r.name)
	}
	a.history = append(a.history, llm.ToolMessage(content, r.id))
}

func resultToEvent(r toolExecResult) AgentEvent {
	if r.err != nil {
		return AgentEvent{
			Type: "tool-result",
			Data: map[string]interface{}{
				"id":    r.id,
				"name":  r.name,
				"args":  r.args,
				"error": r.err.Error(),
			},
		}
	}
	data := map[string]interface{}{
		"id":     r.id,
		"name":   r.name,
		"args":   r.args,
		"output": r.output,
	}
	if r.metadata != nil {
		if diff, ok := r.metadata["diff"]; ok {
			data["diff"] = diff
		}
		if diffs, ok := r.metadata["diffs"]; ok {
			data["diffs"] = diffs
		}
		if server, ok := r.metadata["server"]; ok {
			data["server"] = server
		}
	}
	return AgentEvent{
		Type: "tool-result",
		Data: data,
	}
}

func (a *Agent) emit(ctx context.Context, events chan<- AgentEvent, evt AgentEvent) bool {
	select {
	case events <- evt:
		return true
	case <-ctx.Done():
		return false
	}
}

// --- helpers ---

func toToolCallParams(tcs []openai.ChatCompletionMessageToolCall) []openai.ChatCompletionMessageToolCallParam {
	params := make([]openai.ChatCompletionMessageToolCallParam, len(tcs))
	for i, tc := range tcs {
		params[i] = openai.ChatCompletionMessageToolCallParam{
			ID: tc.ID,
			Function: openai.ChatCompletionMessageToolCallFunctionParam{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return params
}
