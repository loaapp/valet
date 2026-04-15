package api

import (
	"context"

	"github.com/loaapp/valet/pkg/client"
)

type LogsService struct {
	ctx    context.Context
	client *client.Client
}

func NewLogsService(c *client.Client) *LogsService {
	return &LogsService{client: c}
}

func (s *LogsService) SetContext(ctx context.Context) { s.ctx = ctx }

func (s *LogsService) GetLogs(limit int) ([]map[string]any, error) {
	return s.client.GetLogs(limit, 0, "")
}

func (s *LogsService) GetLogsSince(since float64) ([]map[string]any, error) {
	return s.client.GetLogs(0, since, "")
}

func (s *LogsService) GetDNSLogs(limit int) ([]map[string]any, error) {
	return s.client.GetDNSLogs(limit)
}
