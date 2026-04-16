package domain

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/loaapp/valet/valetd/internal/db"
	"github.com/loaapp/valet/valetd/internal/dns"
)

type DNSEntryService struct {
	database  *sql.DB
	dnsServer *dns.Server
}

func NewDNSEntryService(database *sql.DB, dnsServer *dns.Server) *DNSEntryService {
	return &DNSEntryService{database: database, dnsServer: dnsServer}
}

func (s *DNSEntryService) List(tld string) ([]db.DNSEntry, error) {
	if tld != "" {
		return db.ListDNSEntriesByTLD(s.database, tld)
	}
	return db.ListDNSEntries(s.database)
}

func (s *DNSEntryService) Add(domainName, tld, target string) (*db.DNSEntry, error) {
	domainName = strings.TrimSpace(strings.ToLower(domainName))
	tld = strings.TrimSpace(strings.TrimPrefix(tld, "."))
	target = strings.TrimSpace(target)

	if domainName == "" {
		return nil, fmt.Errorf("domain is required")
	}
	if tld == "" {
		return nil, fmt.Errorf("tld is required")
	}

	// Validate domain format
	if _, err := ValidateDomain(domainName); err != nil {
		return nil, err
	}

	if target == "" {
		target = "127.0.0.1"
	}

	entry, err := db.CreateDNSEntry(s.database, domainName, tld, target)
	if err != nil {
		return nil, err
	}
	s.SyncEntries()
	return entry, nil
}

func (s *DNSEntryService) Remove(domainName string) error {
	if err := db.DeleteDNSEntry(s.database, domainName); err != nil {
		return err
	}
	s.SyncEntries()
	return nil
}

func (s *DNSEntryService) SyncEntries() {
	entries, err := db.ListDNSEntries(s.database)
	if err != nil {
		return
	}
	dnsEntries := make([]dns.DNSEntry, len(entries))
	for i, e := range entries {
		dnsEntries[i] = dns.DNSEntry{Domain: e.Domain, Target: e.Target}
	}
	s.dnsServer.SetEntries(dnsEntries)
}
