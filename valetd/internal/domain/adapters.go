package domain

import (
	"github.com/loaapp/valet/valetd/internal/caddy"
	"github.com/loaapp/valet/valetd/internal/certs"
	"github.com/loaapp/valet/valetd/internal/db"
	"github.com/loaapp/valet/valetd/internal/dns"
	"github.com/loaapp/valet/valetd/internal/hosts"
	"github.com/loaapp/valet/valetd/internal/resolver"
)

// CertManagerAdapter wraps certs.Manager to satisfy CertManager.
type CertManagerAdapter struct {
	Mgr *certs.Manager
}

func (a *CertManagerAdapter) MkcertAvailable() bool {
	return certs.MkcertAvailable()
}

func (a *CertManagerAdapter) GenerateCombinedCert(domains []string) (string, string, error) {
	return a.Mgr.GenerateCombinedCert(domains)
}

func (a *CertManagerAdapter) CombinedCertPath() (string, string) {
	return a.Mgr.CombinedCertPath()
}

func (a *CertManagerAdapter) RemoveCert(domain string) error {
	return a.Mgr.RemoveCert(domain)
}

// CaddyAdapter wraps the caddy package functions to satisfy CaddyReloader.
type CaddyAdapter struct{}

func (CaddyAdapter) Reload(routes []db.Route, combinedCert, combinedKey, dataDir string) error {
	return caddy.Reload(routes, combinedCert, combinedKey, dataDir)
}

// DNSAdapter wraps dns.Server to satisfy DNSUpdater.
type DNSAdapter struct {
	Server *dns.Server
}

func (a *DNSAdapter) SetDomains(domains []string) {
	a.Server.SetDomains(domains)
}

// HostsAdapter wraps the hosts package to satisfy HostsSyncer.
type HostsAdapter struct{}

func (HostsAdapter) Sync(domains []string) error {
	return hosts.Sync(domains)
}

// ResolverAdapter wraps the resolver package to satisfy ResolverChecker.
type ResolverAdapter struct{}

func (ResolverAdapter) IsInstalled(tld string) bool {
	return resolver.IsInstalled(tld)
}
