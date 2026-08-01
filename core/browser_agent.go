package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	openai "github.com/openai/openai-go"

	"demios/llm"
	"demios/tools"
)

type BrowserAgent struct {
	Name              string
	SystemPrompt      string
	Workspace         string
	TargetURL         string
	client            *Client
	serverManager     *tools.ServerManager
	history           []llm.Message
	historyReasonings []string
	tools             map[string]tools.Tool
	toolDefs          []llm.ToolDefinition
	browserSession    *tools.BrowserSession
	PermissionMode    string

	permissionRequests map[string]chan bool
	permissionMu       sync.Mutex

	humanInputRequests map[string]chan string
	humanInputMu       sync.Mutex

	currentEvents chan<- AgentEvent
}

func NewBrowserAgent(name string, client *Client, serverManager *tools.ServerManager) *BrowserAgent {
	ba := &BrowserAgent{
		Name:               name,
		client:             client,
		serverManager:      serverManager,
		browserSession:     tools.NewBrowserSession(),
		PermissionMode:     "allow_all",
		permissionRequests: make(map[string]chan bool),
		humanInputRequests: make(map[string]chan string),
	}

	browserTools := map[string]tools.Tool{
		"browser_navigate":   tools.BrowserNavigate,
		"browser_click":      tools.BrowserClick,
		"browser_type":       tools.BrowserType,
		"browser_fill":       tools.BrowserFill,
		"browser_screenshot": tools.BrowserScreenshot,
		"browser_extract":    tools.BrowserExtract,
		"browser_scroll":     tools.BrowserScroll,
		"browser_press":      tools.BrowserPress,
		"browser_back":       tools.BrowserBack,
		"browser_wait":       tools.BrowserWait,
		"browser_stop":       tools.BrowserStop,
		"browser_test":       tools.BrowserTest,
	}
	ba.tools = browserTools
	ba.toolDefs = tools.AllToolDefs(ba.tools)

	ba.SystemPrompt = `You are a browser automation agent. You control a real Chromium browser window and can navigate the web, click elements, type and fill text, take screenshots, and extract content.

You have access to the following browser tools:
- browser_navigate: Navigate to a URL.
- browser_click: Click an element by CSS selector or text.
- browser_fill: Fill an input field by CSS selector with a value (use for form fields like login forms).
- browser_type: Type text into the active input field.
- browser_screenshot: Capture the current viewport as a base64 PNG image.
- browser_extract: Extract visible text from the page or a specific element.
- browser_scroll: Scroll the page up or down.
- browser_press: Press a keyboard key (Enter, Tab, Escape, etc.).
- browser_back: Navigate back in browser history.
- browser_wait: Wait for a selector to appear or wait a fixed number of milliseconds.
- browser_stop: Stop the browser session and close Chromium.
- browser_test: Test a website by navigating to it, taking a screenshot, and reporting what you see.

RULES — follow them strictly:
- The user has given you a COMPLETE TASK. Execute EVERY step of it in order. Do not stop partway through.
- The task message begins with 'TARGET URL'. Your very FIRST action MUST be browser_navigate to that exact URL.
- NEVER guess, probe, or hunt for other ports or URLs. If the TARGET URL fails to load, report the failure clearly and stop trying other addresses.
- Use browser_navigate first to reach the target URL.
- Use browser_wait after navigating and after clicks to let dynamic content load.
- Use browser_fill to fill form inputs by selector (#username, #password, etc.). Use browser_click for buttons.
- Take a browser_screenshot after significant actions and describe what you see (colors, theme, buttons, layout) based on the extracted text and your observations.
- Use browser_extract to read page content instead of guessing. Extract after each step that changes the page.
- If a selector is not found, try alternatives (partial text, placeholder, class) and report the difficulty.
- Keep actions deliberate and sequential.
- After completing ALL steps, write a DETAILED FINAL REPORT: walk through each step, state whether it succeeded, describe what you saw (page title, buttons, colors/theme, form fields, items in lists), and flag anything that failed.
- Only call browser_stop after you have completed the task and written your report. Do not stop the browser early.`

	return ba
}

func (ba *BrowserAgent) StepStream(ctx context.Context, input string, events chan<- AgentEvent) string {
	defer close(events)

	log.Printf("[browser-agent] StepStream called: %d chars", len(input))
	if ba.TargetURL != "" {
		header := fmt.Sprintf("TARGET URL (you MUST browser_navigate to this exact URL first): %s\n\n", ba.TargetURL)
		input = header + input
	}
	ba.history = append(ba.history, llm.UserMessage(input))

	if err := ba.browserSession.Start(ctx); err != nil {
		log.Printf("[browser-agent] failed to start browser: %v", err)
		ba.emit(ctx, events, AgentEvent{Type: "browser-error", Data: map[string]string{
			"error": fmt.Sprintf("Failed to start browser: %v", err),
		}})
		return fmt.Sprintf("Failed to start browser: %v", err)
	}

	ba.emit(ctx, events, AgentEvent{Type: "browser-open", Data: map[string]string{
		"status": "Browser opened — Chromium popup visible",
	}})

	maxIter := 25
	for i := 0; i < maxIter; i++ {
		log.Printf("[browser-agent] iteration %d/%d", i+1, maxIter)

		finalText, done := ba.loopStepStream(ctx, events)
		if done {
			log.Printf("[browser-agent] done at iteration %d", i+1)
			return finalText
		}
	}

	log.Printf("[browser-agent] exceeded max iterations")
	ba.emit(ctx, events, AgentEvent{Type: "browser-error", Data: map[string]string{
		"error": fmt.Sprintf("Browser agent exceeded max iterations (%d)", maxIter),
	}})
	return fmt.Sprintf("Browser agent exceeded max iterations (%d)", maxIter)
}

func (ba *BrowserAgent) loopStepStream(ctx context.Context, events chan<- AgentEvent) (string, bool) {
	ba.currentEvents = events
	if !ba.emit(ctx, events, AgentEvent{Type: "iteration", Data: map[string]interface{}{}}) {
		return "", true
	}
	log.Printf("[browser-agent] calling LLM with %d tools", len(ba.toolDefs))

	stream, err := ba.client.ChatStream(ctx, ba.SystemPrompt, ba.history, ba.toolDefs)
	if err != nil {
		log.Printf("[browser-agent] LLM error: %v", err)
		ba.emit(ctx, events, AgentEvent{Type: "error", Data: map[string]string{"error": err.Error()}})
		return "", true
	}

	var textContent strings.Builder
	var reasoningContent strings.Builder
	var toolCalls []openai.ChatCompletionMessageToolCallParam

	for event := range stream {
		switch event.Type {
		case "text":
			textContent.WriteString(event.Text)
			if !ba.emit(ctx, events, AgentEvent{Type: "token", Data: map[string]string{"token": event.Text}}) {
				return "", true
			}

		case "think":
			reasoningContent.WriteString(event.Thinking)
			if !ba.emit(ctx, events, AgentEvent{Type: "think", Data: map[string]string{"token": event.Thinking}}) {
				return "", true
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
					log.Printf("[browser-agent] tool %s: failed to parse args JSON: %v", tc.Name, err)
					args = map[string]interface{}{}
				}
				ba.emit(ctx, events, AgentEvent{
					Type: "tool-call",
					Data: map[string]interface{}{
						"id":   tc.ID,
						"name": tc.Name,
						"args": args,
					},
				})
			}

		case "error":
			ba.emit(ctx, events, AgentEvent{Type: "error", Data: map[string]string{"error": event.Error}})
			return "", true

		case "done":
		}
	}

	text := textContent.String()

	if text == "" && len(toolCalls) == 0 {
		text = "I completed the browser task."
	}

	ba.history = append(ba.history, llm.AssistantMessageWithTools(text, toolCalls))
	ba.historyReasonings = append(ba.historyReasonings, reasoningContent.String())

	if len(toolCalls) > 0 {
		log.Printf("[browser-agent] received %d tool calls", len(toolCalls))

		results := ba.execToolCalls(ctx, toolCalls, events)
		for _, r := range results {
			ba.addToolResultToHistory(r)
			if !ba.emit(ctx, events, ba.resultToEvent(r)) {
				return "", true
			}
			ba.emitBrowserAction(ctx, events, r)
		}

		return "", false
	}

	log.Printf("[browser-agent] final text response: %d chars", textContent.Len())
	ba.emit(ctx, events, AgentEvent{Type: "browser-done", Data: map[string]string{}})
	return text, true
}

func (ba *BrowserAgent) execToolCalls(ctx context.Context, toolCalls []openai.ChatCompletionMessageToolCallParam, eventChans ...chan<- AgentEvent) []toolExecResult {
	ctx = tools.WithWorkspace(ctx, ba.Workspace)
	ctx = tools.WithBrowserSession(ctx, ba.browserSession)
	if ba.serverManager != nil {
		ctx = tools.WithBrowserExcludedPorts(ctx, ba.serverManager.ExcludedPorts())
	}
	log.Printf("[browser-agent] executing %d tools", len(toolCalls))

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

			tool, ok := ba.tools[tc.Function.Name]
			if !ok {
				results[i] = toolExecResult{index: i, name: tc.Function.Name, id: tc.ID, err: fmt.Errorf("unknown tool: %s", tc.Function.Name)}
				return
			}

			rawArgs := json.RawMessage(tc.Function.Arguments)
			if string(rawArgs) == "" || string(rawArgs) == "null" {
				rawArgs = json.RawMessage("{}")
			}

			var parsedArgs map[string]interface{}
			_ = json.Unmarshal(rawArgs, &parsedArgs)

			if events != nil {
				allowed, err := ba.RequestPermission(ctx, events, tc.ID, tc.Function.Name, parsedArgs)
				if err != nil {
					results[i] = toolExecResult{index: i, name: tc.Function.Name, id: tc.ID, args: parsedArgs, err: fmt.Errorf("permission denied: %w", err)}
					return
				}
				if !allowed {
					results[i] = toolExecResult{index: i, name: tc.Function.Name, id: tc.ID, args: parsedArgs, err: fmt.Errorf("permission denied by user")}
					return
				}
			}

			execResult, err := tool.Execute(ctx, rawArgs)
			if err != nil {
				results[i] = toolExecResult{index: i, name: tc.Function.Name, id: tc.ID, args: parsedArgs, err: err}
				return
			}
			results[i] = toolExecResult{index: i, name: tc.Function.Name, id: tc.ID, args: parsedArgs, output: execResult.Output, metadata: execResult.Metadata}
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

func (ba *BrowserAgent) addToolResultToHistory(r toolExecResult) {
	var content string
	if r.err != nil {
		content = fmt.Sprintf("Tool %q failed: %v", r.name, r.err)
	} else {
		content = r.output
	}
	if content == "" {
		content = fmt.Sprintf("[Tool %q returned empty output]", r.name)
	}
	ba.history = append(ba.history, llm.ToolMessage(content, r.id))
}

func (ba *BrowserAgent) resultToEvent(r toolExecResult) AgentEvent {
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
		if screenshot, ok := r.metadata["screenshot"]; ok {
			data["screenshot"] = screenshot
		}
		if url, ok := r.metadata["url"]; ok {
			data["url"] = url
		}
		if title, ok := r.metadata["title"]; ok {
			data["title"] = title
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

// emitBrowserAction translates a browser tool result into the UI event types
// the frontend topology/browser panel already understands.
func (ba *BrowserAgent) emitBrowserAction(ctx context.Context, events chan<- AgentEvent, r toolExecResult) {
	if r.err != nil {
		ba.emit(ctx, events, AgentEvent{Type: "browser-action", Data: map[string]string{
			"status": fmt.Sprintf("Browser action failed: %v\n", r.err),
		}})
		return
	}
	if r.metadata != nil {
		switch action, _ := r.metadata["action"].(string); action {
		case "navigate", "test":
			url, _ := r.metadata["url"].(string)
			title, _ := r.metadata["title"].(string)
			ba.emit(ctx, events, AgentEvent{Type: "page-navigated", Data: map[string]string{
				"url":   url,
				"title": title,
			}})
		case "screenshot":
			b64, _ := r.metadata["screenshot"].(string)
			ba.emit(ctx, events, AgentEvent{Type: "browser-screenshot", Data: map[string]string{
				"screenshot": b64,
			}})
		}
	}
	status := r.title
	if status == "" {
		status = "Browser action completed"
	}
	ba.emit(ctx, events, AgentEvent{Type: "browser-action", Data: map[string]string{
		"status": status + "\n",
	}})
}

func (ba *BrowserAgent) emit(ctx context.Context, events chan<- AgentEvent, evt AgentEvent) bool {
	select {
	case events <- evt:
		return true
	case <-ctx.Done():
		return false
	}
}

func (ba *BrowserAgent) RequestPermission(ctx context.Context, events chan<- AgentEvent, id, toolName string, args map[string]interface{}) (bool, error) {
	if ba.PermissionMode == "allow_all" {
		return true, nil
	}
	return false, fmt.Errorf("permission check not implemented for browser agent")
}

func (ba *BrowserAgent) StopBrowser() {
	if ba.browserSession != nil {
		_ = ba.browserSession.Stop(context.Background())
	}
}

func (ba *BrowserAgent) Reset() {
	ba.history = nil
	ba.historyReasonings = nil
	tools.ClearBackups()
}