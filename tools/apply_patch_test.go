package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditViaOldNewString(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	os.WriteFile(path, []byte("hello world\n"), 0644)

	ctx := WithWorkspace(context.Background(), dir)
	raw := json.RawMessage(`{"path": "hello.txt", "old_string": "hello world", "new_string": "goodbye world"}`)

	res, err := ApplyPatch.Execute(ctx, raw)
	if err != nil {
		t.Fatalf("Edit failed: %v", err)
	}
	if !strings.Contains(res.Output, "Applied 1 replacement") {
		t.Errorf("unexpected output: %s", res.Output)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "goodbye world\n" {
		t.Errorf("file = %q, want %q", string(data), "goodbye world\n")
	}
}

func TestEditViaCamelCaseAliases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.go")
	os.WriteFile(path, []byte("package main\n\nfunc main() {}\n"), 0644)

	ctx := WithWorkspace(context.Background(), dir)
	raw := json.RawMessage(`{"path": "app.go", "oldContent": "func main() {}", "newContent": "func main() { println(1) }"}`)

	res, err := ApplyPatch.Execute(ctx, raw)
	if err != nil {
		t.Fatalf("Edit via camelCase aliases failed: %v", err)
	}
	if !strings.Contains(res.Output, "Applied 1 replacement") {
		t.Errorf("unexpected output: %s", res.Output)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "func main() { println(1) }") {
		t.Errorf("alias replacement not applied: %s", string(data))
	}
}

func TestEditViaFilepathAlias(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	os.WriteFile(path, []byte("foo bar\n"), 0644)

	ctx := WithWorkspace(context.Background(), dir)
	raw := json.RawMessage(`{"file_path": "a.txt", "old_string": "foo bar", "new_string": "baz"}`)

	if _, err := ApplyPatch.Execute(ctx, raw); err != nil {
		t.Fatalf("Edit via file_path alias failed: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "baz\n" {
		t.Errorf("file = %q, want %q", string(data), "baz\n")
	}
}

func TestEditViaReplaces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.txt")
	os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0644)

	ctx := WithWorkspace(context.Background(), dir)
	raw := json.RawMessage(`{
		"path": "multi.txt",
		"replaces": [
			{"old_string": "one", "new_string": "ONE"},
			{"old_string": "three", "new_string": "THREE"}
		]
	}`)

	res, err := ApplyPatch.Execute(ctx, raw)
	if err != nil {
		t.Fatalf("Edit via replaces failed: %v", err)
	}
	if !strings.Contains(res.Output, "Applied 2 replacement(s)") {
		t.Errorf("unexpected output: %s", res.Output)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "ONE\ntwo\nTHREE\n" {
		t.Errorf("file = %q, want %q", string(data), "ONE\ntwo\nTHREE\n")
	}
}

func TestEditViaPatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "patch.txt")
	os.WriteFile(path, []byte("alpha\nbeta\ngamma\n"), 0644)

	ctx := WithWorkspace(context.Background(), dir)
	diff := "diff --git a/patch.txt b/patch.txt\n" +
		"--- a/patch.txt\n" +
		"+++ b/patch.txt\n" +
		"@@ -1,3 +1,3 @@\n" +
		" alpha\n" +
		"-beta\n" +
		"+BETA\n" +
		" gamma\n"
	raw := json.RawMessage(`{"patch": ` + jsonQuote(diff) + `}`)

	if _, err := ApplyPatch.Execute(ctx, raw); err != nil {
		t.Fatalf("Edit via patch failed: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "alpha\nBETA\ngamma\n" {
		t.Errorf("file = %q, want %q", string(data), "alpha\nBETA\ngamma\n")
	}
}

func TestEditMissingArgsGivesHelpfulError(t *testing.T) {
	ctx := context.Background()
	raw := json.RawMessage(`{"startLine": 26, "endLine": 50}`)

	_, err := ApplyPatch.Execute(ctx, raw)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"patch", "path", "old_string"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message should mention %q: %s", want, msg)
		}
	}
}

func TestEditNotFoundError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	os.WriteFile(path, []byte("original content\n"), 0644)

	ctx := WithWorkspace(context.Background(), dir)
	raw := json.RawMessage(`{"path": "x.txt", "old_string": "does not exist here", "new_string": "nope"}`)

	_, err := ApplyPatch.Execute(ctx, raw)
	if err == nil {
		t.Fatal("expected error when old_string not found")
	}
	if !strings.Contains(err.Error(), "replacement 1") {
		t.Errorf("error should identify the failing replacement: %v", err)
	}
}

func jsonQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
