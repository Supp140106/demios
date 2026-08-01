package db

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Init(dbPath string) (*sql.DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS sessions (
		id         TEXT PRIMARY KEY,
		title      TEXT NOT NULL DEFAULT 'New Chat',
		workspace  TEXT NOT NULL DEFAULT '',
		history    TEXT NOT NULL DEFAULT '[]',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS messages (
		id            TEXT PRIMARY KEY,
		session_id    TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
		role          TEXT NOT NULL CHECK(role IN ('user','assistant')),
		content       TEXT,
		thinking      TEXT,
		tool_calls    TEXT,
		timestamp     TEXT NOT NULL DEFAULT (datetime('now'))
	);

	CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, timestamp);
	`
	_, err := db.Exec(schema)
	return err
}
