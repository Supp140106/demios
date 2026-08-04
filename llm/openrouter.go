package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type ModelConfig struct {
	ID              string            `json:"ID"`
	Label           string            `json:"Label"`
	BaseURL         string            `json:"BaseURL"`
	APIKey          string            `json:"APIKey"`
	Model           string            `json:"Model"`
	BackendType     string            `json:"BackendType"`     // "openai" (default) or "genai"
	AuthType        string            `json:"AuthType"`        // "env" (default), "bearer", "api-key", "none"
	Headers         map[string]string `json:"Headers"`         // extra headers (e.g. Azure api-key, Anthropic x-api-key)
	CompletionsURL  string            `json:"CompletionsURL"`  // full URL override for chat/completions endpoint
	ExtraBody       map[string]any    `json:"ExtraBody"`       // extra JSON body fields to include in API requests
	BuiltIn         bool              `json:"BuiltIn"`
	EnvVarName      string            `json:"EnvVarName"`      // env var name for built-in providers
}

var AvailableModels = []ModelConfig{
	{
		ID:          "nvidia-inkling",
		Label:       "Nvidia NIM (Inkling)",
		BaseURL:     "https://integrate.api.nvidia.com/v1",
		APIKey:      "NVIDIA_API_KEY",
		Model:       "thinkingmachines/inkling",
		BuiltIn:     true,
		EnvVarName:  "NVIDIA_API_KEY",
	},
	{
		ID:          "openrouter-nemotron",
		Label:       "OpenRouter (Nemotron)",
		BaseURL:     "https://openrouter.ai/api/v1",
		APIKey:      "OPENROUTER_API_KEY",
		Model:       "nvidia/nemotron-3-super-120b-a12b:free",
		BuiltIn:     true,
		EnvVarName:  "OPENROUTER_API_KEY",
	},
	{
		ID:          "mistral-large",
		Label:       "Mistral Large",
		BaseURL:     "https://api.mistral.ai/v1",
		APIKey:      "MISTRAL_API_KEY",
		Model:       "mistral-large-latest",
		BuiltIn:     true,
		EnvVarName:  "MISTRAL_API_KEY",
	},
	{
		ID:          "mistral-small",
		Label:       "Mistral Small",
		BaseURL:     "https://api.mistral.ai/v1",
		APIKey:      "MISTRAL_API_KEY",
		Model:       "mistral-small-latest",
		BuiltIn:     true,
		EnvVarName:  "MISTRAL_API_KEY",
	},
	{
		ID:          "groq-gpt-oss",
		Label:       "Groq (GPT-OSS 120B)",
		BaseURL:     "https://api.groq.com/openai/v1",
		APIKey:      "GROQ_API_KEY",
		Model:       "openai/gpt-oss-120b",
		BuiltIn:     true,
		EnvVarName:  "GROQ_API_KEY",
	},
	{
		ID:          "github-gpt-4o",
		Label:       "GitHub Models (GPT-4o)",
		BaseURL:     "https://models.github.ai/inference",
		APIKey:      "GITHUB_TOKEN",
		Model:       "openai/gpt-4o",
		BuiltIn:     true,
		EnvVarName:  "GITHUB_TOKEN",
	},
	{
		ID:          "gemini-gemma",
		Label:       "Gemini (Gemma 4)",
		BackendType: "genai",
		APIKey:      "GEMINI_API_KEY",
		Model:       "gemma-4-31b-it",
		BuiltIn:     true,
		EnvVarName:  "GEMINI_API_KEY",
	},
	{
		ID:         "local-mimo",
		Label:      "north mini code",
		BaseURL:    "http://localhost:20128/v1",
		APIKey:     "LOCAL_API_KEY",
		Model:      "oc/north-mini-code-free",
		BuiltIn:    true,
		EnvVarName: "LOCAL_API_KEY",
	},
	{
		ID:         "PoolSide",
		Label:      "PoolSide (Local)",
		BaseURL:    "http://localhost:20128/v1",
		APIKey:     "LOCAL_API_KEY",
		Model:      "ps/poolside/laguna-s-2.1",
		BuiltIn:    true,
		EnvVarName: "LOCAL_API_KEY",
	},
	{
		ID:         "poolside-cloud",
		Label:      "PoolSide Cloud",
		BaseURL:    "https://inference.poolside.ai/v1",
		APIKey:     "POOLSIDE_API_KEY",
		Model:      "poolside/laguna-s-2.1",
		BuiltIn:    true,
		EnvVarName: "POOLSIDE_API_KEY",
	},
	{
		ID:         "Big Pickle",
		Label:      "Big Pickle",
		BaseURL:    "http://localhost:20128/v1",
		APIKey:     "LOCAL_API_KEY",
		Model:      "oc/big-pickle",
		BuiltIn:    true,
		EnvVarName: "LOCAL_API_KEY",
	},
	{
		ID:         "deepseek-v4-flash",
		Label:      "DeepSeek V4 Flash (BazaarLink)",
		BaseURL:    "https://bazaarlink.ai/api/v1",
		APIKey:     "BAZAARLINK_API_KEY",
		Model:      "deepseek-v4-flash",
		BuiltIn:    true,
		EnvVarName: "BAZAARLINK_API_KEY",
	},
}

// Client wraps the OpenAI SDK client for any OpenAI-compatible provider.
type Client struct {
	sdk    *openai.Client
	genai  *geminiClient
	config ModelConfig
}

// NewClient creates a new LLM client using the first available model config.
func NewClient() *Client {
	return NewClientWithModel(AvailableModels[0].ID)
}

// NewClientWithModel creates a new LLM client using the specified model ID.
func NewClientWithModel(modelID string) *Client {
	cfg := findModelConfig(modelID)
	return newClientWithConfig(cfg)
}

func resolveAPIKey(cfg ModelConfig) string {
	if cfg.BuiltIn || cfg.AuthType == "env" || cfg.AuthType == "" {
		return os.Getenv(cfg.APIKey)
	}
	return cfg.APIKey
}

func newClientWithConfig(cfg ModelConfig) *Client {
	if cfg.BackendType == "genai" {
		return &Client{
			genai:  newGeminiClient(cfg),
			config: cfg,
		}
	}

	apiKey := resolveAPIKey(cfg)
	opts := []option.RequestOption{
		option.WithBaseURL(cfg.BaseURL),
		option.WithHeader("Authorization", "Bearer "+apiKey),
	}
	for k, v := range cfg.Headers {
		opts = append(opts, option.WithHeader(k, v))
	}
	c := openai.NewClient(opts...)
	return &Client{
		sdk:    &c,
		config: cfg,
	}
}

func findModelConfig(id string) ModelConfig {
	return FindModelConfig(id)
}

// SetModel switches the client to the specified model ID.
// This creates a new underlying SDK client with the new config.
func (c *Client) SetModel(modelID string) error {
	cfg := findModelConfig(modelID)
	if cfg.ID == "" {
		return fmt.Errorf("unknown model: %s", modelID)
	}

	if cfg.BackendType == "genai" {
		c.genai = newGeminiClient(cfg)
		c.sdk = nil
		c.config = cfg
		return nil
	}

	apiKey := resolveAPIKey(cfg)
	opts := []option.RequestOption{
		option.WithBaseURL(cfg.BaseURL),
		option.WithHeader("Authorization", "Bearer "+apiKey),
	}
	for k, v := range cfg.Headers {
		opts = append(opts, option.WithHeader(k, v))
	}
	sdk := openai.NewClient(opts...)
	c.sdk = &sdk
	c.genai = nil
	c.config = cfg
	return nil
}

// GetCurrentModel returns the current model config.
func (c *Client) GetCurrentModel() ModelConfig {
	return c.config
}

// GetModels returns all available model configs.
func (c *Client) GetModels() []ModelConfig {
	return GetAllModels()
}

// Chat sends a non-streaming request with tools.
func (c *Client) Chat(ctx context.Context, systemPrompt string, history []Message, tools []ToolDefinition) (*openai.ChatCompletion, error) {
	if c.config.BackendType == "genai" {
		if c.genai == nil {
			return nil, fmt.Errorf("gemini client not configured")
		}
		return c.genai.Chat(ctx, systemPrompt, history, tools)
	}

	msgs := c.buildMessages(systemPrompt, history)

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(c.config.Model),
		Messages: msgs,
		Tools:    tools,
	}

	opts := c.extraBodyOpts()
	completion, err := c.sdk.Chat.Completions.New(ctx, params, opts...)
	if err != nil {
		return nil, fmt.Errorf("chat completion: %w", err)
	}
	return completion, nil
}

// ChatStream sends a streaming request with tools and returns parsed StreamEvents.
// Uses raw HTTP to handle both SSE and non-SSE (plain JSON) responses gracefully.
func (c *Client) ChatStream(ctx context.Context, systemPrompt string, history []Message, tools []ToolDefinition) (<-chan StreamEvent, error) {
	if c.config.BackendType == "genai" {
		if c.genai == nil {
			return nil, fmt.Errorf("gemini client not configured")
		}
		return c.genai.ChatStream(ctx, systemPrompt, history, tools)
	}

	body, err := c.buildJSONBody(systemPrompt, history, tools, true)
	if err != nil {
		return nil, fmt.Errorf("build request body: %w", err)
	}

	apiKey := resolveAPIKey(c.config)
	completionsURL := c.config.CompletionsURL
	if completionsURL == "" {
		completionsURL = c.config.BaseURL + "/chat/completions"
	}
	req, err := http.NewRequestWithContext(ctx, "POST", completionsURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	for k, v := range c.config.Headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
	}

	out := make(chan StreamEvent, 100)

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		go c.readSSE(resp, out)
	} else {
		// Non-SSE: parse as a single JSON ChatCompletion response.
		// This handles local servers that don't support streaming but
		// still accept stream:true in the request body.
		go c.readJSON(resp, out)
	}

	return out, nil
}

// buildJSONBody serializes the request into a JSON byte slice for raw HTTP use.
func (c *Client) buildJSONBody(systemPrompt string, history []Message, tools []ToolDefinition, stream bool) ([]byte, error) {
	rawMsgs := make([]json.RawMessage, 0, len(history)+1)
	if systemPrompt != "" {
		sys, _ := json.Marshal(map[string]string{"role": "system", "content": systemPrompt})
		rawMsgs = append(rawMsgs, sys)
	}
	for _, msg := range history {
		b, err := json.Marshal(msg)
		if err != nil {
			return nil, fmt.Errorf("marshal message: %w", err)
		}
		rawMsgs = append(rawMsgs, b)
	}

	rawTools := make([]json.RawMessage, len(tools))
	for i, t := range tools {
		b, err := json.Marshal(t)
		if err != nil {
			return nil, fmt.Errorf("marshal tool: %w", err)
		}
		rawTools[i] = b
	}

	body := map[string]any{
		"model":    c.config.Model,
		"messages": rawMsgs,
		"stream":   stream,
	}
	if len(tools) > 0 {
		body["tools"] = rawTools
	}
	for k, v := range c.config.ExtraBody {
		body[k] = v
	}

	return json.Marshal(body)
}

// readSSE reads a proper SSE response body, emitting StreamEvents.
func (c *Client) readSSE(resp *http.Response, out chan StreamEvent) {
	defer close(out)
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	acc := openai.ChatCompletionAccumulator{}
	pending := map[int]*pendingTool{}

	var dataBuf bytes.Buffer

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			if strings.HasPrefix(data, " ") {
				data = data[1:]
			}
			dataBuf.WriteString(data)
			continue
		}

		if line == "" && dataBuf.Len() > 0 {
			raw := dataBuf.Bytes()
			dataBuf.Reset()

			rawStr := strings.TrimSpace(string(raw))
			if rawStr == "[DONE]" {
				continue
			}

			if reasoning := extractReasoning(raw); reasoning != "" {
				out <- StreamEvent{Type: "think", Thinking: reasoning}
			}

			var chunk openai.ChatCompletionChunk
			if err := json.Unmarshal(raw, &chunk); err != nil {
				log.Printf("[openrouter] failed to parse SSE data: %v", err)
				continue
			}

			c.processChunk(chunk, &acc, pending, out)
		}
	}

	if dataBuf.Len() > 0 {
		raw := dataBuf.Bytes()
		dataBuf.Reset()

		rawStr := strings.TrimSpace(string(raw))
		if rawStr != "[DONE]" {
			if reasoning := extractReasoning(raw); reasoning != "" {
				out <- StreamEvent{Type: "think", Thinking: reasoning}
			}

			var chunk openai.ChatCompletionChunk
			if err := json.Unmarshal(raw, &chunk); err == nil {
				c.processChunk(chunk, &acc, pending, out)
			}
		}
	}

	log.Printf("[openrouter] SSE stream ended")
	if err := scanner.Err(); err != nil {
		out <- StreamEvent{Type: "error", Error: err.Error()}
		return
	}

	c.flushPending(pending, out)
	out <- StreamEvent{Type: "done", Done: true}
}

// readJSON handles a non-SSE (plain JSON) response, emitting a single completion as events.
func (c *Client) readJSON(resp *http.Response, out chan StreamEvent) {
	defer close(out)
	defer resp.Body.Close()

	var chat openai.ChatCompletion
	if err := json.NewDecoder(resp.Body).Decode(&chat); err != nil {
		out <- StreamEvent{Type: "error", Error: fmt.Sprintf("decode non-streaming response: %v", err)}
		return
	}

	if len(chat.Choices) == 0 {
		out <- StreamEvent{Type: "done", Done: true}
		return
	}

	choice := chat.Choices[0]
	msg := choice.Message

	if msg.Content != "" {
		log.Printf("[openrouter] non-sse text: %q", msg.Content)
		out <- StreamEvent{
			Type: "text",
			Text: msg.Content,
		}
	}

	for i, tc := range msg.ToolCalls {
		log.Printf("[openrouter] non-sse tool call: %s(%s)", tc.Function.Name, tc.Function.Arguments)
		out <- StreamEvent{
			Type: "tool_call",
			ToolCall: &openai.FinishedChatCompletionToolCall{
				ChatCompletionMessageToolCallFunction: openai.ChatCompletionMessageToolCallFunction{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
				ID:    tc.ID,
				Index: i,
			},
		}
	}

	out <- StreamEvent{Type: "done", Done: true}
}

func (c *Client) processChunk(
	chunk openai.ChatCompletionChunk,
	acc *openai.ChatCompletionAccumulator,
	pending map[int]*pendingTool,
	out chan StreamEvent,
) {
	acc.AddChunk(chunk)

	if len(chunk.Choices) == 0 {
		return
	}
	delta := chunk.Choices[0].Delta

	if delta.Content != "" {
		log.Printf("[openrouter] text chunk: %q", delta.Content)
		out <- StreamEvent{
			Type: "text",
			Text: delta.Content,
		}
	}

	for _, tc := range delta.ToolCalls {
		idx := int(tc.Index)
		pt, exists := pending[idx]
		if !exists {
			pt = &pendingTool{id: tc.ID, name: tc.Function.Name}
			pending[idx] = pt
		}
		if tc.ID != "" {
			pt.id = tc.ID
		}
		if tc.Function.Name != "" {
			pt.name = tc.Function.Name
		}
		if tc.Function.Arguments != "" {
			pt.args += tc.Function.Arguments
		}
	}

	if tool, ok := acc.JustFinishedToolCall(); ok {
		log.Printf("[openrouter] finished tool call: %s(%s)", tool.Name, tool.Arguments)
		out <- StreamEvent{
			Type:     "tool_call",
			ToolCall: &tool,
		}
		delete(pending, tool.Index)
	}
}

type pendingTool struct {
	id   string
	name string
	args string
}

func (c *Client) flushPending(pending map[int]*pendingTool, out chan StreamEvent) {
	for idx, pt := range pending {
		if pt.id != "" && pt.name != "" {
			out <- StreamEvent{
				Type: "tool_call",
				ToolCall: &openai.FinishedChatCompletionToolCall{
					ChatCompletionMessageToolCallFunction: openai.ChatCompletionMessageToolCallFunction{
						Name:      pt.name,
						Arguments: pt.args,
					},
					ID:    pt.id,
					Index: idx,
				},
			}
		}
	}
}

func (c *Client) extraBodyOpts() []option.RequestOption {
	var opts []option.RequestOption
	for key, val := range c.config.ExtraBody {
		opts = append(opts, option.WithJSONSet(key, val))
	}
	return opts
}

// extractReasoning looks for reasoning content in a streaming chunk's delta.
// Different providers use different field names.
func extractReasoning(raw []byte) string {
	var data struct {
		Choices []struct {
			Delta struct {
				ReasoningContent string `json:"reasoning_content"`
				Reasoning        string `json:"reasoning"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return ""
	}
	if len(data.Choices) > 0 {
		if data.Choices[0].Delta.ReasoningContent != "" {
			return data.Choices[0].Delta.ReasoningContent
		}
		return data.Choices[0].Delta.Reasoning
	}
	return ""
}

func (c *Client) buildMessages(systemPrompt string, history []Message) []Message {
	var msgs []Message
	if systemPrompt != "" {
		msgs = append(msgs, SystemMessage(systemPrompt))
	}
	msgs = append(msgs, history...)
	return msgs
}
