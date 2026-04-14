package templates

import (
	"encoding/json"
	"fmt"
	"strings"
)

// TemplateDef defines a route template with its parameters and Apply logic.
type TemplateDef struct {
	Slug        string
	Name        string
	Description string
	Params      []Param
	apply       func(params map[string]string) (matchConfig, handlerConfig string, err error)
}

// Param describes a single template parameter.
type Param struct {
	Key         string
	Label       string
	Placeholder string
	Required    bool
}

// Apply executes the template with the given parameters, returning match and handler JSON configs.
func (t *TemplateDef) Apply(params map[string]string) (matchConfig, handlerConfig string, err error) {
	// Validate required params
	for _, p := range t.Params {
		if p.Required {
			if v, ok := params[p.Key]; !ok || v == "" {
				return "", "", fmt.Errorf("required parameter %q is missing", p.Key)
			}
		}
	}
	return t.apply(params)
}

// Registry holds all available route templates.
var Registry = []TemplateDef{
	templateSimple(),
	templateSPAAPI(),
	templateWebsocket(),
	templateCORSProxy(),
	templateMultiUpstream(),
}

// Get returns the template with the given slug, or nil if not found.
func Get(slug string) *TemplateDef {
	for i := range Registry {
		if Registry[i].Slug == slug {
			return &Registry[i]
		}
	}
	return nil
}

func mustMarshal(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func templateSimple() TemplateDef {
	return TemplateDef{
		Slug:        "simple",
		Name:        "Simple Reverse Proxy",
		Description: "Routes traffic from a domain to a single upstream using the domain and upstream columns directly.",
		Params:      nil,
		apply: func(params map[string]string) (string, string, error) {
			return "", "", nil
		},
	}
}

func templateSPAAPI() TemplateDef {
	return TemplateDef{
		Slug:        "spa-api",
		Name:        "SPA + API Backend",
		Description: "Serves a single-page app frontend while proxying API requests to a separate backend.",
		Params: []Param{
			{Key: "frontendUpstream", Label: "Frontend Upstream", Placeholder: "localhost:3000", Required: true},
			{Key: "apiUpstream", Label: "API Upstream", Placeholder: "localhost:8080", Required: true},
			{Key: "apiPath", Label: "API Path", Placeholder: "/api/*", Required: false},
		},
		apply: func(params map[string]string) (string, string, error) {
			frontendUpstream := params["frontendUpstream"]
			apiUpstream := params["apiUpstream"]
			apiPath := params["apiPath"]
			if apiPath == "" {
				apiPath = "/api/*"
			}

			handler := []map[string]any{
				{
					"handler": "subroute",
					"routes": []map[string]any{
						{
							"match": []map[string]any{
								{"path": []string{apiPath}},
							},
							"handle": []map[string]any{
								{
									"handler":   "reverse_proxy",
									"upstreams": []map[string]any{{"dial": apiUpstream}},
								},
							},
						},
						{
							"handle": []map[string]any{
								{
									"handler":   "reverse_proxy",
									"upstreams": []map[string]any{{"dial": frontendUpstream}},
								},
							},
						},
					},
				},
			}

			return "", mustMarshal(handler), nil
		},
	}
}

func templateWebsocket() TemplateDef {
	return TemplateDef{
		Slug:        "websocket",
		Name:        "WebSocket Proxy",
		Description: "Proxies WebSocket connections with streaming support (flush_interval=-1).",
		Params:      nil,
		apply: func(params map[string]string) (string, string, error) {
			upstream := params["upstream"]
			if upstream == "" {
				return "", "", fmt.Errorf("upstream is required for websocket template")
			}

			handler := []map[string]any{
				{
					"handler":        "reverse_proxy",
					"upstreams":      []map[string]any{{"dial": upstream}},
					"flush_interval": -1,
				},
			}

			return "", mustMarshal(handler), nil
		},
	}
}

func templateCORSProxy() TemplateDef {
	return TemplateDef{
		Slug:        "cors-proxy",
		Name:        "CORS Proxy",
		Description: "Reverse proxy with CORS headers (Access-Control-Allow-Origin, Methods, Headers).",
		Params: []Param{
			{Key: "allowOrigin", Label: "Allow Origin", Placeholder: "*", Required: false},
		},
		apply: func(params map[string]string) (string, string, error) {
			upstream := params["upstream"]
			if upstream == "" {
				return "", "", fmt.Errorf("upstream is required for cors-proxy template")
			}

			allowOrigin := params["allowOrigin"]
			if allowOrigin == "" {
				allowOrigin = "*"
			}

			handler := []map[string]any{
				{
					"handler": "headers",
					"response": map[string]any{
						"set": map[string][]string{
							"Access-Control-Allow-Origin":  {allowOrigin},
							"Access-Control-Allow-Methods": {"GET, POST, PUT, DELETE, OPTIONS"},
							"Access-Control-Allow-Headers": {"Content-Type, Authorization"},
						},
					},
				},
				{
					"handler":   "reverse_proxy",
					"upstreams": []map[string]any{{"dial": upstream}},
				},
			}

			return "", mustMarshal(handler), nil
		},
	}
}

func templateMultiUpstream() TemplateDef {
	return TemplateDef{
		Slug:        "multi-upstream",
		Name:        "Multi-Upstream Load Balancer",
		Description: "Distributes traffic across multiple upstreams with round-robin load balancing.",
		Params: []Param{
			{Key: "upstreams", Label: "Upstreams (comma-separated)", Placeholder: "localhost:8080,localhost:8081", Required: true},
		},
		apply: func(params map[string]string) (string, string, error) {
			raw := params["upstreams"]
			if raw == "" {
				return "", "", fmt.Errorf("upstreams parameter is required")
			}

			parts := strings.Split(raw, ",")
			upstreams := make([]map[string]any, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					upstreams = append(upstreams, map[string]any{"dial": p})
				}
			}

			handler := []map[string]any{
				{
					"handler":   "reverse_proxy",
					"upstreams": upstreams,
					"load_balancing": map[string]any{
						"selection_policy": map[string]any{
							"policy": "round_robin",
						},
					},
				},
			}

			return "", mustMarshal(handler), nil
		},
	}
}
