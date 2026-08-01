package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"demios/core"
	"demios/internal/db"
	"demios/internal/server/sse"
	"demios/llm"
)

type chatRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id"`
}

func HandleChatStream(agent *core.Agent, database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sse.WriteError(w, err)
			return
		}

		if strings.HasPrefix(strings.TrimSpace(req.Message), "@browser") {
			browserPrompt := strings.TrimSpace(strings.TrimPrefix(req.Message, "@browser"))
			HandleBrowserStart(agent, agent.ServerManager(), w, r, browserPrompt)
			return
		}

		if req.SessionID != "" && database != nil {
			session, err := db.GetSession(database, req.SessionID)
			if err == nil && session.History != "" && session.History != "[]" {
				history, err := db.UnmarshalHistory(session.History)
				if err == nil && len(history) > 0 {
					agent.RestoreHistory(history)
					log.Printf("[handler] restored %d history messages for session %s", len(history), req.SessionID)
				}
			}
		}

		sse.WriteHeaders(w)

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		events := make(chan core.AgentEvent)
		go agent.StepStream(ctx, req.Message, events)

		for event := range events {
			payload, err := json.Marshal(event.Data)
			if err != nil {
				log.Printf("[handler] failed to marshal event: %v", err)
				cancel()
				break
			}
			if err := sse.WriteRaw(w, event.Type, payload); err != nil {
				log.Printf("[handler] client disconnected: %v", err)
				cancel()
				break
			}
		}

		sse.WriteDone(w)

		if req.SessionID != "" && database != nil {
			saveSessionData(database, agent, req.SessionID, req.Message)
		}
	}
}

func saveSessionData(database *sql.DB, agent *core.Agent, sessionID, userMessage string) {
	history := agent.GetHistory()
	if len(history) == 0 {
		return
	}

	historyJSON, err := db.MarshalHistory(history)
	if err != nil {
		log.Printf("[handler] marshal history error: %v", err)
		return
	}
	if err := db.UpdateSessionHistory(database, sessionID, historyJSON); err != nil {
		log.Printf("[handler] save history error: %v", err)
	}

	reasonings := agent.GetHistoryReasonings()
	for len(reasonings) < len(history) {
		reasonings = append(reasonings, "")
	}
	msgs := extractDisplayMessages(history, reasonings)
	if len(msgs) > 0 {
		if err := db.SaveMessages(database, sessionID, msgs); err != nil {
			log.Printf("[handler] save messages error: %v", err)
		}
	}

	existing, err := db.GetMessages(database, sessionID)
	if err == nil && len(existing) <= 2 {
		title := truncate(userMessage, 40)
		if err := db.RenameSession(database, sessionID, title); err != nil {
			log.Printf("[handler] rename session error: %v", err)
		}
	}
}

type tcState struct {
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Args   interface{} `json:"args"`
	Status string      `json:"status"`
	Output string      `json:"output"`
}

func flushPending(msgs *[]db.Message, pending *[]tcState) {
	if len(*pending) == 0 {
		return
	}
	tcJSON, err := json.Marshal(*pending)
	if err != nil {
		log.Printf("[handler] failed to marshal tool calls: %v", err)
	} else {
		*msgs = append(*msgs, db.Message{Role: "assistant", ToolCalls: string(tcJSON)})
	}
	*pending = nil
}

func extractDisplayMessages(history []llm.Message, reasonings []string) []db.Message {
	var msgs []db.Message
	var pending []tcState

	for i, m := range history {
		switch {
		case m.OfUser != nil:
			flushPending(&msgs, &pending)
			msgs = append(msgs, db.Message{Role: "user", Content: llm.ContentString(m.OfUser.Content)})

		case m.OfAssistant != nil:
			thinking := ""
			if i < len(reasonings) {
				thinking = reasonings[i]
			}

			if len(m.OfAssistant.ToolCalls) > 0 {
				flushPending(&msgs, &pending)
				for _, tc := range m.OfAssistant.ToolCalls {
					var args interface{}
					if tc.Function.Arguments != "" {
						if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
							log.Printf("[handler] failed to parse tool call args for %s: %v", tc.Function.Name, err)
						}
					}
					pending = append(pending, tcState{
						ID: tc.ID, Name: tc.Function.Name,
						Args: args, Status: "completed",
					})
				}
			}

			if c := llm.ContentString(m.OfAssistant.Content); c != "" {
				flushPending(&msgs, &pending)
				msgs = append(msgs, db.Message{Role: "assistant", Content: c, Thinking: thinking})
			}

		case m.OfTool != nil:
			for j := range pending {
				if pending[j].ID == m.OfTool.ToolCallID {
					pending[j].Output = llm.ContentString(m.OfTool.Content)
					pending[j].Status = "completed"
					break
				}
			}
		}
	}

	flushPending(&msgs, &pending)

	return msgs
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
