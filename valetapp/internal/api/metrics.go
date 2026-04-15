package api

import (
	"context"

	"github.com/loaapp/valet/pkg/client"
)

type MetricsService struct {
	ctx    context.Context
	client *client.Client
}

func NewMetricsService(c *client.Client) *MetricsService {
	return &MetricsService{client: c}
}

func (s *MetricsService) SetContext(ctx context.Context) { s.ctx = ctx }

func (s *MetricsService) GetCurrent() (map[string]any, error) {
	return s.client.GetMetricsCurrent()
}

func (s *MetricsService) GetHistory(rangeStr string) (map[string]any, error) {
	return s.client.GetMetricsHistory(rangeStr, "")
}
