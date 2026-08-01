package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/invopop/jsonschema"
)

type ReadRelatedArgs struct {
	Paths []string `json:"paths" jsonschema:"title=Paths,description=File paths to read (relative to workspace)"`
}

var ReadRelated = Tool{
	ID:          "ReadRelated",
	Description: "Read multiple files at once into context. Use this to load related files (e.g. imports, dependencies, tests) alongside the main file you're working on.",
	Schema:      jsonschema.Reflect(&ReadRelatedArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args ReadRelatedArgs
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		if len(args.Paths) == 0 {
			return ExecuteResult{}, fmt.Errorf("at least one path is required")
		}

		var results []string
		for _, path := range args.Paths {
			resolved := resolvePath(ctx, path)
			data, err := os.ReadFile(resolved)
			if err != nil {
				results = append(results, fmt.Sprintf("=== %s ===\n[Error: %v]", path, err))
				continue
			}
			results = append(results, fmt.Sprintf("=== %s ===\n%s", path, string(data)))
		}

		return ExecuteResult{
			Title:  fmt.Sprintf("Read %d file(s)", len(args.Paths)),
			Output: strings.Join(results, "\n\n"),
			Metadata: map[string]any{
				"files": args.Paths,
				"count": len(args.Paths),
			},
		}, nil
	},
}

type ProjectStructureArgs struct {
	Path string `json:"path,omitempty" jsonschema:"title=Path,description=Subdirectory to show structure for (default: workspace root)"`
}

var ProjectStructure = Tool{
	ID:          "ProjectStructure",
	Description: "Get the project's file structure. Returns a tree of all files. Use this to understand the codebase layout before making changes.",
	Schema:      jsonschema.Reflect(&ProjectStructureArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args ProjectStructureArgs
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}

		searchDir := resolvePath(ctx, args.Path)
		if searchDir == "" {
			return ExecuteResult{}, fmt.Errorf("no workspace configured")
		}

		rgPath, err := findRg()
		if err != nil {
			return ExecuteResult{}, err
		}

		cmd := exec.Command(rgPath, "--files", "--no-require-git", "--hidden", searchDir)
		output, err := cmd.Output()
		if err != nil {
			if len(output) == 0 {
				return ExecuteResult{}, fmt.Errorf("failed to list files: %w", err)
			}
		}

		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) == 1 && lines[0] == "" {
			lines = nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Project structure (%d files):\n\n", len(lines)))

		ws := WorkspaceFrom(ctx)
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if ws != "" {
				rel, err := filepath.Rel(ws, line)
				if err == nil {
					sb.WriteString(rel)
				} else {
					sb.WriteString(line)
				}
			} else {
				sb.WriteString(line)
			}
			sb.WriteString("\n")
		}

		return ExecuteResult{
			Title:  fmt.Sprintf("Project structure (%d files)", len(lines)),
			Output: sb.String(),
			Metadata: map[string]any{
				"count": len(lines),
			},
		}, nil
	},
}
