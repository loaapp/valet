package db

import "database/sql"

func ListTLDs(db *sql.DB) ([]ManagedTLD, error) {
	rows, err := db.Query(`SELECT tld, resolver_installed, created_at FROM managed_tlds ORDER BY tld`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tlds []ManagedTLD
	for rows.Next() {
		var t ManagedTLD
		if err := rows.Scan(&t.TLD, &t.ResolverInstalled, &t.CreatedAt); err != nil {
			return nil, err
		}
		tlds = append(tlds, t)
	}
	return tlds, rows.Err()
}

func GetTLD(db *sql.DB, tld string) (*ManagedTLD, error) {
	var t ManagedTLD
	err := db.QueryRow(
		`SELECT tld, resolver_installed, created_at FROM managed_tlds WHERE tld = ?`, tld,
	).Scan(&t.TLD, &t.ResolverInstalled, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func CreateTLD(db *sql.DB, tld string) (*ManagedTLD, error) {
	_, err := db.Exec(`INSERT INTO managed_tlds (tld) VALUES (?)`, tld)
	if err != nil {
		return nil, err
	}
	return GetTLD(db, tld)
}

func UpdateTLDResolver(db *sql.DB, tld string, installed bool) error {
	_, err := db.Exec(`UPDATE managed_tlds SET resolver_installed = ? WHERE tld = ?`, installed, tld)
	return err
}

func DeleteTLD(db *sql.DB, tld string) error {
	_, err := db.Exec(`DELETE FROM managed_tlds WHERE tld = ?`, tld)
	return err
}
