package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ServerInstance represents a running dev server process.
type ServerInstance struct {
	ID         string
	PID        int
	Port       int
	URL        string
	ProjectDir string
	Status     string
	StartTime  time.Time
	Command    string

	stdout bytes.Buffer
	stderr bytes.Buffer
	cmd    *exec.Cmd
	mu     sync.RWMutex

	// portFromURL is true once the port was derived from an actual URL line
	// (e.g. "Local: http://localhost:5174/"). URL-derived ports are treated as
	// authoritative and override earlier keyword-based guesses.
	portFromURL bool

	// sawConflict is true when any output line announced a port conflict
	// ("Port 5173 is in use, trying another one..."). Used to decide whether
	// the LLM resolver should be consulted for the actual URL.
	sawConflict bool

	// onOutput, when set, is invoked with each line of server output as it
	// arrives so the agent can stream it to the UI in real time.
	onOutput func(s *ServerInstance, line string)
}

// Stdout returns captured stdout output.
func (s *ServerInstance) Stdout() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stdout.String()
}

// Stderr returns captured stderr output.
func (s *ServerInstance) Stderr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stderr.String()
}

// recordLine appends one line of server output to the given buffer and updates
// the detected port. URL-derived ports are authoritative and override earlier
// keyword-based guesses; port-conflict warnings are recorded but never used as
// the server port.
// stripANSI removes ANSI escape sequences (colour/bold codes) from a line of
// terminal output. Vite and other CLIs wrap values like the port number in
// SGR codes ("http://localhost:\x1b[1m5174\x1b[22m/"), which would otherwise
// hide the digits from the port regexes.
func stripANSI(line string) string {
	return ansiRegex.ReplaceAllString(line, "")
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func (s *ServerInstance) recordLine(buffer *bytes.Buffer, line string) {
	s.mu.Lock()
	buffer.WriteString(line + "\n")

	port, fromURL, conflict := detectPortFromLine(stripANSI(line))
	if conflict {
		s.sawConflict = true
	} else if port != 0 && (fromURL || !s.portFromURL) {
		s.Port = port
		s.URL = fmt.Sprintf("http://127.0.0.1:%d", port)
		s.portFromURL = fromURL
	}
	cb := s.onOutput
	s.mu.Unlock()

	if cb != nil {
		cb(s, line)
	}
}

// ServerManager maintains the lifecycle of dev server processes,

// resolveSettleDelay is how long StartServer waits for startup output to
// accumulate before asking the LLM for the actual URL. Vite prints the
// "Port X is in use, trying another one..." warning before the real
// "Local: http://localhost:PORT/" line, so a short settle lets the full
// banner reach the buffer.
const resolveSettleDelay = 6 * time.Second

// resolveLLMTimeout bounds a single LLM URL-resolution call.
const resolveLLMTimeout = 15 * time.Second

// resolveURLWithLLM waits for startup output to begin flowing, gives it a
// 6-second latency to accumulate the full banner, then asks the attached LLM
// resolver for the actual URL a server is listening on. It returns "" when
// there is nothing useful to send, the resolver is unavailable, or the
// resolved URL is not reachable. The fallback URL is used when resolution
// fails.
func (sm *ServerManager) resolveURLWithLLM(ctx context.Context, instance *ServerInstance, fallbackURL string) string {
	sm.mu.Lock()
	resolver := sm.resolveURLFromOutput
	sm.mu.Unlock()
	if resolver == nil {
		return ""
	}

	hasOutput := func() bool {
		instance.mu.Lock()
		defer instance.mu.Unlock()
		return instance.stdout.Len() > 0 || instance.stderr.Len() > 0
	}

	// Wait up to resolveSettleDelay for output to begin, then extend the
	// deadline by another resolveSettleDelay from first output so the full
	// banner (e.g. Vite's "Local: http://localhost:PORT/") reaches the buffer.
	firstSeen := time.Time{}
	deadline := time.Now().Add(resolveSettleDelay)
	for time.Now().Before(deadline) {
		if hasOutput() {
			if firstSeen.IsZero() {
				firstSeen = time.Now()
				deadline = firstSeen.Add(resolveSettleDelay)
			}
			break
		}
		select {
		case <-ctx.Done():
			return ""
		case <-time.After(250 * time.Millisecond):
		}
	}
	if firstSeen.IsZero() {
		log.Printf("[server] %s: ambiguous port but no output captured, skipping LLM resolution", instance.ID)
		return ""
	}
	if sleep := time.Until(firstSeen.Add(resolveSettleDelay)); sleep > 0 {
		select {
		case <-ctx.Done():
			return ""
		case <-time.After(sleep):
		}
	}

	instance.mu.Lock()
	output := instance.stdout.String() + "\n" + instance.stderr.String()
	instance.mu.Unlock()
	if strings.TrimSpace(output) == "" {
		return ""
	}

	rctx, cancel := context.WithTimeout(ctx, resolveLLMTimeout)
	defer cancel()
	resolved, err := resolver(rctx, output)
	if err != nil {
		log.Printf("[server] %s: LLM URL resolution failed: %v", instance.ID, err)
		return ""
	}
	resolved = strings.TrimSpace(resolved)
	if resolved == "" {
		return ""
	}
	if !sm.urlReachable(resolved) {
		log.Printf("[server] %s: LLM resolved %q but it is not reachable, keeping %s", instance.ID, resolved, fallbackURL)
		return ""
	}
	log.Printf("[server] %s: LLM resolved actual URL %s", instance.ID, resolved)
	return resolved
}

// independent of any single agent request context.
type ServerManager struct {
	mu      sync.Mutex
	servers map[string]*ServerInstance
	nextID  int

	// excludePorts are local ports that must never be picked up by the
	// auto-detection scan (e.g. the Demios backend's own random port).
	excludePorts map[int]bool

	// persistencePath, when set, persists the server registry to disk so the
	// lifecycle survives across processes (used by the headless CLI harness).
	// Empty disables persistence (default, keeps in-app behavior unchanged).
	persistencePath string

	// resolveURLFromOutput, when set, asks the LLM to determine the actual URL
	// a server is listening on from its startup output. Only consulted when
	// output parsing is ambiguous (port conflict or no URL detected).
	resolveURLFromOutput func(ctx context.Context, output string) (string, error)
}

// persistedServer is the on-disk record for a ServerInstance.
type persistedServer struct {
	ID         string    `json:"id"`
	PID        int       `json:"pid"`
	Port       int       `json:"port"`
	URL        string    `json:"url"`
	ProjectDir string    `json:"project_dir"`
	Status     string    `json:"status"`
	StartTime  time.Time `json:"start_time"`
	Command    string    `json:"command"`
}

// defaultExcludedPorts are local ports that must never be picked up by the
// auto-detection scan. Nothing is hardcoded here on purpose: the Demios
// backend's own random port is excluded dynamically at startup, and the user
// can add more via the DEMIOS_EXCLUDE_PORTS env var (comma-separated).
var defaultExcludedPorts = []int{}

// envExcludedPorts parses DEMIOS_EXCLUDE_PORTS (comma-separated integers)
// into a list of extra ports that must never be auto-detected.
func envExcludedPorts() []int {
	raw := os.Getenv("DEMIOS_EXCLUDE_PORTS")
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var ports []int
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if p, err := strconv.Atoi(part); err == nil && p > 0 {
			ports = append(ports, p)
		}
	}
	return ports
}

// NewServerManager creates a new ServerManager.
func NewServerManager() *ServerManager {
	sm := &ServerManager{
		servers:      make(map[string]*ServerInstance),
		nextID:       1,
		excludePorts: make(map[int]bool),
	}
	sm.ExcludePorts(defaultExcludedPorts...)
	sm.ExcludePorts(envExcludedPorts()...)
	return sm
}

// SetURLResolver attaches a function that resolves a server's actual URL from
// its startup output using the LLM. It is only called when output parsing is
// ambiguous (a port conflict was seen or no URL could be parsed).
func (sm *ServerManager) SetURLResolver(fn func(ctx context.Context, output string) (string, error)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.resolveURLFromOutput = fn
}

// ExcludePort marks a local port so the auto-detection scan never selects it.
func (sm *ServerManager) ExcludePort(port int) {
	if port <= 0 {
		return
	}
	sm.mu.Lock()
	sm.excludePorts[port] = true
	sm.mu.Unlock()
}

// ExcludePorts marks multiple local ports so the auto-detection scan never
// selects them.
func (sm *ServerManager) ExcludePorts(ports ...int) {
	for _, p := range ports {
		sm.ExcludePort(p)
	}
}

func (sm *ServerManager) isExcluded(port int) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.excludePorts[port]
}

// ExcludedPorts returns a copy of the currently excluded ports.
func (sm *ServerManager) ExcludedPorts() []int {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	ports := make([]int, 0, len(sm.excludePorts))
	for p := range sm.excludePorts {
		ports = append(ports, p)
	}
	return ports
}

// EnablePersistence turns on on-disk server tracking at path so the registry
// survives across processes. Existing records for still-alive processes are
// loaded back into the manager. Used by the headless CLI harness; the in-app
// agent keeps the default in-memory behavior.
func (sm *ServerManager) EnablePersistence(path string) error {
	sm.mu.Lock()
	sm.persistencePath = path
	sm.mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var recs []persistedServer
	if err := json.Unmarshal(data, &recs); err != nil {
		return fmt.Errorf("read server registry: %w", err)
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()
	for _, rec := range recs {
		if rec.ID == "" || !processAlive(rec.PID) {
			continue
		}
		sm.servers[rec.ID] = &ServerInstance{
			ID:         rec.ID,
			PID:        rec.PID,
			Port:       rec.Port,
			URL:        rec.URL,
			ProjectDir: rec.ProjectDir,
			Status:     "running",
			StartTime:  rec.StartTime,
			Command:    rec.Command,
		}
		if n := serverIDNum(rec.ID); n >= sm.nextID {
			sm.nextID = n + 1
		}
	}
	return nil
}

// serverIDNum extracts the numeric suffix from a server ID like "server-3".
func serverIDNum(id string) int {
	idx := strings.LastIndexByte(id, '-')
	if idx < 0 {
		return 0
	}
	n, _ := strconv.Atoi(id[idx+1:])
	return n
}

// persist writes the current server registry to disk when persistence is
// enabled. It never fails the caller: failures are logged only.
func (sm *ServerManager) persist() {
	sm.mu.Lock()
	path := sm.persistencePath
	if path == "" {
		sm.mu.Unlock()
		return
	}
	recs := make([]persistedServer, 0, len(sm.servers))
	for _, s := range sm.servers {
		s.mu.RLock()
		recs = append(recs, persistedServer{
			ID:         s.ID,
			PID:        s.PID,
			Port:       s.Port,
			URL:        s.URL,
			ProjectDir: s.ProjectDir,
			Status:     s.Status,
			StartTime:  s.StartTime,
			Command:    s.Command,
		})
		s.mu.RUnlock()
	}
	sm.mu.Unlock()

	data, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		log.Printf("[server] persist marshal: %v", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		log.Printf("[server] persist mkdir: %v", err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("[server] persist write: %v", err)
	}
}

// StartServer spawns a dev server, detects its port from stdout,
// waits until the server is ready, and returns immediately.
// The process continues running independently of the caller's context.
func (sm *ServerManager) StartServer(ctx context.Context, workdir, command string, preferPort int) (*ServerInstance, error) {
	sm.mu.Lock()
	for _, s := range sm.servers {
		if s.ProjectDir == workdir && s.Status == "running" {
			s.mu.Lock()
			status := s.Status
			url := s.URL
			s.mu.Unlock()
			if status == "running" && url != "" {
				sm.mu.Unlock()
				return s, nil
			}
		}
	}
	sm.mu.Unlock()

	if workdir == "" {
		var err error
		workdir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("cannot determine working directory: %w", err)
		}
	}
	absWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		return nil, fmt.Errorf("invalid workdir: %w", err)
	}
	info, err := os.Stat(absWorkdir)
	if err != nil {
		return nil, fmt.Errorf("workdir does not exist: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("workdir is not a directory: %s", absWorkdir)
	}

	if command == "" {
		command = findStartCommand(absWorkdir)
	}
	if command == "" {
		port := preferPort
		if port == 0 {
			port = sm.findFreePort()
		}
		command = fmt.Sprintf("npx serve -s %s -p %d", absWorkdir, port)
	}

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = absWorkdir

	if preferPort > 0 {
		cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", preferPort))
	}

	setProcessGroup(cmd)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	sm.mu.Lock()
	id := fmt.Sprintf("server-%d", sm.nextID)
	sm.nextID++
	instance := &ServerInstance{
		ID:         id,
		PID:        cmd.Process.Pid,
		Port:       preferPort,
		URL:        "",
		ProjectDir: absWorkdir,
		Status:     "starting",
		StartTime:  time.Now(),
		Command:    command,
		cmd:        cmd,
	}
	sm.servers[id] = instance
	sm.mu.Unlock()

	// Stream the server's output to the UI in real time when the caller
	// attached an event emitter to the context (the StartServer tool does).
	if emitter := EventEmitterFrom(ctx); emitter != nil {
		toolCallID := ToolCallIDFrom(ctx)
		instance.onOutput = func(s *ServerInstance, line string) {
			emitter("server-output", map[string]any{
				"tool_call_id": toolCallID,
				"server_id":    s.ID,
				"line":         line,
			})
		}
	}

	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			instance.recordLine(&instance.stdout, scanner.Text())
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			instance.recordLine(&instance.stderr, scanner.Text())
		}
	}()

	readyCtx, readyCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer readyCancel()

	readyURL := waitForServer(readyCtx, instance, 60, sm)
	if readyURL == "" {
		if instance.cmd != nil && instance.cmd.Process != nil {
			instance.cmd.Process.Kill()
			instance.cmd.Wait()
		}
		stderr := instance.stderr.String()
		sm.mu.Lock()
		delete(sm.servers, id)
		sm.mu.Unlock()
		sm.persist()
		if stderr != "" {
			return nil, fmt.Errorf("server failed to become ready within 60s. stderr: %s", stderr)
		}
		return nil, fmt.Errorf("server failed to become ready within 60s")
	}

	// Hybrid resolution: output parsing is ambiguous only when we could NOT
	// derive the port from an authoritative URL line. A clean "Local:
	// http://localhost:PORT/" parse is trusted as-is (fast path). The LLM is
	// consulted when nothing was parsed at all, or when a port conflict was
	// seen but no URL line resolved it — and only if the buffer has output.
	instance.mu.Lock()
	ambiguous := instance.Port == 0 || (instance.sawConflict && !instance.portFromURL)
	instance.mu.Unlock()

	if ambiguous && sm.resolveURLFromOutput != nil {
		if resolved := sm.resolveURLWithLLM(ctx, instance, readyURL); resolved != "" {
			readyURL = resolved
		}
	}

	instance.mu.Lock()
	instance.Status = "running"
	instance.URL = readyURL
	if instance.Port == 0 {
		if port := parsePortFromURL(readyURL); port > 0 {
			instance.Port = port
		}
	}
	instance.mu.Unlock()
	sm.persist()

	log.Printf("[server] started %s (pid=%d, url=%s)", id, cmd.Process.Pid, readyURL)
	return instance, nil
}

// StopServer stops a running server by ID.
func (sm *ServerManager) StopServer(id string) error {
	sm.mu.Lock()
	instance, ok := sm.servers[id]
	if !ok {
		sm.mu.Unlock()
		return fmt.Errorf("server %s not found", id)
	}
	sm.mu.Unlock()

	instance.mu.Lock()
	if instance.Status != "running" && instance.Status != "starting" {
		instance.mu.Unlock()
		return fmt.Errorf("server %s is not running (status: %s)", id, instance.Status)
	}
	cmd := instance.cmd
	pid := instance.PID
	instance.mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		killProcessTree(cmd.Process.Pid)

		waitDone := make(chan struct{})
		go func() {
			cmd.Wait()
			close(waitDone)
		}()

		select {
		case <-waitDone:
		case <-time.After(5 * time.Second):
			cmd.Process.Kill()
			<-waitDone
		}
	} else if pid > 0 {
		// Instance was restored from disk (nil cmd): kill by PID only.
		killProcessTree(pid)
	}

	sm.mu.Lock()
	instance.mu.Lock()
	instance.Status = "stopped"
	instance.mu.Unlock()
	delete(sm.servers, id)
	sm.mu.Unlock()
	sm.persist()

	log.Printf("[server] stopped %s", id)
	return nil
}

// GetServer returns the server instance for the given ID.
func (sm *ServerManager) GetServer(id string) (*ServerInstance, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	instance, ok := sm.servers[id]
	if !ok {
		return nil, fmt.Errorf("server %s not found", id)
	}
	return instance, nil
}

// ListServers returns all server instances.
func (sm *ServerManager) ListServers() []*ServerInstance {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	result := make([]*ServerInstance, 0, len(sm.servers))
	for _, s := range sm.servers {
		result = append(result, s)
	}
	return result
}

// RestartServer stops and starts a server.
func (sm *ServerManager) RestartServer(ctx context.Context, id string) (*ServerInstance, error) {
	sm.mu.Lock()
	instance, ok := sm.servers[id]
	if !ok {
		sm.mu.Unlock()
		return nil, fmt.Errorf("server %s not found", id)
	}
	workdir := instance.ProjectDir
	command := instance.Command
	port := instance.Port
	sm.mu.Unlock()

	if err := sm.StopServer(id); err != nil {
		return nil, fmt.Errorf("failed to stop server: %w", err)
	}

	return sm.StartServer(ctx, workdir, command, port)
}

// FindByWorkdir returns a running server for the given workdir, if any.
func (sm *ServerManager) FindByWorkdir(workdir string) *ServerInstance {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for _, s := range sm.servers {
		if s.ProjectDir == workdir && s.Status == "running" {
			s.mu.RLock()
			url := s.URL
			port := s.Port
			s.mu.RUnlock()
			if url != "" {
				return s
			}
			_ = port
		}
	}
	return nil
}

// findRunningServer returns the first running server that has a URL, if any.
func (sm *ServerManager) findRunningServer() *ServerInstance {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for _, s := range sm.servers {
		s.mu.RLock()
		status := s.Status
		url := s.URL
		s.mu.RUnlock()
		if status == "running" && url != "" {
			return s
		}
	}
	return nil
}

// urlReachable reports whether a URL serves a real HTML page and is not on the
// excluded-port list (the Demios backend's own random port, etc.).
func (sm *ServerManager) urlReachable(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if parsed.Port() != "" {
		if p, err := strconv.Atoi(parsed.Port()); err == nil && sm.isExcluded(p) {
			return false
		}
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return isDevServerResponse(resp)
}

// ResolveBrowserTarget determines the URL a browser test should run against.
// Priority:
//  1. explicitURL — if provided and it is a reachable HTML page, it wins.
//  2. the first running server managed by this ServerManager (the one we
//     actually started, so we never test an unrelated scanned port like the
//     Demios frontend).
//  3. an active dev server already running on the machine (e.g. manually started).
//  4. an auto-started server for the given workspace.
//
// This is the single place the "which server do we test?" decision is made so
// the main agent and the CLI harness behave identically and always hand the
// port/URL to the browser agent.
func (sm *ServerManager) ResolveBrowserTarget(ctx context.Context, workspace, explicitURL string) (string, error) {
	if explicitURL != "" {
		if sm.urlReachable(explicitURL) {
			return explicitURL, nil
		}
		log.Printf("[server] explicit URL %s not reachable, falling back to a managed server", explicitURL)
	}

	// Prefer a server we actually started and know the exact URL of. The
	// port scan below can otherwise pick up the first reachable HTML page on
	// a standard port (e.g. Demios's own frontend), which is wrong.
	if running := sm.findRunningServer(); running != nil {
		running.mu.RLock()
		url := running.URL
		running.mu.RUnlock()
		if url != "" && sm.urlReachable(url) {
			return url, nil
		}
	}

	// Scan for any dev server already running on standard ports (e.g. started by the user)
	if existing := sm.findExistingDevServer(); existing != "" {
		log.Printf("[server] detected existing dev server running at %s", existing)
		return existing, nil
	}

	if workspace == "" {
		workspace, _ = os.Getwd()
	}
	instance, err := sm.StartServer(ctx, workspace, "", 0)
	if err != nil {
		return "", fmt.Errorf("no reachable URL and failed to start a server: %w", err)
	}
	return instance.URL, nil
}

// StopAll stops all running servers.
func (sm *ServerManager) StopAll() {
	sm.mu.Lock()
	ids := make([]string, 0, len(sm.servers))
	for id := range sm.servers {
		ids = append(ids, id)
	}
	sm.mu.Unlock()

	for _, id := range ids {
		sm.StopServer(id)
	}
}

// --- Helper functions ---

var portURLRegex = regexp.MustCompile(`(?:https?://)?(?:localhost|127\.0\.0\.1|0\.0\.0\.0|::1|\[::\]|\[::1\]|\[0\.0\.0\.0\]):(\d+)`)
var portKeywordRegex = regexp.MustCompile(`(?:port|PORT|Port)[\s:=]+(\d+)`)

// portConflictRegex matches lines announcing that a port is already taken.
// Such lines must never be treated as the server's actual port (e.g. Vite's
// "Port 5173 is in use, trying another one..." which precedes the real URL).
var portConflictRegex = regexp.MustCompile(`(?i)(in use|already in use|already taken|trying another|unavailable|occupied|cannot (?:be )?used|is taken)`)

// detectPortFromLine inspects one line of server output and reports a port
// candidate. fromURL is true when the port came from an actual URL (the most
// authoritative signal); conflict is true when the line is a port-conflict
// warning that must be ignored.
func detectPortFromLine(line string) (port int, fromURL bool, conflict bool) {
	if portConflictRegex.MatchString(line) {
		return 0, false, true
	}
	if matches := portURLRegex.FindStringSubmatch(line); len(matches) > 1 {
		if p, err := strconv.Atoi(matches[1]); err == nil {
			return p, true, false
		}
	}
	if matches := portKeywordRegex.FindStringSubmatch(line); len(matches) > 1 {
		if p, err := strconv.Atoi(matches[1]); err == nil {
			return p, false, false
		}
	}
	return 0, false, false
}

// parsePortFromLine is a convenience wrapper that returns 0 for conflict lines
// and otherwise returns the first port found in a line of output.
func parsePortFromLine(line string) int {
	port, _, conflict := detectPortFromLine(line)
	if conflict {
		return 0
	}
	return port
}

func parsePortFromURL(url string) int {
	if matches := portURLRegex.FindStringSubmatch(url); len(matches) > 1 {
		if port, err := strconv.Atoi(matches[1]); err == nil {
			return port
		}
	}
	return 0
}

func findStartCommand(dir string) string {
	pkgPath := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return ""
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}

	pm := "npm"
	if _, err := os.Stat(filepath.Join(dir, "pnpm-lock.yaml")); err == nil {
		pm = "pnpm"
	} else if _, err := os.Stat(filepath.Join(dir, "yarn.lock")); err == nil {
		pm = "yarn"
	} else if _, err := os.Stat(filepath.Join(dir, "bun.lockb")); err == nil {
		pm = "bun"
	}

	if _, ok := pkg.Scripts["dev"]; ok {
		return fmt.Sprintf("%s dev", pm)
	}
	if _, ok := pkg.Scripts["start"]; ok {
		return fmt.Sprintf("%s start", pm)
	}
	if _, ok := pkg.Scripts["serve"]; ok {
		return fmt.Sprintf("%s run serve", pm)
	}

	return ""
}

// findFreePort returns a free port in common dev-server ranges that is not on
// this manager's exclude list.
func (sm *ServerManager) findFreePort() int {
	for port := 5173; port < 5300; port++ {
		if sm.isExcluded(port) {
			continue
		}
		if isPortOpen(port) {
			return port
		}
	}
	for port := 3000; port < 3010; port++ {
		if sm.isExcluded(port) {
			continue
		}
		if isPortOpen(port) {
			return port
		}
	}
	for port := 8000; port < 8010; port++ {
		if sm.isExcluded(port) {
			continue
		}
		if isPortOpen(port) {
			return port
		}
	}
	return 8888
}

func isPortOpen(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

func isPortInUse(port int) bool {
	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// commonDevPortRanges lists the port ranges most likely to host a local dev
// server (vite, next, remix, angular, react-scripts, django, parcel, etc.).
var commonDevPortRanges = [][2]int{
	{5173, 5200}, // vite / astro / sveltekit defaults and increments
	{3000, 3010}, // next / nuxt / react-scripts
	{8000, 8010}, // python http.server / django / many tools
	{8080, 8090}, // common alternates
	{5000, 5010}, // flask / serve
	{4200, 4210}, // angular
}

func (sm *ServerManager) findExistingDevServer() string {
	httpClient := &http.Client{Timeout: 800 * time.Millisecond}
	for _, r := range commonDevPortRanges {
		for port := r[0]; port <= r[1]; port++ {
			if sm.isExcluded(port) {
				continue
			}
			if !isPortInUse(port) {
				continue
			}
			url := fmt.Sprintf("http://localhost:%d", port)
			resp, err := httpClient.Get(url)
			if err != nil {
				continue
			}
			if isDevServerResponse(resp) {
				resp.Body.Close()
				return url
			}
			resp.Body.Close()
		}
	}
	return ""
}

// isDevServerResponse reports whether an HTTP response looks like a real web
// page rather than a JSON/plain-text API (which the Demios backend returns).
func isDevServerResponse(resp *http.Response) bool {
	if resp.StatusCode >= 500 {
		return false
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(ct, "text/html") {
		return true
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "<!doctype") || strings.Contains(lower, "<html")
}

func waitForServer(ctx context.Context, instance *ServerInstance, timeoutSec int, sm *ServerManager) string {
	httpClient := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	lastPortScan := time.Time{}

	// probe returns url when it is reachable with a non-5xx status.
	probe := func(url string) string {
		resp, err := httpClient.Get(url)
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return url
			}
		}
		return ""
	}

	for time.Now().Before(deadline) {
		instance.mu.Lock()
		url := instance.URL
		port := instance.Port
		instance.mu.Unlock()

		if url != "" {
			if ok := probe(url); ok != "" {
				return ok
			}
		}

		// A dev server may bind only one loopback family (Vite uses ::1).
		// Probe both host forms of a parsed port so an IPv6-only bind still
		// becomes reachable via "localhost".
		if port > 0 {
			if ok := probe(fmt.Sprintf("http://127.0.0.1:%d", port)); ok != "" {
				return ok
			}
			if ok := probe(fmt.Sprintf("http://localhost:%d", port)); ok != "" {
				return ok
			}
		}

		// Fall back to the port scan after a generous grace period so a
		// fast-printing server (vite) wins via output parsing. This runs even
		// when a URL was parsed but is not yet reachable, so an IPv6-only bind
		// that "localhost" cannot reach still gets caught by the scan.
		if time.Since(lastPortScan) > 10*time.Second {
			lastPortScan = time.Now()
			if existing := sm.findExistingDevServer(); existing != "" {
				return existing
			}
		}

		select {
		case <-ctx.Done():
			return ""
		case <-time.After(500 * time.Millisecond):
		}
	}

	if instance.Port == 0 {
		if existing := sm.findExistingDevServer(); existing != "" {
			return existing
		}
	}

	return ""
}
