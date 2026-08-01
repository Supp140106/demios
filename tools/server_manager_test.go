package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func writeTestScript(t *testing.T, dir, content string) string {
	t.Helper()
	ext := ".ps1"
	if runtime.GOOS != "windows" {
		ext = ".sh"
	}
	scriptPath := filepath.Join(dir, "test_server"+ext)
	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
		t.Fatal(err)
	}
	return scriptPath
}

func TestParsePortFromLine(t *testing.T) {
	tests := []struct {
		line string
		want int
	}{
		{"Local: http://localhost:5173/", 5173},
		{"http://127.0.0.1:3000", 3000},
		{"PORT=8080", 8080},
		{"Listening on port 4000", 4000},
		{"no port here", 0},
		{"", 0},
		{"Local: http://[::]:5174/", 5174},
		{"Local: http://[0.0.0.0]:5175/", 5175},
		{"Server listening on http://[::1]:3001", 3001},
		{"  VITE v8.1.5  ready in 500 ms", 0},
		{"➜  Local:   http://localhost:5176/", 5176},
		{"Port 5173 is in use, trying another one...", 0},
		{"Port 5173 is already in use. Using 5174 instead.", 0},
		{"Port 5173 is occupied by another process", 0},
	}

	for _, tt := range tests {
		got := parsePortFromLine(tt.line)
		if got != tt.want {
			t.Errorf("parsePortFromLine(%q) = %d, want %d", tt.line, got, tt.want)
		}
	}
}

func TestRecordLinePrefersURL(t *testing.T) {
	inst := &ServerInstance{}

	inst.recordLine(&inst.stdout, "Port 5173 is in use, trying another one...")
	if inst.Port != 0 || inst.URL != "" {
		t.Fatalf("conflict line should not set a port, got port=%d url=%q", inst.Port, inst.URL)
	}
	if !inst.sawConflict {
		t.Error("sawConflict should be true after a conflict line")
	}

	inst.recordLine(&inst.stdout, "➜  Local:   http://localhost:5174/")
	if inst.Port != 5174 {
		t.Errorf("URL line should set port 5174, got %d", inst.Port)
	}
	if inst.URL != "http://127.0.0.1:5174" {
		t.Errorf("URL = %q, want http://127.0.0.1:5174", inst.URL)
	}
	if !inst.portFromURL {
		t.Error("portFromURL should be true after a URL line")
	}
}

func TestRecordLineKeywordThenURLOverrides(t *testing.T) {
	inst := &ServerInstance{}

	inst.recordLine(&inst.stdout, "Listening on port 3000")
	if inst.Port != 3000 {
		t.Fatalf("keyword line should set port 3000, got %d", inst.Port)
	}

	inst.recordLine(&inst.stdout, "Local: http://localhost:5173/")
	if inst.Port != 5173 {
		t.Errorf("URL line should override keyword guess to 5173, got %d", inst.Port)
	}
}

func TestResolveURLWithLLMEmptyOutputSkipped(t *testing.T) {
	sm := NewServerManager()
	called := false
	sm.SetURLResolver(func(ctx context.Context, output string) (string, error) {
		called = true
		return "", nil
	})

	inst := &ServerInstance{}
	if got := sm.resolveURLWithLLM(context.Background(), inst, "http://127.0.0.1:5173"); got != "" {
		t.Errorf("expected empty result for empty output, got %q", got)
	}
	if called {
		t.Error("resolver should not be called when output is empty")
	}
}

func TestResolveURLWithLLMUsesResolvedReachableURL(t *testing.T) {
	port := freePortInDevRange(t)
	startTestHTTPServer(t, port, "text/html", "<h1>dev</h1>")

	sm := NewServerManager()
	resolvedURL := fmt.Sprintf("http://localhost:%d", port)
	sm.SetURLResolver(func(ctx context.Context, output string) (string, error) {
		return resolvedURL, nil
	})

	inst := &ServerInstance{}
	inst.stdout.WriteString("Port 5173 is in use, trying another one...\nLocal: http://localhost:5173/\n")

	got := sm.resolveURLWithLLM(context.Background(), inst, "http://127.0.0.1:5173")
	if got != resolvedURL {
		t.Errorf("resolveURLWithLLM = %q, want %q", got, resolvedURL)
	}
}

func TestIsDevServerResponse(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
		contentType string
		body       string
		want       bool
	}{
		{"html content type", 200, "text/html; charset=utf-8", "<html>hi</html>", true},
		{"doctype body", 200, "application/octet-stream", "<!doctype html><title>a</title>", true},
		{"json api", 200, "application/json", `{"id":1}`, false},
		{"plain text 404", 404, "text/plain", "404 page not found", false},
		{"server error", 500, "text/html", "<html>boom</html>", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: c.statusCode,
				Header:     http.Header{"Content-Type": []string{c.contentType}},
				Body:       io.NopCloser(strings.NewReader(c.body)),
			}
			if got := isDevServerResponse(resp); got != c.want {
				t.Errorf("isDevServerResponse = %v, want %v", got, c.want)
			}
		})
	}
}

// freePortInDevRange finds a port inside the auto-detection scan range
// (5173-5200) that is not excluded by default and is not already in use on
// loopback (IPv4 or IPv6), so scan tests are deterministic.
func freePortInDevRange(t *testing.T) int {
	t.Helper()
	for port := 5173; port <= 5200; port++ {
		if isDefaultExcluded(port) {
			continue
		}
		if isPortInUse(port) {
			continue
		}
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue
		}
		ln.Close()
		return port
	}
	t.Fatal("no free non-excluded port in dev range 5173-5200")
	return 0
}

func isDefaultExcluded(port int) bool {
	for _, p := range defaultExcludedPorts {
		if p == port {
			return true
		}
	}
	return false
}

func startTestHTTPServer(t *testing.T, port int, contentType, body string) {
	t.Helper()
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("listen :%d: %v", port, err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.Write([]byte(body))
	})}
	go srv.Serve(ln)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	})
}

func TestFindExistingDevServerSkipsNonHTML(t *testing.T) {
	port := freePortInDevRange(t)
	startTestHTTPServer(t, port, "text/plain", "plain text, not a web page")

	sm := NewServerManager()
	got := sm.findExistingDevServer()
	wantURL := fmt.Sprintf("http://localhost:%d", port)
	if got == wantURL {
		t.Errorf("scan picked up non-HTML responder on port %d: %s", port, got)
	}
	t.Logf("scan result (should not be %s): %q", wantURL, got)}

func TestFindExistingDevServerExcludePort(t *testing.T) {
	port := freePortInDevRange(t)
	startTestHTTPServer(t, port, "text/html", "<h1>dev</h1>")

	sm := NewServerManager()
	sm.ExcludePort(port)

	if !sm.isExcluded(port) {
		t.Fatalf("expected port %d to be marked excluded", port)
	}
	wantURL := fmt.Sprintf("http://localhost:%d", port)
	if got := sm.findExistingDevServer(); got == wantURL {
		t.Errorf("excluded port %d was still picked up: %s", port, got)
	}
}

func TestExcludePortIgnoresInvalid(t *testing.T) {
	sm := NewServerManager()
	sm.ExcludePort(0)
	sm.ExcludePort(-5)
	if sm.isExcluded(0) || sm.isExcluded(-5) {
		t.Error("invalid ports should not be recorded as excluded")
	}
}

func TestNewServerManagerDefaults(t *testing.T) {
	sm := NewServerManager()
	// Generalized: nothing is hardcoded by default. Only ports the user (or
	// the backend self-exclusion) adds should ever be excluded.
	for _, port := range []int{4096, 5173, 5174, 8080} {
		if sm.isExcluded(port) {
			t.Errorf("port %d should not be excluded by default", port)
		}
	}
}

func TestExcludePortsEnv(t *testing.T) {
	t.Setenv("DEMIOS_EXCLUDE_PORTS", "7000, 8001")
	sm := NewServerManager()
	if !sm.isExcluded(7000) || !sm.isExcluded(8001) {
		t.Error("env-configured ports should be excluded")
	}
	if sm.isExcluded(8080) {
		t.Error("non-env port should not be excluded")
	}
}

func TestExcludedPortsAccessor(t *testing.T) {
	sm := NewServerManager()
	sm.ExcludePorts(4000, 5000)
	got := sm.ExcludedPorts()
	seen := map[int]bool{}
	for _, p := range got {
		seen[p] = true
	}
	if !seen[4000] || !seen[5000] {
		t.Errorf("ExcludedPorts() = %v, want it to include 4000 and 5000", got)
	}
}

func TestExcludePortsBatch(t *testing.T) {
	sm := NewServerManager()
	sm.ExcludePorts(3001, 3002)
	for _, port := range []int{3001, 3002} {
		if !sm.isExcluded(port) {
			t.Errorf("batch-excluded port %d was not registered", port)
		}
	}
}


func TestFindFreePort(t *testing.T) {
	sm := NewServerManager()
	port := sm.findFreePort()
	if port == 0 {
		t.Error("findFreePort returned 0")
	}
	if sm.isExcluded(port) {
		t.Errorf("findFreePort returned excluded port %d", port)
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		t.Errorf("Port %d is not available: %v", port, err)
	}
	ln.Close()

	t.Logf("findFreePort returned %d", port)
}

func TestURLReachableRejectsExcluded(t *testing.T) {
	sm := NewServerManager()
	sm.ExcludePort(4096)
	if sm.urlReachable("http://localhost:4096") {
		t.Error("urlReachable accepted excluded port 4096")
	}
	if sm.urlReachable("not a url") {
		t.Error("urlReachable accepted invalid URL")
	}
	if sm.urlReachable("http://127.0.0.1:1") {
		t.Error("urlReachable accepted unreachable port 1")
	}
}

func TestBrowserURLBlocked(t *testing.T) {
	ctx := WithBrowserExcludedPorts(context.Background(), []int{4096, 5173})
	for _, u := range []string{"http://localhost:5173", "http://localhost:5173/", "http://127.0.0.1:4096", "http://localhost:4096/path"} {
		if !browserURLBlocked(ctx, u) {
			t.Errorf("browserURLBlocked(%q) = false, want true", u)
		}
	}
	for _, u := range []string{"http://localhost:5174", "http://localhost:3000", "http://127.0.0.1:8080", "not-a-url"} {
		if browserURLBlocked(ctx, u) {
			t.Errorf("browserURLBlocked(%q) = true, want false", u)
		}
	}
	// With no exclusions configured, nothing is blocked (generalized behavior).
	if browserURLBlocked(context.Background(), "http://localhost:5173") {
		t.Error("browserURLBlocked blocked a port with no exclusions configured")
	}
}

func TestStartServerDetectsPortFromStdout(t *testing.T) {
	sm := NewServerManager()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dir := t.TempDir()
	httpScript := `
$listener = New-Object System.Net.HttpListener
$listener.Prefixes.Add("http://localhost:16300/")
$listener.Start()
Write-Host "Local: http://localhost:16300/"
while ($listener.IsListening) {
    $ctx = $listener.GetContext()
    $bytes = [System.Text.Encoding]::UTF8.GetBytes("hello")
    $ctx.Response.ContentType = "text/plain"
    $ctx.Response.ContentLength64 = $bytes.Length
    $ctx.Response.OutputStream.Write($bytes, 0, $bytes.Length)
    $ctx.Response.Close()
}
`
	scriptPath := writeTestScript(t, dir, httpScript)
	cmd := fmt.Sprintf("powershell -NoProfile -NonInteractive -File %s", scriptPath)

	instance, err := sm.StartServer(ctx, dir, cmd, 0)
	if err != nil {
		t.Fatalf("StartServer failed: %v", err)
	}
	defer sm.StopServer(instance.ID)

	t.Logf("Detected: port=%d url=%s status=%s", instance.Port, instance.URL, instance.Status)

	if instance.Status != "running" {
		t.Errorf("Status = %q, want running", instance.Status)
	}
	if instance.URL == "" {
		t.Error("URL is empty")
	}
	if instance.Port == 0 {
		t.Error("Port is 0")
	}

	resp, err := http.Get(instance.URL)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("HTTP GET -> %d", resp.StatusCode)
}

func TestStartServerAutoDetectCommand(t *testing.T) {
	sm := NewServerManager()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dir := t.TempDir()
	httpScript := `
$listener = New-Object System.Net.HttpListener
$listener.Prefixes.Add("http://localhost:15201/")
$listener.Start()
Write-Host "Server listening on http://localhost:15201/"
while ($listener.IsListening) {
    $ctx = $listener.GetContext()
    $bytes = [System.Text.Encoding]::UTF8.GetBytes("<h1>works</h1>")
    $ctx.Response.ContentType = "text/html"
    $ctx.Response.ContentLength64 = $bytes.Length
    $ctx.Response.OutputStream.Write($bytes, 0, $bytes.Length)
    $ctx.Response.Close()
}
`
	scriptPath := writeTestScript(t, dir, httpScript)
	cmd := fmt.Sprintf("powershell -NoProfile -NonInteractive -File %s", scriptPath)

	instance, err := sm.StartServer(ctx, dir, cmd, 0)
	if err != nil {
		t.Fatalf("StartServer failed: %v", err)
	}
	defer sm.StopServer(instance.ID)

	t.Logf("Server: id=%s pid=%d port=%d url=%s status=%s",
		instance.ID, instance.PID, instance.Port, instance.URL, instance.Status)

	if instance.Status != "running" {
		t.Errorf("Status = %q, want running", instance.Status)
	}
	if instance.URL == "" {
		t.Error("URL is empty")
	}

	resp, err := http.Get(instance.URL)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("HTTP GET %s -> %d", instance.URL, resp.StatusCode)
}

func TestGetServerStatus(t *testing.T) {
	sm := NewServerManager()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dir := t.TempDir()
	httpScript := `
$listener = New-Object System.Net.HttpListener
$listener.Prefixes.Add("http://localhost:15400/")
$listener.Start()
Write-Host "Listening on http://localhost:15400/"
while ($listener.IsListening) {
    $ctx = $listener.GetContext()
    $ctx.Response.StatusCode = 200
    $ctx.Response.Close()
}
`
	scriptPath := writeTestScript(t, dir, httpScript)
	cmd := fmt.Sprintf("powershell -NoProfile -NonInteractive -File %s", scriptPath)

	instance, err := sm.StartServer(ctx, dir, cmd, 0)
	if err != nil {
		t.Fatalf("StartServer failed: %v", err)
	}
	defer sm.StopServer(instance.ID)

	got, err := sm.GetServer(instance.ID)
	if err != nil {
		t.Fatalf("GetServer failed: %v", err)
	}
	if got.ID != instance.ID {
		t.Errorf("ID = %q, want %q", got.ID, instance.ID)
	}
	if got.Status != "running" {
		t.Errorf("Status = %q, want running", got.Status)
	}

	servers := sm.ListServers()
	if len(servers) != 1 {
		t.Errorf("ListServers returned %d, want 1", len(servers))
	}

	t.Logf("GetServerStatus OK: id=%s status=%s url=%s", got.ID, got.Status, got.URL)
}

func TestStopServer(t *testing.T) {
	sm := NewServerManager()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dir := t.TempDir()
	httpScript := `
$listener = New-Object System.Net.HttpListener
$listener.Prefixes.Add("http://localhost:15500/")
$listener.Start()
Write-Host "Listening on http://localhost:15500/"
while ($listener.IsListening) {
    $ctx = $listener.GetContext()
    $ctx.Response.StatusCode = 200
    $ctx.Response.Close()
}
`
	scriptPath := writeTestScript(t, dir, httpScript)
	cmd := fmt.Sprintf("powershell -NoProfile -NonInteractive -File %s", scriptPath)

	instance, err := sm.StartServer(ctx, dir, cmd, 0)
	if err != nil {
		t.Fatalf("StartServer failed: %v", err)
	}

	pid := instance.PID
	t.Logf("PID before stop: %d", pid)

	if err := sm.StopServer(instance.ID); err != nil {
		t.Fatalf("StopServer failed: %v", err)
	}

	time.Sleep(1 * time.Second)

	servers := sm.ListServers()
	for _, s := range servers {
		if s.PID == pid {
			t.Error("Stopped server still in list")
		}
	}

	t.Log("Server stopped and removed from list")
}

func TestFindByWorkdir(t *testing.T) {
	sm := NewServerManager()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dir := t.TempDir()
	httpScript := `
$listener = New-Object System.Net.HttpListener
$listener.Prefixes.Add("http://localhost:15600/")
$listener.Start()
Write-Host "Listening on http://localhost:15600/"
while ($listener.IsListening) {
    $ctx = $listener.GetContext()
    $ctx.Response.StatusCode = 200
    $ctx.Response.Close()
}
`
	scriptPath := writeTestScript(t, dir, httpScript)
	cmd := fmt.Sprintf("powershell -NoProfile -NonInteractive -File %s", scriptPath)

	instance, err := sm.StartServer(ctx, dir, cmd, 0)
	if err != nil {
		t.Fatalf("StartServer failed: %v", err)
	}
	defer sm.StopServer(instance.ID)

	found := sm.FindByWorkdir(dir)
	if found == nil {
		t.Fatal("FindByWorkdir returned nil")
	}
	if found.ID != instance.ID {
		t.Errorf("FindByWorkdir returned %q, want %q", found.ID, instance.ID)
	}

	notFound := sm.FindByWorkdir("/nonexistent/path/that/does/not/exist")
	if notFound != nil {
		t.Error("FindByWorkdir should return nil for nonexistent dir")
	}
}

func TestDuplicateServerSameWorkdir(t *testing.T) {
	sm := NewServerManager()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dir := t.TempDir()
	httpScript := `
$listener = New-Object System.Net.HttpListener
$listener.Prefixes.Add("http://localhost:15700/")
$listener.Start()
Write-Host "Listening on http://localhost:15700/"
while ($listener.IsListening) {
    $ctx = $listener.GetContext()
    $ctx.Response.StatusCode = 200
    $ctx.Response.Close()
}
`
	scriptPath := writeTestScript(t, dir, httpScript)
	cmd := fmt.Sprintf("powershell -NoProfile -NonInteractive -File %s", scriptPath)

	inst1, err := sm.StartServer(ctx, dir, cmd, 0)
	if err != nil {
		t.Fatalf("First StartServer failed: %v", err)
	}

	inst2, err := sm.StartServer(ctx, dir, cmd, 0)
	if err != nil {
		t.Fatalf("Second StartServer failed: %v", err)
	}

	if inst1.ID != inst2.ID {
		t.Errorf("Expected same server instance, got %q and %q", inst1.ID, inst2.ID)
	}

	sm.StopAll()
}

func TestStopAll(t *testing.T) {
	sm := NewServerManager()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	makeCmd := func(dir string, port int) string {
		httpScript := fmt.Sprintf(`
$listener = New-Object System.Net.HttpListener
$listener.Prefixes.Add("http://localhost:%d/")
$listener.Start()
Write-Host "Listening on http://localhost:%d/"
while ($listener.IsListening) {
    $ctx = $listener.GetContext()
    $ctx.Response.StatusCode = 200
    $ctx.Response.Close()
}
`, port, port)
		scriptPath := writeTestScript(t, dir, httpScript)
		return fmt.Sprintf("powershell -NoProfile -NonInteractive -File %s", scriptPath)
	}

	dir1 := t.TempDir()
	dir2 := t.TempDir()
	sm.StartServer(ctx, dir1, makeCmd(dir1, 15801), 0)
	sm.StartServer(ctx, dir2, makeCmd(dir2, 15802), 0)

	if len(sm.ListServers()) != 2 {
		t.Fatalf("Expected 2 servers, got %d", len(sm.ListServers()))
	}

	sm.StopAll()

	if len(sm.ListServers()) != 0 {
		t.Errorf("Expected 0 servers after StopAll, got %d", len(sm.ListServers()))
	}
}

func TestReusesExistingServer(t *testing.T) {
	sm := NewServerManager()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dir := t.TempDir()
	httpScript := `
$listener = New-Object System.Net.HttpListener
$listener.Prefixes.Add("http://localhost:15900/")
$listener.Start()
Write-Host "Listening on http://localhost:15900/"
while ($listener.IsListening) {
    $ctx = $listener.GetContext()
    $ctx.Response.StatusCode = 200
    $ctx.Response.Close()
}
`
	scriptPath := writeTestScript(t, dir, httpScript)
	cmd := fmt.Sprintf("powershell -NoProfile -NonInteractive -File %s", scriptPath)

	inst1, err := sm.StartServer(ctx, dir, cmd, 0)
	if err != nil {
		t.Fatalf("First StartServer failed: %v", err)
	}

	inst2, err := sm.StartServer(ctx, dir, cmd, 0)
	if err != nil {
		t.Fatalf("Second StartServer failed: %v", err)
	}

	if inst1.ID != inst2.ID {
		t.Errorf("Expected reuse, got %q and %q", inst1.ID, inst2.ID)
	}

	t.Logf("Reused server: id=%s url=%s", inst1.ID, inst1.URL)
	sm.StopAll()
}

// TestEnablePersistenceRoundTrip verifies the registry file is written and can
// be re-enabled without errors.
func TestEnablePersistenceRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "servers.json")

	sm := NewServerManager()
	if err := sm.EnablePersistence(path); err != nil {
		t.Fatalf("EnablePersistence: %v", err)
	}
	sm.persist()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("registry file was not written: %v", err)
	}

	sm2 := NewServerManager()
	if err := sm2.EnablePersistence(path); err != nil {
		t.Fatalf("EnablePersistence reload: %v", err)
	}
	if len(sm2.ListServers()) != 0 {
		t.Errorf("reloaded manager should have no servers, got %d", len(sm2.ListServers()))
	}
}

// TestPersistenceAcrossManagers verifies a running server survives into a new
// manager (the headless CLI harness model) and can be stopped through it.
func TestPersistenceAcrossManagers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "servers.json")

	sm := NewServerManager()
	if err := sm.EnablePersistence(path); err != nil {
		t.Fatalf("EnablePersistence: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dir := t.TempDir()
	httpScript := `
$listener = New-Object System.Net.HttpListener
$listener.Prefixes.Add("http://localhost:16600/")
$listener.Start()
Write-Host "Listening on http://localhost:16600/"
while ($listener.IsListening) {
    $ctx = $listener.GetContext()
    $bytes = [System.Text.Encoding]::UTF8.GetBytes("<h1>persist</h1>")
    $ctx.Response.ContentType = "text/html"
    $ctx.Response.ContentLength64 = $bytes.Length
    $ctx.Response.OutputStream.Write($bytes, 0, $bytes.Length)
    $ctx.Response.Close()
}
`
	scriptPath := writeTestScript(t, dir, httpScript)
	cmd := fmt.Sprintf("powershell -NoProfile -NonInteractive -File %s", scriptPath)

	inst, err := sm.StartServer(ctx, dir, cmd, 0)
	if err != nil {
		t.Fatalf("StartServer failed: %v", err)
	}

	sm2 := NewServerManager()
	if err := sm2.EnablePersistence(path); err != nil {
		t.Fatalf("EnablePersistence reload: %v", err)
	}
	servers := sm2.ListServers()
	if len(servers) != 1 {
		sm.StopServer(inst.ID)
		t.Fatalf("reloaded manager sees %d servers, want 1", len(servers))
	}
	reloaded, err := sm2.GetServer(inst.ID)
	if err != nil {
		sm.StopServer(inst.ID)
		t.Fatalf("GetServer: %v", err)
	}
	if reloaded.URL != inst.URL {
		sm.StopServer(inst.ID)
		t.Errorf("reloaded URL = %q, want %q", reloaded.URL, inst.URL)
	}

	if err := sm2.StopServer(inst.ID); err != nil {
		sm.StopServer(inst.ID)
		t.Fatalf("stop via reloaded manager failed: %v", err)
	}
	if _, err := os.Stat(path); err == nil {
		data, _ := os.ReadFile(path)
		if strings.Contains(string(data), inst.ID) {
			t.Errorf("stopped server still present in registry: %s", string(data))
		}
	}
}

// TestTestWebsiteFallsBackToRunningServer verifies that TestWebsite ignores a
// dead/excluded URL and uses an already-running server instead.
func TestTestWebsiteFallsBackToRunningServer(t *testing.T) {
	sm := NewServerManager()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dir := t.TempDir()
	httpScript := `
$listener = New-Object System.Net.HttpListener
$listener.Prefixes.Add("http://localhost:16400/")
$listener.Start()
Write-Host "Listening on http://localhost:16400/"
while ($listener.IsListening) {
    $ctx = $listener.GetContext()
    $bytes = [System.Text.Encoding]::UTF8.GetBytes("<!doctype html><html><body>hello</body></html>")
    $ctx.Response.ContentType = "text/html"
    $ctx.Response.ContentLength64 = $bytes.Length
    $ctx.Response.OutputStream.Write($bytes, 0, $bytes.Length)
    $ctx.Response.Close()
}
`
	scriptPath := writeTestScript(t, dir, httpScript)
	cmd := fmt.Sprintf("powershell -NoProfile -NonInteractive -File %s", scriptPath)

	inst, err := sm.StartServer(ctx, dir, cmd, 0)
	if err != nil {
		t.Fatalf("StartServer failed: %v", err)
	}
	defer sm.StopServer(inst.ID)

	var gotURL string
	tool := MakeTestWebsiteTool(sm, func(ctx context.Context, url string, prompt string) (string, error) {
		gotURL = url
		return "report ok", nil
	})

	raw, _ := json.Marshal(map[string]string{
		"url":    "http://localhost:1",
		"prompt": "test it",
	})
	if _, err := tool.Execute(ctx, raw); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if gotURL == "http://localhost:1" {
		t.Errorf("TestWebsite used dead URL %q, want running server %s", gotURL, inst.URL)
	}
	if gotURL != inst.URL {
		t.Errorf("TestWebsite used %q, want running server %s", gotURL, inst.URL)
	}
	t.Logf("TestWebsite fell back to running server: %s", gotURL)
}
