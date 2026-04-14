package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/glebarez/go-sqlite"
)

type AppDB struct {
	DB *sql.DB
}

// DataDir returns the Valet data directory (~/.valet/).
func DataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".valet"), nil
}

// Open opens the Valet SQLite database, creating it if necessary.
func Open() (*AppDB, error) {
	dir, err := DataDir()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dir, "valet.db")
	conn, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=foreign_keys(1)&_pragma=journal_mode(wal)")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := Migrate(conn); err != nil {
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return &AppDB{DB: conn}, nil
}

func (a *AppDB) Close() error {
	return a.DB.Close()
}
