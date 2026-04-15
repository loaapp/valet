package db

import "database/sql"

func ListDNSEntries(db *sql.DB) ([]DNSEntry, error) {
	rows, err := db.Query(`SELECT domain, tld, target, created_at FROM dns_entries ORDER BY tld, domain`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []DNSEntry
	for rows.Next() {
		var e DNSEntry
		if err := rows.Scan(&e.Domain, &e.TLD, &e.Target, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func ListDNSEntriesByTLD(db *sql.DB, tld string) ([]DNSEntry, error) {
	rows, err := db.Query(`SELECT domain, tld, target, created_at FROM dns_entries WHERE tld = ? ORDER BY domain`, tld)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []DNSEntry
	for rows.Next() {
		var e DNSEntry
		if err := rows.Scan(&e.Domain, &e.TLD, &e.Target, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func GetDNSEntry(db *sql.DB, domain string) (*DNSEntry, error) {
	var e DNSEntry
	err := db.QueryRow(
		`SELECT domain, tld, target, created_at FROM dns_entries WHERE domain = ?`, domain,
	).Scan(&e.Domain, &e.TLD, &e.Target, &e.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func CreateDNSEntry(db *sql.DB, domain, tld, target string) (*DNSEntry, error) {
	if target == "" {
		target = "127.0.0.1"
	}
	_, err := db.Exec(`INSERT INTO dns_entries (domain, tld, target) VALUES (?, ?, ?)`, domain, tld, target)
	if err != nil {
		return nil, err
	}
	return GetDNSEntry(db, domain)
}

func DeleteDNSEntry(db *sql.DB, domain string) error {
	_, err := db.Exec(`DELETE FROM dns_entries WHERE domain = ?`, domain)
	return err
}
