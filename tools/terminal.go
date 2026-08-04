package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/invopop/jsonschema"
)

// ---------------------------------------------------------------------------
// Terminal session manager (similar to BrowserSession)
// ---------------------------------------------------------------------------

type TerminalSession struct {
	ID    string
	Cmd   *exec.Cmd
	PTY   *os.File
	Cols  uint16
	Rows  uint16
	mu    sync.Mutex
	closed bool
	output *bytes.Buffer
}

func NewTerminalSession(id, shell, workdir string) (*TerminalSession, error) {
	if shell == "" {
		shell = defaultShell()
	}

	cmd := exec.Command(shell)
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptm, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start PTY: %w", err)
	}

	// Set initial size
	if err := pty.Setsize(ptm, &pty.Winsize{Cols: 80, Rows: 24}); err != nil {
		log.Printf("[terminal] set size failed: %v", err)
	}

	ts := &TerminalSession{
		ID:     id,
		Cmd:    cmd,
		PTY:    ptm,
		Cols:   80,
		Rows:   24,
		output: bytes.NewBuffer(nil),
	}

	// Background reader that captures PTY output
	go ts.readLoop()

	return ts, nil
}

func (ts *TerminalSession) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := ts.PTY.Read(buf)
		if n > 0 {
			ts.mu.Lock()
			ts.output.Write(buf[:n])
			ts.mu.Unlock()
		}
		if err != nil {
			break
		}
	}
}

// Write sends raw input to the PTY.
func (ts *TerminalSession) Write(data []byte) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.closed {
		return fmt.Errorf("terminal session is closed")
	}
	_, err := ts.PTY.Write(data)
	return err
}

// ReadOutput returns all buffered output since last read and clears the buffer.
func (ts *TerminalSession) ReadOutput() string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	out := ts.output.String()
	ts.output.Reset()
	return out
}

// Resize changes the PTY dimensions.
func (ts *TerminalSession) Resize(cols, rows uint16) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.closed {
		return fmt.Errorf("terminal session is closed")
	}
	ts.Cols = cols
	ts.Rows = rows
	return pty.Setsize(ts.PTY, &pty.Winsize{Cols: cols, Rows: rows})
}

// Close terminates the PTY and the shell process.
func (ts *TerminalSession) Close() error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.closed {
		return nil
	}
	ts.closed = true
	var err error
	if ts.PTY != nil {
		err = ts.PTY.Close()
	}
	if ts.Cmd != nil && ts.Cmd.Process != nil {
		ts.Cmd.Process.Kill()
		ts.Cmd.Wait()
	}
	return err
}

// IsRunning reports whether the shell process is still alive.
func (ts *TerminalSession) IsRunning() bool {
	if ts.Cmd == nil || ts.Cmd.Process == nil {
		return false
	}
	return ts.Cmd.ProcessState == nil
}

func defaultShell() string {
	if runtime.GOOS == "windows" {
		return "powershell"
	}
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh
	}
	return "/bin/bash"
}

// ---------------------------------------------------------------------------
// Terminal manager — holds multiple concurrent sessions
// ---------------------------------------------------------------------------

type terminalManagerKey struct{}

type TerminalManager struct {
	mu       sync.Mutex
	sessions map[string]*TerminalSession
	counter  int
}

func NewTerminalManager() *TerminalManager {
	return &TerminalManager{sessions: make(map[string]*TerminalSession)}
}

func (tm *TerminalManager) Create(shell, workdir string) (*TerminalSession, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.counter++
	id := fmt.Sprintf("term-%d", tm.counter)
	sess, err := NewTerminalSession(id, shell, workdir)
	if err != nil {
		return nil, err
	}
	tm.sessions[id] = sess
	return sess, nil
}

func (tm *TerminalManager) Get(id string) (*TerminalSession, bool) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	sess, ok := tm.sessions[id]
	return sess, ok
}

func (tm *TerminalManager) Remove(id string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if sess, ok := tm.sessions[id]; ok {
		sess.Close()
		delete(tm.sessions, id)
	}
}

func (tm *TerminalManager) ActiveIDs() []string {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	ids := make([]string, 0, len(tm.sessions))
	for id := range tm.sessions {
		ids = append(ids, id)
	}
	return ids
}

// WithTerminalManager attaches a terminal manager to context.
func WithTerminalManager(ctx context.Context, tm *TerminalManager) context.Context {
	return context.WithValue(ctx, terminalManagerKey{}, tm)
}

// TerminalManagerFrom retrieves the terminal manager from context.
func TerminalManagerFrom(ctx context.Context) *TerminalManager {
	tm, _ := ctx.Value(terminalManagerKey{}).(*TerminalManager)
	return tm
}

// ---------------------------------------------------------------------------
// Tool definitions
// ---------------------------------------------------------------------------

type TerminalCreateArgs struct {
	Shell   string `json:"shell,omitempty" jsonschema:"title=Shell,description=Shell to use (default: $SHELL or /bin/bash)"`
	Workdir string `json:"workdir,omitempty" jsonschema:"title=Workdir,description=Working directory for the terminal session"`
}

var TerminalCreate = Tool{
	ID:          "terminal_create",
	Description: "Create a new interactive terminal session with a PTY. Returns a session ID for subsequent commands. Multiple sessions can run concurrently.",
	Schema:      jsonschema.Reflect(&TerminalCreateArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args TerminalCreateArgs
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}

		tm := TerminalManagerFrom(ctx)
		if tm == nil {
			return ExecuteResult{}, fmt.Errorf("terminal manager not available")
		}

		workdir := args.Workdir
		if workdir == "" {
			workdir = WorkspaceFrom(ctx)
		} else if !filepath.IsAbs(workdir) {
			if ws := WorkspaceFrom(ctx); ws != "" {
				workdir = filepath.Join(ws, workdir)
			}
		}

		sess, err := tm.Create(args.Shell, workdir)
		if err != nil {
			return ExecuteResult{}, fmt.Errorf("failed to create terminal: %w", err)
		}

		// Wait a moment for the shell to start and print its banner
		time.Sleep(200 * time.Millisecond)
		output := sess.ReadOutput()

		return ExecuteResult{
			Title:  fmt.Sprintf("Terminal %s created", sess.ID),
			Output: fmt.Sprintf("Terminal session %s created. Shell: %s, Workdir: %s\n%s", sess.ID, defaultShell(), workdir, output),
			Metadata: map[string]any{
				"terminal_id":    sess.ID,
				"terminal_output": output,
				"action":         "create",
			},
		}, nil
	},
}

type TerminalExecuteArgs struct {
	Command   string `json:"command" jsonschema:"title=Command,description=Command to execute in the terminal"`
	SessionID string `json:"session_id,omitempty" jsonschema:"title=SessionId,description=Terminal session ID (uses most recent if empty)"`
	Timeout   int    `json:"timeout,omitempty" jsonschema:"title=Timeout,description=Seconds to wait for output (default 10, max 120)"`
}

var TerminalExecute = Tool{
	ID:          "terminal_execute",
	Description: "Execute a command in an existing terminal session. The command runs in the terminal's shell (preserving environment, cwd, etc). Returns the output after the command completes.",
	Schema:      jsonschema.Reflect(&TerminalExecuteArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args TerminalExecuteArgs
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		if args.Command == "" {
			return ExecuteResult{}, fmt.Errorf("command is required")
		}

		tm := TerminalManagerFrom(ctx)
		if tm == nil {
			return ExecuteResult{}, fmt.Errorf("terminal manager not available")
		}

		sess := resolveTerminalSession(tm, args.SessionID)
		if sess == nil {
			return ExecuteResult{}, fmt.Errorf("terminal session not found: %s", args.SessionID)
		}

		timeout := 10
		if args.Timeout > 0 {
			timeout = args.Timeout
		}
		if timeout > 120 {
			timeout = 120
		}

		// Clear any pending output before sending the command
		sess.ReadOutput()

		// Send command + newline
		cmd := args.Command + "\n"
		if err := sess.Write([]byte(cmd)); err != nil {
			return ExecuteResult{}, fmt.Errorf("failed to write to terminal: %w", err)
		}

		// Poll for output with timeout
		var output string
		deadline := time.Now().Add(time.Duration(timeout) * time.Second)
		for time.Now().Before(deadline) {
			time.Sleep(200 * time.Millisecond)
			out := sess.ReadOutput()
			if out != "" {
				output += out
				// Reset deadline when we get output (command is still producing)
				deadline = time.Now().Add(2 * time.Second)
			}
		}

		if output == "" {
			output = "(no output)"
		}

		return ExecuteResult{
			Title:  fmt.Sprintf("$ %s", truncateStr(args.Command, 60)),
			Output: output,
			Metadata: map[string]any{
				"terminal_id":     sess.ID,
				"terminal_output": output,
				"action":          "execute",
			},
		}, nil
	},
}

type TerminalReadArgs struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"title=SessionId,description=Terminal session ID (uses most recent if empty)"`
}

var TerminalRead = Tool{
	ID:          "terminal_read",
	Description: "Read the current output buffer of a terminal session. Use this to check on a running process or see what's on screen.",
	Schema:      jsonschema.Reflect(&TerminalReadArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args TerminalReadArgs
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}

		tm := TerminalManagerFrom(ctx)
		if tm == nil {
			return ExecuteResult{}, fmt.Errorf("terminal manager not available")
		}

		sess := resolveTerminalSession(tm, args.SessionID)
		if sess == nil {
			return ExecuteResult{}, fmt.Errorf("terminal session not found: %s", args.SessionID)
		}

		output := sess.ReadOutput()
		if output == "" {
			output = "(no new output)"
		}

		return ExecuteResult{
			Title:  fmt.Sprintf("Terminal %s output", sess.ID),
			Output: output,
			Metadata: map[string]any{
				"terminal_id":     sess.ID,
				"terminal_output": output,
				"action":          "read",
			},
		}, nil
	},
}

type TerminalResizeArgs struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"title=SessionId,description=Terminal session ID (uses most recent if empty)"`
	Cols      int    `json:"cols" jsonschema:"title=Cols,description=Number of columns (default 80)"`
	Rows      int    `json:"rows" jsonschema:"title=Rows,description=Number of rows (default 24)"`
}

var TerminalResize = Tool{
	ID:          "terminal_resize",
	Description: "Resize a terminal session's viewport. Useful when the display area changes size.",
	Schema:      jsonschema.Reflect(&TerminalResizeArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args TerminalResizeArgs
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}

		tm := TerminalManagerFrom(ctx)
		if tm == nil {
			return ExecuteResult{}, fmt.Errorf("terminal manager not available")
		}

		sess := resolveTerminalSession(tm, args.SessionID)
		if sess == nil {
			return ExecuteResult{}, fmt.Errorf("terminal session not found: %s", args.SessionID)
		}

		cols := uint16(args.Cols)
		if cols == 0 {
			cols = 80
		}
		rows := uint16(args.Rows)
		if rows == 0 {
			rows = 24
		}

		if err := sess.Resize(cols, rows); err != nil {
			return ExecuteResult{}, fmt.Errorf("failed to resize: %w", err)
		}

		return ExecuteResult{
			Title:  fmt.Sprintf("Terminal %s resized to %dx%d", sess.ID, cols, rows),
			Output: fmt.Sprintf("Terminal %s resized to %d columns x %d rows", sess.ID, cols, rows),
			Metadata: map[string]any{
				"terminal_id": sess.ID,
				"action":      "resize",
				"cols":        int(cols),
				"rows":        int(rows),
			},
		}, nil
	},
}

type TerminalCloseArgs struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"title=SessionId,description=Terminal session ID (uses most recent if empty)"`
}

var TerminalClose = Tool{
	ID:          "terminal_close",
	Description: "Close a terminal session and terminate its shell process.",
	Schema:      jsonschema.Reflect(&TerminalCloseArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args TerminalCloseArgs
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}

		tm := TerminalManagerFrom(ctx)
		if tm == nil {
			return ExecuteResult{}, fmt.Errorf("terminal manager not available")
		}

		sess := resolveTerminalSession(tm, args.SessionID)
		if sess == nil {
			return ExecuteResult{}, fmt.Errorf("terminal session not found: %s", args.SessionID)
		}

		id := sess.ID
		tm.Remove(id)

		return ExecuteResult{
			Title:  fmt.Sprintf("Terminal %s closed", id),
			Output: fmt.Sprintf("Terminal session %s has been closed.", id),
			Metadata: map[string]any{
				"terminal_id": id,
				"action":      "close",
			},
		}, nil
	},
}

// resolveTerminalSession finds a session by ID, or returns the most recent one.
func resolveTerminalSession(tm *TerminalManager, id string) *TerminalSession {
	if id != "" {
		sess, ok := tm.Get(id)
		if !ok {
			return nil
		}
		return sess
	}
	// Find the most recently created session
	ids := tm.ActiveIDs()
	if len(ids) == 0 {
		return nil
	}
	sess, _ := tm.Get(ids[len(ids)-1])
	return sess
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// Ensure we use io to avoid unused import errors
var _ = io.Copy
