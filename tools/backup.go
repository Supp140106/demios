package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/invopop/jsonschema"
)

type fileBackup struct {
	path    string
	content string
}

var (
	backupMu sync.Mutex
	backups  []fileBackup
)

func BackupFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	backupMu.Lock()
	defer backupMu.Unlock()
	backups = append(backups, fileBackup{path: path, content: string(data)})
}

func UndoLastBackup() (string, bool) {
	backupMu.Lock()
	defer backupMu.Unlock()
	if len(backups) == 0 {
		return "", false
	}
	last := backups[len(backups)-1]
	backups = backups[:len(backups)-1]
	if err := os.WriteFile(last.path, []byte(last.content), 0644); err != nil {
		backups = append(backups, last)
		return "", false
	}
	return last.path, true
}

func ClearBackups() {
	backupMu.Lock()
	defer backupMu.Unlock()
	backups = nil
}

type UndoLastArgs struct{}

var UndoLast = Tool{
	ID:          "Undo",
	Description: "Undo the most recent file edit. Reverts the last file that was modified by the Edit tool. Call this if a change was incorrect.",
	Schema:      jsonschema.Reflect(&UndoLastArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		path, ok := UndoLastBackup()
		if !ok {
			return ExecuteResult{}, fmt.Errorf("nothing to undo")
		}
		return ExecuteResult{
			Title:  "Undone",
			Output: fmt.Sprintf("Reverted changes to '%s'.", path),
			Metadata: map[string]any{
				"path": path,
			},
		}, nil
	},
}
