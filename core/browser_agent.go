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

	// humanInputFn is called to request input from the human user.
	// It blocks until the user responds or the context is cancelled.
	humanInputFn func(ctx context.Context, question string, options []string) (string, error)

	currentEvents chan<- AgentEvent

	// Duplicate-action detection: last N actions (tool name + args).
	lastActions []string
}

func NewBrowserAgent(name string, client *Client, serverManager *tools.ServerManager, humanInputFn func(ctx context.Context, question string, options []string) (string, error)) *BrowserAgent {
	ba := &BrowserAgent{
		Name:               name,
		client:             client,
		serverManager:      serverManager,
		browserSession:     tools.NewBrowserSession(),
		PermissionMode:     "allow_all",
		permissionRequests: make(map[string]chan bool),
		humanInputRequests: make(map[string]chan string),
		humanInputFn:       humanInputFn,
	}

	browserTools := map[string]tools.Tool{
		"browser_navigate":      tools.BrowserNavigate,
		"browser_click":         tools.BrowserClick,
		"browser_type":          tools.BrowserType,
		"browser_fill":          tools.BrowserFill,
		"browser_screenshot":    tools.BrowserScreenshot,
		"browser_extract":       tools.BrowserExtract,
		"browser_scroll":        tools.BrowserScroll,
		"browser_press":         tools.BrowserPress,
		"browser_back":          tools.BrowserBack,
		"browser_reload":        tools.BrowserReload,
		"browser_wait":          tools.BrowserWait,
		"browser_stop":          tools.BrowserStop,
		"SearchUIComponents":    tools.SearchUIComponents,
	}
	ba.tools = browserTools
	ba.toolDefs = tools.AllToolDefs(ba.tools)

	ba.SystemPrompt = `You are a Browser Automation Agent with VISION. You control a real Chromium browser and can SEE pages visually via screenshots.

You have access to the following browser tools:
- browser_navigate: Navigate to a URL.
- browser_click: Click an element by CSS selector or text.
- browser_fill: Fill an input field by CSS selector with a value.
- browser_type: Type text into a specific element by CSS selector.
- browser_screenshot: Capture the current viewport as a base64 PNG image. YOU WILL SEE THIS IMAGE — analyze it visually.
- browser_extract: Extract visible text from the page or a specific element.
- browser_scroll: Scroll the page vertically and/or horizontally.
- browser_press: Press a keyboard key (Enter, Tab, Escape, etc.).
- browser_back: Navigate back in browser history.
- browser_reload: Reload the current page.
- browser_wait: Wait for a selector to appear or wait a fixed number of milliseconds.
- browser_stop: Stop the browser session and close Chromium.
- SearchUIComponents: Search for UI components from shadcn/ui, ReactBits, Magic UI, and Aceternity UI registries.

YOU HAVE VISION — THIS IS CRITICAL:
After browser_screenshot, you RECEIVE THE ACTUAL PAGE IMAGE. Analyze it:
- What layout, grid, sections do you see?
- What text, headings, buttons are visible?
- What colors, theme, spacing, typography?
- Are there errors, broken layouts, missing images, 404s?
- Does the page match what was expected?
Use this visual information to guide your next action. Do NOT ignore what you see.

PLANNING:
- Before acting, think step-by-step about your approach.
- After each significant action, take a screenshot and analyze what you see.
- If the page doesn't match expectations, investigate before continuing.
- If stuck, try a fundamentally different approach — not the same action again.

EXECUTION RULES — follow strictly:
- The task begins with 'TARGET URL'. Your FIRST action MUST be browser_navigate to that URL.
- If navigation fails, analyze the error and decide: wrong URL? server down? network issue?
- ALWAYS take browser_screenshot after significant actions.
- ALWAYS use browser_extract to read content — never guess at selectors or text.
- Use the PAGE SNAPSHOT for real form field selectors — never hallucinate selectors like #username.
- If a selector fails, try alternatives: text content, placeholder, partial class, aria-label.
- Keep actions deliberate and sequential. NEVER repeat the same action.
- Use browser_wait after navigating and after clicks to let dynamic content load.
- Use browser_fill for form inputs. Use browser_click for buttons and links.

ASK FOR HELP:
- If unsure what to do next, describe what you see and ask for guidance.
- If the page shows an unexpected error, explain what you see visually.
- If a form requires credentials you don't have, stop and ask.
- Never guess at credentials, API keys, or hidden values.

FINAL REPORT — after completing ALL steps:
Write a DETAILED report:
1. Walk through each step — state whether it succeeded or failed.
2. Describe what you SAW on each page (layout, colors, elements, text).
3. Note any visual bugs: broken layouts, overlapping elements, wrong colors, missing content.
4. Flag anything that failed or looked wrong.
5. Only call browser_stop after writing the report.`

	return ba
}

func (ba *BrowserAgent) StepStream(ctx context.Context, input string, events chan<- AgentEvent) string {
	defer close(events)

	log.Printf("[browser-agent] StepStream called: %d chars", len(input))
	if ba.TargetURL != "" {
		header := fmt.Sprintf("TARGET URL (this is the exact URL to test): %s\n\n", ba.TargetURL)
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

	// Ask the human which model to use for the Browser Agent.
	if ba.humanInputFn != nil {
		ba.emit(ctx, events, AgentEvent{Type: "human-input-request", Data: map[string]interface{}{
			"question": "Would you like to continue with the same model or choose a different model for the Browser Agent?",
			"options":  []string{"Same Model", "Choose Different Model"},
		}})

		choice, err := ba.humanInputFn(ctx, "Would you like to continue with the same model or choose a different model for the Browser Agent?", []string{"Same Model", "Choose Different Model"})
		if err != nil {
			log.Printf("[browser-agent] human input error (using same model): %v", err)
		} else if choice == "Choose Different Model" {
			// Build model options from all available models
			allModels := ba.client.GetModels()
			modelOptions := make([]string, 0, len(allModels))
			for _, m := range allModels {
				modelOptions = append(modelOptions, m.ID)
			}
			if len(modelOptions) > 15 {
				modelOptions = modelOptions[:15]
			}

			ba.emit(ctx, events, AgentEvent{Type: "human-input-request", Data: map[string]interface{}{
				"question": "Select a model for the Browser Agent:",
				"options":  modelOptions,
			}})

			selectedModel, err := ba.humanInputFn(ctx, "Select a model for the Browser Agent:", modelOptions)
			if err != nil {
				log.Printf("[browser-agent] human input error (model selection): %v", err)
			} else if selectedModel != "" {
				if setErr := ba.client.SetModel(selectedModel); setErr != nil {
					log.Printf("[browser-agent] failed to switch model to %s: %v", selectedModel, setErr)
				} else {
					log.Printf("[browser-agent] switched to model: %s", selectedModel)
					ba.emit(ctx, events, AgentEvent{Type: "browser-model-switched", Data: map[string]string{
						"model": selectedModel,
					}})
				}
			}
		}
	}

	// Guarantee the browser is already on the target page BEFORE the model does
	// anything — this no longer depends on the model remembering to call
	// browser_navigate. Surfing always works.
	if ba.TargetURL != "" {
		if navErr := ba.browserSession.Navigate(ba.TargetURL); navErr != nil {
			log.Printf("[browser-agent] auto-navigate to %s failed: %v", ba.TargetURL, navErr)
			// Inject the error into history so the LLM can analyze it
			// (was it a wrong URL? server not running? network issue?).
			ba.history = append(ba.history, llm.UserMessage(
				fmt.Sprintf("[SYSTEM] Auto-navigation to %s failed: %v. Analyze this error and decide what to do. You may try alternative approaches or report the failure.", ba.TargetURL, navErr),
			))
		} else {
			log.Printf("[browser-agent] auto-navigated to %s", ba.TargetURL)
		}
	}

	// Inject a ground-truth page snapshot (URL, title, real form-field selectors,
	// visible text) so the model fills actual fields instead of guessing
	// selectors like #username. Text entry/clicking always has real targets.
	if snap := ba.snapshotPage(); snap != "" {
		ba.history = append(ba.history, llm.UserMessage(snap))
	}

	ba.emit(ctx, events, AgentEvent{Type: "browser-open", Data: map[string]string{
		"status": "Browser opened — Chromium popup visible",
	}})

	maxIter := 40
	for i := 0; i < maxIter; i++ {
		log.Printf("[browser-agent] iteration %d/%d", i+1, maxIter)

		if err := ba.maybePruneContext(ctx); err != nil {
			log.Printf("[browser-agent] context pruning error: %v", err)
		}

		finalText, done := ba.loopStepStream(ctx, events)
		if done {
			log.Printf("[browser-agent] done at iteration %d", i+1)
			return finalText
		}
	}

	log.Printf("[browser-agent] exceeded max iterations")
	ba.StopBrowser()
	ba.emit(ctx, events, AgentEvent{Type: "browser-done", Data: map[string]string{}})
	return fmt.Sprintf("Browser agent completed after %d iterations. The browser has been closed. Review the actions above for results.", maxIter)
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

		// Duplicate-action detection: if the same tool+args was called in the
		// last 3 iterations, inject a warning so the LLM stops looping.
		if dup := ba.detectDuplicate(toolCalls); dup != "" {
			ba.history = append(ba.history, llm.UserMessage(
				fmt.Sprintf("[SYSTEM] You are repeating the same action: %s. You have already done this. Do something different or write your final report and call browser_stop.", dup),
			))
		}

		results := ba.execToolCalls(ctx, toolCalls, events)

		// Track actions for duplicate detection.
		ba.trackActions(toolCalls)

		// Check if browser_stop was called — if so, break the loop cleanly.
		stopped := false
		var finalText string
		for _, r := range results {
			ba.addToolResultToHistory(r)
			if !ba.emit(ctx, events, ba.resultToEvent(r)) {
				return "", true
			}
			ba.emitBrowserAction(ctx, events, r)
			if r.name == "browser_stop" {
				stopped = true
				if r.err == nil {
					finalText = r.output
				}
			}
		}

		if stopped {
			if finalText == "" {
				finalText = "Browser session ended."
			}
			log.Printf("[browser-agent] browser_stop called, ending session")
			ba.emit(ctx, events, AgentEvent{Type: "browser-done", Data: map[string]string{}})
			return finalText, true
		}

		// Re-inject a fresh page snapshot so the LLM always sees the
		// current state of the page (URL, title, form fields, visible text).
		// This prevents hallucinated selectors and stale page state.
		if snap := ba.snapshotPage(); snap != "" {
			ba.history = append(ba.history, llm.UserMessage(snap))
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

	// For browser_screenshot, add the tool result AND send the screenshot
	// image as a user message so the LLM can actually see the page visually.
	if r.name == "browser_screenshot" && r.err == nil && r.metadata != nil {
		if screenshotB64, ok := r.metadata["screenshot"].(string); ok && screenshotB64 != "" {
			ba.history = append(ba.history, llm.ToolMessage(content, r.id))
			ba.history = append(ba.history, llm.UserMessageWithImages(
				"Here is the current screenshot of the browser page. Analyze it carefully — look at the layout, text, buttons, forms, colors, and any visible content. Use this visual information to decide your next action.",
				[]string{screenshotB64},
			))
			return
		}
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

// snapshotPage builds a ground-truth description of the currently loaded page
// (URL, title, interactive form fields with real selectors, and visible text)
// and injects it into the browser agent's context. Best-effort: any failure
// returns "" and the agent proceeds normally.
func (ba *BrowserAgent) snapshotPage() string {
	sess := ba.browserSession
	if sess == nil || !sess.IsOpen() || sess.Page() == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("PAGE SNAPSHOT (the browser is already on this page — verify it, do not re-navigate unless the task requires a different URL):\n")

	url, _ := sess.CurrentURL()
	sb.WriteString(fmt.Sprintf("URL: %s\n", url))
	if title, err := sess.CurrentTitle(); err == nil && title != "" {
		sb.WriteString(fmt.Sprintf("Title: %s\n", title))
	}

	if fields := ba.formFieldSnapshot(); len(fields) > 0 {
		sb.WriteString("Interactive elements (use these real selectors for fill/click/type):\n")
		for _, f := range fields {
			sb.WriteString("  - " + f + "\n")
		}
	}

	if text, err := sess.Page().TextContent("body"); err == nil {
		text = strings.TrimSpace(text)
		if text == "" {
			text = "(empty page — it may still be loading; use browser_wait and browser_extract)"
		}
		if len(text) > 2000 {
			text = text[:2000] + "..."
		}
		sb.WriteString("Visible text (truncated):\n" + text + "\n")
	}

	return sb.String()
}

// formFieldSnapshot lists input/textarea/select/button elements on the page
// with their id, name, type, placeholder and visible label, so the model can
// target real elements instead of guessing CSS selectors.
func (ba *BrowserAgent) formFieldSnapshot() []string {
	sess := ba.browserSession
	if sess == nil || !sess.IsOpen() || sess.Page() == nil {
		return nil
	}

	const js = `(els) => els.map((el) => ({
		tag: el.tagName.toLowerCase(),
		id: el.id || "",
		name: el.name || "",
		type: el.type || "",
		placeholder: el.placeholder || "",
		text: (el.innerText || "").trim().slice(0, 80)
	})).slice(0, 40)`

	res, err := sess.Page().Locator("input, textarea, select, button").EvaluateAll(js)
	if err != nil || res == nil {
		return nil
	}
	raw, err := json.Marshal(res)
	if err != nil {
		return nil
	}
	var items []map[string]string
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}

	out := make([]string, 0, len(items))
	for _, it := range items {
		var parts []string
		if it["tag"] != "" {
			parts = append(parts, "<"+it["tag"]+">")
		}
		if it["id"] != "" {
			parts = append(parts, "id="+it["id"])
		}
		if it["name"] != "" {
			parts = append(parts, "name="+it["name"])
		}
		if it["type"] != "" && it["type"] != "text" {
			parts = append(parts, "type="+it["type"])
		}
		if it["placeholder"] != "" {
			parts = append(parts, "placeholder=\""+it["placeholder"]+"\"")
		}
		if it["text"] != "" {
			parts = append(parts, "label=\""+it["text"]+"\"")
		}
		if len(parts) > 0 {
			out = append(out, strings.Join(parts, " "))
		}
	}
	return out
}

func (ba *BrowserAgent) StopBrowser() {
	if ba.browserSession != nil {
		_ = ba.browserSession.Stop(context.Background())
	}
}

func (ba *BrowserAgent) Reset() {
	ba.history = nil
	ba.historyReasonings = nil
	ba.lastActions = nil
	tools.ClearBackups()
}

// maybePruneContext summarises old history when it grows too large, keeping
// the most recent messages intact. Browser tasks are shorter than main-agent
// tasks so the threshold is lower (128KB vs 320KB).
func (ba *BrowserAgent) maybePruneContext(ctx context.Context) error {
	const targetBytes = 128000

	total := 0
	for _, msg := range ba.history {
		data, _ := json.Marshal(msg)
		total += len(data)
	}

	if total < targetBytes {
		return nil
	}

	keep := 8
	if len(ba.history) <= keep+1 {
		return nil
	}

	toSummarize := ba.history[:len(ba.history)-keep]

	summary, err := ba.summarizeMessages(ctx, toSummarize)
	if err != nil {
		return fmt.Errorf("summarization failed: %w", err)
	}

	ba.history = append(
		[]llm.Message{llm.SystemMessage("Previous conversation summary:\n" + summary)},
		ba.history[len(ba.history)-keep:]...,
	)

	log.Printf("[browser-agent] pruned context: summarized %d messages, kept %d recent", len(toSummarize), keep)
	return nil
}

func (ba *BrowserAgent) summarizeMessages(ctx context.Context, messages []llm.Message) (string, error) {
	var sb strings.Builder
	sb.WriteString("Summarize the following browser automation conversation concisely. Focus on:\n")
	sb.WriteString("- What URL was tested\n")
	sb.WriteString("- Steps attempted and their results (success/failure)\n")
	sb.WriteString("- What was seen on the page (title, elements, errors)\n")
	sb.WriteString("- What worked and what failed\n\n")
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

	resp, err := ba.client.Chat(ctx,
		"You are a precise summarizer. Produce a concise factual summary. Include URLs tested, steps taken, and what succeeded or failed.",
		[]llm.Message{llm.UserMessage(sb.String())},
		nil,
	)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from summarizer")
	}
	return resp.Choices[0].Message.Content, nil
}

// detectDuplicate returns the name+args of a tool call if the exact same call
// appears in the last 3 tracked actions. Returns "" if no duplicate.
func (ba *BrowserAgent) detectDuplicate(toolCalls []openai.ChatCompletionMessageToolCallParam) string {
	if len(ba.lastActions) == 0 || len(toolCalls) == 0 {
		return ""
	}
	tc := toolCalls[0]
	sig := tc.Function.Name + ":" + tc.Function.Arguments
	for _, prev := range ba.lastActions {
		if prev == sig {
			return tc.Function.Name
		}
	}
	return ""
}

// trackActions records the latest tool calls for duplicate detection,
// keeping at most 5 entries.
func (ba *BrowserAgent) trackActions(toolCalls []openai.ChatCompletionMessageToolCallParam) {
	for _, tc := range toolCalls {
		sig := tc.Function.Name + ":" + tc.Function.Arguments
		ba.lastActions = append(ba.lastActions, sig)
	}
	if len(ba.lastActions) > 5 {
		ba.lastActions = ba.lastActions[len(ba.lastActions)-5:]
	}
}