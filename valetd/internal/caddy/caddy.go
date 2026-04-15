package caddy

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"

	caddyv2 "github.com/caddyserver/caddy/v2"
	"github.com/loaapp/valet/valetd/internal/db"

	// Import standard Caddy modules so they register when embedded
	_ "github.com/caddyserver/caddy/v2/modules/standard"
)

// BuildConfig generates a full Caddy JSON config from the current routes.
// combinedCert/combinedKey point to a single mkcert cert covering all domains + localhost.
func BuildConfig(routes []db.Route, combinedCert, combinedKey, dataDir string) ([]byte, error) {
	if len(routes) == 0 {
		return emptyConfig()
	}

	caddyRoutes := make([]map[string]any, 0, len(routes))
	for _, r := range routes {
		route := buildRouteHandler(r)
		caddyRoutes = append(caddyRoutes, route)
	}

	// Build the server config
	server := map[string]any{
		"listen": []string{":443", ":80"},
		"routes": caddyRoutes,
		"logs": map[string]any{
			"default_logger_name": "access",
		},
	}

	// If we have a combined cert, configure TLS with default_sni so that
	// connections via localhost/127.0.0.1 (no matching SNI) still get served.
	if combinedCert != "" && combinedKey != "" {
		// Pick the first domain as the default SNI
		var defaultSNI string
		for _, r := range routes {
			if r.TLSEnabled {
				defaultSNI = r.Domain
				break
			}
		}

		server["tls_connection_policies"] = []map[string]any{
			{
				"default_sni": defaultSNI,
			},
		}
	}

	tlsApp := buildTLSApp(routes, combinedCert, combinedKey)

	cfg := map[string]any{
		"admin": map[string]any{
			"listen": "127.0.0.1:2019",
		},
		"logging": map[string]any{
			"logs": map[string]any{
				"access": map[string]any{
					"writer": map[string]any{
						"output":   "file",
						"filename": filepath.Join(dataDir, "access.log"),
					},
					"encoder": map[string]any{"format": "json"},
					"include": []string{"http.log.access.*"},
				},
			},
		},
		"apps": map[string]any{
			"http": map[string]any{
				"metrics": map[string]any{
					"per_host": true,
				},
				"servers": map[string]any{
					"valet": server,
				},
			},
		},
	}

	if tlsApp != nil {
		apps := cfg["apps"].(map[string]any)
		apps["tls"] = tlsApp
	}

	return json.Marshal(cfg)
}

func buildRouteHandler(r db.Route) map[string]any {
	// Determine matchers
	var matchers []map[string]any
	if r.MatchConfig != "" {
		if err := json.Unmarshal([]byte(r.MatchConfig), &matchers); err != nil {
			log.Printf("WARN: invalid match_config for route %s, falling back to host match: %v", r.ID, err)
			matchers = nil
		}
	}
	if matchers == nil {
		matchers = []map[string]any{
			{"host": []string{r.Domain}},
		}
	}

	// Determine handlers
	var handlers []map[string]any
	if r.HandlerConfig != "" {
		if err := json.Unmarshal([]byte(r.HandlerConfig), &handlers); err != nil {
			log.Printf("WARN: invalid handler_config for route %s, falling back to reverse_proxy: %v", r.ID, err)
			handlers = nil
		}
	}
	if handlers == nil {
		handlers = []map[string]any{
			{
				"handler": "reverse_proxy",
				"upstreams": []map[string]any{
					{"dial": r.Upstream},
				},
			},
		}
	}

	route := map[string]any{
		"match":  matchers,
		"handle": handlers,
	}

	if r.TLSEnabled && r.CertPath != "" && r.KeyPath != "" {
		route["terminal"] = true
	}

	return route
}

// BuildRoutePreview builds and returns the Caddy JSON for a single route (for preview).
func BuildRoutePreview(r db.Route) ([]byte, error) {
	handler := buildRouteHandler(r)
	return json.MarshalIndent(handler, "", "  ")
}

func buildTLSApp(routes []db.Route, combinedCert, combinedKey string) map[string]any {
	if combinedCert == "" || combinedKey == "" {
		return nil
	}

	var subjects []string
	for _, r := range routes {
		if r.TLSEnabled {
			subjects = append(subjects, r.Domain)
		}
	}

	if len(subjects) == 0 {
		return nil
	}

	// Single combined cert covering all domains + localhost + 127.0.0.1
	tlsApp := map[string]any{
		"certificates": map[string]any{
			"load_files": []map[string]any{
				{
					"certificate": combinedCert,
					"key":         combinedKey,
					"tags":        []string{"valet"},
				},
			},
		},
		"automation": map[string]any{
			"policies": []map[string]any{
				{
					"subjects": subjects,
					"issuers": []map[string]any{
						{"module": "internal"},
					},
				},
			},
		},
	}

	return tlsApp
}

func emptyConfig() ([]byte, error) {
	cfg := map[string]any{
		"admin": map[string]any{
			"listen": "127.0.0.1:2019",
		},
	}
	return json.Marshal(cfg)
}

// Load applies the given Caddy JSON config.
func Load(configJSON []byte) error {
	log.Printf("Loading Caddy config (%d bytes)", len(configJSON))
	return caddyv2.Load(configJSON, false)
}

// Stop gracefully stops the embedded Caddy instance.
func Stop() error {
	return caddyv2.Stop()
}

// Reload builds a new config from routes and applies it.
func Reload(routes []db.Route, combinedCert, combinedKey, dataDir string) error {
	cfg, err := BuildConfig(routes, combinedCert, combinedKey, dataDir)
	if err != nil {
		return fmt.Errorf("build config: %w", err)
	}
	return Load(cfg)
}
