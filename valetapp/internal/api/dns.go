package api

import (
	"context"

	"github.com/loaapp/valet/pkg/client"
	"github.com/loaapp/valet/pkg/models"
)

type DNSService struct {
	ctx    context.Context
	client *client.Client
}

func NewDNSService(c *client.Client) *DNSService {
	return &DNSService{client: c}
}

func (s *DNSService) SetContext(ctx context.Context) { s.ctx = ctx }

func (s *DNSService) ListEntries(tld string) ([]models.DNSEntry, error) {
	entries, err := s.client.ListDNSEntries(tld)
	if err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []models.DNSEntry{}
	}
	return entries, nil
}

func (s *DNSService) CreateEntry(domain, tld, target string) (*models.DNSEntry, error) {
	return s.client.CreateDNSEntry(domain, tld, target)
}

func (s *DNSService) DeleteEntry(domain string) error {
	return s.client.DeleteDNSEntry(domain)
}
