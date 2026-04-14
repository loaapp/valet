package api

import (
	"context"

	"github.com/loaapp/valet/pkg/client"
	"github.com/loaapp/valet/pkg/models"
)

type TLDService struct {
	ctx    context.Context
	client *client.Client
}

func NewTLDService(c *client.Client) *TLDService {
	return &TLDService{client: c}
}

func (s *TLDService) SetContext(ctx context.Context) { s.ctx = ctx }

func (s *TLDService) List() ([]models.ManagedTLD, error) {
	tlds, err := s.client.ListTLDs()
	if err != nil {
		return nil, err
	}
	if tlds == nil {
		tlds = []models.ManagedTLD{}
	}
	return tlds, nil
}

func (s *TLDService) Create(tld string) (*models.ManagedTLD, error) {
	return s.client.CreateTLD(tld)
}

func (s *TLDService) Delete(tld string) error {
	return s.client.DeleteTLD(tld)
}
