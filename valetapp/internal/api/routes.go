package api

import (
	"context"

	"github.com/loaapp/valet/pkg/client"
	"github.com/loaapp/valet/pkg/models"
)

type RouteService struct {
	ctx    context.Context
	client *client.Client
}

func NewRouteService(c *client.Client) *RouteService {
	return &RouteService{client: c}
}

func (s *RouteService) SetContext(ctx context.Context) { s.ctx = ctx }

func (s *RouteService) List() ([]models.Route, error) {
	routes, err := s.client.ListRoutes()
	if err != nil {
		return nil, err
	}
	if routes == nil {
		routes = []models.Route{}
	}
	return routes, nil
}

func (s *RouteService) Get(id string) (*models.Route, error) {
	return s.client.GetRoute(id)
}

func (s *RouteService) Create(req models.CreateRouteRequest) (*models.Route, error) {
	return s.client.CreateRoute(req)
}

func (s *RouteService) Update(id string, req models.CreateRouteRequest) (*models.Route, error) {
	return s.client.UpdateRoute(id, req)
}

func (s *RouteService) Delete(id string) error {
	return s.client.DeleteRoute(id)
}

func (s *RouteService) ListTemplates() ([]models.Template, error) {
	return s.client.ListTemplates()
}

func (s *RouteService) Preview(req models.CreateRouteRequest) (string, error) {
	return s.client.PreviewRoute(req)
}
