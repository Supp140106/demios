package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/invopop/jsonschema"
)

type BashArgs struct {
	Command string `json:"command" jsonschema:"title=Command,description=Shell command to execute (PowerShell syntax)"`
	Workdir string `json:"workdir,omitempty" jsonschema:"title=Workdir,description=Working directory for the command"`
	Timeout int    `json:"timeout,omitempty" jsonschema:"title=Timeout,description=Timeout in seconds (default 120, max 600)"`
}

type bashResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

var Bash = Tool{
	ID:          "Bash",
	Description: "Execute a shell command. Runs via PowerShell. Use this for running tests, git operations, building, installing packages, and any terminal command.",

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

		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", args.Command)

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
			cmd.Process.Kill()
			<-done
			return ExecuteResult{
				Title:  fmt.Sprintf("Command timed out (%ds)", timeout),
				Output: stdout.String() + "\n[ERROR: Command timed out after " + fmt.Sprintf("%d seconds", timeout) + "]\n" + stderr.String(),
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
