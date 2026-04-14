package db

import (
	"database/sql"
	"fmt"
	"strconv"
)

var migrations = []struct {
	version int
	sql     string
}{
	{1, `CREATE TABLE IF NOT EXISTS routes (
		id          TEXT PRIMARY KEY,
		domain      TEXT NOT NULL UNIQUE,
		upstream    TEXT NOT NULL,
		tls_enabled BOOLEAN NOT NULL DEFAULT 1,
		cert_path   TEXT NOT NULL DEFAULT '',
		key_path    TEXT NOT NULL DEFAULT '',
		created_at  TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
	)`},
	{2, `CREATE TABLE IF NOT EXISTS managed_tlds (
		tld                TEXT PRIMARY KEY,
		resolver_installed BOOLEAN NOT NULL DEFAULT 0,
		created_at         TEXT NOT NULL DEFAULT (datetime('now'))
	)`},
	{3, `CREATE TABLE IF NOT EXISTS settings (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL DEFAULT ''
	)`},
	{4, `ALTER TABLE routes ADD COLUMN match_config TEXT NOT NULL DEFAULT ''`},
	{5, `ALTER TABLE routes ADD COLUMN handler_config TEXT NOT NULL DEFAULT ''`},
	{6, `ALTER TABLE routes ADD COLUMN template TEXT NOT NULL DEFAULT ''`},
	{7, `ALTER TABLE routes ADD COLUMN description TEXT NOT NULL DEFAULT ''`},
}

func Migrate(db *sql.DB) error {
	// Ensure settings table exists first (needed for version tracking)
	db.Exec(`CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL DEFAULT '')`)

	current := getSchemaVersion(db)

	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		if _, err := db.Exec(m.sql); err != nil {
			return fmt.Errorf("migration v%d: %w", m.version, err)
		}
		setSchemaVersion(db, m.version)
	}
	return nil
}

func getSchemaVersion(db *sql.DB) int {
	var val string
	err := db.QueryRow(`SELECT value FROM settings WHERE key = 'schema_version'`).Scan(&val)
	if err != nil {
		return 0
	}
	v, _ := strconv.Atoi(val)
	return v
}

func setSchemaVersion(db *sql.DB, version int) {
	db.Exec(
		`INSERT INTO settings (key, value) VALUES ('schema_version', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		strconv.Itoa(version),
	)
}
