package caddy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/loaapp/valet/valetd/internal/db"
)

func TestBuildRouteHandler_DefaultProxy(t *testing.T) {
	r := db.Route{
		ID:       "1",
		Domain:   "myapp.test",
		Upstream: "localhost:3000",
	}

	handler := buildRouteHandler(r)

	// Should have host matcher
	matchers, ok := handler["match"].([]map[string]any)
	if !ok || len(matchers) == 0 {
		t.Fatal("expected host matchers")
	}
	hosts, ok := matchers[0]["host"].([]string)
	if !ok || len(hosts) != 1 || hosts[0] != "myapp.test" {
		t.Errorf("expected host matcher for myapp.test, got %v", matchers[0])
	}

	// Should have reverse_proxy handler
	handlers, ok := handler["handle"].([]map[string]any)
	if !ok || len(handlers) == 0 {
		t.Fatal("expected handlers")
	}
	if handlers[0]["handler"] != "reverse_proxy" {
		t.Errorf("handler = %v, want reverse_proxy", handlers[0]["handler"])
	}
}

func TestBuildRouteHandler_CustomMatchConfig(t *testing.T) {
	r := db.Route{
		ID:          "1",
		Domain:      "myapp.test",
		Upstream:    "localhost:3000",
		MatchConfig: `[{"path":["/api/*"]}]`,
	}

	handler := buildRouteHandler(r)
	matchers := handler["match"].([]map[string]any)

	// Should use custom matcher, not host
	if _, hasHost := matchers[0]["host"]; hasHost {
		t.Error("should use custom match config, not host matcher")
	}
	paths, ok := matchers[0]["path"].([]any)
	if !ok || len(paths) == 0 {
		t.Fatal("expected path matcher")
	}
}

func TestBuildRouteHandler_InvalidMatchConfig_FallsBack(t *testing.T) {
	r := db.Route{
		ID:          "1",
		Domain:      "myapp.test",
		Upstream:    "localhost:3000",
		MatchConfig: `{not valid json`,
	}

	handler := buildRouteHandler(r)
	matchers := handler["match"].([]map[string]any)

	// Should fall back to host matcher
	hosts, ok := matchers[0]["host"].([]string)
	if !ok || hosts[0] != "myapp.test" {
		t.Error("expected fallback to host matcher on invalid JSON")
	}
}

func TestBuildRouteHandler_CustomHandlerConfig(t *testing.T) {
	handlerJSON := `[{"handler":"subroute","routes":[]}]`
	r := db.Route{
		ID:            "1",
		Domain:        "myapp.test",
		HandlerConfig: handlerJSON,
	}

	handler := buildRouteHandler(r)
	handlers := handler["handle"].([]map[string]any)

	if handlers[0]["handler"] != "subroute" {
		t.Errorf("expected custom handler subroute, got %v", handlers[0]["handler"])
	}
}

func TestBuildRouteHandler_InvalidHandlerConfig_FallsBack(t *testing.T) {
	r := db.Route{
		ID:            "1",
		Domain:        "myapp.test",
		Upstream:      "localhost:3000",
		HandlerConfig: `not json`,
	}

	handler := buildRouteHandler(r)
	handlers := handler["handle"].([]map[string]any)

	// Should fall back to reverse_proxy
	if handlers[0]["handler"] != "reverse_proxy" {
		t.Error("expected fallback to reverse_proxy on invalid handler JSON")
	}
}

func TestBuildRoutePreview(t *testing.T) {
	r := db.Route{
		ID:       "1",
		Domain:   "myapp.test",
		Upstream: "localhost:3000",
	}

	out, err := BuildRoutePreview(r)
	if err != nil {
		t.Fatalf("BuildRoutePreview: %v", err)
	}
	if !json.Valid(out) {
		t.Fatal("output is not valid JSON")
	}
	if !strings.Contains(string(out), "myapp.test") {
		t.Error("preview should contain domain")
	}
	if !strings.Contains(string(out), "localhost:3000") {
		t.Error("preview should contain upstream")
	}
}

func TestBuildConfig_Empty(t *testing.T) {
	cfg, err := BuildConfig(nil, "", "", "")
	if err != nil {
		t.Fatalf("BuildConfig empty: %v", err)
	}
	if !json.Valid(cfg) {
		t.Fatal("empty config is not valid JSON")
	}
}

func TestBuildConfig_SingleRoute(t *testing.T) {
	routes := []db.Route{
		{
			ID:         "1",
			Domain:     "myapp.test",
			Upstream:   "localhost:3000",
			TLSEnabled: true,
		},
	}

	cfg, err := BuildConfig(routes, "/tmp/cert.pem", "/tmp/key.pem", "")
	if err != nil {
		t.Fatalf("BuildConfig: %v", err)
	}
	if !json.Valid(cfg) {
		t.Fatal("config is not valid JSON")
	}

	var parsed map[string]any
	if err := json.Unmarshal(cfg, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Should have apps.http
	apps, ok := parsed["apps"].(map[string]any)
	if !ok {
		t.Fatal("missing apps key")
	}
	httpApp, ok := apps["http"].(map[string]any)
	if !ok {
		t.Fatal("missing apps.http")
	}

	// Should have servers
	servers, ok := httpApp["servers"].(map[string]any)
	if !ok {
		t.Fatal("missing servers")
	}
	if _, ok := servers["valet"]; !ok {
		t.Fatal("missing valet server")
	}
}

func TestBuildConfig_MultipleRoutes(t *testing.T) {
	routes := []db.Route{
		{ID: "1", Domain: "app1.test", Upstream: "localhost:3000", TLSEnabled: true},
		{ID: "2", Domain: "app2.test", Upstream: "localhost:4000", TLSEnabled: true},
	}

	cfg, err := BuildConfig(routes, "/tmp/cert.pem", "/tmp/key.pem", "")
	if err != nil {
		t.Fatalf("BuildConfig: %v", err)
	}

	s := string(cfg)
	if !strings.Contains(s, "app1.test") {
		t.Error("config should contain app1.test")
	}
	if !strings.Contains(s, "app2.test") {
		t.Error("config should contain app2.test")
	}
	if !strings.Contains(s, "localhost:3000") {
		t.Error("config should contain localhost:3000")
	}
	if !strings.Contains(s, "localhost:4000") {
		t.Error("config should contain localhost:4000")
	}
}
