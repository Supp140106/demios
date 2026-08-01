package tools

import (
	"fmt"
	"strings"
)

type hunk struct {
	oldContent string
	newContent string
}

type filePatch struct {
	path  string
	hunks []hunk
}

func normalize(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

type diffEdit struct {
	kind byte
	text string
}

func computeEdits(oldLines, newLines []string) []diffEdit {
	m, n := len(oldLines), len(newLines)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldLines[i-1] == newLines[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] > dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	var edits []diffEdit
	i, j := m, n
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			edits = append(edits, diffEdit{' ', oldLines[i-1]})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			edits = append(edits, diffEdit{'+', newLines[j-1]})
			j--
		} else {
			edits = append(edits, diffEdit{'-', oldLines[i-1]})
			i--
		}
	}

	for i, j := 0, len(edits)-1; i < j; i, j = i+1, j-1 {
		edits[i], edits[j] = edits[j], edits[i]
	}

	return edits
}

func createUnifiedDiff(filePath, oldContent, newContent string) string {
	normalize := func(s string) []string {
		s = strings.ReplaceAll(s, "\r\n", "\n")
		lines := strings.Split(s, "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		return lines
	}

	oldLines := normalize(oldContent)
	newLines := normalize(newContent)

	if len(oldLines) == 0 && len(newLines) == 0 {
		return ""
	}
	if len(oldLines) == len(newLines) {
		same := true
		for i := range oldLines {
			if oldLines[i] != newLines[i] {
				same = false
				break
			}
		}
		if same {
			return ""
		}
	}

	edits := computeEdits(oldLines, newLines)

	type hunkLine struct {
		kind byte
		num  int
		text string
	}

	var hunks []hunkLine
	for _, e := range edits {
		hunks = append(hunks, hunkLine{kind: e.kind, text: e.text})
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", filePath, filePath))
	sb.WriteString(fmt.Sprintf("--- a/%s\n", filePath))
	sb.WriteString(fmt.Sprintf("+++ b/%s\n", filePath))

	ctxBefore := 3
	ctxAfter := 3

	for i := 0; i < len(hunks); {
		if hunks[i].kind == ' ' {
			i++
			continue
		}

		start := i
		if start-ctxBefore >= 0 {
			start = start - ctxBefore
		} else {
			start = 0
		}

		end := i
		for end < len(hunks) && hunks[end].kind != ' ' {
			end++
		}
		if end+ctxAfter > len(hunks) {
			end = len(hunks)
		} else {
			end = end + ctxAfter
		}

		var hunkLines []hunkLine
		for j := start; j < i; j++ {
			hunkLines = append(hunkLines, hunks[j])
		}
		for j := i; j < end; j++ {
			hunkLines = append(hunkLines, hunks[j])
		}

		oldStart := 0
		newStart := 0
		oldCount := 0
		newCount := 0

		pos := 0
		for _, hl := range hunkLines {
			switch hl.kind {
			case ' ':
				pos++
			case '-':
				pos++
				oldCount++
			case '+':
				newCount++
			}
		}

		for k := 0; k < start; k++ {
			if hunks[k].kind != '+' {
				oldStart++
			}
			if hunks[k].kind != '-' {
				newStart++
			}
		}

		oldStart++
		newStart++
		oldCount += pos
		newCount += pos

		sb.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount))
		for _, hl := range hunkLines {
			sb.WriteString(string(hl.kind) + hl.text + "\n")
		}

		i = end
	}

	return sb.String()
}

func fuzzyApplyHunk(content, oldContent, newContent string) (string, error) {
	if oldContent == "" {
		return content, nil
	}

	if n := strings.Count(content, oldContent); n == 1 {
		return strings.Replace(content, oldContent, newContent, 1), nil
	} else if n > 1 {
		idx := fuzzyFind(content, oldContent)
		if idx >= 0 {
			return replaceAtLine(content, oldContent, newContent, idx)
		}
		return "", fmt.Errorf("ambiguous match (%d occurrences) and fuzzy disambiguation failed", n)
	}

	idx := fuzzyFind(content, oldContent)
	if idx < 0 {
		return "", fmt.Errorf("match not found (tried exact + fuzzy)")
	}

	return replaceAtLine(content, oldContent, newContent, idx)
}

func fuzzyFind(content, oldContent string) int {
	fileLines := strings.Split(content, "\n")
	oldLines := strings.Split(oldContent, "\n")

	if len(oldLines) == 0 || len(oldLines) > len(fileLines) {
		return -1
	}

	for i := 0; i+len(oldLines) <= len(fileLines); i++ {
		matches := 0
		for j := 0; j < len(oldLines); j++ {
			if strings.TrimSpace(fileLines[i+j]) == strings.TrimSpace(oldLines[j]) {
				matches++
			}
		}
		threshold := len(oldLines) * 2 / 3
		if threshold < 1 {
			threshold = 1
		}
		if matches >= threshold {
			return i
		}
	}
	return -1
}

func replaceAtLine(content, oldContent, newContent string, _ int) (string, error) {
	fileLines := strings.Split(content, "\n")
	oldLines := strings.Split(oldContent, "\n")

	if len(oldLines) > len(fileLines) {
		return "", fmt.Errorf("old content longer than file")
	}

	idx := fuzzyFind(content, oldContent)
	if idx < 0 {
		return "", fmt.Errorf("internal: fuzzy re-find failed")
	}

	exactOld := strings.Join(fileLines[idx:idx+len(oldLines)], "\n")

	if !strings.Contains(content, exactOld) {
		return "", fmt.Errorf("internal: extracted text not found in content")
	}
	if strings.Count(content, exactOld) > 1 {
		return "", fmt.Errorf("internal: ambiguous after extraction")
	}

	return strings.Replace(content, exactOld, newContent, 1), nil
}

func parseHunks(patch string) ([]hunk, error) {
	lines := strings.Split(patch, "\n")

	var hunks []hunk
	var current []string
	inHunk := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "@@") {
			if inHunk && len(current) > 0 {
				h, err := buildHunk(current)
				if err != nil {
					return nil, fmt.Errorf("hunk %d: %w", len(hunks)+1, err)
				}
				if h != nil {
					hunks = append(hunks, *h)
				}
				current = nil
			}
			inHunk = true
			continue
		}
		if inHunk {
			current = append(current, line)
		}
	}

	if inHunk && len(current) > 0 {
		h, err := buildHunk(current)
		if err != nil {
			return nil, fmt.Errorf("hunk %d: %w", len(hunks)+1, err)
		}
		if h != nil {
			hunks = append(hunks, *h)
		}
	}

	return hunks, nil
}

func buildHunk(lines []string) (*hunk, error) {
	var oldLines, newLines []string

	for _, line := range lines {
		if len(line) == 0 {
			oldLines = append(oldLines, "")
			newLines = append(newLines, "")
			continue
		}
		prefix := line[0]
		rest := line[1:]

		switch prefix {
		case ' ':
			oldLines = append(oldLines, rest)
			newLines = append(newLines, rest)
		case '-':
			oldLines = append(oldLines, rest)
		case '+':
			newLines = append(newLines, rest)
		case '\\':
			continue
		default:
			return nil, fmt.Errorf("bad prefix %q in line %q", string(prefix), line)
		}
	}

	oldContent := strings.Join(oldLines, "\n")
	newContent := strings.Join(newLines, "\n")

	if oldContent == "" && newContent == "" {
		return nil, nil
	}

	return &hunk{
		oldContent: oldContent,
		newContent: newContent,
	}, nil
}
