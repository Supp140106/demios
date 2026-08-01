package db

import (
	"encoding/json"
	"fmt"

	"demios/llm"
	openai "github.com/openai/openai-go"
)

type historyEntry struct {
	Role       string          `json:"role"`
	Content    string          `json:"content,omitempty"`
	ToolCalls  []toolCallEntry `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

type toolCallEntry struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function funcEntry    `json:"function"`
}

type funcEntry struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func MarshalHistory(history []llm.Message) (string, error) {
	entries := make([]historyEntry, len(history))

	for i, m := range history {
		switch {
		case m.OfUser != nil:
			entries[i] = historyEntry{Role: "user", Content: llm.ContentString(m.OfUser.Content)}

		case m.OfAssistant != nil:
			e := historyEntry{Role: "assistant", Content: llm.ContentString(m.OfAssistant.Content)}
			if len(m.OfAssistant.ToolCalls) > 0 {
				e.ToolCalls = make([]toolCallEntry, len(m.OfAssistant.ToolCalls))
				for j, tc := range m.OfAssistant.ToolCalls {
					e.ToolCalls[j] = toolCallEntry{
						ID:   tc.ID,
						Type: "function",
						Function: funcEntry{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
				}
			}
			entries[i] = e

		case m.OfTool != nil:
			entries[i] = historyEntry{Role: "tool", Content: llm.ContentString(m.OfTool.Content), ToolCallID: m.OfTool.ToolCallID}
		}
	}

	data, err := json.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("marshal history: %w", err)
	}
	return string(data), nil
}

func UnmarshalHistory(data string) ([]llm.Message, error) {
	if data == "" || data == "null" {
		return nil, nil
	}

	var entries []historyEntry
	if err := json.Unmarshal([]byte(data), &entries); err != nil {
		return nil, fmt.Errorf("unmarshal history: %w", err)
	}

	history := make([]llm.Message, len(entries))
	for i, e := range entries {
		switch e.Role {
		case "user":
			history[i] = llm.UserMessage(e.Content)

		case "assistant":
			if len(e.ToolCalls) > 0 {
				tcParams := make([]openai.ChatCompletionMessageToolCallParam, len(e.ToolCalls))
				for j, tc := range e.ToolCalls {
					tcParams[j] = openai.ChatCompletionMessageToolCallParam{
						ID:   tc.ID,
						Type: "function",
						Function: openai.ChatCompletionMessageToolCallFunctionParam{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
				}
				history[i] = llm.AssistantMessageWithTools(e.Content, tcParams)
			} else {
				history[i] = llm.AssistantMessageWithTools(e.Content, nil)
			}

		case "tool":
			history[i] = llm.ToolMessage(e.Content, e.ToolCallID)
		}
	}

	return history, nil
}
