package db

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	_ "github.com/glebarez/go-sqlite"
)

type AppDB struct {
	DB *sql.DB
}

// DataDir returns the Valet data directory.
// macOS standard: ~/Library/Application Support/run.loa.valet/
func DataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, "Library", "Application Support", "run.loa.valet"), nil
}

// LogDir returns the Valet log directory.
// macOS standard: ~/Library/Logs/Valet/
func LogDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, "Library", "Logs", "Valet"), nil
}

// MigrateDataDir moves data from the legacy ~/.valet/ directory to the
// macOS-standard Application Support location on first run.
func MigrateDataDir() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	oldDir := filepath.Join(home, ".valet")
	newDir, err := DataDir()
	if err != nil {
		return
	}

	// Nothing to migrate if old dir doesn't exist or new dir already has data.
	if _, err := os.Stat(oldDir); os.IsNotExist(err) {
		return
	}
	if _, err := os.Stat(filepath.Join(newDir, "valet.db")); err == nil {
		return // already migrated
	}

	log.Printf("Migrating data from %s to %s", oldDir, newDir)
	os.MkdirAll(newDir, 0o755)

	// Move key files and directories.
	for _, name := range []string{"valet.db", "valet.db-shm", "valet.db-wal", "certs"} {
		src := filepath.Join(oldDir, name)
		dst := filepath.Join(newDir, name)
		if _, err := os.Stat(src); err == nil {
			if err := os.Rename(src, dst); err != nil {
				// Cross-device rename; fall back to copy.
				copyFileOrDir(src, dst)
			}
		}
	}

	// Move conversations.db too.
	for _, name := range []string{"conversations.db", "conversations.db-shm", "conversations.db-wal"} {
		src := filepath.Join(oldDir, name)
		dst := filepath.Join(newDir, name)
		if _, err := os.Stat(src); err == nil {
			os.Rename(src, dst)
		}
	}

	log.Printf("Migration complete")
}

func copyFileOrDir(src, dst string) {
	info, err := os.Stat(src)
	if err != nil {
		return
	}
	if info.IsDir() {
		os.MkdirAll(dst, info.Mode())
		entries, _ := os.ReadDir(src)
		for _, e := range entries {
			copyFileOrDir(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()))
		}
		return
	}
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer out.Close()
	io.Copy(out, in)
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
