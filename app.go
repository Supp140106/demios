package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"demios/core"
	"demios/internal/db"
	"demios/internal/server"
	"demios/llm"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx           context.Context
	server        *http.Server
	port          string
	agent         *core.Agent
	browserAgent  *core.BrowserAgent
	db            *sql.DB
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.agent = core.NewAgent("Demios")

	dbPath := filepath.Join(userDataDir(), "chat.db")
	var err error
	a.db, err = db.Init(dbPath)
	if err != nil {
		log.Printf("WARNING: failed to init DB: %v", err)
	} else {
		log.Printf("DB initialized at %s", dbPath)
	}

	srv, port, err := server.StartServer(a.agent, a.db, "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	a.server = srv
	a.port = port

	if p := parsePortFromAddr(port); p > 0 {
		a.agent.ServerManager().ExcludePort(p)
		log.Printf("Excluded backend port %d from dev-server detection", p)
	}

	log.Printf("Server running on %s", a.port)
}

func parsePortFromAddr(addr string) int {
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	p, err := strconv.Atoi(portStr)
	if err != nil {
		return 0
	}
	return p
}

func userDataDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "demios")
}

func (a *App) GetServerPort() string {
	return a.port
}

func (a *App) SetWorkspace(path string) {
	a.agent.SetWorkspace(path)
	log.Printf("Workspace set to %s", path)
}

func (a *App) PickDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Workspace",
	})
}

func (a *App) SetModel(id string) error {
	log.Printf("Switching model to: %s", id)
	return a.agent.SetModel(id)
}

func (a *App) GetModels() []llm.ModelConfig {
	return a.agent.GetModels()
}

func (a *App) GetCurrentModel() string {
	return a.agent.GetCurrentModel()
}

func (a *App) GetSessions(workspace string) []db.Session {
	if a.db == nil {
		return nil
	}
	sessions, err := db.ListSessions(a.db, workspace)
	if err != nil {
		log.Printf("GetSessions error: %v", err)
		return nil
	}
	return sessions
}

func (a *App) CreateSession(workspace string) db.Session {
	if a.db == nil {
		return db.Session{}
	}
	session, err := db.CreateSession(a.db, "New Chat", workspace)
	if err != nil {
		log.Printf("CreateSession error: %v", err)
		return db.Session{}
	}
	return session
}

func (a *App) DeleteSession(id string) error {
	if a.db == nil {
		return nil
	}
	return db.DeleteSession(a.db, id)
}

func (a *App) RenameSession(id, title string) error {
	if a.db == nil {
		return nil
	}
	return db.RenameSession(a.db, id, title)
}

func (a *App) GetSessionMessages(id string) []db.Message {
	if a.db == nil {
		return nil
	}
	msgs, err := db.GetMessages(a.db, id)
	if err != nil {
		log.Printf("GetSessionMessages error: %v", err)
		return nil
	}
	return msgs
}

func (a *App) GetWorkspace() string {
	return a.agent.Workspace
}

func (a *App) SetBrowserAgent(ba *core.BrowserAgent) {
	a.browserAgent = ba
}

func (a *App) StopBrowserAgent() {
	if a.browserAgent != nil {
		a.browserAgent.StopBrowser()
		a.browserAgent = nil
	}
}

func (a *App) GetBrowserAgent() *core.BrowserAgent {
	return a.browserAgent
}

func (a *App) shutdown(ctx context.Context) {
	a.server.Shutdown(ctx)
	if a.db != nil {
		a.db.Close()
	}
}
