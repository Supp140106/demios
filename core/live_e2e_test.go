package core

import (
	"context"
	"demios/tools"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveE2E(t *testing.T) {
	if os.Getenv("DEMIOS_LIVE") == "" {
		t.Skip("set DEMIOS_LIVE=1 to run the live end-to-end test")
	}
	if os.Getenv("OPENROUTER_API_KEY") == "" {
		t.Skip("OPENROUTER_API_KEY not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Second)
	defer cancel()

	sm := tools.NewServerManager()
	defer sm.StopAll()

	inst, err := sm.StartServer(ctx, `C:\Code\hoka`, "", 0)
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	t.Logf("server url=%s port=%d status=%s", inst.URL, inst.Port, inst.Status)
	if inst.Port == 4096 {
		t.Fatalf("detected excluded backend port 4096 — port detection regression")
	}

	url := inst.URL
	a := NewAgent("live-e2e")
	if err := a.SetModel("openrouter-nemotron"); err != nil {
		t.Fatalf("SetModel: %v", err)
	}
	ba := NewBrowserAgent("browser-test", a.client, sm)
	ba.Workspace = a.Workspace
	ba.TargetURL = url

	events := make(chan AgentEvent)
	done := make(chan string, 1)

	go func() {
		for evt := range events {
			data, _ := json.Marshal(evt.Data)
			t.Logf("[event] %-18s %s", evt.Type, data)
		}
	}()

	prompt := fmt.Sprintf(`Test the website at %s. Execute every step in order:

1. browser_navigate to %s
2. browser_wait for the page to load, then browser_extract the visible text
3. browser_screenshot the homepage and describe the colors/theme/layout
4. Click the "Get Started" button (browser_click)
5. browser_fill #username with "demo" and #password with "demo123"
6. browser_click the sign-in button
7. Add 3 todos using the todo input form
8. browser_screenshot and describe what you see
9. Write a DETAILED FINAL REPORT: walk through each step, whether it succeeded, what colors/theme you observed, the page titles, buttons, form fields, and any failures.`, url, url)

	go func() {
		done <- ba.StepStream(ctx, prompt, events)
	}()

	select {
	case report := <-done:
		t.Logf("REPORT:\n%s", report)
		if strings.TrimSpace(report) == "" {
			t.Fatal("browser agent returned an empty report")
		}
		if !strings.Contains(strings.ToLower(report), "http") && len(report) < 50 {
			t.Fatal("report looks truncated or unrelated")
		}
	case <-ctx.Done():
		ba.StopBrowser()
		t.Fatal("timed out waiting for browser agent report")
	}
}
