package domain

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/loaapp/valet/valetd/internal/db"
	"github.com/loaapp/valet/valetd/internal/dns"
	"github.com/loaapp/valet/valetd/internal/resolver"
)

type TLDService struct {
	database  *sql.DB
	dnsServer *dns.Server
}

func NewTLDService(database *sql.DB, dnsServer *dns.Server) *TLDService {
	return &TLDService{database: database, dnsServer: dnsServer}
}

func (s *TLDService) List() ([]TLDStatus, error) {
	tlds, err := db.ListTLDs(s.database)
	if err != nil {
		return nil, err
	}
	var result []TLDStatus
	for _, t := range tlds {
		result = append(result, TLDStatus{
			TLD:               t.TLD,
			ResolverInstalled: resolver.IsInstalled(t.TLD),
			CreatedAt:         t.CreatedAt,
		})
	}
	return result, nil
}

func (s *TLDService) Register(tld string) (*db.ManagedTLD, error) {
	tld = strings.TrimPrefix(strings.TrimSpace(tld), ".")
	if tld == "" {
		return nil, fmt.Errorf("tld is required")
	}

	existing, _ := db.GetTLD(s.database, tld)
	if existing != nil {
		return nil, fmt.Errorf("TLD .%s already managed", tld)
	}

	result, err := db.CreateTLD(s.database, tld)
	if err != nil {
		return nil, err
	}
	s.SyncDNS()
	return result, nil
}

func (s *TLDService) Unregister(tld string) error {
	tld = strings.TrimPrefix(strings.TrimSpace(tld), ".")
	if err := db.DeleteTLD(s.database, tld); err != nil {
		return err
	}
	resolver.Remove(tld)
	s.SyncDNS()
	return nil
}

func (s *TLDService) SyncDNS() {
	tlds, err := db.ListTLDs(s.database)
	if err != nil {
		return
	}
	tldList := make([]string, len(tlds))
	for i, t := range tlds {
		tldList[i] = t.TLD
	}
	s.dnsServer.SetTLDs(tldList)
}
