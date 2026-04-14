package api

import (
	"context"

	"github.com/loaapp/valet/pkg/client"
)

type SettingsService struct {
	ctx    context.Context
	client *client.Client
}

func NewSettingsService(c *client.Client) *SettingsService {
	return &SettingsService{client: c}
}

func (s *SettingsService) SetContext(ctx context.Context) { s.ctx = ctx }

func (s *SettingsService) GetAll() (map[string]string, error) {
	settings, err := s.client.GetAllSettings()
	if err != nil {
		return nil, err
	}
	if settings == nil {
		settings = map[string]string{}
	}
	return settings, nil
}

func (s *SettingsService) Set(key, value string) error {
	return s.client.SetSetting(key, value)
}
