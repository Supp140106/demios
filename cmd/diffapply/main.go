package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

type Hunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []string
}

type DiffFile struct {
	OldPath string
	NewPath string
	Hunks   []Hunk
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: diffapply <diff-file> [target-file]\n")
		fmt.Fprintf(os.Stderr, "  If target-file is omitted, it's read from the diff header\n")
		os.Exit(1)
	}

	diffPath := os.Args[1]
	var targetPath string
	if len(os.Args) >= 3 {
		targetPath = os.Args[2]
	} else {
		diffContent, err := os.ReadFile(diffPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading diff: %v\n", err)
			os.Exit(1)
		}
		diffFile := parseDiff(string(diffContent))
		if diffFile == nil {
			fmt.Fprintf(os.Stderr, "Failed to parse diff\n")
			os.Exit(1)
		}
		if strings.HasPrefix(diffFile.OldPath, "a/") {
			targetPath = diffFile.OldPath[2:]
		} else if diffFile.OldPath != "/dev/null" {
			targetPath = diffFile.OldPath
		} else if strings.HasPrefix(diffFile.NewPath, "b/") {
			targetPath = diffFile.NewPath[2:]
		} else {
			targetPath = diffFile.NewPath
		}
	}

	content, err := os.ReadFile(targetPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading target file: %v\n", err)
		os.Exit(1)
	}

	diffContent, err := os.ReadFile(diffPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading diff: %v\n", err)
		os.Exit(1)
	}

	result, err := applyPatch(string(content), string(diffContent))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error applying patch: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(targetPath, []byte(result), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing result: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Applied patch to %s\n", targetPath)
}

func parseDiff(diff string) *DiffFile {
	lines := strings.Split(diff, "\n")
	df := &DiffFile{}
	var currentHunk *Hunk
	inHunk := false

	for _, line := range lines {
		if strings.HasPrefix(line, "--- ") {
			df.OldPath = strings.TrimPrefix(line, "--- ")
		} else if strings.HasPrefix(line, "+++ ") {
			df.NewPath = strings.TrimPrefix(line, "+++ ")
		} else if strings.HasPrefix(line, "@@") {
			if currentHunk != nil {
				df.Hunks = append(df.Hunks, *currentHunk)
			}
			currentHunk = parseHunkHeader(line)
			inHunk = true
		} else if inHunk && currentHunk != nil {
			currentHunk.Lines = append(currentHunk.Lines, line)
		}
	}

	if currentHunk != nil {
		df.Hunks = append(df.Hunks, *currentHunk)
	}

	return df
}

var hunkHeaderRe = regexp.MustCompile(`@@ -(\d+),?(\d*) \+(\d+),?(\d*) @@`)

func parseHunkHeader(line string) *Hunk {
	m := hunkHeaderRe.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	h := &Hunk{}
	h.OldStart = atoi(m[1])
	if m[2] == "" {
		h.OldCount = 1
	} else {
		h.OldCount = atoi(m[2])
	}
	h.NewStart = atoi(m[3])
	if m[4] == "" {
		h.NewCount = 1
	} else {
		h.NewCount = atoi(m[4])
	}
	return h
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

func applyPatch(content, diff string) (string, error) {
	df := parseDiff(diff)
	if df == nil || len(df.Hunks) == 0 {
		return "", fmt.Errorf("no hunks found in diff")
	}

	fileLines := strings.Split(content, "\n")
	// Preserve trailing newline
	hasTrailingNewline := strings.HasSuffix(content, "\n")

	result := applyHunks(fileLines, df.Hunks)

	if hasTrailingNewline {
		return strings.Join(result, "\n") + "\n", nil
	}
	return strings.Join(result, "\n"), nil
}

func applyHunks(fileLines []string, hunks []Hunk) []string {
	result := make([]string, len(fileLines))
	copy(result, fileLines)
	offset := 0

	for _, hunk := range hunks {
		oldLines := extractOldLines(hunk.Lines)
		newLines := extractNewLines(hunk.Lines)

		idx := findHunk(result, oldLines, hunk.OldStart-1+offset)
		if idx < 0 {
			fmt.Fprintf(os.Stderr, "Warning: could not find hunk starting at line %d, trying fuzzy match...\n", hunk.OldStart+offset)
			idx = fuzzyFindHunk(result, oldLines)
			if idx < 0 {
				fmt.Fprintf(os.Stderr, "ERROR: hunk not found even with fuzzy matching\n")
				continue
			}
		}

		before := result[:idx]
		after := result[idx+len(oldLines):]

		replacement := make([]string, 0, len(before)+len(newLines)+len(after))
		replacement = append(replacement, before...)
		replacement = append(replacement, newLines...)
		replacement = append(replacement, after...)

		result = replacement
		offset += len(newLines) - len(oldLines)
	}

	return result
}

func extractOldLines(hunkLines []string) []string {
	var old []string
	for _, line := range hunkLines {
		if len(line) > 0 && line[0] == '-' {
			old = append(old, line[1:])
		} else if len(line) > 0 && line[0] == ' ' {
			old = append(old, line[1:])
		}
	}
	return old
}

func extractNewLines(hunkLines []string) []string {
	var newLines []string
	for _, line := range hunkLines {
		if len(line) > 0 && line[0] == '+' {
			newLines = append(newLines, line[1:])
		} else if len(line) > 0 && line[0] == ' ' {
			newLines = append(newLines, line[1:])
		}
	}
	return newLines
}

func findHunk(lines []string, oldLines []string, hint int) int {
	// Try hint position first
	if hint >= 0 && hint+len(oldLines) <= len(lines) {
		if matchLines(lines[hint:hint+len(oldLines)], oldLines) {
			return hint
		}
	}

	// Search forward from hint
	for i := hint + 1; i+len(oldLines) <= len(lines); i++ {
		if matchLines(lines[i:i+len(oldLines)], oldLines) {
			return i
		}
	}

	// Search backward from hint
	for i := hint - 1; i >= 0; i-- {
		if i+len(oldLines) <= len(lines) && matchLines(lines[i:i+len(oldLines)], oldLines) {
			return i
		}
	}

	return -1
}

func fuzzyFindHunk(lines []string, oldLines []string) int {
	for i := 0; i+len(oldLines) <= len(lines); i++ {
		if fuzzyMatchLines(lines[i:i+len(oldLines)], oldLines) {
			return i
		}
	}
	return -1
}

func matchLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func fuzzyMatchLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	matches := 0
	for i := range a {
		if strings.TrimSpace(a[i]) == strings.TrimSpace(b[i]) {
			matches++
		}
	}
	return matches >= len(a)*2/3
}


