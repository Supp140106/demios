package core

import (
	"context"
	"demios/llm"

	openai "github.com/openai/openai-go"
)

type Client struct {
	llm *llm.Client
}

func NewClient() *Client {
	return &Client{llm: llm.NewClient()}
}

func (c *Client) SetModel(modelID string) error {
	return c.llm.SetModel(modelID)
}

func (c *Client) GetCurrentModel() llm.ModelConfig {
	return c.llm.GetCurrentModel()
}

func (c *Client) GetModels() []llm.ModelConfig {
	return c.llm.GetModels()
}

func (c *Client) Chat(ctx context.Context, systemPrompt string, history []llm.Message, toolDefs []llm.ToolDefinition) (*openai.ChatCompletion, error) {
	return c.llm.Chat(ctx, systemPrompt, history, toolDefs)
}

func (c *Client) ChatStream(ctx context.Context, systemPrompt string, history []llm.Message, toolDefs []llm.ToolDefinition) (<-chan llm.StreamEvent, error) {
	return c.llm.ChatStream(ctx, systemPrompt, history, toolDefs)
}
