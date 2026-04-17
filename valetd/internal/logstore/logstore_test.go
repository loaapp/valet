package logstore

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/loaapp/valet/valetd/internal/db"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	conn, err := sql.Open("sqlite", "file::memory:?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return New(conn)
}

func TestHTTPLogs(t *testing.T) {
	s := testStore(t)

	// Empty
	logs, err := s.GetHTTPLogs(100, 0, "")
	if err != nil {
		t.Fatalf("GetHTTPLogs: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("expected 0 logs, got %d", len(logs))
	}

	// Push entries
	now := float64(time.Now().Unix())
	for i := 0; i < 5; i++ {
		err := s.PushHTTP(HTTPLogEntry{
			Timestamp: now + float64(i),
			Host:      "myapp.test",
			Method:    "GET",
			URI:       "/page",
			Status:    200,
			Duration:  0.05,
		})
		if err != nil {
			t.Fatalf("PushHTTP: %v", err)
		}
	}

	// Get all
	logs, err = s.GetHTTPLogs(100, 0, "")
	if err != nil {
		t.Fatalf("GetHTTPLogs: %v", err)
	}
	if len(logs) != 5 {
		t.Fatalf("expected 5 logs, got %d", len(logs))
	}

	// Newest first (DESC)
	if logs[0].Timestamp < logs[4].Timestamp {
		t.Error("expected newest first ordering")
	}

	// Limit
	logs, err = s.GetHTTPLogs(2, 0, "")
	if err != nil {
		t.Fatalf("GetHTTPLogs limit: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs with limit, got %d", len(logs))
	}

	// Since
	logs, err = s.GetHTTPLogs(100, now+2, "")
	if err != nil {
		t.Fatalf("GetHTTPLogs since: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs after since, got %d", len(logs))
	}

	// Route filter
	s.PushHTTP(HTTPLogEntry{Timestamp: now + 10, Host: "other.test", Method: "POST", URI: "/api", Status: 201})
	logs, err = s.GetHTTPLogs(100, 0, "other.test")
	if err != nil {
		t.Fatalf("GetHTTPLogs route: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log for other.test, got %d", len(logs))
	}

	// Since + route
	logs, err = s.GetHTTPLogs(100, now+9, "other.test")
	if err != nil {
		t.Fatalf("GetHTTPLogs since+route: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1, got %d", len(logs))
	}

	// Clear
	if err := s.ClearHTTP(); err != nil {
		t.Fatalf("ClearHTTP: %v", err)
	}
	logs, err = s.GetHTTPLogs(100, 0, "")
	if err != nil {
		t.Fatalf("GetHTTPLogs after clear: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("expected 0 after clear, got %d", len(logs))
	}
}

func TestDNSLogs(t *testing.T) {
	s := testStore(t)

	now := float64(time.Now().Unix())
	for i := 0; i < 3; i++ {
		err := s.PushDNS(DNSLogEntry{
			Timestamp: now + float64(i),
			Domain:    "myapp.test",
			Type:      "A",
			Action:    "local",
			Result:    "127.0.0.1",
		})
		if err != nil {
			t.Fatalf("PushDNS: %v", err)
		}
	}

	logs, err := s.GetDNSLogs(100)
	if err != nil {
		t.Fatalf("GetDNSLogs: %v", err)
	}
	if len(logs) != 3 {
		t.Fatalf("expected 3 DNS logs, got %d", len(logs))
	}

	// Newest first
	if logs[0].Timestamp < logs[2].Timestamp {
		t.Error("expected newest first ordering")
	}

	// Clear
	if err := s.ClearDNS(); err != nil {
		t.Fatalf("ClearDNS: %v", err)
	}
	logs, err = s.GetDNSLogs(100)
	if err != nil {
		t.Fatalf("GetDNSLogs after clear: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("expected 0 after clear, got %d", len(logs))
	}
}

func TestCleanup(t *testing.T) {
	s := testStore(t)

	old := float64(time.Now().Unix()) - 100000 // > 24h ago
	recent := float64(time.Now().Unix())

	s.PushHTTP(HTTPLogEntry{Timestamp: old, Host: "old.test", Method: "GET", URI: "/", Status: 200})
	s.PushHTTP(HTTPLogEntry{Timestamp: recent, Host: "new.test", Method: "GET", URI: "/", Status: 200})
	s.PushDNS(DNSLogEntry{Timestamp: old, Domain: "old.test", Type: "A", Action: "local", Result: "127.0.0.1"})
	s.PushDNS(DNSLogEntry{Timestamp: recent, Domain: "new.test", Type: "A", Action: "local", Result: "127.0.0.1"})

	if err := s.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	httpLogs, _ := s.GetHTTPLogs(100, 0, "")
	if len(httpLogs) != 1 {
		t.Errorf("expected 1 HTTP log after cleanup, got %d", len(httpLogs))
	}

	dnsLogs, _ := s.GetDNSLogs(100)
	if len(dnsLogs) != 1 {
		t.Errorf("expected 1 DNS log after cleanup, got %d", len(dnsLogs))
	}
}
