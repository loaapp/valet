package db

import (
	"database/sql"
	"testing"

	_ "github.com/glebarez/go-sqlite"
)

// testDB creates an in-memory SQLite database with all migrations applied.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", "file::memory:?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := Migrate(conn); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestMigrations(t *testing.T) {
	db := testDB(t)

	// Verify key tables exist by running queries against them
	tables := []string{"routes", "managed_tlds", "settings", "dns_entries", "http_logs", "dns_logs", "metrics"}
	for _, table := range tables {
		t.Run(table, func(t *testing.T) {
			_, err := db.Exec("SELECT count(*) FROM " + table)
			if err != nil {
				t.Errorf("table %s not found: %v", table, err)
			}
		})
	}
}

func TestRoutes_CRUD(t *testing.T) {
	db := testDB(t)

	// List empty
	routes, err := ListRoutes(db)
	if err != nil {
		t.Fatalf("ListRoutes: %v", err)
	}
	if len(routes) != 0 {
		t.Fatalf("expected 0 routes, got %d", len(routes))
	}

	// Create
	r, err := CreateRoute(db, "myapp.test", "localhost:3000", true, "", "", "", "", "", "my app")
	if err != nil {
		t.Fatalf("CreateRoute: %v", err)
	}
	if r.Domain != "myapp.test" {
		t.Errorf("Domain = %q, want %q", r.Domain, "myapp.test")
	}
	if r.Upstream != "localhost:3000" {
		t.Errorf("Upstream = %q, want %q", r.Upstream, "localhost:3000")
	}
	if r.Description != "my app" {
		t.Errorf("Description = %q, want %q", r.Description, "my app")
	}
	if r.ID == "" {
		t.Error("expected non-empty ID")
	}

	// Get by ID
	got, err := GetRoute(db, r.ID)
	if err != nil {
		t.Fatalf("GetRoute: %v", err)
	}
	if got.Domain != "myapp.test" {
		t.Errorf("GetRoute Domain = %q, want %q", got.Domain, "myapp.test")
	}

	// Get by domain
	got, err = GetRouteByDomain(db, "myapp.test")
	if err != nil {
		t.Fatalf("GetRouteByDomain: %v", err)
	}
	if got.ID != r.ID {
		t.Errorf("GetRouteByDomain ID = %q, want %q", got.ID, r.ID)
	}

	// Get nonexistent
	got, err = GetRoute(db, "nonexistent")
	if err != nil {
		t.Fatalf("GetRoute nonexistent: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent route")
	}

	// Update
	updated, err := UpdateRoute(db, r.ID, "updated.test", "localhost:4000", true, "/cert", "/key", "", "", "", "updated")
	if err != nil {
		t.Fatalf("UpdateRoute: %v", err)
	}
	if updated.Domain != "updated.test" {
		t.Errorf("updated Domain = %q, want %q", updated.Domain, "updated.test")
	}
	if updated.Upstream != "localhost:4000" {
		t.Errorf("updated Upstream = %q, want %q", updated.Upstream, "localhost:4000")
	}

	// List should have 1
	routes, err = ListRoutes(db)
	if err != nil {
		t.Fatalf("ListRoutes: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}

	// Delete
	if err := DeleteRoute(db, r.ID); err != nil {
		t.Fatalf("DeleteRoute: %v", err)
	}
	routes, err = ListRoutes(db)
	if err != nil {
		t.Fatalf("ListRoutes after delete: %v", err)
	}
	if len(routes) != 0 {
		t.Fatalf("expected 0 routes after delete, got %d", len(routes))
	}
}

func TestRoutes_DuplicateDomain(t *testing.T) {
	db := testDB(t)

	_, err := CreateRoute(db, "myapp.test", "localhost:3000", true, "", "", "", "", "", "")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err = CreateRoute(db, "myapp.test", "localhost:4000", true, "", "", "", "", "", "")
	if err == nil {
		t.Error("expected error for duplicate domain, got nil")
	}
}

func TestDNSEntries_CRUD(t *testing.T) {
	db := testDB(t)

	// List empty
	entries, err := ListDNSEntries(db)
	if err != nil {
		t.Fatalf("ListDNSEntries: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}

	// Create with default target
	e, err := CreateDNSEntry(db, "app.example.com", "example.com", "")
	if err != nil {
		t.Fatalf("CreateDNSEntry: %v", err)
	}
	if e.Domain != "app.example.com" {
		t.Errorf("Domain = %q, want %q", e.Domain, "app.example.com")
	}
	if e.Target != "127.0.0.1" {
		t.Errorf("Target = %q, want %q (default)", e.Target, "127.0.0.1")
	}

	// Create with custom target
	e2, err := CreateDNSEntry(db, "api.example.com", "example.com", "10.0.0.1")
	if err != nil {
		t.Fatalf("CreateDNSEntry custom target: %v", err)
	}
	if e2.Target != "10.0.0.1" {
		t.Errorf("Target = %q, want %q", e2.Target, "10.0.0.1")
	}

	// List all
	entries, err = ListDNSEntries(db)
	if err != nil {
		t.Fatalf("ListDNSEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// List by TLD
	entries, err = ListDNSEntriesByTLD(db, "example.com")
	if err != nil {
		t.Fatalf("ListDNSEntriesByTLD: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries for example.com, got %d", len(entries))
	}

	entries, err = ListDNSEntriesByTLD(db, "other.com")
	if err != nil {
		t.Fatalf("ListDNSEntriesByTLD other: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for other.com, got %d", len(entries))
	}

	// Get
	got, err := GetDNSEntry(db, "app.example.com")
	if err != nil {
		t.Fatalf("GetDNSEntry: %v", err)
	}
	if got.TLD != "example.com" {
		t.Errorf("TLD = %q, want %q", got.TLD, "example.com")
	}

	// Get nonexistent
	got, err = GetDNSEntry(db, "nope.com")
	if err != nil {
		t.Fatalf("GetDNSEntry nonexistent: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent entry")
	}

	// Delete
	if err := DeleteDNSEntry(db, "app.example.com"); err != nil {
		t.Fatalf("DeleteDNSEntry: %v", err)
	}
	entries, err = ListDNSEntries(db)
	if err != nil {
		t.Fatalf("ListDNSEntries after delete: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after delete, got %d", len(entries))
	}
}
