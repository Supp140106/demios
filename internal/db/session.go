package db

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"time"
)

type Session struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Workspace string `json:"workspace"`
	History   string `json:"-"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func newID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func CreateSession(db *sql.DB, title, workspace string) (Session, error) {
	id := newID()
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := db.Exec(
		`INSERT INTO sessions (id, title, workspace, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		id, title, workspace, now, now,
	)
	if err != nil {
		return Session{}, fmt.Errorf("create session: %w", err)
	}
	return Session{ID: id, Title: title, Workspace: workspace, CreatedAt: now, UpdatedAt: now}, nil
}

func ListSessions(db *sql.DB, workspace string) ([]Session, error) {
	rows, err := db.Query(
		`SELECT id, title, workspace, created_at, updated_at FROM sessions ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.Title, &s.Workspace, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func GetSession(db *sql.DB, id string) (Session, error) {
	var s Session
	err := db.QueryRow(
		`SELECT id, title, workspace, history, created_at, updated_at FROM sessions WHERE id = ?`, id,
	).Scan(&s.ID, &s.Title, &s.Workspace, &s.History, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return Session{}, fmt.Errorf("get session: %w", err)
	}
	return s, nil
}

func UpdateSessionHistory(db *sql.DB, id string, history string) error {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := db.Exec(
		`UPDATE sessions SET history = ?, updated_at = ? WHERE id = ?`,
		history, now, id,
	)
	if err != nil {
		return fmt.Errorf("update session history: %w", err)
	}
	return nil
}

func RenameSession(db *sql.DB, id, title string) error {
	_, err := db.Exec(`UPDATE sessions SET title = ? WHERE id = ?`, title, id)
	if err != nil {
		return fmt.Errorf("rename session: %w", err)
	}
	return nil
}

func DeleteSession(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM messages WHERE session_id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete session messages: %w", err)
	}
	_, err = db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}
