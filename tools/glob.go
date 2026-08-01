package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/invopop/jsonschema"
)

type GlobArgs struct {
	Pattern string `json:"pattern" jsonschema:"title=Pattern,description=Glob pattern to match files (e.g. **/*.go, src/**/*.ts, *)"`
	Path    string `json:"path,omitempty" jsonschema:"title=Path,description=Directory to search in (default: current dir)"`
	Exclude string `json:"exclude,omitempty" jsonschema:"title=Exclude,description=Glob pattern to exclude (e.g. vendor/**)"`
}

type fileEntry struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"is_dir"`
	ModTime string `json:"mod_time"`
}

var Glob = Tool{
	ID:          "Glob",
	Description: "Find files matching a glob pattern (ripgrep-powered). Fast file listing with sizes. Use ** for recursive matching (e.g. **/*.go). Prefer this over Bash+Get-ChildItem for file listing.",

	Schema: jsonschema.Reflect(&GlobArgs{}),

	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args GlobArgs
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		if args.Path == "" {
			args.Path = extractPath(rawArgs)
		}
		if args.Pattern == "" {
			return ExecuteResult{}, fmt.Errorf("pattern is required")
		}

		rg, err := findRg()
		if err != nil {
			return ExecuteResult{}, err
		}
		rgArgs := []string{"--files", "--hidden", "--no-require-git"}

		rgArgs = append(rgArgs, "--glob", args.Pattern)

		if args.Exclude != "" {
			rgArgs = append(rgArgs, "--glob", "!"+args.Exclude)
		}

		searchDir := resolvePath(ctx, args.Path)
		if searchDir == "" {
			searchDir = "."
		}

		rgArgs = append(rgArgs, searchDir)

		cmd := exec.Command(rg, rgArgs...)
		output, err := cmd.Output()
		if err != nil {
			if len(output) == 0 {
				return ExecuteResult{}, fmt.Errorf("rg failed: %s", err)
			}
			return ExecuteResult{}, fmt.Errorf("rg failed: %w\n%s", err, string(output))
		}

		allFiles := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(allFiles) == 1 && allFiles[0] == "" {
			allFiles = nil
		}

		var entries []fileEntry
		for _, f := range allFiles {
			f = strings.TrimSpace(f)
			if f == "" {
				continue
			}

			info, err := os.Stat(f)
			if err != nil {
				continue
			}
			entries = append(entries, fileEntry{
				Path:    f,
				Size:    info.Size(),
				IsDir:   info.IsDir(),
				ModTime: info.ModTime().Format("Jan _2 15:04"),
			})
		}

		height := len(entries)
		var sb strings.Builder
		if height == 0 {
			sb.WriteString("No files matched.")
		} else {
			sb.WriteString(fmt.Sprintf("Found %d file(s):\n\n", height))
			for _, e := range entries {
				prefix := " "
				if e.IsDir {
					prefix = "d"
				}
				sb.WriteString(fmt.Sprintf("%s %s  %s  %s\n", prefix, e.ModTime, formatSize(e.Size), e.Path))
			}
		}

		return ExecuteResult{
			Title:  fmt.Sprintf("Glob: %s (%d files)", args.Pattern, height),
			Output: sb.String(),
			Metadata: map[string]any{
				"pattern": args.Pattern,
				"files":   entries,
				"count":   height,
			},
		}, nil
	},
}

func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%dKB", size/1024)
	} else {
		return fmt.Sprintf("%.1fMB", float64(size)/(1024*1024))
	}
}
