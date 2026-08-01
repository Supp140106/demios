package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	openai "github.com/openai/openai-go"
	"google.golang.org/genai"
)

type geminiClient struct {
	sdk    *genai.Client
	config ModelConfig
}

func newGeminiClient(cfg ModelConfig) *geminiClient {
	apiKey := os.Getenv(cfg.APIKey)
	if apiKey == "" {
		log.Printf("[gemini] no API key found for %s", cfg.APIKey)
		return &geminiClient{config: cfg}
	}
	gc, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Printf("[gemini] failed to create client: %v", err)
		return &geminiClient{config: cfg}
	}
	return &geminiClient{sdk: gc, config: cfg}
}

func (g *geminiClient) Chat(ctx context.Context, systemPrompt string, history []Message, tools []ToolDefinition) (*openai.ChatCompletion, error) {
	if g.sdk == nil {
		return nil, fmt.Errorf("gemini client not initialized")
	}
	contents, sysInstr := convertHistory(history, systemPrompt)

	config := &genai.GenerateContentConfig{}
	if sysInstr != nil {
		config.SystemInstruction = sysInstr
	}
	if len(tools) > 0 {
		config.Tools = convertToolDefs(tools)
	}

	resp, err := g.sdk.Models.GenerateContent(ctx, g.config.Model, contents, config)
	if err != nil {
		return nil, fmt.Errorf("gemini chat: %w", err)
	}

	return convertResponse(resp), nil
}

func (g *geminiClient) ChatStream(ctx context.Context, systemPrompt string, history []Message, tools []ToolDefinition) (<-chan StreamEvent, error) {
	if g.sdk == nil {
		return nil, fmt.Errorf("gemini client not initialized")
	}
	contents, sysInstr := convertHistory(history, systemPrompt)

	config := &genai.GenerateContentConfig{}
	if sysInstr != nil {
		config.SystemInstruction = sysInstr
	}
	if len(tools) > 0 {
		config.Tools = convertToolDefs(tools)
	}

	stream := g.sdk.Models.GenerateContentStream(ctx, g.config.Model, contents, config)

	out := make(chan StreamEvent)

	go func() {
		defer close(out)

		for resp, err := range stream {
			if err != nil {
				out <- StreamEvent{Type: "error", Error: err.Error()}
				return
			}
			if resp == nil || resp.Candidates == nil || len(resp.Candidates) == 0 {
				continue
			}

			candidate := resp.Candidates[0]
			if candidate.Content == nil || candidate.Content.Parts == nil {
				continue
			}

			for _, part := range candidate.Content.Parts {
				if part == nil {
					continue
				}
				if part.Text != "" {
					out <- StreamEvent{
						Type: "text",
						Text: part.Text,
					}
				}
				if part.FunctionCall != nil {
					fc := part.FunctionCall
					argsJSON, _ := json.Marshal(fc.Args)
					out <- StreamEvent{
						Type: "tool_call",
						ToolCall: &openai.FinishedChatCompletionToolCall{
							ChatCompletionMessageToolCallFunction: openai.ChatCompletionMessageToolCallFunction{
								Name:      fc.Name,
								Arguments: string(argsJSON),
							},
							ID:    fc.ID,
							Index: 0,
						},
					}
				}
			}
		}

		out <- StreamEvent{Type: "done", Done: true}
	}()

	return out, nil
}

func convertHistory(history []Message, systemPrompt string) ([]*genai.Content, *genai.Content) {
	var sysInstr *genai.Content
	var contents []*genai.Content

	if systemPrompt != "" {
		sysInstr = &genai.Content{
			Parts: []*genai.Part{
				{Text: systemPrompt},
			},
			Role: "user",
		}
	}

	for _, msg := range history {
		content := convertMessage(msg)
		if content != nil {
			contents = append(contents, content)
		}
	}

	return contents, sysInstr
}

func convertMessage(msg Message) *genai.Content {
	if msg.OfUser != nil {
		c := &genai.Content{Role: "user"}
		if msg.OfUser.Content.OfString.Valid() {
			c.Parts = []*genai.Part{{Text: msg.OfUser.Content.OfString.Value}}
		}
		return c
	}

	if msg.OfAssistant != nil {
		c := &genai.Content{Role: "model"}
		if msg.OfAssistant.Content.OfString.Valid() {
			c.Parts = []*genai.Part{{Text: msg.OfAssistant.Content.OfString.Value}}
		}
		for _, tc := range msg.OfAssistant.ToolCalls {
			if tc.Function.Name != "" {
				var args map[string]any
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					log.Printf("[gemini] failed to parse tool call args for %s: %v", tc.Function.Name, err)
				}
				c.Parts = append(c.Parts, &genai.Part{
					FunctionCall: &genai.FunctionCall{
						Name: tc.Function.Name,
						Args: args,
					},
				})
			}
		}
		return c
	}

	if msg.OfTool != nil {
		c := &genai.Content{Role: "user"}
		if msg.OfTool.Content.OfString.Valid() {
			c.Parts = []*genai.Part{
				{
					FunctionResponse: &genai.FunctionResponse{
						Name: msg.OfTool.ToolCallID,
						Response: map[string]any{
							"output": msg.OfTool.Content.OfString.Value,
						},
					},
				},
			}
		}
		return c
	}

	return nil
}

func convertToolDefs(tools []ToolDefinition) []*genai.Tool {
	var result []*genai.Tool
	for _, t := range tools {
		fd := &genai.FunctionDeclaration{
			Name: t.Function.Name,
		}
		if t.Function.Description.Valid() {
			fd.Description = t.Function.Description.Value
		}
		if len(t.Function.Parameters) > 0 {
			schema := convertParamsToSchema(t.Function.Parameters)
			if schema != nil {
				fd.Parameters = schema
			}
		}
		result = append(result, &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{fd},
		})
	}
	return result
}

func convertParamsToSchema(params map[string]any) *genai.Schema {
	schema := &genai.Schema{
		Type:       genai.TypeObject,
		Properties: make(map[string]*genai.Schema),
	}

	props, _ := params["properties"].(map[string]any)
	for name, prop := range props {
		p, _ := prop.(map[string]any)
		if p == nil {
			continue
		}
		ps := &genai.Schema{}
		typeStr, _ := p["type"].(string)
		ps.Type = schemaTypeFromString(typeStr)
		if desc, ok := p["description"].(string); ok {
			ps.Description = desc
		}
		if enumVals, ok := p["enum"].([]any); ok {
			for _, e := range enumVals {
				ps.Enum = append(ps.Enum, fmt.Sprintf("%v", e))
			}
		}
		schema.Properties[name] = ps
	}

	req, _ := params["required"].([]any)
	for _, r := range req {
		if rStr, ok := r.(string); ok {
			schema.Required = append(schema.Required, rStr)
		}
	}

	if len(schema.Properties) == 0 {
		return nil
	}
	return schema
}

func schemaTypeFromString(s string) genai.Type {
	switch strings.ToLower(s) {
	case "string":
		return genai.TypeString
	case "number":
		return genai.TypeNumber
	case "integer":
		return genai.TypeInteger
	case "boolean":
		return genai.TypeBoolean
	case "array":
		return genai.TypeArray
	case "object":
		return genai.TypeObject
	default:
		return genai.TypeString
	}
}

func convertResponse(resp *genai.GenerateContentResponse) *openai.ChatCompletion {
	cc := &openai.ChatCompletion{
		Choices: make([]openai.ChatCompletionChoice, 0),
	}

	if resp == nil || resp.Candidates == nil {
		return cc
	}

	for _, cand := range resp.Candidates {
		if cand == nil || cand.Content == nil {
			continue
		}

		var textBuilder strings.Builder
		var toolCalls []openai.ChatCompletionMessageToolCall

		for _, part := range cand.Content.Parts {
			if part == nil {
				continue
			}
			if part.Text != "" {
				textBuilder.WriteString(part.Text)
			}
			if part.FunctionCall != nil {
				argsJSON, _ := json.Marshal(part.FunctionCall.Args)
				toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCall{
					ID:   part.FunctionCall.ID,
					Type: "function",
					Function: openai.ChatCompletionMessageToolCallFunction{
						Name:      part.FunctionCall.Name,
						Arguments: string(argsJSON),
					},
				})
			}
		}

		msg := openai.ChatCompletionMessage{
			Role:      "assistant",
			Content:   textBuilder.String(),
			ToolCalls: toolCalls,
		}

		finishReason := "stop"
		if cand.FinishReason != "" {
			finishReason = string(cand.FinishReason)
		}

		cc.Choices = append(cc.Choices, openai.ChatCompletionChoice{
			Index:        int64(cand.Index),
			Message:      msg,
			FinishReason: finishReason,
		})
	}

	return cc
}
