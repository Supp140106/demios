package tools

import (
	"context"
	"encoding/json"
	"sort"

	"demios/llm"

	"github.com/invopop/jsonschema"
	openai "github.com/openai/openai-go"
)

type Tool struct {
	ID          string
	Description string
	Schema      *jsonschema.Schema
	Execute     func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error)
}

type ExecuteResult struct {
	Title    string         // shown in the UI
	Output   string         // this goes back to the LLM as the tool result
	Metadata map[string]any // structured data for the UI only, never sent to the model
}

// ToToolDef converts a Tool to an OpenAI SDK tool definition.
func ToToolDef(t Tool) llm.ToolDefinition {
	params := map[string]interface{}{
		"type":                 "object",
		"properties":           map[string]interface{}{},
		"additionalProperties": false,
	}

	if t.Schema != nil && t.Schema.Properties != nil {
		props := map[string]interface{}{}
		requiredSet := make(map[string]bool)
		for _, r := range t.Schema.Required {
			requiredSet[r] = true
		}

		for name, prop := range t.Schema.Properties.FromOldest() {
			p := map[string]interface{}{
				"type": prop.Type,
			}
			if prop.Description != "" {
				p["description"] = prop.Description
			}
			if prop.Enum != nil {
				p["enum"] = prop.Enum
			}
			props[name] = p
		}

		var required []string
		for name := range requiredSet {
			required = append(required, name)
		}

		params["properties"] = props
		if len(required) > 0 {
			params["required"] = required
		}
	}

	return llm.ToolDefinition{
		Function: openai.FunctionDefinitionParam{
			Name:        t.ID,
			Description: openai.String(t.Description),
			Parameters:  params,
		},
	}
}

// AllToolDefs converts all registered tools to SDK tool definitions,
// sorted by tool ID for deterministic ordering.
func AllToolDefs(toolMap map[string]Tool) []llm.ToolDefinition {
	ids := make([]string, 0, len(toolMap))
	for id := range toolMap {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	defs := make([]llm.ToolDefinition, len(ids))
	for i, id := range ids {
		defs[i] = ToToolDef(toolMap[id])
	}
	return defs
}
