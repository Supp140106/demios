package llm

type ProviderPreset struct {
	Name        string            `json:"Name"`
	Icon        string            `json:"Icon"`
	BaseURL     string            `json:"BaseURL"`
	Model       string            `json:"Model"`
	AuthType    string            `json:"AuthType"`    // "bearer", "api-key", "none"
	HeaderName  string            `json:"HeaderName"`  // for api-key auth
	EnvVar      string            `json:"EnvVar"`
	Description string            `json:"Description"`
	Models      []string          `json:"Models"`
	ExtraFields []ExtraFieldDef   `json:"ExtraFields"`
	AutoDetect  bool              `json:"AutoDetect"`  // true for Ollama/LM Studio
}

type ExtraFieldDef struct {
	Key         string `json:"Key"`
	Label       string `json:"Label"`
	Placeholder string `json:"Placeholder"`
	EnvVar      string `json:"EnvVar"`
}

var ProviderPresets = []ProviderPreset{
	{
		Name:        "OpenAI",
		Icon:        "OpenAI",
		BaseURL:     "https://api.openai.com/v1",
		Model:       "gpt-4o",
		AuthType:    "bearer",
		EnvVar:      "OPENAI_API_KEY",
		Description: "Visit the OpenAI dashboard to generate an API key.",
		Models: []string{
			"gpt-4o", "gpt-4o-mini", "gpt-4o-2024-08-06", "gpt-4o-2024-11-20",
			"gpt-4.1", "gpt-4.1-mini", "gpt-4.1-nano",
			"gpt-4.1-2025-04-14", "gpt-4.1-mini-2025-04-14", "gpt-4.1-nano-2025-04-14",
			"gpt-5", "gpt-5-mini", "gpt-5-nano", "gpt-5-pro",
			"o3", "o3-mini", "o3-pro",
			"o4-mini", "o4-mini-2025-04-16",
			"gpt-oss-120b", "gpt-oss-20b",
			"gpt-image-2",
			"gpt-realtime-2.1", "gpt-realtime-2.1-mini", "gpt-realtime-2",
			"tts-1", "tts-1-hd", "whisper-1",
			"text-embedding-3-large", "text-embedding-3-small", "text-embedding-ada-002",
		},
	},
	{
		Name:        "Anthropic",
		Icon:        "Anthropic",
		BaseURL:     "https://api.anthropic.com/v1",
		Model:       "claude-sonnet-4-5-20250514",
		AuthType:    "api-key",
		HeaderName:  "x-api-key",
		EnvVar:      "ANTHROPIC_API_KEY",
		Description: "Visit the Anthropic dashboard to generate an API key.",
		Models: []string{
			"claude-sonnet-4-5-20250514", "claude-sonnet-4-20250514", "claude-haiku-4-20250514",
			"claude-opus-4-5-20250514", "claude-opus-4-20250514",
			"claude-sonnet-4-6-20250514", "claude-opus-4-6-20250514",
			"claude-fable-5-20250514",
			"claude-3-5-sonnet-latest", "claude-3-5-haiku-latest", "claude-3-opus-latest",
			"claude-3-1-sonnet-latest", "claude-3-1-haiku-latest",
		},
		ExtraFields: []ExtraFieldDef{
			{Key: "anthropic-version", Label: "API Version", Placeholder: "2023-06-01", EnvVar: "ANTHROPIC_VERSION"},
		},
	},
	{
		Name:        "Amazon Bedrock",
		Icon:        "Bedrock",
		BaseURL:     "https://bedrock-mantle.us-east-1.api.aws/openai/v1",
		Model:       "openai.gpt-4o",
		AuthType:    "bearer",
		EnvVar:      "BEDROCK_API_KEY",
		Description: "Set a custom authentication strategy or use static credentials. Mantle-only models require IAM permissions for the bedrock-mantle endpoint.",
		Models: []string{
			"openai.gpt-4o", "openai.gpt-4o-mini", "openai.gpt-4.1", "openai.gpt-4.1-mini",
			"anthropic.claude-sonnet-4-5-20250514", "anthropic.claude-opus-4-5-20250514", "anthropic.claude-haiku-4-5-20250514",
			"deepseek.deepseek-r1", "deepseek.deepseek-v3",
			"mistral.mistral-large-2411", "mistral.mistral-small-2411",
			"meta.llama3-3-70b-instruct", "meta.llama3-1-8b-instruct",
			"cohere.command-a-03-2025", "cohere.command-r-plus-08-2024",
			"ai21.jamba-1-5-large", "ai21.jamba-1-5-mini",
			"google.gemini-3-pro-preview", "google.gemini-3-flash-preview",
		},
		ExtraFields: []ExtraFieldDef{
			{Key: "region", Label: "AWS Region", Placeholder: "us-east-1", EnvVar: "AWS_REGION"},
		},
	},
	{
		Name:        "Azure OpenAI",
		Icon:        "Azure",
		BaseURL:     "",
		Model:       "",
		AuthType:    "api-key",
		HeaderName:  "api-key",
		EnvVar:      "AZURE_OPENAI_API_KEY",
		Description: "Visit the Azure OpenAI resource to generate an API key.",
		Models: []string{
			"gpt-4o", "gpt-4o-mini", "gpt-4.1", "gpt-4.1-mini", "gpt-4.1-nano",
			"o3", "o3-mini", "o4-mini",
			"gpt-5", "gpt-5-mini", "gpt-5-nano", "gpt-5-pro",
			"gpt-oss-120b", "gpt-oss-20b",
		},
		ExtraFields: []ExtraFieldDef{
			{Key: "resource", Label: "Resource Name", Placeholder: "my-resource", EnvVar: "AZURE_OPENAI_RESOURCE"},
			{Key: "deployment", Label: "Deployment Name", Placeholder: "gpt-4o", EnvVar: "AZURE_OPENAI_DEPLOYMENT"},
			{Key: "api-version", Label: "API Version", Placeholder: "2024-10-21", EnvVar: "AZURE_OPENAI_API_VERSION"},
		},
	},
	{
		Name:        "Google Vertex AI",
		Icon:        "Google",
		BaseURL:     "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/PROJECT/locations/us-central1/endpoints/openapi",
		Model:       "gemini-2.5-flash",
		AuthType:    "bearer",
		EnvVar:      "GOOGLE_APPLICATION_CREDENTIALS",
		Description: "Use a Google Cloud OAuth token. Generate via gcloud auth print-access-token.",
		Models: []string{
			"gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.5-flash-lite",
			"gemini-3-flash-preview", "gemini-3.1-pro-preview", "gemini-3.1-flash-preview",
			"gemini-2.0-flash", "gemini-2.0-flash-001", "gemini-2.0-flash-lite",
			"gemini-1.5-pro", "gemini-1.5-flash", "gemini-1.5-flash-8b",
			"gemma-3-27b-it", "gemma-3-12b-it", "gemma-3-4b-it", "gemma-3-1b-it",
			"gemma-2-27b-it", "gemma-2-9b-it", "gemma-2-2b-it",
		},
		ExtraFields: []ExtraFieldDef{
			{Key: "project", Label: "Project ID", Placeholder: "my-gcp-project", EnvVar: "GCP_PROJECT"},
			{Key: "location", Label: "Region", Placeholder: "us-central1", EnvVar: "GCP_LOCATION"},
		},
	},
	{
		Name:        "Cohere",
		Icon:        "Cohere",
		BaseURL:     "https://api.cohere.com/compatibility-api",
		Model:       "command-a-2025-05",
		AuthType:    "bearer",
		EnvVar:      "COHERE_API_KEY",
		Description: "Visit the Cohere dashboard to generate an API key.",
		Models: []string{
			"command-a-plus-05-2026", "command-a-03-2025", "command-r7b-12-2024",
			"command-a-translate-08-2025", "command-a-reasoning-08-2025", "command-a-vision-07-2025",
			"command-r-plus-08-2024", "command-r-08-2024", "command-r",
			"command", "command-light",
		},
	},
	{
		Name:        "Mistral",
		Icon:        "Mistral",
		BaseURL:     "https://api.mistral.ai/v1",
		Model:       "mistral-large-latest",
		AuthType:    "bearer",
		EnvVar:      "MISTRAL_API_KEY",
		Description: "Visit the Mistral dashboard to generate an API key.",
		Models: []string{
			"mistral-large-latest", "mistral-medium-latest", "mistral-small-latest",
			"codestral-latest", "codestral-embed-latest",
			"ministral-3-14b", "ministral-3-8b", "ministral-3-3b",
			"mistral-ocr-4", "mistral-ocr-3",
			"mistral-moderation-2-latest", "mistral-embed-latest",
			"voxtral-mini-transcribe-latest", "voxtral-small-latest",
		},
	},
	{
		Name:        "Kimi",
		Icon:        "Kimi",
		BaseURL:     "https://api.moonshot.cn/v1",
		Model:       "kimi-k3",
		AuthType:    "bearer",
		EnvVar:      "KIMI_API_KEY",
		Description: "Visit the Kimi dashboard to generate an API key.",
		Models: []string{
			"kimi-k3", "kimi-k2.7-code", "kimi-k2.7-code-highspeed", "kimi-k2.6",
			"kimi-k2.5", "kimi-k2-turbo", "kimi-k2-thinking",
		},
	},
	{
		Name:        "Groq",
		Icon:        "Groq",
		BaseURL:     "https://api.groq.com/openai/v1",
		Model:       "llama-3.3-70b-versatile",
		AuthType:    "bearer",
		EnvVar:      "GROQ_API_KEY",
		Description: "Visit the Groq dashboard to generate an API key.",
		Models: []string{
			"llama-3.3-70b-versatile", "llama-3.1-8b-instant", "llama-3.1-70b-versatile",
			"llama-3-8b-instant", "llama-3-70b-8192", "llama3-8b-8192", "llama3-70b-8192",
			"mixtral-8x7b-32768", "openai/gpt-oss-120b", "openai/gpt-oss-20b",
			"qwen/qwen3.6-27b", "qwen/qwq-32b",
			"gemma2-9b-it", "gemma-7b-it",
			"deepseek-r1-distill-llama-70b", "deepseek-r1-distill-qwen-32b",
			"whisper-large-v3", "whisper-large-v3-turbo",
		},
	},
	{
		Name:        "DeepSeek",
		Icon:        "DeepSeek",
		BaseURL:     "https://api.deepseek.com/v1",
		Model:       "deepseek-chat",
		AuthType:    "bearer",
		EnvVar:      "DEEPSEEK_API_KEY",
		Description: "Visit the DeepSeek dashboard to generate an API key.",
		Models: []string{
			"deepseek-v4-flash", "deepseek-v4-pro", "deepseek-chat", "deepseek-reasoner",
		},
	},
	{
		Name:        "Ollama",
		Icon:        "Ollama",
		BaseURL:     "http://localhost:11434/v1",
		Model:       "",
		AuthType:    "none",
		EnvVar:      "",
		Description: "Local Ollama instance. Models are auto-detected from the running Ollama server.",
		Models:      []string{},
		AutoDetect:  true,
	},
	{
		Name:        "LM Studio",
		Icon:        "LMStudio",
		BaseURL:     "http://localhost:1234/v1",
		Model:       "",
		AuthType:    "none",
		EnvVar:      "",
		Description: "Local LM Studio instance. Models are auto-detected from the running LM Studio server.",
		Models:      []string{},
		AutoDetect:  true,
	},
	{
		Name:        "OpenRouter",
		Icon:        "OpenRouter",
		BaseURL:     "https://openrouter.ai/api/v1",
		Model:       "nvidia/nemotron-3-super-120b-a12b:free",
		AuthType:    "bearer",
		EnvVar:      "OPENROUTER_API_KEY",
		Description: "Visit the OpenRouter dashboard to generate an API key.",
		Models: []string{
			"openai/gpt-4o", "openai/gpt-4o-mini", "openai/gpt-4.1", "openai/gpt-4.1-mini",
			"anthropic/claude-sonnet-4-5", "anthropic/claude-opus-4-5", "anthropic/claude-haiku-4",
			"deepseek/deepseek-v4-flash", "deepseek/deepseek-v4-pro", "deepseek/deepseek-chat",
			"mistralai/mistral-large-latest", "mistralai/mistral-small-latest",
			"meta-llama/llama-3.3-70b-instruct", "meta-llama/llama-3.1-8b-instruct",
			"qwen/qwen3-235b-a22b", "qwen/qwen3-coder-480b-a35b",
			"google/gemini-2.5-flash", "google/gemini-2.5-pro",
			"moonshotai/kimi-k3", "moonshotai/kimi-k2.5",
			"nvidia/nemotron-3-super-120b-a12b:free",
			"openai/gpt-5", "openai/o3", "openai/o4-mini",
		},
	},
}

func GetPresetByName(name string) *ProviderPreset {
	for i, p := range ProviderPresets {
		if p.Name == name {
			return &ProviderPresets[i]
		}
	}
	return nil
}