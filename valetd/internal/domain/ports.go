package domain

import "github.com/loaapp/valet/valetd/internal/db"

// CertManager abstracts certificate operations for testability.
type CertManager interface {
	MkcertAvailable() bool
	GenerateCombinedCert(domains []string) (certPath, keyPath string, err error)
	CombinedCertPath() (certPath, keyPath string)
	RemoveCert(domain string) error
}

// CaddyReloader abstracts Caddy config reloading for testability.
type CaddyReloader interface {
	Reload(routes []db.Route, combinedCert, combinedKey, dataDir string) error
}

// DNSUpdater abstracts DNS server domain updates for testability.
type DNSUpdater interface {
	SetDomains(domains []string)
}

// HostsSyncer abstracts /etc/hosts file synchronization for testability.
type HostsSyncer interface {
	Sync(domains []string) error
}

// ResolverChecker abstracts checking if a resolver is installed for a TLD.
type ResolverChecker interface {
	IsInstalled(tld string) bool
}
