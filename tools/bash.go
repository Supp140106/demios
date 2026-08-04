package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/invopop/jsonschema"
)

type BashArgs struct {
	Command string `json:"command" jsonschema:"title=Command,description=Shell command to execute (bash syntax on Linux/macOS, PowerShell syntax on Windows)"`
	Workdir string `json:"workdir,omitempty" jsonschema:"title=Workdir,description=Working directory for the command"`
	Timeout int    `json:"timeout,omitempty" jsonschema:"title=Timeout,description=Timeout in seconds (default 120, max 600)"`
}

type bashResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

func shellCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		return "powershell", []string{"-NoProfile", "-NonInteractive", "-Command"}
	}
	return "bash", []string{"-c"}
}

var Bash = Tool{
	ID:          "Bash",
	Description: "Execute a shell command. Runs via bash on Linux/macOS, PowerShell on Windows. Use this for running tests, git operations, building, installing packages, and any terminal command.",

	Schema: jsonschema.Reflect(&BashArgs{}),

	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args BashArgs
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		if args.Command == "" {
			return ExecuteResult{}, fmt.Errorf("command is required")
		}

		timeout := 120
		if args.Timeout > 0 {
			timeout = args.Timeout
		}
		if timeout > 600 {
			timeout = 600
		}

		shell, shellArgs := shellCommand()
		cmd := exec.Command(shell, append(shellArgs, args.Command)...)

		if devServerReason, ok := looksLikeDevServerCommand(args.Command); ok {
			return ExecuteResult{}, fmt.Errorf(
				"blocked: %q looks like a long-running dev-server command (%s). "+
					"Do NOT start dev servers through Bash — it blocks until timeout and cannot report a reliable port. "+
					"Use the StartServer tool instead: StartServer(command=%q, workdir=<project dir>). "+
					"It streams the output in real time, waits until the server is ready, and returns the authoritative URL/port. "+
					"Then call TestWebsite(url=<that URL>, prompt=<detailed test instructions>) to test the app.",
				args.Command, devServerReason, args.Command)
		}

		workdir := args.Workdir
		if workdir == "" {
			workdir = WorkspaceFrom(ctx)
		} else if !filepath.IsAbs(workdir) {
			if ws := WorkspaceFrom(ctx); ws != "" {
				workdir = filepath.Join(ws, workdir)
			}
		}

		if workdir != "" {
			abs, err := filepath.Abs(workdir)
			if err != nil {
				return ExecuteResult{}, fmt.Errorf("invalid workdir: %w", err)
			}
			info, err := os.Stat(abs)
			if err != nil {
				return ExecuteResult{}, fmt.Errorf("workdir does not exist: %w", err)
			}
			if !info.IsDir() {
				return ExecuteResult{}, fmt.Errorf("workdir is not a directory: %s", abs)
			}
			cmd.Dir = abs
		}

		var stdout, stderr strings.Builder
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		start := time.Now()

		// Put the whole process tree in one group so a timeout can kill the
		// server AND its children — otherwise orphaned Vite/Next processes keep
		// running and re-take the port (the "it went to 3000 several times" bug).
		setProcessGroup(cmd)

		err := cmd.Start()
		if err != nil {
			return ExecuteResult{}, fmt.Errorf("failed to start command: %w", err)
		}

		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		select {
		case <-time.After(time.Duration(timeout) * time.Second):
			if cmd.Process != nil {
				killProcessTree(cmd.Process.Pid)
			}
			<-done
			out := stdout.String() + "\n[ERROR: Command timed out after " + fmt.Sprintf("%d seconds", timeout) + "]\n" + stderr.String()
			// Safety net: if the timed-out command was actually a dev server
			// (it printed a URL banner before the timeout), tell the agent the
			// real URL it bound to so it never reports the wrong port.
			if detected := extractServerURLFromOutput(stdout.String() + "\n" + stderr.String()); detected != "" {
				out += fmt.Sprintf("\n[server] This command started a long-running dev server. Detected URL: %s. "+
					"The Bash process was terminated on timeout. Use StartServer to keep it running, then pass this URL to TestWebsite.", detected)
			}
			return ExecuteResult{
				Title:  fmt.Sprintf("Command timed out (%ds)", timeout),
				Output: out,
				Metadata: map[string]any{
					"command":   args.Command,
					"exit_code": -1,
					"duration":  time.Since(start).String(),
					"timed_out": true,
				},
			}, nil

		case err := <-done:
			duration := time.Since(start)
			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					exitCode = -1
				}
			}

			out := stdout.String()
			errOut := stderr.String()

			if exitCode != 0 {
				out += "\n[EXIT " + fmt.Sprintf("%d", exitCode) + "]\n" + errOut
			}

			title := fmt.Sprintf("$ %s", args.Command)
			if len(title) > 80 {
				title = title[:77] + "..."
			}

			return ExecuteResult{
				Title:  title,
				Output: out,
				Metadata: map[string]any{
					"command":   args.Command,
					"exit_code": exitCode,
					"duration":  duration.String(),
					"timed_out": false,
				},
			}, nil
		}
	},
}

// devServerCommandPatterns matches commands that start long-running dev
// servers. These must never be run through Bash (they block until timeout and
// the port the model would read from the banner is unreliable); they belong in
// the StartServer tool, which streams output and returns the authoritative URL.
var devServerCommandPatterns = []*regexp.Regexp{
	// npm/pnpm/yarn/bun run dev|start|serve, or the shorthand forms
	regexp.MustCompile(`(?i)^(npm|pnpm|yarn|bun)\s+(run\s+)?(dev|start|serve)(\s|$)`),
	// well-known framework dev commands
	regexp.MustCompile(`(?i)^(npx)\s+(next|vite|nuxt|ng|astro|webpack|parcel|gatsby|react-scripts|serve)(\s|$)`),
	regexp.MustCompile(`(?i)^(next|vite|nuxt|ng|astro|svelte-kit|gatsby|webpack|parcel)\s+dev(\s|$)`),
	regexp.MustCompile(`(?i)^vite(\s|$)`),
	regexp.MustCompile(`(?i)^ng\s+serve(\s|$)`),
	regexp.MustCompile(`(?i)^react-scripts\s+start(\s|$)`),
	regexp.MustCompile(`(?i)^vue-cli-service\s+serve(\s|$)`),
	regexp.MustCompile(`(?i)^(flask\s+run|uvicorn\s+\S+)(\s|$)`),
	// simple static servers
	regexp.MustCompile(`(?i)^python[23]?\s+-m\s+(http\.server|SimpleHTTPServer)(\s|$)`),
	regexp.MustCompile(`(?i)^(npx\s+)?serve\s+-s(\s|$)`),
}

// looksLikeDevServerCommand reports whether a shell command starts a
// long-running dev server, along with the reason. `go run` and other ambiguous
// commands are intentionally NOT blocked; they are handled by the timeout
// safety-net (kill-tree + URL annotation) instead.
func looksLikeDevServerCommand(command string) (reason string, ok bool) {
	first := strings.Fields(command)
	if len(first) == 0 {
		return "", false
	}
	// Strip common shell prefixes that don't change what the command is.
	for first[0] == "sudo" || first[0] == "nohup" || first[0] == "time" {
		first = first[1:]
		if len(first) == 0 {
			return "", false
		}
	}
	line := strings.Join(first, " ")
	for _, re := range devServerCommandPatterns {
		if re.MatchString(line) {
			return re.String(), true
		}
	}
	return "", false
}

// extractServerURLFromOutput scans captured dev-server output for the URL the
// server actually bound to. It reuses the authoritative parsing from the
// ServerManager (URL lines win, conflict lines are ignored), so the reported
// port is the real one even when the banner also mentions an in-use port.
func extractServerURLFromOutput(output string) string {
	scanner := bufio.NewScanner(strings.NewReader(output))
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	best := ""
	for scanner.Scan() {
		port, fromURL, conflict := detectPortFromLine(stripANSI(scanner.Text()))
		if conflict || port == 0 {
			continue
		}
		if fromURL {
			best = fmt.Sprintf("http://127.0.0.1:%d", port)
		} else if best == "" {
			best = fmt.Sprintf("http://127.0.0.1:%d", port)
		}
	}
	return best
}
