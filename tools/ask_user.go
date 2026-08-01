package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
)

type AskUserArgs struct {
	Question string   `json:"question" jsonschema:"title=Question,description=The question you need the user to answer."`
	Options  []string `json:"options,omitempty" jsonschema:"title=Options,description=Optional multiple-choice options the user can pick from."`
}

type HumanInputRunner func(ctx context.Context, question string, options []string) (string, error)

func MakeAskUserTool(runner HumanInputRunner) Tool {
	return Tool{
		ID:          "AskUser",
		Description: "Ask the user a question when you need their input, clarification, or a decision. Use this when you encounter ambiguity, need to choose between approaches, require approval for a design decision, or need specific information not available in the workspace.",
		Schema:      jsonschema.Reflect(&AskUserArgs{}),
		Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
			var args AskUserArgs
			if err := json.Unmarshal(rawArgs, &args); err != nil {
				return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
			}
			if args.Question == "" {
				return ExecuteResult{}, fmt.Errorf("question is required")
			}

			answer, err := runner(ctx, args.Question, args.Options)
			if err != nil {
				return ExecuteResult{}, fmt.Errorf("user input failed: %w", err)
			}

			return ExecuteResult{
				Title:  "User response",
				Output: answer,
				Metadata: map[string]any{
					"type":     "user_response",
					"question": args.Question,
					"answer":   answer,
				},
			}, nil
		},
	}
}
