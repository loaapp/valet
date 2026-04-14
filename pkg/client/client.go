package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/loaapp/valet/pkg/models"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func New() *Client {
	return &Client{
		BaseURL: "http://localhost:7800",
		HTTP: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Status

func (c *Client) GetStatus() (*models.DaemonStatus, error) {
	var status models.DaemonStatus
	if err := c.get("/api/v1/status", &status); err != nil {
		return nil, err
	}
	return &status, nil
}

func (c *Client) IsDaemonRunning() bool {
	quick := &http.Client{Timeout: 1 * time.Second}
	resp, err := quick.Get(c.BaseURL + "/api/v1/status")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func (c *Client) Trust() error {
	return c.post("/api/v1/trust", nil, nil)
}

// Routes

func (c *Client) ListRoutes() ([]models.Route, error) {
	var routes []models.Route
	if err := c.get("/api/v1/routes", &routes); err != nil {
		return nil, err
	}
	return routes, nil
}

func (c *Client) GetRoute(id string) (*models.Route, error) {
	var route models.Route
	if err := c.get("/api/v1/routes/"+id, &route); err != nil {
		return nil, err
	}
	return &route, nil
}

func (c *Client) CreateRoute(req models.CreateRouteRequest) (*models.Route, error) {
	var route models.Route
	if err := c.post("/api/v1/routes", req, &route); err != nil {
		return nil, err
	}
	return &route, nil
}

func (c *Client) UpdateRoute(id string, req models.CreateRouteRequest) (*models.Route, error) {
	var route models.Route
	if err := c.put("/api/v1/routes/"+id, req, &route); err != nil {
		return nil, err
	}
	return &route, nil
}

func (c *Client) ListTemplates() ([]models.Template, error) {
	var tmpls []models.Template
	if err := c.get("/api/v1/templates", &tmpls); err != nil {
		return nil, err
	}
	return tmpls, nil
}

func (c *Client) PreviewRoute(req models.CreateRouteRequest) (string, error) {
	var raw json.RawMessage
	if err := c.post("/api/v1/routes/preview", req, &raw); err != nil {
		return "", err
	}
	return string(raw), nil
}

func (c *Client) DeleteRoute(id string) error {
	return c.del("/api/v1/routes/" + id)
}

// TLDs

func (c *Client) ListTLDs() ([]models.ManagedTLD, error) {
	var tlds []models.ManagedTLD
	if err := c.get("/api/v1/tlds", &tlds); err != nil {
		return nil, err
	}
	return tlds, nil
}

func (c *Client) CreateTLD(tld string) (*models.ManagedTLD, error) {
	body := map[string]string{"tld": tld}
	var result models.ManagedTLD
	if err := c.post("/api/v1/tlds", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteTLD(tld string) error {
	return c.del("/api/v1/tlds/" + tld)
}

// Settings

func (c *Client) GetAllSettings() (map[string]string, error) {
	var settings map[string]string
	if err := c.get("/api/v1/settings", &settings); err != nil {
		return nil, err
	}
	return settings, nil
}

func (c *Client) SetSetting(key, value string) error {
	body := map[string]string{"value": value}
	return c.put("/api/v1/settings/"+key, body, nil)
}

// Internal HTTP helpers

func (c *Client) get(path string, out any) error {
	resp, err := c.HTTP.Get(c.BaseURL + path)
	if err != nil {
		return fmt.Errorf("cannot connect to valetd: %w", err)
	}
	defer resp.Body.Close()
	return c.decodeResponse(resp, out)
}

func (c *Client) post(path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	resp, err := c.HTTP.Post(c.BaseURL+path, "application/json", reader)
	if err != nil {
		return fmt.Errorf("cannot connect to valetd: %w", err)
	}
	defer resp.Body.Close()
	return c.decodeResponse(resp, out)
}

func (c *Client) put(path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req, _ := http.NewRequest("PUT", c.BaseURL+path, reader)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("cannot connect to valetd: %w", err)
	}
	defer resp.Body.Close()
	return c.decodeResponse(resp, out)
}

func (c *Client) del(path string) error {
	req, _ := http.NewRequest("DELETE", c.BaseURL+path, nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("cannot connect to valetd: %w", err)
	}
	defer resp.Body.Close()
	return c.decodeResponse(resp, nil)
}

func (c *Client) decodeResponse(resp *http.Response, out any) error {
	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		var errResp map[string]string
		json.Unmarshal(data, &errResp)
		if msg, ok := errResp["error"]; ok {
			return fmt.Errorf("%s", msg)
		}
		return fmt.Errorf("API error: %s", resp.Status)
	}

	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}
