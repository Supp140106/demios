package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/invopop/jsonschema"
)

type ReadFileParams struct {
	Path   string `json:"path" validate:"required" jsonschema:"description=Path to the file to read"`
	Offset int    `json:"offset,omitempty" validate:"omitempty,min=1" jsonschema:"description=Starting line number (1-based, default 1)"`
	Limit  int    `json:"limit,omitempty" validate:"omitempty,min=1,max=5000" jsonschema:"description=Max lines to return (default 200)"`
}

var ReadFile = Tool{
	ID:          "Read",
	Description: "Read a file's contents. Returns the file text with line numbers. Use this to inspect files before editing.",

	Schema: jsonschema.Reflect(&ReadFileParams{}),

	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var params ReadFileParams

		if err := json.Unmarshal(rawArgs, &params); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}

		if params.Path == "" {
			params.Path = extractPath(rawArgs)
		}
		if params.Path == "" {
			return ExecuteResult{}, fmt.Errorf("path is required")
		}
		params.Path = resolvePath(ctx, params.Path)

		data, err := os.ReadFile(params.Path)
		if err != nil {
			return ExecuteResult{}, fmt.Errorf("could not read %s: %w", params.Path, err)
		}

		lines := strings.Split(string(data), "\n")
		totalLines := len(lines)

		if params.Offset <= 0 {
			params.Offset = 1
		}
		if params.Limit <= 0 {
			params.Limit = 200
		}

		start := params.Offset - 1
		if start >= totalLines {
			return ExecuteResult{
				Title:  params.Path,
				Output: "",
				Metadata: map[string]any{
					"path":       params.Path,
					"offset":     params.Offset,
					"limit":      params.Limit,
					"totalLines": totalLines,
					"truncated":  false,
				},
			}, nil
		}

		end := start + params.Limit

		end = min(end, totalLines)

		extracted := lines[start:end]

		out := strings.Join(extracted, "\n")

		truncated := end < totalLines

		return ExecuteResult{
			Title:  fmt.Sprintf("%s (lines %d-%d)", params.Path, params.Offset, end),
			Output: out,
			Metadata: map[string]any{
				"path":       params.Path,
				"offset":     params.Offset,
				"limit":      params.Limit,
				"totalLines": totalLines,
				"truncated":  truncated,
			},
		}, nil
	},
}
