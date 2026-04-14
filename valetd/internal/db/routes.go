package db

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

const routeCols = `id, domain, upstream, tls_enabled, cert_path, key_path, match_config, handler_config, template, description, created_at, updated_at`

func scanRoute(scanner interface{ Scan(...any) error }) (*Route, error) {
	var r Route
	err := scanner.Scan(&r.ID, &r.Domain, &r.Upstream, &r.TLSEnabled, &r.CertPath, &r.KeyPath,
		&r.MatchConfig, &r.HandlerConfig, &r.Template, &r.Description, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func ListRoutes(db *sql.DB) ([]Route, error) {
	rows, err := db.Query(`SELECT ` + routeCols + ` FROM routes ORDER BY domain`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []Route
	for rows.Next() {
		r, err := scanRoute(rows)
		if err != nil {
			return nil, err
		}
		routes = append(routes, *r)
	}
	return routes, rows.Err()
}

func GetRoute(db *sql.DB, id string) (*Route, error) {
	r, err := scanRoute(db.QueryRow(`SELECT `+routeCols+` FROM routes WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return r, err
}

func GetRouteByDomain(db *sql.DB, domain string) (*Route, error) {
	r, err := scanRoute(db.QueryRow(`SELECT `+routeCols+` FROM routes WHERE domain = ?`, domain))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return r, err
}

func CreateRoute(db *sql.DB, domain, upstream string, tlsEnabled bool, certPath, keyPath, matchConfig, handlerConfig, template, description string) (*Route, error) {
	id := uuid.New().String()
	_, err := db.Exec(
		`INSERT INTO routes (id, domain, upstream, tls_enabled, cert_path, key_path, match_config, handler_config, template, description) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, domain, upstream, tlsEnabled, certPath, keyPath, matchConfig, handlerConfig, template, description,
	)
	if err != nil {
		return nil, fmt.Errorf("insert route: %w", err)
	}
	return GetRoute(db, id)
}

func UpdateRoute(db *sql.DB, id, domain, upstream string, tlsEnabled bool, certPath, keyPath, matchConfig, handlerConfig, template, description string) (*Route, error) {
	_, err := db.Exec(
		`UPDATE routes SET domain = ?, upstream = ?, tls_enabled = ?, cert_path = ?, key_path = ?, match_config = ?, handler_config = ?, template = ?, description = ?, updated_at = datetime('now') WHERE id = ?`,
		domain, upstream, tlsEnabled, certPath, keyPath, matchConfig, handlerConfig, template, description, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update route: %w", err)
	}
	return GetRoute(db, id)
}

func DeleteRoute(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM routes WHERE id = ?`, id)
	return err
}
