package tools

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSearchUIComponentsLive(t *testing.T) {
	for _, q := range []string{"button", "gradient", "card", "sparkles", "text"} {
		result, err := SearchUIComponents.Execute(context.Background(), []byte(`{"query":"`+q+`","registry":"all"}`))
		if err != nil {
			t.Fatalf("query %q: execute error: %v", q, err)
		}
		if strings.TrimSpace(result.Output) == "" {
			t.Errorf("query %q: empty output", q)
		}
		if !strings.Contains(result.Output, "UI components matching") {
			t.Errorf("query %q: unexpected output header:\n%s", q, result.Output)
		}
	}
}

func TestLoadRegistryIndexCached(t *testing.T) {
	client := &http.Client{Timeout: 20 * time.Second}
	reg := uiRegistries[0]
	first, err := loadRegistryIndex(reg, client)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	if len(first) == 0 {
		t.Fatalf("expected non-empty index for %s", reg.id)
	}

	// Force cache hit by recording fetch time.
	registryCacheMu.Lock()
	if cached, ok := registryCache[reg.id]; ok {
		cached.fetchedAt = time.Now()
	}
	registryCacheMu.Unlock()

	second, err := loadRegistryIndex(reg, client)
	if err != nil {
		t.Fatalf("cached load: %v", err)
	}
	if len(second) == 0 || len(second) != len(first) {
		t.Errorf("cached index length mismatch: got %d want %d", len(second), len(first))
	}
}
