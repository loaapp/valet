package api

import (
	"context"

	"github.com/loaapp/valet/pkg/client"
	"github.com/loaapp/valet/pkg/models"
)

type StatusService struct {
	ctx    context.Context
	client *client.Client
}

func NewStatusService(c *client.Client) *StatusService {
	return &StatusService{client: c}
}

func (s *StatusService) SetContext(ctx context.Context) { s.ctx = ctx }

func (s *StatusService) GetStatus() (*models.DaemonStatus, error) {
	return s.client.GetStatus()
}

func (s *StatusService) IsDaemonRunning() bool {
	return s.client.IsDaemonRunning()
}

func (s *StatusService) GetDNSStatus() ([]map[string]any, error) {
	return s.client.GetDNSStatus()
}
