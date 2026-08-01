package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strings"

	"demios/core"
	"demios/internal/server/sse"
	"demios/tools"
)

var promptURLRegex = regexp.MustCompile(`https?://[^\s"']+`)

// extractURLFromPrompt pulls the first http(s) URL out of a free-form prompt,
// stripping trailing punctuation the user may have typed after it.
func extractURLFromPrompt(prompt string) string {
	m := promptURLRegex.FindString(prompt)
	if m == "" {
		return ""
	}
	return strings.TrimRight(m, ".,);]}")
}

func HandleBrowserStart(agent *core.Agent, serverManager *tools.ServerManager, w http.ResponseWriter, r *http.Request, prompt string) {
	sse.WriteHeaders(w)

	ba := core.NewBrowserAgent("browser", agent.Client(), serverManager)
	ba.Workspace = agent.Workspace

	// Always hand the browser agent a concrete target: if the prompt already
	// names a URL use it, otherwise resolve a running/auto-started server so
	// the port is sent along with the prompt automatically.
	explicitURL := extractURLFromPrompt(prompt)
	if targetURL, err := serverManager.ResolveBrowserTarget(r.Context(), agent.Workspace, explicitURL); err == nil {
		ba.TargetURL = targetURL
		log.Printf("[handler] browser target resolved: %s", targetURL)
	} else {
		log.Printf("[handler] browser target resolution failed (agent will rely on prompt): %v", err)
	}

	sse.WriteEvent(w, "browser-opened", map[string]string{
		"status": "Browser agent started — Chromium popup visible",
	})

	events := make(chan core.AgentEvent)
	go ba.StepStream(r.Context(), prompt, events)

	for event := range events {
		payload, err := json.Marshal(event.Data)
		if err != nil {
			continue
		}
		if err := sse.WriteRaw(w, event.Type, payload); err != nil {
			return
		}
	}

	sse.WriteDone(w)
}

func HandleBrowserStop(w http.ResponseWriter, r *http.Request) {
	sse.WriteHeaders(w)
	sse.WriteEvent(w, "browser-stopped", map[string]string{
		"status": "Browser session stopped",
	})
	sse.WriteDone(w)
}

func HandleBrowserTakeControl(w http.ResponseWriter, r *http.Request) {
	sse.WriteHeaders(w)
	sse.WriteEvent(w, "browser-control", map[string]string{
		"mode":   "user",
		"status": "User is now in control of the browser",
	})
	sse.WriteDone(w)
}

func HandleBrowserGiveControl(w http.ResponseWriter, r *http.Request) {
	sse.WriteHeaders(w)
	sse.WriteEvent(w, "browser-control", map[string]string{
		"mode":   "agent",
		"status": "Agent resumed control",
	})
	sse.WriteDone(w)
}