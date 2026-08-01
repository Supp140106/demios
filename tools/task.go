package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
)

type TaskArgs struct {
	Prompt  string `json:"prompt,omitempty" jsonschema:"title=Prompt,description=The task description for the sub-agent to execute. Be specific and include all context needed."`
	Task    string `json:"task,omitempty" jsonschema:"title=Task,description=Alias for 'prompt'. The task description for the sub-agent to execute."`
	MaxIter int    `json:"max_iter,omitempty" jsonschema:"title=MaxIter,description=Maximum iterations for the sub-agent (default 15, max 30)"`
}

type TaskRunner func(ctx context.Context, prompt string, maxIter int) (string, error)

func MakeTaskTool(runner TaskRunner) Tool {
	return Tool{
		ID:          "Task",
		Description: "Delegate a complex task to a sub-agent. The sub-agent gets its own fresh conversation and can use all the same tools (Read, Write, Edit, Grep, Glob, Bash). Use this for complex multi-step work that would benefit from isolation, for exploratory research that shouldn't pollute the main context, or for tasks that can run in parallel. Returns the sub-agent's final response.",
		Schema:      jsonschema.Reflect(&TaskArgs{}),
		Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
			var args TaskArgs
			if err := json.Unmarshal(rawArgs, &args); err != nil {
				return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
			}
			if args.Prompt == "" && args.Task != "" {
				args.Prompt = args.Task
			}
			if args.Prompt == "" {
				return ExecuteResult{}, fmt.Errorf("prompt is required: provide a 'prompt' (or 'task') describing what the sub-agent should do")
			}
			maxIter := args.MaxIter
			if maxIter <= 0 {
				maxIter = 15
			}
			if maxIter > 30 {
				maxIter = 30
			}

			output, err := runner(ctx, args.Prompt, maxIter)
			if err != nil {
				return ExecuteResult{}, fmt.Errorf("sub-agent failed: %w", err)
			}

			return ExecuteResult{
				Title:  "Task completed",
				Output: output,
				Metadata: map[string]any{
					"type":   "task_result",
					"output": output,
				},
			}, nil
		},
	}
}
