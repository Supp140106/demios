package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/invopop/jsonschema"
)

type GrepArgs struct {
	Pattern    string `json:"pattern" jsonschema:"title=Pattern,description=Regex pattern to search for"`
	Path       string `json:"path,omitempty" jsonschema:"title=Path,description=Directory to search in (default: current dir)"`
	Include    string `json:"include,omitempty" jsonschema:"title=Include,description=File glob filter (e.g. *.go, *.{ts,tsx})"`
	Exclude    string `json:"exclude,omitempty" jsonschema:"title=Exclude,description=Skip files matching this glob"`
	Context    int    `json:"context,omitempty" jsonschema:"title=Context,description=Lines of context before/after each match"`
	Max        int    `json:"max,omitempty" jsonschema:"title=Max,description=Max results to return (default 100)"`
	IgnoreCase bool   `json:"ignoreCase,omitempty" jsonschema:"title=IgnoreCase,description=Case-insensitive search"`
}

type grepMatch struct {
	File   string   `json:"file"`
	Line   int      `json:"line"`
	Column int      `json:"column"`
	Text   string   `json:"text"`
	Before []string `json:"before,omitempty"`
	After  []string `json:"after,omitempty"`
}

var Grep = Tool{
	ID:          "Grep",
	Description: "Search file contents using regex patterns (ripgrep-powered). Fast content search with context lines. Prefer this over Bash+Select-String for searching.",

	Schema: jsonschema.Reflect(&GrepArgs{}),

	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args GrepArgs
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
		rgArgs := []string{"--json", "--no-require-git"}

		if args.IgnoreCase {
			rgArgs = append(rgArgs, "-i")
		}
		if args.Max > 0 {
			rgArgs = append(rgArgs, "-m", strconv.Itoa(args.Max))
		} else {
			rgArgs = append(rgArgs, "-m", "100")
		}
		if args.Context > 0 {
			rgArgs = append(rgArgs, "-C", strconv.Itoa(args.Context))
		}
		if args.Include != "" {
			rgArgs = append(rgArgs, "--glob", args.Include)
		}
		if args.Exclude != "" {
			rgArgs = append(rgArgs, "--glob", "!"+args.Exclude)
		}

		rgArgs = append(rgArgs, args.Pattern)
		searchPath := resolvePath(ctx, args.Path)
		if searchPath != "" {
			rgArgs = append(rgArgs, searchPath)
		}

		cmd := exec.Command(rg, rgArgs...)
		output, err := cmd.Output()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return ExecuteResult{
					Title:  "No matches",
					Output: "No matches found.",
					Metadata: map[string]any{
						"pattern": args.Pattern,
						"count":   0,
					},
				}, nil
			}
			return ExecuteResult{}, fmt.Errorf("rg failed: %w\n%s", err, string(output))
		}
		if len(output) == 0 {
			return ExecuteResult{
				Title:  "No matches",
				Output: "No matches found.",
				Metadata: map[string]any{
					"pattern": args.Pattern,
					"count":   0,
				},
			}, nil
		}

		var matches []grepMatch
		var current *grepMatch
		lines := strings.Split(string(output), "\n")

		for _, line := range lines {
			if line == "" {
				continue
			}
			var evt struct {
				Type string          `json:"type"`
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal([]byte(line), &evt); err != nil {
				continue
			}

			switch evt.Type {
			case "match":
				var data struct {
					Path struct {
						Text string `json:"text"`
					} `json:"path"`
					Lines struct {
						Text string `json:"text"`
					} `json:"lines"`
					LineNumber   int `json:"line_number"`
					AbsoluteLine int `json:"absolute_line"`
					Submatch     []struct {
						Start int `json:"start"`
					} `json:"submatches"`
				}
				if err := json.Unmarshal(evt.Data, &data); err != nil {
					continue
				}
				col := 1
				if len(data.Submatch) > 0 {
					col = data.Submatch[0].Start + 1
				}
				m := grepMatch{
					File:   data.Path.Text,
					Line:   data.AbsoluteLine,
					Column: col,
					Text:   strings.TrimRight(data.Lines.Text, "\n\r"),
				}
				matches = append(matches, m)
				current = &matches[len(matches)-1]

			case "context":
				var data struct {
					Lines struct {
						Text string `json:"text"`
					} `json:"lines"`
					LineNumber int `json:"line_number"`
				}
				if err := json.Unmarshal(evt.Data, &data); err != nil {
					continue
				}
				if current != nil {
					trimmed := strings.TrimRight(data.Lines.Text, "\n\r")
					if data.LineNumber < current.Line {
						current.Before = append(current.Before, trimmed)
					} else {
						current.After = append(current.After, trimmed)
					}
				}

			case "summary":
			}
		}

		height := len(matches)
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Found %d match(es):\n\n", height))
		for i, m := range matches {
			sb.WriteString(fmt.Sprintf("%s:%d:%d\n  %s\n", m.File, m.Line, m.Column, m.Text))
			if i < len(matches)-1 {
				sb.WriteString("—\n")
			}
		}

		return ExecuteResult{
			Title:  fmt.Sprintf("Grep: %s (%d matches)", args.Pattern, height),
			Output: sb.String(),
			Metadata: map[string]any{
				"pattern": args.Pattern,
				"matches": matches,
				"count":   height,
			},
		}, nil
	},
}
