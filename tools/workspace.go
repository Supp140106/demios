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
