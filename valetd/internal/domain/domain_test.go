package domain

import "testing"

func TestExtractTLD(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"myapp.test", "test"},
		{"api.myapp.test", "test"},
		{"a.b.c.example.com", "com"},
		{"localhost", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractTLD(tt.input)
			if got != tt.want {
				t.Errorf("extractTLD(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractParentDomain(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"api.myapp.test", "myapp.test"},
		{"a.b.c.test", "b.c.test"},
		{"myapp.test", "myapp.test"},     // 2 parts — returns as-is
		{"localhost", "localhost"},         // 1 part — returns as-is
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractParentDomain(tt.input)
			if got != tt.want {
				t.Errorf("extractParentDomain(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveTemplate_NoTemplate(t *testing.T) {
	req := &AddRouteRequest{
		Domain:   "myapp.test",
		Upstream: "localhost:3000",
	}
	if err := ResolveTemplate(req); err != nil {
		t.Fatalf("ResolveTemplate with no template: %v", err)
	}
	// Should not modify configs
	if req.MatchConfig != "" || req.HandlerConfig != "" {
		t.Error("should not set configs when no template")
	}
}

func TestResolveTemplate_UnknownTemplate(t *testing.T) {
	req := &AddRouteRequest{
		Template: "nonexistent",
	}
	err := ResolveTemplate(req)
	if err == nil {
		t.Fatal("expected error for unknown template")
	}
}

func TestResolveTemplate_Simple(t *testing.T) {
	req := &AddRouteRequest{
		Domain:   "myapp.test",
		Upstream: "localhost:3000",
		Template: "simple",
	}
	if err := ResolveTemplate(req); err != nil {
		t.Fatalf("ResolveTemplate simple: %v", err)
	}
	// Simple template produces no config overrides
	if req.MatchConfig != "" || req.HandlerConfig != "" {
		t.Error("simple template should not set configs")
	}
}

func TestResolveTemplate_SPAAPI(t *testing.T) {
	req := &AddRouteRequest{
		Domain:   "myapp.test",
		Upstream: "localhost:3000",
		Template: "spa-api",
		TemplateParams: map[string]string{
			"frontendUpstream": "localhost:3000",
			"apiUpstream":      "localhost:8080",
		},
	}
	if err := ResolveTemplate(req); err != nil {
		t.Fatalf("ResolveTemplate spa-api: %v", err)
	}
	if req.HandlerConfig == "" {
		t.Error("spa-api should set HandlerConfig")
	}
}

func TestResolveTemplate_MissingRequiredParam(t *testing.T) {
	req := &AddRouteRequest{
		Domain:   "myapp.test",
		Template: "spa-api",
		// Missing frontendUpstream and apiUpstream
	}
	err := ResolveTemplate(req)
	if err == nil {
		t.Fatal("expected error for missing required params")
	}
}

func TestResolveTemplate_UpstreamPassedAsParam(t *testing.T) {
	// The upstream field should be available as a template param
	req := &AddRouteRequest{
		Domain:   "myapp.test",
		Upstream: "localhost:9000",
		Template: "websocket",
	}
	if err := ResolveTemplate(req); err != nil {
		t.Fatalf("ResolveTemplate websocket: %v", err)
	}
	if req.HandlerConfig == "" {
		t.Error("websocket should set HandlerConfig")
	}
}
