package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/invopop/jsonschema"
)

// --- StartServer ---

type StartServerArgs struct {
	Command string `json:"command,omitempty" jsonschema:"title=Command,description=Command to start the server (e.g. 'npm run dev'). If empty, auto-detected from package.json."`
	Workdir string `json:"workdir,omitempty" jsonschema:"title=Workdir,description=Working directory for the server (default: workspace root)"`
	Port    int    `json:"port,omitempty" jsonschema:"title=Port,description=Preferred port number (default: auto-detect from server output)"`
}

func MakeStartServerTool(sm *ServerManager) Tool {
	return Tool{
		ID:          "StartServer",
		Description: "Start a local dev server. Spawns the process in the background, detects the actual port from server output, waits until the server is ready, and returns immediately with the server URL. The server continues running after this tool returns. Use StopServer to stop it later. If a server is already running for the same project, returns that server instead.",
		Schema:      jsonschema.Reflect(&StartServerArgs{}),
		Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
			var args StartServerArgs
			if err := json.Unmarshal(rawArgs, &args); err != nil {
				return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
			}

			workdir := args.Workdir
			if workdir == "" {
				workdir = WorkspaceFrom(ctx)
			} else if !filepath.IsAbs(workdir) {
				if ws := WorkspaceFrom(ctx); ws != "" {
					workdir = filepath.Join(ws, workdir)
				}
			}

			instance, err := sm.StartServer(ctx, workdir, args.Command, args.Port)
			if err != nil {
				return ExecuteResult{}, err
			}

			captured := strings.TrimSpace(instance.Stdout())
			if stderr := strings.TrimSpace(instance.Stderr()); stderr != "" {
				if captured != "" {
					captured += "\n"
				}
				captured += stderr
			}

			output := fmt.Sprintf("Server started successfully.\nID: %s\nURL: %s\nPort: %d\nPID: %d\nProject: %s\nStatus: %s\nCommand: %s\nUptime: just started",
				instance.ID, instance.URL, instance.Port, instance.PID, instance.ProjectDir, instance.Status, instance.Command)
			if captured != "" {
				output += "\n\nConsole output:\n" + captured
			}

			return ExecuteResult{
				Title:  fmt.Sprintf("Server started at %s", instance.URL),
				Output: output,
				Metadata: map[string]any{
					"server": map[string]any{
						"id":          instance.ID,
						"pid":         instance.PID,
						"port":        instance.Port,
						"url":         instance.URL,
						"project_dir": instance.ProjectDir,
						"status":      instance.Status,
						"command":     instance.Command,
					},
				},
			}, nil
		},
	}
}

// --- StopServer ---

type StopServerArgs struct {
	ID string `json:"id" jsonschema:"title=ID,description=Server ID to stop (e.g. 'server-1')"`
}

func MakeStopServerTool(sm *ServerManager) Tool {
	return Tool{
		ID:          "StopServer",
		Description: "Stop a running dev server by its ID. Use ListServers or GetServerStatus to find server IDs. The process and all its children are killed.",
		Schema:      jsonschema.Reflect(&StopServerArgs{}),
		Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
			var args StopServerArgs
			if err := json.Unmarshal(rawArgs, &args); err != nil {
				return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
			}
			if args.ID == "" {
				return ExecuteResult{}, fmt.Errorf("id is required")
			}

			if err := sm.StopServer(args.ID); err != nil {
				return ExecuteResult{}, err
			}

			return ExecuteResult{
				Title:  fmt.Sprintf("Stopped server %s", args.ID),
				Output: fmt.Sprintf("Server %s has been stopped.", args.ID),
				Metadata: map[string]any{
					"server": map[string]any{
						"id":     args.ID,
						"status": "stopped",
					},
				},
			}, nil
		},
	}
}

// --- GetServerStatus ---

type GetServerStatusArgs struct {
	ID string `json:"id,omitempty" jsonschema:"title=ID,description=Server ID to check (e.g. 'server-1'). If empty, returns all servers."`
}

func MakeGetServerStatusTool(sm *ServerManager) Tool {
	return Tool{
		ID:          "GetServerStatus",
		Description: "Get the status of a running server by ID, or list all running servers if no ID is provided.",
		Schema:      jsonschema.Reflect(&GetServerStatusArgs{}),
		Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
			var args GetServerStatusArgs
			if err := json.Unmarshal(rawArgs, &args); err != nil {
				return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
			}

			if args.ID != "" {
				instance, err := sm.GetServer(args.ID)
				if err != nil {
					return ExecuteResult{}, err
				}
				instance.mu.Lock()
				defer instance.mu.Unlock()

				uptime := time.Since(instance.StartTime).Round(time.Second).String()
				output := fmt.Sprintf("Server: %s\nStatus: %s\nURL: %s\nPort: %d\nPID: %d\nProject: %s\nCommand: %s\nUptime: %s",
					instance.ID, instance.Status, instance.URL, instance.Port, instance.PID,
					instance.ProjectDir, instance.Command, uptime)

				return ExecuteResult{
					Title:  fmt.Sprintf("Server %s: %s", instance.ID, instance.Status),
					Output: output,
					Metadata: map[string]any{
						"server": map[string]any{
							"id":          instance.ID,
							"pid":         instance.PID,
							"port":        instance.Port,
							"url":         instance.URL,
							"project_dir": instance.ProjectDir,
							"status":      instance.Status,
							"command":     instance.Command,
							"uptime":      uptime,
						},
					},
				}, nil
			}

			servers := sm.ListServers()
			if len(servers) == 0 {
				if existing := sm.findExistingDevServer(); existing != "" {
					return ExecuteResult{
						Title:  "No managed servers running (existing server detected)",
						Output: fmt.Sprintf("No dev servers are currently managed by this agent.\nHowever, an existing dev server was detected running at: %s", existing),
					}, nil
				}
				return ExecuteResult{
					Title:  "No servers running",
					Output: "No dev servers are currently running.",
				}, nil
			}

			var sb string
			for _, s := range servers {
				s.mu.Lock()
				sb += fmt.Sprintf("Server: %s\nStatus: %s\nURL: %s\nPort: %d\nPID: %d\nProject: %s\nCommand: %s\nUptime: %s\n\n",
					s.ID, s.Status, s.URL, s.Port, s.PID, s.ProjectDir, s.Command,
					time.Since(s.StartTime).Round(time.Second).String())
				s.mu.Unlock()
			}

			return ExecuteResult{
				Title:  fmt.Sprintf("%d server(s) running", len(servers)),
				Output: sb,
			}, nil
		},
	}
}

// --- RestartServer ---

type RestartServerArgs struct {
	ID string `json:"id" jsonschema:"title=ID,description=Server ID to restart (e.g. 'server-1')"`
}

func MakeRestartServerTool(sm *ServerManager) Tool {
	return Tool{
		ID:          "RestartServer",
		Description: "Restart a running dev server by its ID. Stops the existing process and starts a new one with the same command and port.",
		Schema:      jsonschema.Reflect(&RestartServerArgs{}),
		Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
			var args RestartServerArgs
			if err := json.Unmarshal(rawArgs, &args); err != nil {
				return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
			}
			if args.ID == "" {
				return ExecuteResult{}, fmt.Errorf("id is required")
			}

			instance, err := sm.RestartServer(ctx, args.ID)
			if err != nil {
				return ExecuteResult{}, err
			}

			output := fmt.Sprintf("Server restarted successfully.\nID: %s\nURL: %s\nPort: %d\nPID: %d\nProject: %s\nStatus: %s\nCommand: %s",
				instance.ID, instance.URL, instance.Port, instance.PID, instance.ProjectDir, instance.Status, instance.Command)

			return ExecuteResult{
				Title:  fmt.Sprintf("Server restarted at %s", instance.URL),
				Output: output,
				Metadata: map[string]any{
					"server": map[string]any{
						"id":          instance.ID,
						"pid":         instance.PID,
						"port":        instance.Port,
						"url":         instance.URL,
						"project_dir": instance.ProjectDir,
						"status":      instance.Status,
						"command":     instance.Command,
					},
				},
			}, nil
		},
	}
}

// --- ListServers ---

func MakeListServersTool(sm *ServerManager) Tool {
	return Tool{
		ID:          "ListServers",
		Description: "List all running dev servers managed by this agent.",
		Schema:      jsonschema.Reflect(&struct{}{}),
		Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
			servers := sm.ListServers()
			if len(servers) == 0 {
				if existing := sm.findExistingDevServer(); existing != "" {
					return ExecuteResult{
						Title:  "No managed servers running (existing server detected)",
						Output: fmt.Sprintf("No dev servers are currently managed by this agent.\nHowever, an existing dev server was detected running at: %s", existing),
					}, nil
				}
				return ExecuteResult{
					Title:  "No servers running",
					Output: "No dev servers are currently running.",
				}, nil
			}

			var sb string
			for _, s := range servers {
				s.mu.Lock()
				sb += fmt.Sprintf("Server: %s\nStatus: %s\nURL: %s\nPort: %d\nPID: %d\nProject: %s\nCommand: %s\nUptime: %s\n\n",
					s.ID, s.Status, s.URL, s.Port, s.PID, s.ProjectDir, s.Command,
					time.Since(s.StartTime).Round(time.Second).String())
				s.mu.Unlock()
			}

			return ExecuteResult{
				Title:  fmt.Sprintf("%d server(s) running", len(servers)),
				Output: sb,
			}, nil
		},
	}
}

// --- TestWebsite ---

// BrowserTestAction is a single scripted browser step. When provided, the
// actions are converted into a prompt that the Browser Agent executes
// autonomously in its own conversation and reports back on.
type BrowserTestAction struct {
	Action   string `json:"action" jsonschema:"title=Action,description=The browser action to perform,enum=[wait_for_selector,wait,click,fill,type,screenshot,extract,scroll,press,navigate,stop]"`
	Selector string `json:"selector,omitempty" jsonschema:"title=Selector,description=CSS selector (for wait_for_selector, click, fill, type, extract)"`
	Value    string `json:"value,omitempty" jsonschema:"title=Value,description=Text value (for fill and type)"`
	Name     string `json:"name,omitempty" jsonschema:"title=Name,description=Name/label (for screenshot)"`
	Timeout  int    `json:"timeout,omitempty" jsonschema:"title=Timeout,description=Timeout in milliseconds (for wait_for_selector)"`
	MS       int    `json:"ms,omitempty" jsonschema:"title=Milliseconds,description=Milliseconds to wait (for wait)"`
}

type TestWebsiteArgs struct {
	URL     string             `json:"url,omitempty" jsonschema:"title=Url,description=URL to test (e.g. http://localhost:3000). If empty, starts a dev server and tests it."`
	Prompt  string             `json:"prompt,omitempty" jsonschema:"title=Prompt,description=Detailed instructions for the Browser Agent: exactly what to test, what to click/fill, what to verify, and what to report. The Browser Agent executes this autonomously and returns a report."`
	Task    string             `json:"task,omitempty" jsonschema:"title=Task,description=Alias for 'prompt'. Detailed instructions for the Browser Agent."`
	Actions []BrowserTestAction `json:"actions,omitempty" jsonschema:"title=Actions,description=Optional ordered list of scripted browser actions to run on the URL. Converted into a prompt for the Browser Agent when 'prompt' is not provided."`
}

func MakeTestWebsiteTool(sm *ServerManager, browserTestFunc func(ctx context.Context, url string, prompt string) (string, error)) Tool {
	return Tool{
		ID: "TestWebsite",
		Description: "Delegate website testing to the Browser Agent (agent-to-agent call). The Browser Agent opens a real Chromium browser, runs the test flow autonomously in its own conversation, and returns a detailed report. Provide 'url' plus either a detailed 'prompt' describing exactly what to test/click/fill/verify, or an 'actions' list. Use this to verify a website works correctly.",
		Schema:      jsonschema.Reflect(&TestWebsiteArgs{}),
		Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
			var args TestWebsiteArgs
			if err := json.Unmarshal(rawArgs, &args); err != nil {
				return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
			}

			if args.Prompt == "" && args.Task != "" {
				args.Prompt = args.Task
			}

			targetURL, err := sm.ResolveBrowserTarget(ctx, WorkspaceFrom(ctx), args.URL)
			if err != nil {
				return ExecuteResult{}, err
			}
			if targetURL == "" {
				return ExecuteResult{}, fmt.Errorf("no URL to test: provide 'url' or start a dev server first")
			}

			prompt := args.Prompt
			if prompt == "" && len(args.Actions) > 0 {
				prompt = actionsToPrompt(targetURL, args.Actions)
			}
			if prompt == "" {
				prompt = fmt.Sprintf("Navigate to %s, wait for the page to load, take a screenshot, extract the visible text content, and report what you see including the page title and key content. Verify the page renders correctly.", targetURL)
			}

			output, err := browserTestFunc(ctx, targetURL, prompt)
			if err != nil {
				return ExecuteResult{}, err
			}

			return ExecuteResult{
				Title:  fmt.Sprintf("Tested %s", targetURL),
				Output: output,
				Metadata: map[string]any{
					"url":    targetURL,
					"action": "test-website",
				},
			}, nil
		},
	}
}

// actionsToPrompt converts a structured action list into a natural-language
// prompt for the Browser Agent so it executes the scripted flow autonomously.
func actionsToPrompt(url string, actions []BrowserTestAction) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Test the website at %s by performing the following steps IN ORDER. After each step, verify it worked (did the element appear? did navigation happen?) before moving on. Take screenshots where requested and note the colors/styles you observe (are primary buttons green? what is the theme?).\n\n", url))
	for i, a := range actions {
		step := fmt.Sprintf("Step %d: %s", i+1, a.Action)
		switch a.Action {
		case "wait_for_selector":
			timeout := a.Timeout
			if timeout <= 0 {
				timeout = 8000
			}
			step = fmt.Sprintf("Step %d: wait for the element '%s' to appear (timeout %dms). Report whether it appeared.", i+1, a.Selector, timeout)
		case "wait":
			ms := a.MS
			if ms <= 0 {
				ms = 1000
			}
			step = fmt.Sprintf("Step %d: wait %dms for the page to settle.", i+1, ms)
		case "click":
			step = fmt.Sprintf("Step %d: click the element '%s'. Report whether it was found and clicked.", i+1, a.Selector)
		case "fill":
			step = fmt.Sprintf("Step %d: fill the input '%s' with the value \"%s\".", i+1, a.Selector, a.Value)
		case "type":
			step = fmt.Sprintf("Step %d: type \"%s\" into the active input.", i+1, a.Value)
		case "screenshot":
			step = fmt.Sprintf("Step %d: take a screenshot (name: %s) and describe the visible colors and layout.", i+1, a.Name)
		case "extract":
			if a.Selector != "" {
				step = fmt.Sprintf("Step %d: extract the visible text of element '%s'.", i+1, a.Selector)
			} else {
				step = fmt.Sprintf("Step %d: extract the visible text of the page.", i+1)
			}
		case "scroll":
			step = fmt.Sprintf("Step %d: scroll the page.", i+1)
		case "press":
			step = fmt.Sprintf("Step %d: press the '%s' key.", i+1, a.Value)
		case "navigate":
			step = fmt.Sprintf("Step %d: navigate to '%s'.", i+1, a.Value)
		case "stop":
			step = fmt.Sprintf("Step %d: close the browser session.", i+1)
		default:
			step = fmt.Sprintf("Step %d: %s", i+1, a.Action)
		}
		sb.WriteString(step)
		sb.WriteString("\n")
	}
	sb.WriteString("\nAfter completing all steps, write a DETAILED report: for each step state whether it succeeded, describe what you saw (page title, buttons, colors/theme, form fields, todos in the list), and flag anything that did not work. Then stop the browser.\n")
	return sb.String()
}
