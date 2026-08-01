package handler

import (
	"encoding/json"
	"net/http"

	"demios/core"
)

type HumanInputRequest struct {
	ID    string `json:"id"`
	Input string `json:"input"`
}

func HandleHumanInputResponse(agent *core.Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req HumanInputRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
			return
		}
		if req.ID == "" {
			http.Error(w, `{"error":"id is required"}`, http.StatusBadRequest)
			return
		}
		if req.Input == "" {
			req.Input = "[user cancelled]"
		}

		if !agent.RespondHumanInput(req.ID, req.Input) {
			http.Error(w, `{"error":"human input request not found or already responded"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}
}
