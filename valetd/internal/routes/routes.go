package routes

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/loaapp/valet/valetd/internal/caddy"
	"github.com/loaapp/valet/valetd/internal/certs"
	"github.com/loaapp/valet/valetd/internal/db"
	"github.com/loaapp/valet/valetd/internal/dns"
	"github.com/loaapp/valet/valetd/internal/hosts"
)

// Manager coordinates route changes across DB, certs, Caddy, DNS, and hosts.
type Manager struct {
	database  *sql.DB
	certMgr   *certs.Manager
	dnsServer *dns.Server
}

func NewManager(database *sql.DB, certMgr *certs.Manager, dnsServer *dns.Server) *Manager {
	return &Manager{
		database:  database,
		certMgr:   certMgr,
		dnsServer: dnsServer,
	}
}

// Add creates a new route and reloads Caddy (which regenerates the combined cert).
func (m *Manager) Add(domain, upstream string, tlsEnabled bool, matchConfig, handlerConfig, template, description string) (*db.Route, error) {
	upstream = normalizeUpstream(upstream)

	// Check for duplicate
	existing, err := db.GetRouteByDomain(m.database, domain)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("route for %s already exists", domain)
	}

	if tlsEnabled && !certs.MkcertAvailable() {
		return nil, fmt.Errorf("mkcert is not installed; run: brew install mkcert && mkcert -install")
	}

	// Cert paths are set during syncAll — the combined cert covers all domains
	route, err := db.CreateRoute(m.database, domain, upstream, tlsEnabled, "", "", matchConfig, handlerConfig, template, description)
	if err != nil {
		return nil, err
	}

	if err := m.syncAll(); err != nil {
		return nil, fmt.Errorf("sync after add: %w", err)
	}

	return route, nil
}

// Remove deletes a route and its certificate, then reloads Caddy.
func (m *Manager) Remove(domain string) error {
	route, err := db.GetRouteByDomain(m.database, domain)
	if err != nil {
		return err
	}
	if route == nil {
		return fmt.Errorf("no route for %s", domain)
	}

	if err := db.DeleteRoute(m.database, route.ID); err != nil {
		return err
	}

	m.certMgr.RemoveCert(domain)

	return m.syncAll()
}

// List returns all routes.
func (m *Manager) List() ([]db.Route, error) {
	return db.ListRoutes(m.database)
}

// Sync regenerates the combined TLS cert, reloads Caddy, and updates hosts.
// Exported so the server can call it after direct DB updates.
func (m *Manager) Sync() error {
	return m.syncAll()
}

// syncAll regenerates the combined TLS cert, reloads Caddy, and updates hosts.
func (m *Manager) syncAll() error {
	routes, err := db.ListRoutes(m.database)
	if err != nil {
		return err
	}

	// Collect all TLS-enabled domains and regenerate a single combined cert
	var tlsDomains []string
	for _, r := range routes {
		if r.TLSEnabled {
			tlsDomains = append(tlsDomains, r.Domain)
		}
	}

	var combinedCert, combinedKey string
	if len(tlsDomains) > 0 && certs.MkcertAvailable() {
		var err error
		combinedCert, combinedKey, err = m.certMgr.GenerateCombinedCert(tlsDomains)
		if err != nil {
			return fmt.Errorf("generate combined cert: %w", err)
		}
	}

	// Reload Caddy with current routes and combined cert
	if err := caddy.Reload(routes, combinedCert, combinedKey); err != nil {
		return fmt.Errorf("caddy reload: %w", err)
	}

	// Determine which domains need /etc/hosts entries
	// (domains whose TLD is NOT managed by our DNS server)
	tlds, err := db.ListTLDs(m.database)
	if err != nil {
		return err
	}
	managedTLDs := make(map[string]bool, len(tlds))
	for _, t := range tlds {
		managedTLDs[t.TLD] = true
	}

	var hostsDomains []string
	for _, r := range routes {
		tld := extractTLD(r.Domain)
		if !managedTLDs[tld] {
			hostsDomains = append(hostsDomains, r.Domain)
		}
	}

	if err := hosts.Sync(hostsDomains); err != nil {
		return fmt.Errorf("hosts sync: %w", err)
	}

	return nil
}

// SyncDNS updates the DNS server's TLD list from the database.
func (m *Manager) SyncDNS() error {
	tlds, err := db.ListTLDs(m.database)
	if err != nil {
		return err
	}
	tldList := make([]string, len(tlds))
	for i, t := range tlds {
		tldList[i] = t.TLD
	}
	m.dnsServer.SetTLDs(tldList)
	return nil
}

func extractTLD(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-1]
}

// normalizeUpstream strips protocol prefixes and adds localhost if just a port.
func normalizeUpstream(upstream string) string {
	upstream = strings.TrimPrefix(upstream, "http://")
	upstream = strings.TrimPrefix(upstream, "https://")
	if strings.HasPrefix(upstream, ":") {
		upstream = "localhost" + upstream
	}
	return upstream
}
