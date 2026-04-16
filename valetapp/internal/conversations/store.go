// Package conversations provides SQLite-backed conversation storage for the AI agent.
package conversations

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

type Message struct {
	ID         int     `json:"id"`
	Timestamp  float64 `json:"ts"`
	Role       string  `json:"role"` // "user", "assistant", "toolcall", "toolresult", "error", "tombstone"
	Content    string  `json:"content"`
	ToolName   string  `json:"toolName,omitempty"`
	ToolArgs   string  `json:"toolArgs,omitempty"`
	ToolResult string  `json:"toolResult,omitempty"`
}

type Store struct {
	db *sql.DB
}

func New() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, "Library", "Application Support", "run.loa.valet")
	os.MkdirAll(dir, 0o755)

	dbPath := filepath.Join(dir, "conversations.db")
	conn, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=foreign_keys(1)&_pragma=journal_mode(wal)")
	if err != nil {
		return nil, fmt.Errorf("open conversations db: %w", err)
	}

	// Create table
	_, err = conn.Exec(`CREATE TABLE IF NOT EXISTS messages (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		ts          REAL NOT NULL,
		role        TEXT NOT NULL,
		content     TEXT NOT NULL DEFAULT '',
		tool_name   TEXT NOT NULL DEFAULT '',
		tool_args   TEXT NOT NULL DEFAULT '',
		tool_result TEXT NOT NULL DEFAULT ''
	)`)
	if err != nil {
		return nil, fmt.Errorf("create messages table: %w", err)
	}

	return &Store{db: conn}, nil
}

func (s *Store) Close() {
	if s.db != nil {
		s.db.Close()
	}
}

// Push adds a message to the conversation log.
func (s *Store) Push(role, content, toolName, toolArgs, toolResult string) error {
	ts := float64(time.Now().UnixMilli()) / 1000.0
	_, err := s.db.Exec(
		`INSERT INTO messages (ts, role, content, tool_name, tool_args, tool_result) VALUES (?, ?, ?, ?, ?, ?)`,
		ts, role, content, toolName, toolArgs, toolResult,
	)
	return err
}

// InsertTombstone marks a conversation boundary.
func (s *Store) InsertTombstone() error {
	ts := float64(time.Now().UnixMilli()) / 1000.0
	_, err := s.db.Exec(
		`INSERT INTO messages (ts, role, content) VALUES (?, 'tombstone', '')`,
		ts,
	)
	return err
}

// GetCurrentSession returns all messages since the last tombstone.
// If there's no tombstone, returns all messages.
func (s *Store) GetCurrentSession() ([]Message, error) {
	var lastTombstoneID int
	s.db.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM messages WHERE role = 'tombstone'`).Scan(&lastTombstoneID)

	rows, err := s.db.Query(
		`SELECT id, ts, role, content, tool_name, tool_args, tool_result
		 FROM messages WHERE id > ? AND role != 'tombstone' ORDER BY id`,
		lastTombstoneID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Timestamp, &m.Role, &m.Content, &m.ToolName, &m.ToolArgs, &m.ToolResult); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}
