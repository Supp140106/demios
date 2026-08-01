package db

import (
	"database/sql"
	"fmt"
	"time"
)

type Message struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Thinking  string `json:"thinking"`
	ToolCalls string `json:"tool_calls"`
	Timestamp string `json:"timestamp"`
}

func SaveMessages(db *sql.DB, sessionID string, msgs []Message) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("save messages begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM messages WHERE session_id = ?`, sessionID); err != nil {
		return fmt.Errorf("save messages delete old: %w", err)
	}

	stmt, err := tx.Prepare(
		`INSERT INTO messages (id, session_id, role, content, thinking, tool_calls, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("save messages prepare: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	for i, m := range msgs {
		id := m.ID
		if id == "" {
			id = newID()
			msgs[i].ID = id
		}
		ts := m.Timestamp
		if ts == "" {
			ts = now
		}
		_, err := stmt.Exec(id, sessionID, m.Role, m.Content, m.Thinking, m.ToolCalls, ts)
		if err != nil {
			return fmt.Errorf("save messages insert: %w", err)
		}
	}

	return tx.Commit()
}

func GetMessages(db *sql.DB, sessionID string) ([]Message, error) {
	rows, err := db.Query(
		`SELECT id, session_id, role, content, COALESCE(thinking,''), COALESCE(tool_calls,''), timestamp FROM messages WHERE session_id = ? ORDER BY timestamp ASC`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.Thinking, &m.ToolCalls, &m.Timestamp); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func DeleteMessages(db *sql.DB, sessionID string) error {
	_, err := db.Exec(`DELETE FROM messages WHERE session_id = ?`, sessionID)
	if err != nil {
		return fmt.Errorf("delete messages: %w", err)
	}
	return nil
}
