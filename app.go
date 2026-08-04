package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"demios/core"
	"demios/internal/db"
	"demios/internal/server"
	"demios/llm"
	"demios/tools"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type FileEntry struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"is_dir"`
}

type App struct {
	ctx             context.Context
	server          *http.Server
	port            string
	agent           *core.Agent
	browserAgent    *core.BrowserAgent
	db              *sql.DB
	terminalManager *tools.TerminalManager
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.agent = core.NewAgent("Demios")
	a.terminalManager = tools.NewTerminalManager()

	dbPath := filepath.Join(userDataDir(), "chat.db")
	var err error
	a.db, err = db.Init(dbPath)
	if err != nil {
		log.Printf("WARNING: failed to init DB: %v", err)
	} else {
		log.Printf("DB initialized at %s", dbPath)
	}

	llm.SetProvidersDir(userDataDir())

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

func (a *App) GetProviderPresets() []llm.ProviderPreset {
	return llm.ProviderPresets
}

func (a *App) ListWorkspaceFiles(pattern string) []FileEntry {
	ws := a.agent.Workspace
	if ws == "" {
		return nil
	}

	if entries := a.listViaRg(ws, pattern); entries != nil {
		return entries
	}
	if entries := a.listViaGit(ws, pattern); entries != nil {
		return entries
	}
	return a.listViaWalk(ws, pattern)
}

var ignoreDirs = map[string]bool{
	"node_modules": true, "dist": true, "build": true, ".next": true,
	"target": true, "vendor": true, ".git": true, "__pycache__": true,
	"coverage": true, ".venv": true, "venv": true, ".cache": true,
	".turbo": true, ".nuxt": true, ".output": true, "out": true,
	".svelte-kit": true, ".parcel-cache": true,
}

var ignoreExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".ico": true, ".svg": true, ".woff": true, ".woff2": true, ".ttf": true,
	".eot": true, ".wasm": true, ".bin": true, ".exe": true, ".dll": true,
	".so": true, ".dylib": true, ".zip": true, ".tar": true, ".gz": true,
	".rar": true, ".7z": true, ".pdf": true, ".mp3": true, ".mp4": true,
	".avi": true, ".mov": true, ".lock": true, ".sum": true,
}

func (a *App) listViaRg(ws, pattern string) []FileEntry {
	rg, err := exec.LookPath("rg")
	if err != nil {
		return nil
	}

	args := []string{"--files", "--no-require-git", ws}
	cmd := exec.Command(rg, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	query := strings.ToLower(strings.TrimPrefix(pattern, "**/*"))
	query = strings.TrimSuffix(query, "*")

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var files []FileEntry
	for _, f := range lines {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		rel, _ := filepath.Rel(ws, f)
		if query != "" && !strings.Contains(strings.ToLower(rel), query) {
			continue
		}
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		files = append(files, FileEntry{
			Path:  rel,
			Name:  filepath.Base(f),
			Size:  info.Size(),
			IsDir: false,
		})
		if len(files) >= 200 {
			break
		}
	}
	return injectDirs(files, query)
}

func injectDirs(files []FileEntry, query string) []FileEntry {
	dirs := map[string]bool{}
	for _, f := range files {
		dir := filepath.Dir(f.Path)
		for dir != "." && dir != "" {
			dirs[dir] = true
			dir = filepath.Dir(dir)
		}
	}

	maxDirs := 50
	if query == "" {
		maxDirs = 30
	}

	var dirEntries []FileEntry
	for d := range dirs {
		name := filepath.Base(d)
		if query != "" {
			if !strings.Contains(strings.ToLower(name), query) &&
				!strings.Contains(strings.ToLower(d), query) {
				continue
			}
		} else {
			parts := strings.Split(d, string(filepath.Separator))
			if len(parts) > 2 {
				continue
			}
		}
		if ignoreDirs[name] || strings.HasPrefix(name, ".") {
			continue
		}
		dirEntries = append(dirEntries, FileEntry{
			Path:  d,
			Name:  name,
			Size:  0,
			IsDir: true,
		})
		if len(dirEntries) >= maxDirs {
			break
		}
	}

	sort.Slice(dirEntries, func(i, j int) bool {
		return dirEntries[i].Path < dirEntries[j].Path
	})
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	maxTotal := 100
	if query == "" {
		maxTotal = 60
	}

	result := make([]FileEntry, 0, len(dirEntries)+len(files))
	result = append(result, dirEntries...)
	result = append(result, files...)
	if len(result) > maxTotal {
		result = result[:maxTotal]
	}
	return result
}

func (a *App) listViaGit(ws, pattern string) []FileEntry {
	cmd := exec.Command("git", "-C", ws, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	query := strings.ToLower(strings.TrimPrefix(pattern, "**/*"))
	query = strings.TrimSuffix(query, "*")

	parts := strings.Split(string(output), "\x00")
	var files []FileEntry
	for _, rel := range parts {
		rel = strings.TrimSpace(rel)
		if rel == "" {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(rel), query) {
			continue
		}
		full := filepath.Join(ws, rel)
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		files = append(files, FileEntry{
			Path:  rel,
			Name:  filepath.Base(rel),
			Size:  info.Size(),
			IsDir: false,
		})
		if len(files) >= 200 {
			break
		}
	}
	return injectDirs(files, query)
}

func (a *App) listViaWalk(ws, pattern string) []FileEntry {
	query := strings.ToLower(strings.TrimPrefix(pattern, "**/*"))
	query = strings.TrimSuffix(query, "*")

	var files []FileEntry
	filepath.Walk(ws, func(path string, info os.FileInfo, err error) error {
		if err != nil || len(files) >= 200 {
			return nil
		}
		rel, _ := filepath.Rel(ws, path)
		if rel == "." {
			return nil
		}
		name := info.Name()
		if info.IsDir() {
			if ignoreDirs[name] || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
		} else {
			if strings.HasPrefix(name, ".") {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(name))
			if ignoreExts[ext] {
				return nil
			}
			if query != "" && !strings.Contains(strings.ToLower(rel), query) {
				return nil
			}
			files = append(files, FileEntry{
				Path:  rel,
				Name:  name,
				Size:  info.Size(),
				IsDir: false,
			})
		}
		return nil
	})
	return injectDirs(files, query)
}

func (a *App) ReadWorkspaceFile(path string) (string, error) {
	ws := a.agent.Workspace
	var full string
	if filepath.IsAbs(path) {
		full = path
	} else if ws != "" {
		full = filepath.Join(ws, path)
	} else {
		full = path
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type FolderContent struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (a *App) ReadWorkspaceFolder(path string, maxFiles int) []FolderContent {
	ws := a.agent.Workspace
	var dir string
	if filepath.IsAbs(path) {
		dir = path
	} else if ws != "" {
		dir = filepath.Join(ws, path)
	} else {
		dir = path
	}

	if maxFiles <= 0 {
		maxFiles = 30
	}

	var results []FolderContent
	totalSize := 0
	const maxSize = 500 * 1024

	filepath.Walk(dir, func(fullPath string, info os.FileInfo, err error) error {
		if err != nil || len(results) >= maxFiles || totalSize >= maxSize {
			return filepath.SkipDir
		}
		if info.IsDir() {
			name := info.Name()
			if ignoreDirs[name] || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		name := info.Name()
		if strings.HasPrefix(name, ".") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(name))
		if ignoreExts[ext] {
			return nil
		}
		if info.Size() > 100*1024 {
			return nil
		}

		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil
		}

		rel, _ := filepath.Rel(ws, fullPath)
		results = append(results, FolderContent{
			Path:    rel,
			Content: string(data),
		})
		totalSize += len(data)
		return nil
	})
	return results
}

func (a *App) GetProviders() []llm.ProviderConfig {
	return llm.GetUserProviders()
}

func (a *App) AddProvider(cfg llm.ProviderConfig) error {
	return llm.AddUserProvider(cfg)
}

func (a *App) UpdateProvider(cfg llm.ProviderConfig) error {
	return llm.UpdateUserProvider(cfg)
}

func (a *App) RemoveProvider(id string) error {
	return llm.RemoveUserProvider(id)
}

func (a *App) shutdown(ctx context.Context) {
	a.server.Shutdown(ctx)
	if a.db != nil {
		a.db.Close()
	}
	// Close all terminal sessions
	if a.terminalManager != nil {
		for _, id := range a.terminalManager.ActiveIDs() {
			a.terminalManager.Remove(id)
		}
	}
}

// ---------------------------------------------------------------------------
// Terminal bindings — interactive PTY for the user
// ---------------------------------------------------------------------------

type TerminalInfo struct {
	ID      string `json:"id"`
	Shell   string `json:"shell"`
	Workdir string `json:"workdir"`
}

// CreateTerminal creates a new interactive terminal session.
func (a *App) CreateTerminal(shell string, workdir string) (TerminalInfo, error) {
	if a.terminalManager == nil {
		return TerminalInfo{}, nil
	}
	if workdir == "" {
		workdir = a.agent.Workspace
	}
	sess, err := a.terminalManager.Create(shell, workdir)
	if err != nil {
		return TerminalInfo{}, err
	}
	return TerminalInfo{
		ID:      sess.ID,
		Shell:   shell,
		Workdir: workdir,
	}, nil
}

// WriteTerminal sends input to a terminal session.
func (a *App) WriteTerminal(id string, data string) error {
	if a.terminalManager == nil {
		return nil
	}
	sess, ok := a.terminalManager.Get(id)
	if !ok {
		return nil
	}
	return sess.Write([]byte(data))
}

// ReadTerminal reads and clears the output buffer of a terminal session.
func (a *App) ReadTerminal(id string) string {
	if a.terminalManager == nil {
		return ""
	}
	sess, ok := a.terminalManager.Get(id)
	if !ok {
		return ""
	}
	return sess.ReadOutput()
}

// ResizeTerminal resizes a terminal session's viewport.
func (a *App) ResizeTerminal(id string, cols int, rows int) error {
	if a.terminalManager == nil {
		return nil
	}
	sess, ok := a.terminalManager.Get(id)
	if !ok {
		return nil
	}
	return sess.Resize(uint16(cols), uint16(rows))
}

// CloseTerminal closes a terminal session.
func (a *App) CloseTerminal(id string) {
	if a.terminalManager == nil {
		return
	}
	a.terminalManager.Remove(id)
}

// ListTerminals returns all active terminal session IDs.
func (a *App) ListTerminals() []string {
	if a.terminalManager == nil {
		return nil
	}
	return a.terminalManager.ActiveIDs()
}
