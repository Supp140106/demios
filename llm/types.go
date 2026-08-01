package llm

import (
	openai "github.com/openai/openai-go"
)

func ContentString(v any) string {
	switch c := v.(type) {
	case openai.ChatCompletionUserMessageParamContentUnion:
		if c.OfString.Valid() {
			return c.OfString.Value
		}
	case openai.ChatCompletionAssistantMessageParamContentUnion:
		if c.OfString.Valid() {
			return c.OfString.Value
		}
	case openai.ChatCompletionToolMessageParamContentUnion:
		if c.OfString.Valid() {
			return c.OfString.Value
		}
	}
	return ""
}

// Re-export SDK types for use across the codebase.
type Message = openai.ChatCompletionMessageParamUnion
type ToolDefinition = openai.ChatCompletionToolParam
type ToolCall = openai.ChatCompletionMessageToolCall

type StreamEvent struct {
	Type     string // "text", "think", "tool_call", "done", "error"
	Text     string
	Thinking string // reasoning/thinking content for think events
	ToolCall *openai.FinishedChatCompletionToolCall
	Done     bool
	Error    string
}

func SystemMessage(content string) Message {
	return openai.SystemMessage(content)
}

func UserMessage(content string) Message {
	return openai.UserMessage(content)
}

func UserMessageWithImages(text string, images []string) Message {
	parts := make([]openai.ChatCompletionContentPartUnionParam, 0, 1+len(images))
	parts = append(parts, openai.TextContentPart(text))
	for _, img := range images {
		parts = append(parts, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
			URL: "data:image/png;base64," + img,
		}))
	}
	return openai.UserMessage(parts)
}

func ToolMessage(content string, id string) Message {
	return openai.ToolMessage(content, id)
}

// AssistantMessageWithTools creates an assistant message that includes tool calls.
func AssistantMessageWithTools(content string, toolCalls []openai.ChatCompletionMessageToolCallParam) Message {
	param := &openai.ChatCompletionAssistantMessageParam{
		ToolCalls: toolCalls,
		Content: openai.ChatCompletionAssistantMessageParamContentUnion{
			OfString: openai.String(content),
		},
	}
	return Message{
		OfAssistant: param,
	}
}
