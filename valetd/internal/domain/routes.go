package domain

import (
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/loaapp/valet/valetd/internal/caddy"
	"github.com/loaapp/valet/valetd/internal/db"
)

type RouteService struct {
	db       *sql.DB
	certMgr  CertManager
	caddy    CaddyReloader
	dns      DNSUpdater
	hosts    HostsSyncer
	resolver ResolverChecker
	dataDir  string
}

func NewRouteService(database *sql.DB, certMgr CertManager, caddy CaddyReloader, dns DNSUpdater, hosts HostsSyncer, resolver ResolverChecker, dataDir string) *RouteService {
	return &RouteService{db: database, certMgr: certMgr, caddy: caddy, dns: dns, hosts: hosts, resolver: resolver, dataDir: dataDir}
}

func (s *RouteService) Add(req AddRouteRequest) (*db.Route, error) {
	// Validate domain
	domain, err := ValidateDomain(req.Domain)
	if err != nil {
		return nil, fmt.Errorf("invalid domain: %w", err)
	}

	// Validate/normalize upstream
	upstream, err := ValidateUpstream(req.Upstream)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream: %w", err)
	}

	// Resolve template
	req.Domain = domain
	req.Upstream = upstream
	if err := ResolveTemplate(&req); err != nil {
		return nil, err
	}

	// Upstream required if no template
	if req.Template == "" && upstream == "" {
		return nil, fmt.Errorf("upstream is required when not using a template")
	}

	// Check mkcert
	if !s.certMgr.MkcertAvailable() {
		return nil, fmt.Errorf("mkcert is not installed; run: brew install mkcert && mkcert -install")
	}

	// Check duplicate
	existing, err := db.GetRouteByDomain(s.db, domain)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("route for %s already exists", domain)
	}

	// Create (TLS always true)
	route, err := db.CreateRoute(s.db, domain, upstream, true, req.TLSUpstream, "", "",
		req.MatchConfig, req.HandlerConfig, req.Template, req.Description)
	if err != nil {
		return nil, err
	}

	if err := s.Sync(); err != nil {
		return nil, fmt.Errorf("sync after add: %w", err)
	}
	return route, nil
}

func (s *RouteService) Update(id string, req UpdateRouteRequest) (*db.Route, error) {
	existing, err := db.GetRoute(s.db, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("route not found")
	}

	// Merge: use new values if provided, keep existing otherwise
	domain := existing.Domain
	if req.Domain != nil {
		d, err := ValidateDomain(*req.Domain)
		if err != nil {
			return nil, fmt.Errorf("invalid domain: %w", err)
		}
		domain = d
	}

	upstream := existing.Upstream
	upstreamChanged := false
	if req.Upstream != nil {
		u, err := ValidateUpstream(*req.Upstream)
		if err != nil {
			return nil, fmt.Errorf("invalid upstream: %w", err)
		}
		upstream = u
		upstreamChanged = true
	}

	description := existing.Description
	if req.Description != nil {
		description = *req.Description
	}

	matchConfig := existing.MatchConfig
	if req.MatchConfig != nil {
		matchConfig = *req.MatchConfig
	}

	handlerConfig := existing.HandlerConfig
	if req.HandlerConfig != nil {
		handlerConfig = *req.HandlerConfig
	} else if upstreamChanged && existing.Template == "" {
		// Clear stale handlerConfig when upstream changes on a simple route
		handlerConfig = ""
	}

	tlsUpstream := existing.TLSUpstream
	if req.TLSUpstream != nil {
		tlsUpstream = *req.TLSUpstream
	}

	tmpl := existing.Template
	if req.Template != nil {
		tmpl = *req.Template
		// Resolve template if provided
		addReq := AddRouteRequest{Domain: domain, Upstream: upstream, Template: tmpl}
		if err := ResolveTemplate(&addReq); err != nil {
			return nil, err
		}
		if addReq.MatchConfig != "" {
			matchConfig = addReq.MatchConfig
		}
		if addReq.HandlerConfig != "" {
			handlerConfig = addReq.HandlerConfig
		}
	}

	route, err := db.UpdateRoute(s.db, id, domain, upstream, true, tlsUpstream,
		existing.CertPath, existing.KeyPath, matchConfig, handlerConfig, tmpl, description)
	if err != nil {
		return nil, err
	}

	if err := s.Sync(); err != nil {
		return route, fmt.Errorf("sync after update: %w", err)
	}
	return route, nil
}

func (s *RouteService) Remove(domain string) error {
	route, err := db.GetRouteByDomain(s.db, domain)
	if err != nil {
		return err
	}
	if route == nil {
		return fmt.Errorf("no route for %s", domain)
	}
	if err := db.DeleteRoute(s.db, route.ID); err != nil {
		return err
	}
	_ = s.certMgr.RemoveCert(domain)
	return s.Sync()
}

func (s *RouteService) List() ([]db.Route, error) {
	return db.ListRoutes(s.db)
}

func (s *RouteService) Get(id string) (*db.Route, error) {
	return db.GetRoute(s.db, id)
}

func (s *RouteService) Preview(req AddRouteRequest) ([]byte, error) {
	domain, _ := ValidateDomain(req.Domain)
	if domain == "" {
		domain = req.Domain
	}
	upstream := NormalizeUpstream(req.Upstream)
	req.Domain = domain
	req.Upstream = upstream
	ResolveTemplate(&req) // ignore error for preview

	fakeRoute := db.Route{
		ID: "preview", Domain: domain, Upstream: upstream, TLSEnabled: true,
		MatchConfig: req.MatchConfig, HandlerConfig: req.HandlerConfig,
		Template: req.Template, Description: req.Description,
	}
	return caddy.BuildRoutePreview(fakeRoute)
}

func (s *RouteService) Diagnose(domainName string) (*DiagnosticResult, error) {
	result := &DiagnosticResult{Domain: domainName}

	// DNS lookup
	addrs, err := net.LookupHost(domainName)
	if err != nil {
		result.Checks = append(result.Checks, DiagnosticCheck{"DNS lookup", "fail", err.Error()})
	} else {
		result.Checks = append(result.Checks, DiagnosticCheck{"DNS lookup", "pass", strings.Join(addrs, ", ")})
	}

	// Route lookup
	route, err := db.GetRouteByDomain(s.db, domainName)
	if err != nil {
		result.Checks = append(result.Checks, DiagnosticCheck{"Route lookup", "fail", err.Error()})
		return result, nil
	}
	if route == nil {
		result.Checks = append(result.Checks, DiagnosticCheck{"Route lookup", "fail", "no route configured"})
		return result, nil
	}
	result.Checks = append(result.Checks, DiagnosticCheck{"Route lookup", "pass",
		fmt.Sprintf("upstream=%s tls=%v", route.Upstream, route.TLSEnabled)})

	// TCP connect
	conn, err := net.DialTimeout("tcp", route.Upstream, 3*time.Second)
	if err != nil {
		result.Checks = append(result.Checks, DiagnosticCheck{"TCP connect", "fail", err.Error()})
	} else {
		conn.Close()
		result.Checks = append(result.Checks, DiagnosticCheck{"TCP connect", "pass", route.Upstream})
	}

	// HTTP GET
	httpClient := &http.Client{Timeout: 3 * time.Second}
	resp, err := httpClient.Get("http://" + route.Upstream)
	if err != nil {
		result.Checks = append(result.Checks, DiagnosticCheck{"HTTP GET", "fail", err.Error()})
	} else {
		resp.Body.Close()
		result.Checks = append(result.Checks, DiagnosticCheck{"HTTP GET", "pass", fmt.Sprintf("status %d", resp.StatusCode)})
	}

	return result, nil
}

// Sync regenerates certs, reloads Caddy, updates DNS and hosts.
func (s *RouteService) Sync() error {
	routes, err := db.ListRoutes(s.db)
	if err != nil {
		return err
	}

	// Collect TLS domains and regenerate combined cert
	var tlsDomains []string
	for _, r := range routes {
		if r.TLSEnabled {
			tlsDomains = append(tlsDomains, r.Domain)
		}
	}

	var combinedCert, combinedKey string
	if len(tlsDomains) > 0 && s.certMgr.MkcertAvailable() {
		combinedCert, combinedKey, err = s.certMgr.GenerateCombinedCert(tlsDomains)
		if err != nil {
			return fmt.Errorf("generate combined cert: %w", err)
		}
	}

	// Reload Caddy
	if err := s.caddy.Reload(routes, combinedCert, combinedKey, s.dataDir); err != nil {
		return fmt.Errorf("caddy reload: %w", err)
	}

	// Update DNS server with route domains
	var allDomains []string
	for _, r := range routes {
		allDomains = append(allDomains, r.Domain)
	}
	s.dns.SetDomains(allDomains)

	// Hosts file fallback for domains without resolver
	tlds, err := db.ListTLDs(s.db)
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
		parent := extractParentDomain(r.Domain)
		if !managedTLDs[tld] && !s.resolver.IsInstalled(parent) && !s.resolver.IsInstalled(tld) {
			hostsDomains = append(hostsDomains, r.Domain)
		}
	}
	if err := s.hosts.Sync(hostsDomains); err != nil {
		return fmt.Errorf("hosts sync: %w", err)
	}
	return nil
}

func extractTLD(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-1]
}

func extractParentDomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) < 3 {
		return domain
	}
	return strings.Join(parts[1:], ".")
}
