package server

import (
	"database/sql"
	"demios/core"
	"demios/internal/server/handler"
	"net"
	"net/http"
)

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func StartServer(agent *core.Agent, database *sql.DB, addr string) (*http.Server, string, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, "", err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/chat/stream", handler.HandleChatStream(agent, database))
	mux.HandleFunc("POST /api/permission/respond", handler.HandlePermissionResponse(agent))
	mux.HandleFunc("POST /api/human-input/respond", handler.HandleHumanInputResponse(agent))

	srv := &http.Server{Handler: corsMiddleware(mux)}
	go srv.Serve(listener)

	return srv, listener.Addr().String(), nil
}
