package metrics

import (
	"database/sql"
	"fmt"
	"time"
)

// DataPoint represents a single time-series data point.
type DataPoint struct {
	Timestamp  int64   `json:"ts"`
	Requests   int     `json:"reqs"`
	Errors     int     `json:"errs"`
	AvgLatency float64 `json:"latMs"`
	BytesOut   int64   `json:"bytesOut"`
}

// Store provides SQLite-backed metrics storage.
type Store struct {
	db *sql.DB
}

// NewStore creates a new metrics store backed by the given database.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Push inserts a data point for the given host.
func (s *Store) Push(host string, dp DataPoint) error {
	_, err := s.db.Exec(
		`INSERT INTO metrics (ts, host, reqs, errs, latency_ms, bytes_out) VALUES (?, ?, ?, ?, ?, ?)`,
		dp.Timestamp, host, dp.Requests, dp.Errors, dp.AvgLatency, dp.BytesOut,
	)
	return err
}

// GetCurrent returns the latest data point per host.
func (s *Store) GetCurrent() (map[string]DataPoint, error) {
	rows, err := s.db.Query(`
		SELECT m.host, m.ts, m.reqs, m.errs, m.latency_ms, m.bytes_out
		FROM metrics m
		INNER JOIN (SELECT host, MAX(ts) AS max_ts FROM metrics GROUP BY host) latest
		ON m.host = latest.host AND m.ts = latest.max_ts
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]DataPoint)
	for rows.Next() {
		var host string
		var dp DataPoint
		if err := rows.Scan(&host, &dp.Timestamp, &dp.Requests, &dp.Errors, &dp.AvgLatency, &dp.BytesOut); err != nil {
			return nil, err
		}
		// If multiple rows share the same max ts for a host, accumulate
		if existing, ok := result[host]; ok {
			existing.Requests += dp.Requests
			existing.Errors += dp.Errors
			existing.BytesOut += dp.BytesOut
			result[host] = existing
		} else {
			result[host] = dp
		}
	}
	return result, nil
}

// parseRange returns (cutoff timestamp, bucket size in seconds, resolution label).
func parseRange(rangeStr string) (int64, int64, string) {
	now := time.Now().Unix()
	switch rangeStr {
	case "1h":
		return now - 3600, 60, "1m"
	case "24h":
		return now - 86400, 3600, "1h"
	default: // "5m" or anything else
		return now - 300, 0, "1s"
	}
}

// GetHistory returns historical data with time bucketing via SQL.
// Returns (totals, perRoute, error).
func (s *Store) GetHistory(rangeStr string) ([]DataPoint, map[string][]DataPoint, error) {
	cutoff, bucket, _ := parseRange(rangeStr)

	totals, err := s.getTotals(cutoff, bucket)
	if err != nil {
		return nil, nil, fmt.Errorf("get totals: %w", err)
	}

	perRoute, err := s.getPerRoute(cutoff, bucket)
	if err != nil {
		return nil, nil, fmt.Errorf("get per-route: %w", err)
	}

	return totals, perRoute, nil
}

func (s *Store) getTotals(cutoff, bucket int64) ([]DataPoint, error) {
	var query string
	if bucket > 0 {
		query = fmt.Sprintf(`
			SELECT (ts/%d)*%d AS bucket, SUM(reqs), SUM(errs),
				CASE WHEN SUM(reqs) > 0 THEN SUM(reqs * latency_ms) / SUM(reqs) ELSE 0 END,
				SUM(bytes_out)
			FROM metrics
			WHERE ts > ?
			GROUP BY bucket
			ORDER BY bucket
		`, bucket, bucket)
	} else {
		query = `
			SELECT ts AS bucket, SUM(reqs), SUM(errs),
				CASE WHEN SUM(reqs) > 0 THEN SUM(reqs * latency_ms) / SUM(reqs) ELSE 0 END,
				SUM(bytes_out)
			FROM metrics
			WHERE ts > ?
			GROUP BY bucket
			ORDER BY bucket
		`
	}

	rows, err := s.db.Query(query, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []DataPoint
	for rows.Next() {
		var dp DataPoint
		if err := rows.Scan(&dp.Timestamp, &dp.Requests, &dp.Errors, &dp.AvgLatency, &dp.BytesOut); err != nil {
			return nil, err
		}
		points = append(points, dp)
	}
	return points, nil
}

func (s *Store) getPerRoute(cutoff, bucket int64) (map[string][]DataPoint, error) {
	var query string
	if bucket > 0 {
		query = fmt.Sprintf(`
			SELECT (ts/%d)*%d AS bucket, host, SUM(reqs), SUM(errs),
				CASE WHEN SUM(reqs) > 0 THEN SUM(reqs * latency_ms) / SUM(reqs) ELSE 0 END,
				SUM(bytes_out)
			FROM metrics
			WHERE ts > ?
			GROUP BY bucket, host
			ORDER BY bucket
		`, bucket, bucket)
	} else {
		query = `
			SELECT ts AS bucket, host, SUM(reqs), SUM(errs),
				CASE WHEN SUM(reqs) > 0 THEN SUM(reqs * latency_ms) / SUM(reqs) ELSE 0 END,
				SUM(bytes_out)
			FROM metrics
			WHERE ts > ?
			GROUP BY bucket, host
			ORDER BY bucket
		`
	}

	rows, err := s.db.Query(query, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]DataPoint)
	for rows.Next() {
		var host string
		var dp DataPoint
		if err := rows.Scan(&dp.Timestamp, &host, &dp.Requests, &dp.Errors, &dp.AvgLatency, &dp.BytesOut); err != nil {
			return nil, err
		}
		result[host] = append(result[host], dp)
	}
	return result, nil
}

// Cleanup removes metrics older than 24 hours.
func (s *Store) Cleanup() error {
	cutoff := time.Now().Unix() - 86400
	_, err := s.db.Exec(`DELETE FROM metrics WHERE ts < ?`, cutoff)
	return err
}
