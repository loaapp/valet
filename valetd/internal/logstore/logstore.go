package logstore

import (
	"database/sql"
	"strings"
	"time"
)

// HTTPLogEntry represents a parsed HTTP access log entry.
type HTTPLogEntry struct {
	Timestamp  float64 `json:"ts"`
	Host       string  `json:"host"`
	Method     string  `json:"method"`
	URI        string  `json:"uri"`
	Status     int     `json:"status"`
	Duration   float64 `json:"duration"`
	Size       int     `json:"size"`
	RemoteAddr string  `json:"remoteAddr"`
	Upstream   string  `json:"upstream"`
	Error      string  `json:"error"`
}

// DNSLogEntry represents a single DNS query event.
type DNSLogEntry struct {
	Timestamp float64 `json:"ts"`
	Domain    string  `json:"domain"`
	Type      string  `json:"type"`
	Action    string  `json:"action"`
	Result    string  `json:"result"`
}

// Store persists HTTP and DNS logs in SQLite.
type Store struct {
	db *sql.DB
}

// New creates a new log store backed by the given database.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

const httpCols = `ts, host, method, uri, status, duration, size, remote_addr, upstream, error`

// PushHTTP inserts an HTTP access log entry.
func (s *Store) PushHTTP(entry HTTPLogEntry) error {
	_, err := s.db.Exec(
		`INSERT INTO http_logs (`+httpCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.Timestamp, entry.Host, entry.Method, entry.URI,
		entry.Status, entry.Duration, entry.Size, entry.RemoteAddr,
		entry.Upstream, entry.Error,
	)
	return err
}

// GetHTTPLogs returns HTTP log entries, newest first up to limit.
// If since > 0, only entries with ts > since are returned.
// If route is non-empty, only entries matching that host are returned.
func (s *Store) GetHTTPLogs(limit int, since float64, route string) ([]HTTPLogEntry, error) {
	query := `SELECT ` + httpCols + ` FROM http_logs`
	var args []any
	var clauses []string

	if since > 0 {
		clauses = append(clauses, "ts > ?")
		args = append(args, since)
	}
	if route != "" {
		clauses = append(clauses, "host = ?")
		args = append(args, route)
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY ts DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []HTTPLogEntry
	for rows.Next() {
		var e HTTPLogEntry
		if err := rows.Scan(&e.Timestamp, &e.Host, &e.Method, &e.URI,
			&e.Status, &e.Duration, &e.Size, &e.RemoteAddr,
			&e.Upstream, &e.Error); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}

	return entries, nil
}

// PushDNS inserts a DNS query log entry.
func (s *Store) PushDNS(entry DNSLogEntry) error {
	_, err := s.db.Exec(
		`INSERT INTO dns_logs (ts, domain, type, action, result)
		 VALUES (?, ?, ?, ?, ?)`,
		entry.Timestamp, entry.Domain, entry.Type, entry.Action, entry.Result,
	)
	return err
}

// GetDNSLogs returns the most recent DNS log entries in chronological order.
func (s *Store) GetDNSLogs(limit int) ([]DNSLogEntry, error) {
	rows, err := s.db.Query(
		`SELECT ts, domain, type, action, result
		 FROM dns_logs ORDER BY ts DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []DNSLogEntry
	for rows.Next() {
		var e DNSLogEntry
		if err := rows.Scan(&e.Timestamp, &e.Domain, &e.Type, &e.Action, &e.Result); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}

	return entries, nil
}

// ClearHTTP deletes all HTTP log entries.
func (s *Store) ClearHTTP() error {
	_, err := s.db.Exec(`DELETE FROM http_logs`)
	return err
}

// ClearDNS deletes all DNS log entries.
func (s *Store) ClearDNS() error {
	_, err := s.db.Exec(`DELETE FROM dns_logs`)
	return err
}

// Cleanup deletes log entries older than 24 hours from both tables.
func (s *Store) Cleanup() error {
	cutoff := float64(time.Now().Unix()) - 86400
	if _, err := s.db.Exec(`DELETE FROM http_logs WHERE ts < ?`, cutoff); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM dns_logs WHERE ts < ?`, cutoff); err != nil {
		return err
	}
	return nil
}
