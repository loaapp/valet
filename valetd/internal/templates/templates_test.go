package templates

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGet(t *testing.T) {
	slugs := []string{"simple", "static", "spa-api", "websocket", "cors-proxy", "multi-upstream", "custom"}
	for _, slug := range slugs {
		t.Run(slug, func(t *testing.T) {
			tmpl := Get(slug)
			if tmpl == nil {
				t.Fatalf("Get(%q) returned nil", slug)
			}
			if tmpl.Slug != slug {
				t.Errorf("Slug = %q, want %q", tmpl.Slug, slug)
			}
			if tmpl.Name == "" {
				t.Error("Name is empty")
			}
			if tmpl.Description == "" {
				t.Error("Description is empty")
			}
		})
	}

	if Get("nonexistent") != nil {
		t.Error("Get(nonexistent) should return nil")
	}
}

func TestRegistryLength(t *testing.T) {
	if len(Registry) != 7 {
		t.Errorf("Registry has %d templates, want 7", len(Registry))
	}
}

func TestSimple(t *testing.T) {
	tmpl := Get("simple")
	match, handler, err := tmpl.Apply(nil)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if match != "" || handler != "" {
		t.Errorf("simple template should return empty configs, got match=%q handler=%q", match, handler)
	}
}

func TestStatic(t *testing.T) {
	tmpl := Get("static")

	// Missing required root
	_, _, err := tmpl.Apply(map[string]string{})
	if err == nil {
		t.Error("expected error for missing root")
	}

	// Basic file server
	_, handler, err := tmpl.Apply(map[string]string{"root": "/Users/me/project/dist"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !json.Valid([]byte(handler)) {
		t.Fatal("handler is not valid JSON")
	}
	if !strings.Contains(handler, "file_server") {
		t.Error("handler should contain file_server")
	}
	if !strings.Contains(handler, "/Users/me/project/dist") {
		t.Error("handler should contain root path")
	}
	if strings.Contains(handler, "browse") {
		t.Error("browse should not be set by default")
	}

	// With directory browsing
	_, handler, err = tmpl.Apply(map[string]string{
		"root":   "/Users/me/docs",
		"browse": "true",
	})
	if err != nil {
		t.Fatalf("Apply with browse: %v", err)
	}
	if !strings.Contains(handler, "browse") {
		t.Error("handler should contain browse when enabled")
	}
}

func TestSPAAPI(t *testing.T) {
	tmpl := Get("spa-api")

	// Missing required params
	_, _, err := tmpl.Apply(map[string]string{})
	if err == nil {
		t.Error("expected error for missing required params")
	}

	// Valid params
	_, handler, err := tmpl.Apply(map[string]string{
		"frontendUpstream": "localhost:3000",
		"apiUpstream":      "localhost:8080",
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !json.Valid([]byte(handler)) {
		t.Fatal("handler is not valid JSON")
	}
	if !strings.Contains(handler, "localhost:3000") {
		t.Error("handler should contain frontend upstream")
	}
	if !strings.Contains(handler, "localhost:8080") {
		t.Error("handler should contain API upstream")
	}
	if !strings.Contains(handler, "/api/*") {
		t.Error("handler should contain default apiPath")
	}

	// Custom apiPath
	_, handler, err = tmpl.Apply(map[string]string{
		"frontendUpstream": "localhost:3000",
		"apiUpstream":      "localhost:8080",
		"apiPath":          "/v1/*",
	})
	if err != nil {
		t.Fatalf("Apply with custom apiPath: %v", err)
	}
	if !strings.Contains(handler, "/v1/*") {
		t.Error("handler should contain custom apiPath")
	}
}

func TestWebsocket(t *testing.T) {
	tmpl := Get("websocket")

	// Missing upstream
	_, _, err := tmpl.Apply(map[string]string{})
	if err == nil {
		t.Error("expected error for missing upstream")
	}

	// Valid
	_, handler, err := tmpl.Apply(map[string]string{"upstream": "localhost:9000"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !json.Valid([]byte(handler)) {
		t.Fatal("handler is not valid JSON")
	}
	if !strings.Contains(handler, "flush_interval") {
		t.Error("handler should contain flush_interval")
	}
}

func TestCORSProxy(t *testing.T) {
	tmpl := Get("cors-proxy")

	// Missing upstream
	_, _, err := tmpl.Apply(map[string]string{})
	if err == nil {
		t.Error("expected error for missing upstream")
	}

	// Default origin
	_, handler, err := tmpl.Apply(map[string]string{"upstream": "localhost:8080"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !json.Valid([]byte(handler)) {
		t.Fatal("handler is not valid JSON")
	}
	if !strings.Contains(handler, `"*"`) {
		t.Error("handler should contain default allow-origin *")
	}

	// Custom origin
	_, handler, err = tmpl.Apply(map[string]string{
		"upstream":    "localhost:8080",
		"allowOrigin": "https://example.com",
	})
	if err != nil {
		t.Fatalf("Apply custom origin: %v", err)
	}
	if !strings.Contains(handler, "https://example.com") {
		t.Error("handler should contain custom origin")
	}
}

func TestMultiUpstream(t *testing.T) {
	tmpl := Get("multi-upstream")

	// Missing required param
	_, _, err := tmpl.Apply(map[string]string{})
	if err == nil {
		t.Error("expected error for missing upstreams param")
	}

	// Valid
	_, handler, err := tmpl.Apply(map[string]string{
		"upstreams": "localhost:8080,localhost:8081,localhost:8082",
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !json.Valid([]byte(handler)) {
		t.Fatal("handler is not valid JSON")
	}
	if !strings.Contains(handler, "round_robin") {
		t.Error("handler should contain round_robin policy")
	}
	if !strings.Contains(handler, "localhost:8081") {
		t.Error("handler should contain all upstreams")
	}

	// Spaces in CSV
	_, handler, err = tmpl.Apply(map[string]string{
		"upstreams": " localhost:8080 , localhost:8081 ",
	})
	if err != nil {
		t.Fatalf("Apply with spaces: %v", err)
	}
	if !strings.Contains(handler, "localhost:8080") {
		t.Error("handler should trim spaces and contain upstream")
	}
}

func TestCustom(t *testing.T) {
	tmpl := Get("custom")

	// Missing required handlerJson
	_, _, err := tmpl.Apply(map[string]string{})
	if err == nil {
		t.Error("expected error for missing handlerJson")
	}

	// Invalid JSON
	_, _, err = tmpl.Apply(map[string]string{"handlerJson": "not json"})
	if err == nil {
		t.Error("expected error for invalid handler JSON")
	}

	// Valid handler only
	match, handler, err := tmpl.Apply(map[string]string{
		"handlerJson": `[{"handler":"reverse_proxy","upstreams":[{"dial":"localhost:3000"}]}]`,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !json.Valid([]byte(handler)) {
		t.Error("handler should be valid JSON")
	}
	if match != "" {
		t.Error("match should be empty when matchJson not provided")
	}

	// Valid handler + match
	match, handler, err = tmpl.Apply(map[string]string{
		"handlerJson": `[{"handler":"file_server","root":"/tmp"}]`,
		"matchJson":   `[{"path":["/docs/*"]}]`,
	})
	if err != nil {
		t.Fatalf("Apply with match: %v", err)
	}
	if !json.Valid([]byte(handler)) {
		t.Error("handler should be valid JSON")
	}
	if !json.Valid([]byte(match)) {
		t.Error("match should be valid JSON")
	}

	// Invalid matchJson
	_, _, err = tmpl.Apply(map[string]string{
		"handlerJson": `[{"handler":"reverse_proxy"}]`,
		"matchJson":   "broken",
	})
	if err == nil {
		t.Error("expected error for invalid match JSON")
	}
}

func TestRequiredParamValidation(t *testing.T) {
	tmpl := Get("spa-api")

	// Only one of two required params
	_, _, err := tmpl.Apply(map[string]string{
		"frontendUpstream": "localhost:3000",
	})
	if err == nil {
		t.Error("expected error when missing apiUpstream")
	}
	if !strings.Contains(err.Error(), "apiUpstream") {
		t.Errorf("error should mention apiUpstream, got: %v", err)
	}
}
