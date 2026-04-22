package domain

import (
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/glebarez/go-sqlite"
	"github.com/loaapp/valet/valetd/internal/db"
)

// --- Test doubles ---

type stubCertManager struct {
	available    bool
	generateErr  error
	generateCert string
	generateKey  string
	removeCalled []string
}

func (s *stubCertManager) MkcertAvailable() bool { return s.available }
func (s *stubCertManager) GenerateCombinedCert(domains []string) (string, string, error) {
	return s.generateCert, s.generateKey, s.generateErr
}
func (s *stubCertManager) CombinedCertPath() (string, string) { return s.generateCert, s.generateKey }
func (s *stubCertManager) RemoveCert(domain string) error {
	s.removeCalled = append(s.removeCalled, domain)
	return nil
}

type stubCaddy struct {
	reloadErr    error
	reloadCalled int
	lastRoutes   []db.Route
}

func (s *stubCaddy) Reload(routes []db.Route, cert, key, dataDir string) error {
	s.reloadCalled++
	s.lastRoutes = routes
	return s.reloadErr
}

type stubDNS struct {
	lastDomains []string
}

func (s *stubDNS) SetDomains(domains []string) {
	s.lastDomains = domains
}

type stubHosts struct {
	syncErr    error
	syncCalled int
	lastDomains []string
}

func (s *stubHosts) Sync(domains []string) error {
	s.syncCalled++
	s.lastDomains = domains
	return s.syncErr
}

type stubResolver struct {
	installed map[string]bool
}

func (s *stubResolver) IsInstalled(tld string) bool {
	return s.installed[tld]
}

// --- Helpers ---

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", "file::memory:?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func newTestService(t *testing.T) (*RouteService, *stubCertManager, *stubCaddy, *stubDNS, *stubHosts) {
	database := testDB(t)
	cm := &stubCertManager{available: true, generateCert: "/tmp/cert.pem", generateKey: "/tmp/key.pem"}
	ca := &stubCaddy{}
	dn := &stubDNS{}
	ho := &stubHosts{}
	re := &stubResolver{installed: map[string]bool{}}

	svc := NewRouteService(database, cm, ca, dn, ho, re, "/tmp")
	return svc, cm, ca, dn, ho
}

// --- Tests ---

func TestAdd_ValidRoute(t *testing.T) {
	svc, _, ca, dn, _ := newTestService(t)

	route, err := svc.Add(AddRouteRequest{
		Domain:      "myapp.test",
		Upstream:    "localhost:3000",
		Description: "my app",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if route.Domain != "myapp.test" {
		t.Errorf("Domain = %q, want %q", route.Domain, "myapp.test")
	}
	if route.Upstream != "localhost:3000" {
		t.Errorf("Upstream = %q, want %q", route.Upstream, "localhost:3000")
	}
	if !route.TLSEnabled {
		t.Error("TLS should always be true")
	}
	if ca.reloadCalled != 1 {
		t.Errorf("Caddy reload called %d times, want 1", ca.reloadCalled)
	}
	if len(dn.lastDomains) != 1 || dn.lastDomains[0] != "myapp.test" {
		t.Errorf("DNS domains = %v, want [myapp.test]", dn.lastDomains)
	}
}

func TestAdd_InvalidDomain(t *testing.T) {
	svc, _, _, _, _ := newTestService(t)

	_, err := svc.Add(AddRouteRequest{Domain: "not valid!", Upstream: "localhost:3000"})
	if err == nil {
		t.Fatal("expected error for invalid domain")
	}
}

func TestAdd_InvalidUpstream(t *testing.T) {
	svc, _, _, _, _ := newTestService(t)

	_, err := svc.Add(AddRouteRequest{Domain: "myapp.test", Upstream: "no-port"})
	if err == nil {
		t.Fatal("expected error for invalid upstream")
	}
}

func TestAdd_DuplicateDomain(t *testing.T) {
	svc, _, _, _, _ := newTestService(t)

	_, err := svc.Add(AddRouteRequest{Domain: "myapp.test", Upstream: "localhost:3000"})
	if err != nil {
		t.Fatalf("first add: %v", err)
	}

	_, err = svc.Add(AddRouteRequest{Domain: "myapp.test", Upstream: "localhost:4000"})
	if err == nil {
		t.Fatal("expected error for duplicate domain")
	}
}

func TestAdd_MkcertUnavailable(t *testing.T) {
	svc, cm, _, _, _ := newTestService(t)
	cm.available = false

	_, err := svc.Add(AddRouteRequest{Domain: "myapp.test", Upstream: "localhost:3000"})
	if err == nil {
		t.Fatal("expected error when mkcert unavailable")
	}
}

func TestAdd_UpstreamRequired_NoTemplate(t *testing.T) {
	svc, _, _, _, _ := newTestService(t)

	_, err := svc.Add(AddRouteRequest{Domain: "myapp.test"})
	if err == nil {
		t.Fatal("expected error when no upstream and no template")
	}
}

func TestAdd_WithTemplate(t *testing.T) {
	svc, _, _, _, _ := newTestService(t)

	route, err := svc.Add(AddRouteRequest{
		Domain:   "myapp.test",
		Upstream: "localhost:3000",
		Template: "websocket",
	})
	if err != nil {
		t.Fatalf("Add with template: %v", err)
	}
	if route.Template != "websocket" {
		t.Errorf("Template = %q, want %q", route.Template, "websocket")
	}
	if route.HandlerConfig == "" {
		t.Error("expected HandlerConfig to be set by template")
	}
}

func TestUpdate_PartialMerge(t *testing.T) {
	svc, _, ca, _, _ := newTestService(t)

	route, _ := svc.Add(AddRouteRequest{
		Domain:      "myapp.test",
		Upstream:    "localhost:3000",
		Description: "original",
	})

	newDesc := "updated"
	updated, err := svc.Update(route.ID, UpdateRouteRequest{
		Description: &newDesc,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Description != "updated" {
		t.Errorf("Description = %q, want %q", updated.Description, "updated")
	}
	if updated.Domain != "myapp.test" {
		t.Error("domain should be preserved")
	}
	if updated.Upstream != "localhost:3000" {
		t.Error("upstream should be preserved")
	}
	// Add + Update = 2 syncs
	if ca.reloadCalled != 2 {
		t.Errorf("Caddy reload called %d times, want 2", ca.reloadCalled)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	svc, _, _, _, _ := newTestService(t)

	_, err := svc.Update("nonexistent", UpdateRouteRequest{})
	if err == nil {
		t.Fatal("expected error for nonexistent route")
	}
}

func TestUpdate_UpstreamChange_ClearsHandler(t *testing.T) {
	svc, _, _, _, _ := newTestService(t)

	route, _ := svc.Add(AddRouteRequest{
		Domain:   "myapp.test",
		Upstream: "localhost:3000",
	})

	// Simulate a custom handler config being set
	db.UpdateRoute(testDB2(t, svc), route.ID, "myapp.test", "localhost:3000", true, false, "", "",
		"", `[{"handler":"custom"}]`, "", "")

	newUpstream := "localhost:4000"
	updated, err := svc.Update(route.ID, UpdateRouteRequest{
		Upstream: &newUpstream,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	// HandlerConfig should be cleared because upstream changed and no template
	if updated.HandlerConfig != "" {
		t.Errorf("HandlerConfig should be cleared on upstream change, got %q", updated.HandlerConfig)
	}
}

func testDB2(t *testing.T, svc *RouteService) *sql.DB {
	t.Helper()
	return svc.db
}

func TestRemove(t *testing.T) {
	svc, cm, _, _, _ := newTestService(t)

	svc.Add(AddRouteRequest{Domain: "myapp.test", Upstream: "localhost:3000"})

	err := svc.Remove("myapp.test")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Cert should be removed
	if len(cm.removeCalled) != 1 || cm.removeCalled[0] != "myapp.test" {
		t.Errorf("RemoveCert called with %v, want [myapp.test]", cm.removeCalled)
	}

	// Should be gone
	routes, _ := svc.List()
	if len(routes) != 0 {
		t.Errorf("expected 0 routes after remove, got %d", len(routes))
	}
}

func TestRemove_NotFound(t *testing.T) {
	svc, _, _, _, _ := newTestService(t)

	err := svc.Remove("nonexistent.test")
	if err == nil {
		t.Fatal("expected error for nonexistent route")
	}
}

// --- Fragility: Sync partial failure ---

func TestSync_CertFailure_StopsBefore_CaddyReload(t *testing.T) {
	svc, cm, ca, _, _ := newTestService(t)
	cm.generateErr = fmt.Errorf("mkcert failed")

	// Add route directly to DB (bypass Add which calls Sync)
	db.CreateRoute(svc.db, "myapp.test", "localhost:3000", true, false, "", "", "", "", "", "")

	err := svc.Sync()
	if err == nil {
		t.Fatal("expected error from cert generation failure")
	}
	if ca.reloadCalled != 0 {
		t.Error("Caddy should NOT have been reloaded after cert failure")
	}
}

func TestSync_CaddyFailure_StopsBefore_DNSUpdate(t *testing.T) {
	svc, _, ca, dn, _ := newTestService(t)
	ca.reloadErr = fmt.Errorf("caddy config invalid")

	db.CreateRoute(svc.db, "myapp.test", "localhost:3000", true, false, "", "", "", "", "", "")

	err := svc.Sync()
	if err == nil {
		t.Fatal("expected error from Caddy reload failure")
	}
	if len(dn.lastDomains) != 0 {
		t.Error("DNS should NOT have been updated after Caddy failure")
	}
}

func TestSync_HostsFailure_ReturnsError(t *testing.T) {
	svc, _, _, _, ho := newTestService(t)
	ho.syncErr = fmt.Errorf("permission denied")

	// Add a route with a TLD that has no resolver (so it goes to hosts)
	db.CreateRoute(svc.db, "myapp.custom", "localhost:3000", true, false, "", "", "", "", "", "")

	err := svc.Sync()
	if err == nil {
		t.Fatal("expected error from hosts sync failure")
	}
}

func TestSync_MultipleRoutes_AllSynced(t *testing.T) {
	svc, _, ca, dn, _ := newTestService(t)

	db.CreateRoute(svc.db, "app1.test", "localhost:3000", true, false, "", "", "", "", "", "")
	db.CreateRoute(svc.db, "app2.test", "localhost:4000", true, false, "", "", "", "", "", "")

	err := svc.Sync()
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if len(ca.lastRoutes) != 2 {
		t.Errorf("Caddy got %d routes, want 2", len(ca.lastRoutes))
	}
	if len(dn.lastDomains) != 2 {
		t.Errorf("DNS got %d domains, want 2", len(dn.lastDomains))
	}
}

func TestSync_NoRoutes_StillSucceeds(t *testing.T) {
	svc, _, _, _, _ := newTestService(t)

	err := svc.Sync()
	if err != nil {
		t.Fatalf("Sync with no routes: %v", err)
	}
}

// --- Fragility: Hosts fallback logic ---

func TestSync_HostsFallback_OnlyUnmanagedTLDs(t *testing.T) {
	database := testDB(t)
	cm := &stubCertManager{available: true, generateCert: "/tmp/cert.pem", generateKey: "/tmp/key.pem"}
	ca := &stubCaddy{}
	dn := &stubDNS{}
	ho := &stubHosts{}
	re := &stubResolver{installed: map[string]bool{"test": true}} // test TLD has resolver

	svc := NewRouteService(database, cm, ca, dn, ho, re, "/tmp")

	// Route on managed TLD — should NOT go to hosts
	db.CreateRoute(svc.db, "myapp.test", "localhost:3000", true, false, "", "", "", "", "", "")
	// Route on unmanaged TLD — SHOULD go to hosts
	db.CreateRoute(svc.db, "myapp.custom", "localhost:4000", true, false, "", "", "", "", "", "")

	svc.Sync()

	// Only the unmanaged domain should be in hosts
	if len(ho.lastDomains) != 1 || ho.lastDomains[0] != "myapp.custom" {
		t.Errorf("hosts domains = %v, want [myapp.custom]", ho.lastDomains)
	}
}
