package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/invopop/jsonschema"
)

type WriteFileArgs struct {
	Path    string `json:"path" jsonschema:"title=Path,description=Relative path of the file to create or overwrite."`
	Content string `json:"content" jsonschema:"title=Content,description=Complete UTF-8 contents to write into the file."`
}

var WriteFile = Tool{
	ID:          "Write",
	Description: "Create a new file or completely overwrite an existing file with new content. Creates parent directories automatically.",

	Schema: jsonschema.Reflect(&WriteFileArgs{}),

	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args WriteFileArgs

		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}

		if args.Path == "" {
			args.Path = extractPath(rawArgs)
		}
		if args.Path == "" {
			return ExecuteResult{}, fmt.Errorf("path is required")
		}

		args.Path = resolvePath(ctx, args.Path)

		oldContent := ""
		if _, err := os.Stat(args.Path); err == nil {
			if data, err := os.ReadFile(args.Path); err == nil {
				oldContent = string(data)
			}
		}

		BackupFile(args.Path)

		if err := os.MkdirAll(filepath.Dir(args.Path), 0755); err != nil {
			return ExecuteResult{}, err
		}

		if err := os.WriteFile(args.Path, []byte(args.Content), 0644); err != nil {
			return ExecuteResult{}, err
		}

		result := ExecuteResult{
			Title:  "File Written",
			Output: fmt.Sprintf("Successfully wrote file '%s' (%d bytes).", args.Path, len(args.Content)),
			Metadata: map[string]any{
				"path": args.Path,
				"size": len(args.Content),
			},
		}

		if oldContent != "" && oldContent != args.Content {
			patch := createUnifiedDiff(args.Path, oldContent, args.Content)
			result.Metadata["diff"] = patch
		}

		return result, nil
	},
}
