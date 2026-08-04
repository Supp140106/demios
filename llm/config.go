package llm

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ProviderConfig struct {
	ID             string            `json:"ID"`
	Name           string            `json:"Name"`
	BaseURL        string            `json:"BaseURL"`
	CompletionsURL string            `json:"CompletionsURL"`
	APIKey         string            `json:"APIKey"`
	AuthType       string            `json:"AuthType"`
	HeaderName     string            `json:"HeaderName"`
	Headers        map[string]string `json:"Headers"`
	Models         []string          `json:"Models"`
	ExtraFields    map[string]string `json:"ExtraFields"`
	AutoDetect     bool              `json:"AutoDetect"`
	EnvVar         string            `json:"EnvVar"`
}

var (
	userProviders   []ProviderConfig
	userProvidersMu sync.RWMutex
	providersFile   string
	ollamaCache     = make(map[string][]string)
	lmStudioCache   = make(map[string][]string)
	cacheMu         sync.RWMutex
	cacheTimes      = make(map[string]time.Time)
)

func SetProvidersDir(dir string) {
	providersFile = filepath.Join(dir, "providers.json")
	LoadUserProviders()
}

func LoadUserProviders() {
	if providersFile == "" {
		return
	}
	data, err := os.ReadFile(providersFile)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Printf("[config] failed to load providers: %v", err)
		return
	}
	var providers []ProviderConfig
	if err := json.Unmarshal(data, &providers); err != nil {
		log.Printf("[config] failed to parse providers: %v", err)
		return
	}
	userProvidersMu.Lock()
	userProviders = providers
	userProvidersMu.Unlock()
	log.Printf("[config] loaded %d user providers", len(providers))
}

func SaveUserProviders() error {
	if providersFile == "" {
		return nil
	}
	userProvidersMu.RLock()
	data, err := json.MarshalIndent(userProviders, "", "  ")
	userProvidersMu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(providersFile, data, 0600)
}

func GetUserProviders() []ProviderConfig {
	userProvidersMu.RLock()
	defer userProvidersMu.RUnlock()
	out := make([]ProviderConfig, len(userProviders))
	copy(out, userProviders)
	return out
}

func GetAllModels() []ModelConfig {
	userProvidersMu.RLock()
	defer userProvidersMu.RUnlock()
	all := make([]ModelConfig, 0, len(AvailableModels)+len(userProviders)*5)
	all = append(all, AvailableModels...)
	for _, p := range userProviders {
		models := p.Models
		if p.AutoDetect && len(models) == 0 {
			models = detectLocalModels(p)
		}
		for _, m := range models {
			modelID := p.ID + "--" + m
			cfg := ModelConfig{
				ID:              modelID,
				Label:           p.Name + ": " + m,
				BaseURL:         p.BaseURL,
				APIKey:          p.APIKey,
				Model:           m,
				BackendType:     "",
				AuthType:        p.AuthType,
				Headers:         p.Headers,
				CompletionsURL:  p.CompletionsURL,
				ExtraBody:       map[string]any{},
				BuiltIn:         false,
				EnvVarName:      p.EnvVar,
			}
			all = append(all, cfg)
		}
	}
	return all
}

func FindModelConfig(id string) ModelConfig {
	for _, m := range GetAllModels() {
		if m.ID == id {
			return m
		}
	}
	if len(AvailableModels) > 0 {
		return AvailableModels[0]
	}
	return ModelConfig{}
}

func AddUserProvider(cfg ProviderConfig) error {
	userProvidersMu.Lock()
	defer userProvidersMu.Unlock()
	for i, p := range userProviders {
		if p.ID == cfg.ID {
			userProviders[i] = cfg
			return saveAndReloadLocked()
		}
	}
	userProviders = append(userProviders, cfg)
	return saveAndReloadLocked()
}

func UpdateUserProvider(cfg ProviderConfig) error {
	userProvidersMu.Lock()
	defer userProvidersMu.Unlock()
	for i, p := range userProviders {
		if p.ID == cfg.ID {
			userProviders[i] = cfg
			return saveAndReloadLocked()
		}
	}
	return saveAndReloadLocked()
}

func RemoveUserProvider(id string) error {
	userProvidersMu.Lock()
	defer userProvidersMu.Unlock()
	for i, p := range userProviders {
		if p.ID == id {
			userProviders = append(userProviders[:i], userProviders[i+1:]...)
			return saveAndReloadLocked()
		}
	}
	return nil
}

func saveAndReloadLocked() error {
	data, err := json.MarshalIndent(userProviders, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(providersFile, data, 0600); err != nil {
		return err
	}
	return nil
}

func detectLocalModels(p ProviderConfig) []string {
	if p.AutoDetect {
		if p.BaseURL == "http://localhost:11434/v1" {
			return detectOllamaModels(p.BaseURL)
		}
		if p.BaseURL == "http://localhost:1234/v1" {
			return detectLMStudioModels(p.BaseURL)
		}
	}
	return nil
}

func detectOllamaModels(baseURL string) []string {
	cacheMu.RLock()
	if cached, ok := ollamaCache[baseURL]; ok {
		cacheMu.RUnlock()
		return cached
	}
	cacheMu.RUnlock()

	resp, err := http.Get(baseURL + "/api/tags")
	if err != nil {
		log.Printf("[config] ollama detect error: %v", err)
		return nil
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[config] ollama decode error: %v", err)
		return nil
	}

	models := make([]string, 0, len(result.Models))
	for _, m := range result.Models {
		name := m.Name
		// Strip :tag suffix for display
		if idx := findLastColon(name); idx >= 0 {
			name = name[:idx]
		}
		models = append(models, name)
	}

	cacheMu.Lock()
	ollamaCache[baseURL] = models
	cacheMu.Unlock()
	return models
}

func detectLMStudioModels(baseURL string) []string {
	cacheMu.RLock()
	if cached, ok := lmStudioCache[baseURL]; ok {
		cacheMu.RUnlock()
		return cached
	}
	cacheMu.RUnlock()

	resp, err := http.Get(baseURL + "/v1/models")
	if err != nil {
		log.Printf("[config] lmstudio detect error: %v", err)
		return nil
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[config] lmstudio decode error: %v", err)
		return nil
	}

	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, m.ID)
	}

	cacheMu.Lock()
	lmStudioCache[baseURL] = models
	cacheMu.Unlock()
	return models
}

func findLastColon(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}
