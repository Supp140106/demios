package handler

import (
	"encoding/json"
	"net/http"

	"demios/core"
)

type PermissionRequest struct {
	ID      string `json:"id"`
	Allowed bool   `json:"allowed"`
}

func HandlePermissionResponse(agent *core.Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req PermissionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
			return
		}
		if req.ID == "" {
			http.Error(w, `{"error":"id is required"}`, http.StatusBadRequest)
			return
		}

		if !agent.RespondPermission(req.ID, req.Allowed) {
			http.Error(w, `{"error":"permission request not found or already responded"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}
}
