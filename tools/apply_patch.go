package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/invopop/jsonschema"
)

type ApplyPatchArgs struct {
	Patch     string        `json:"patch,omitempty" jsonschema:"title=Patch,description=Multi-file unified git diff to apply. Use this when editing multiple files or providing a precise diff. Alternative to path + old_string + new_string."`
	Path      string        `json:"path,omitempty" jsonschema:"title=Path,description=Path to the file to edit. Required when not using patch."`
	OldString string        `json:"old_string,omitempty" jsonschema:"title=Old String,description=The exact text currently in the file to replace. Use Read first to get it exactly right (whitespace-sensitive)."`
	NewString string        `json:"new_string,omitempty" jsonschema:"title=New String,description=The replacement text to write in place of old_string."`
	Replaces  []Replacement `json:"replaces,omitempty" jsonschema:"title=Replaces,description=Optional list of find/replace pairs to apply multiple edits to the same file in one call."`
}

type Replacement struct {
	OldString string `json:"old_string" jsonschema:"title=Old String,description=Exact text to find (whitespace-sensitive)."`
	NewString string `json:"new_string" jsonschema:"title=New String,description=Replacement text."`
}

type patchFile struct {
	path  string
	hunks []hunk
	orig  string
}

var ApplyPatch = Tool{
	ID: "Edit",
	Description: "Edit an existing file. Provide EITHER:\n" +
		"  1) A multi-file unified git diff in 'patch', OR\n" +
		"  2) 'path' + 'old_string' + 'new_string' for a simple find-and-replace (recommended for single edits), OR\n" +
		"  3) 'path' + 'replaces' as a list of {old_string, new_string} pairs for multiple edits in one file.\n" +
		"Always Read the file first to get exact text. The old_string must match exactly (whitespace-sensitive).",

	Schema: jsonschema.Reflect(&ApplyPatchArgs{}),

	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args ApplyPatchArgs
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}

		fillArgAliases(rawArgs, &args)

		if args.Patch != "" {
			return applyDiff(ctx, args.Patch)
		}

		if args.Path == "" {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: missing 'patch' or 'path'. Provide one of:\n" +
				"  - {\"patch\": \"<unified git diff>\"}\n" +
				"  - {\"path\": \"src/foo.ts\", \"old_string\": \"<exact old text>\", \"new_string\": \"<new text>\"}\n" +
				"  - {\"path\": \"src/foo.ts\", \"replaces\": [{\"old_string\": \"...\", \"new_string\": \"...\"}]}")
		}

		path := resolvePath(ctx, args.Path)
		if _, err := os.Stat(path); err != nil {
			return ExecuteResult{}, fmt.Errorf("could not read '%s': %w", path, err)
		}

		var replacements []Replacement
		if len(args.Replaces) > 0 {
			replacements = args.Replaces
		} else if args.OldString != "" {
			replacements = []Replacement{{OldString: args.OldString, NewString: args.NewString}}
		} else {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: 'path' was provided but no replacement. Provide 'old_string' + 'new_string', a 'replaces' list, or use 'patch' for a unified diff.")
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return ExecuteResult{}, fmt.Errorf("could not read '%s': %w", path, err)
		}
		orig := normalize(string(data))

		BackupFile(path)

		content := orig
		for i, r := range replacements {
			if r.OldString == "" {
				return ExecuteResult{}, fmt.Errorf("replaces[%d]: old_string must not be empty", i)
			}
			var err error
			content, err = fuzzyApplyHunk(content, r.OldString, r.NewString)
			if err != nil {
				return ExecuteResult{}, fmt.Errorf("'%s' replacement %d: %w", path, i+1, err)
			}
		}

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return ExecuteResult{}, fmt.Errorf("could not write '%s': %w", path, err)
		}

		patch := createUnifiedDiff(path, orig, content)
		return ExecuteResult{
			Title:  fmt.Sprintf("Edited %s (%d replacement(s))", path, len(replacements)),
			Output: fmt.Sprintf("Applied %d replacement(s) to %s.", len(replacements), path),
			Metadata: map[string]any{
				"files": 1,
				"paths": []string{path},
				"diffs": []map[string]string{{"path": path, "patch": patch}},
			},
		}, nil
	},
}

// fillArgAliases fills path/old_string/new_string from common alias keys the
// model may produce (oldContent/newContent, old/new, search/replace, file_path).
func fillArgAliases(rawArgs json.RawMessage, args *ApplyPatchArgs) {
	if args.Patch != "" {
		return
	}
	var m map[string]any
	if err := json.Unmarshal(rawArgs, &m); err != nil {
		return
	}
	str := func(key string) string {
		if v, ok := m[key].(string); ok {
			return v
		}
		return ""
	}
	if args.Path == "" {
		if p := str("file_path"); p != "" {
			args.Path = p
		}
	}
	if args.OldString == "" {
		for _, k := range []string{"old_string", "oldContent", "old_content", "old", "search", "oldSeek", "old_seek"} {
			if v := str(k); v != "" {
				args.OldString = v
				break
			}
		}
	}
	if args.NewString == "" {
		for _, k := range []string{"new_string", "newContent", "new_content", "new", "replace", "newSeek", "new_seek"} {
			if v := str(k); v != "" {
				args.NewString = v
				break
			}
		}
	}
}

func applyDiff(ctx context.Context, patch string) (ExecuteResult, error) {
	pfiles, err := parseDiff(patch)
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("invalid diff: %w", err)
	}
	if len(pfiles) == 0 {
		return ExecuteResult{}, fmt.Errorf("patch contains no file changes")
	}

	for _, pf := range pfiles {
		if len(pf.hunks) == 0 {
			return ExecuteResult{}, fmt.Errorf("file '%s' has no hunks", pf.path)
		}
	}

	for i := range pfiles {
		pfiles[i].path = resolvePath(ctx, pfiles[i].path)
	}

	for i, pf := range pfiles {
		data, err := os.ReadFile(pf.path)
		if err != nil {
			return ExecuteResult{}, fmt.Errorf("could not read '%s': %w", pf.path, err)
		}
		pfiles[i].orig = normalize(string(data))
	}

	results := make([]string, 0, len(pfiles))
	var diffs []map[string]string

	for _, pf := range pfiles {
		BackupFile(pf.path)

		content := pf.orig

		for j, h := range pf.hunks {
			if h.oldContent == "" && h.newContent == "" {
				continue
			}

			var err error
			content, err = fuzzyApplyHunk(content, h.oldContent, h.newContent)
			if err != nil {
				return ExecuteResult{}, fmt.Errorf("'%s' hunk %d: %w", pf.path, j+1, err)
			}
		}

		if err := os.WriteFile(pf.path, []byte(content), 0644); err != nil {
			return ExecuteResult{}, fmt.Errorf("could not write '%s': %w", pf.path, err)
		}

		patch := createUnifiedDiff(pf.path, pf.orig, content)
		diffs = append(diffs, map[string]string{"path": pf.path, "patch": patch})

		results = append(results, fmt.Sprintf("  %s (%d hunk(s))", pf.path, len(pf.hunks)))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Applied patch to %d file(s):\n", len(pfiles)))
	for _, r := range results {
		sb.WriteString(r)
		sb.WriteString("\n")
	}

	return ExecuteResult{
		Title:  fmt.Sprintf("Applied patch (%d files)", len(pfiles)),
		Output: sb.String(),
		Metadata: map[string]any{
			"files": len(pfiles),
			"paths": func() []string {
				paths := make([]string, len(pfiles))
				for i, pf := range pfiles {
					paths[i] = pf.path
				}
				return paths
			}(),
			"diffs": diffs,
		},
	}, nil
}

func parseDiff(patch string) ([]patchFile, error) {
	lines := strings.Split(patch, "\n")

	var files []patchFile
	var currentFile *patchFile
	var hunkLines []string

	flushFile := func() error {
		if currentFile == nil {
			return nil
		}
		hunks, err := parseHunks(strings.Join(hunkLines, "\n"))
		if err != nil {
			return fmt.Errorf("'%s': %w", currentFile.path, err)
		}
		currentFile.hunks = hunks
		files = append(files, *currentFile)
		currentFile = nil
		hunkLines = nil
		return nil
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			if err := flushFile(); err != nil {
				return nil, err
			}

			fields := strings.Fields(line)
			var path string
			for _, f := range fields {
				if strings.HasPrefix(f, "b/") {
					path = strings.TrimPrefix(f, "b/")
					break
				}
			}
			if path == "" && len(fields) >= 4 {
				path = strings.TrimPrefix(fields[3], "b/")
			}
			if path == "" {
				return nil, fmt.Errorf("could not parse path from: %s", line)
			}

			currentFile = &patchFile{path: path}
			continue
		}

		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
			continue
		}

		if strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "new file mode") ||
			strings.HasPrefix(line, "deleted file mode") || strings.HasPrefix(line, "old mode") ||
			strings.HasPrefix(line, "new mode") || strings.HasPrefix(line, "rename from") ||
			strings.HasPrefix(line, "rename to") || strings.HasPrefix(line, "similarity index") ||
			strings.HasPrefix(line, "copy from") || strings.HasPrefix(line, "copy to") {
			continue
		}

		if currentFile != nil {
			hunkLines = append(hunkLines, line)
		}
	}

	if err := flushFile(); err != nil {
		return nil, err
	}

	return files, nil
}


