package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
)

type workspaceKey struct{}

func WithWorkspace(ctx context.Context, ws string) context.Context {
	return context.WithValue(ctx, workspaceKey{}, ws)
}

func WorkspaceFrom(ctx context.Context) string {
	ws, _ := ctx.Value(workspaceKey{}).(string)
	return ws
}

func resolvePath(ctx context.Context, path string) string {
	ws := WorkspaceFrom(ctx)
	if path == "" {
		if ws != "" {
			return ws
		}
		return path
	}
	if filepath.IsAbs(path) {
		return path
	}
	if ws != "" {
		return filepath.Join(ws, path)
	}
	return path
}

func extractPath(rawArgs json.RawMessage) string {
	var m struct {
		Path     string `json:"path"`
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(rawArgs, &m); err != nil {
		return ""
	}
	if m.Path != "" {
		return m.Path
	}
	return m.FilePath
}

// EventEmitter is a callback tools can use to stream progress/events to the
// agent's SSE output while a tool is still executing (e.g. real-time dev-server
// output). It receives an event type and its data payload.
type EventEmitter func(evtType string, data map[string]any)

type eventEmitterKey struct{}

// WithEventEmitter attaches an event emitter to ctx so long-running tools can
// stream output to the user before they return.
func WithEventEmitter(ctx context.Context, em EventEmitter) context.Context {
	return context.WithValue(ctx, eventEmitterKey{}, em)
}

// EventEmitterFrom returns the event emitter attached to ctx, if any.
func EventEmitterFrom(ctx context.Context) EventEmitter {
	em, _ := ctx.Value(eventEmitterKey{}).(EventEmitter)
	return em
}

type toolCallIDKey struct{}

// WithToolCallID attaches the current tool-call ID to ctx so streamed events
// (like server-output) can be correlated with the tool card in the UI.
func WithToolCallID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, toolCallIDKey{}, id)
}

// ToolCallIDFrom returns the tool-call ID attached to ctx, if any.
func ToolCallIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(toolCallIDKey{}).(string)
	return id
}
