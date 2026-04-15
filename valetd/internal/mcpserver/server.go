// Copyright 2025 Richard Clayton. All rights reserved.
// Proprietary — see LICENSE.md.

package mcpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/loaapp/valet/valetd/internal/certs"
	"github.com/loaapp/valet/valetd/internal/db"
	"github.com/loaapp/valet/valetd/internal/resolver"
	"github.com/loaapp/valet/valetd/internal/routes"
	"github.com/loaapp/valet/valetd/internal/templates"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPServer wraps an MCP server with valetd tools.
type MCPServer struct {
	db       *sql.DB
	routeMgr *routes.Manager
	certMgr  *certs.Manager
	server   *mcp.Server
}

// New creates a new MCPServer with all tools registered.
func New(database *sql.DB, routeMgr *routes.Manager, certMgr *certs.Manager) *MCPServer {
	s := &MCPServer{
		db:       database,
		routeMgr: routeMgr,
		certMgr:  certMgr,
	}
	s.server = mcp.NewServer(&mcp.Implementation{
		Name:    "valetd",
		Version: "1.0.0",
	}, nil)
	s.registerTools()
	return s
}

// Server returns the underlying MCP server.
func (s *MCPServer) Server() *mcp.Server {
	return s.server
}

func mustJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error())
	}
	return string(b)
}

func textResult(text string) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil
}

func errResult(err error) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
		IsError: true,
	}, nil
}

// registerTools adds all valetd tools to the MCP server.
func (s *MCPServer) registerTools() {
	s.addListRoutes()
	s.addAddRoute()
	s.addRemoveRoute()
	s.addUpdateRoute()
	s.addGetStatus()
	s.addListTLDs()
	s.addAddTLD()
	s.addRemoveTLD()
	s.addListTemplates()
	s.addPreviewRoute()
	s.addDiagnoseRoute()
	s.addTrust()
}

// --- list_routes ---

func (s *MCPServer) addListRoutes() {
	s.server.AddTool(
		&mcp.Tool{
			Name:        "list_routes",
			Description: "List all configured routes",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			routeList, err := db.ListRoutes(s.db)
			if err != nil {
				return errResult(err)
			}
			return textResult(mustJSON(routeList))
		},
	)
}

// --- add_route ---

type addRouteInput struct {
	Domain         string            `json:"domain" jsonschema:"the domain name (e.g. myapp.test)"`
	Upstream       string            `json:"upstream" jsonschema:"upstream address (e.g. localhost:3000)"`
	TLS            *bool             `json:"tls,omitempty" jsonschema:"enable TLS with mkcert"`
	Description    string            `json:"description,omitempty" jsonschema:"human-readable description"`
	Template       string            `json:"template,omitempty" jsonschema:"route template slug"`
	TemplateParams map[string]string `json:"templateParams,omitempty" jsonschema:"parameters for the template"`
}

func (s *MCPServer) addAddRoute() {
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "add_route",
		Description: "Add a new route mapping a domain to an upstream",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input addRouteInput) (*mcp.CallToolResult, any, error) {
		tlsEnabled := input.TLS != nil && *input.TLS

		var matchConfig, handlerConfig string
		if input.Template != "" {
			tmpl := templates.Get(input.Template)
			if tmpl == nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: unknown template %q", input.Template)}},
					IsError: true,
				}, nil, nil
			}
			params := input.TemplateParams
			if params == nil {
				params = map[string]string{}
			}
			var err error
			matchConfig, handlerConfig, err = tmpl.Apply(params)
			if err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: template apply: %v", err)}},
					IsError: true,
				}, nil, nil
			}
		}

		route, err := s.routeMgr.Add(input.Domain, input.Upstream, tlsEnabled, matchConfig, handlerConfig, input.Template, input.Description)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		return nil, route, nil
	})
}

// --- remove_route ---

type removeRouteInput struct {
	Domain string `json:"domain" jsonschema:"domain of the route to remove"`
}

func (s *MCPServer) addRemoveRoute() {
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "remove_route",
		Description: "Remove a route by domain",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input removeRouteInput) (*mcp.CallToolResult, any, error) {
		if err := s.routeMgr.Remove(input.Domain); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Route for %s removed", input.Domain)}},
		}, nil, nil
	})
}

// --- update_route ---

type updateRouteInput struct {
	ID          string `json:"id" jsonschema:"route ID"`
	Domain      string `json:"domain,omitempty" jsonschema:"new domain"`
	Upstream    string `json:"upstream,omitempty" jsonschema:"new upstream"`
	TLS         *bool  `json:"tls,omitempty" jsonschema:"enable or disable TLS"`
	Description string `json:"description,omitempty" jsonschema:"new description"`
}

func (s *MCPServer) addUpdateRoute() {
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "update_route",
		Description: "Update an existing route by ID",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input updateRouteInput) (*mcp.CallToolResult, any, error) {
		existing, err := db.GetRoute(s.db, input.ID)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		if existing == nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: route %s not found", input.ID)}},
				IsError: true,
			}, nil, nil
		}

		// Merge: use new values if provided, otherwise keep existing
		domain := existing.Domain
		if input.Domain != "" {
			domain = input.Domain
		}
		upstream := existing.Upstream
		if input.Upstream != "" {
			upstream = input.Upstream
		}
		tlsEnabled := existing.TLSEnabled
		if input.TLS != nil {
			tlsEnabled = *input.TLS
		}
		description := existing.Description
		if input.Description != "" {
			description = input.Description
		}

		updated, err := db.UpdateRoute(s.db, input.ID, domain, upstream, tlsEnabled,
			existing.CertPath, existing.KeyPath, existing.MatchConfig, existing.HandlerConfig,
			existing.Template, description)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
				IsError: true,
			}, nil, nil
		}

		if err := s.routeMgr.Sync(); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error syncing: %v", err)}},
				IsError: true,
			}, nil, nil
		}

		return nil, updated, nil
	})
}

// --- get_status ---

type statusOutput struct {
	RouteCount     int  `json:"routeCount"`
	TLDCount       int  `json:"tldCount"`
	MkcertAvailable bool `json:"mkcertAvailable"`
}

func (s *MCPServer) addGetStatus() {
	s.server.AddTool(
		&mcp.Tool{
			Name:        "get_status",
			Description: "Get valetd status: route count, TLD count, mkcert availability",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			routeList, err := db.ListRoutes(s.db)
			if err != nil {
				return errResult(err)
			}
			tldList, err := db.ListTLDs(s.db)
			if err != nil {
				return errResult(err)
			}
			out := statusOutput{
				RouteCount:      len(routeList),
				TLDCount:        len(tldList),
				MkcertAvailable: certs.MkcertAvailable(),
			}
			return textResult(mustJSON(out))
		},
	)
}

// --- list_tlds ---

func (s *MCPServer) addListTLDs() {
	s.server.AddTool(
		&mcp.Tool{
			Name:        "list_tlds",
			Description: "List all managed TLDs",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			tldList, err := db.ListTLDs(s.db)
			if err != nil {
				return errResult(err)
			}
			return textResult(mustJSON(tldList))
		},
	)
}

// --- add_tld ---

type addTLDInput struct {
	TLD string `json:"tld" jsonschema:"top-level domain to add (e.g. test)"`
}

func (s *MCPServer) addAddTLD() {
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "add_tld",
		Description: "Add a managed TLD",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input addTLDInput) (*mcp.CallToolResult, any, error) {
		tld, err := db.CreateTLD(s.db, input.TLD)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		if err := s.routeMgr.SyncDNS(); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error syncing DNS: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		return nil, tld, nil
	})
}

// --- remove_tld ---

type removeTLDInput struct {
	TLD string `json:"tld" jsonschema:"top-level domain to remove"`
}

func (s *MCPServer) addRemoveTLD() {
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "remove_tld",
		Description: "Remove a managed TLD",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input removeTLDInput) (*mcp.CallToolResult, any, error) {
		if err := db.DeleteTLD(s.db, input.TLD); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		// Best-effort remove resolver file
		_ = resolver.Remove(input.TLD)

		if err := s.routeMgr.SyncDNS(); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error syncing DNS: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("TLD .%s removed", input.TLD)}},
		}, nil, nil
	})
}

// --- list_templates ---

type templateInfo struct {
	Slug        string           `json:"slug"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Params      []templateParam  `json:"params,omitempty"`
}

type templateParam struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Placeholder string `json:"placeholder,omitempty"`
	Required    bool   `json:"required"`
}

func (s *MCPServer) addListTemplates() {
	s.server.AddTool(
		&mcp.Tool{
			Name:        "list_templates",
			Description: "List available route templates",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var infos []templateInfo
			for _, t := range templates.Registry {
				info := templateInfo{
					Slug:        t.Slug,
					Name:        t.Name,
					Description: t.Description,
				}
				for _, p := range t.Params {
					info.Params = append(info.Params, templateParam{
						Key:         p.Key,
						Label:       p.Label,
						Placeholder: p.Placeholder,
						Required:    p.Required,
					})
				}
				infos = append(infos, info)
			}
			return textResult(mustJSON(infos))
		},
	)
}

// --- preview_route ---

type previewRouteInput struct {
	Domain         string            `json:"domain" jsonschema:"domain for the preview"`
	Upstream       string            `json:"upstream" jsonschema:"upstream address"`
	Template       string            `json:"template,omitempty" jsonschema:"template slug to apply"`
	TemplateParams map[string]string `json:"templateParams,omitempty" jsonschema:"template parameters"`
}

type previewOutput struct {
	Domain        string `json:"domain"`
	Upstream      string `json:"upstream"`
	MatchConfig   string `json:"matchConfig,omitempty"`
	HandlerConfig string `json:"handlerConfig,omitempty"`
	Template      string `json:"template,omitempty"`
}

func (s *MCPServer) addPreviewRoute() {
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "preview_route",
		Description: "Preview a route configuration without creating it",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input previewRouteInput) (*mcp.CallToolResult, any, error) {
		preview := previewOutput{
			Domain:   input.Domain,
			Upstream: input.Upstream,
			Template: input.Template,
		}

		if input.Template != "" {
			tmpl := templates.Get(input.Template)
			if tmpl == nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: unknown template %q", input.Template)}},
					IsError: true,
				}, nil, nil
			}
			params := input.TemplateParams
			if params == nil {
				params = map[string]string{}
			}
			matchConfig, handlerConfig, err := tmpl.Apply(params)
			if err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
					IsError: true,
				}, nil, nil
			}
			preview.MatchConfig = matchConfig
			preview.HandlerConfig = handlerConfig
		}

		return nil, preview, nil
	})
}

// --- diagnose_route ---

type diagnoseRouteInput struct {
	Domain string `json:"domain" jsonschema:"domain to diagnose"`
}

type diagnoseCheck struct {
	Check   string `json:"check"`
	Status  string `json:"status"` // "pass" or "fail"
	Details string `json:"details,omitempty"`
}

type diagnoseOutput struct {
	Domain string          `json:"domain"`
	Checks []diagnoseCheck `json:"checks"`
}

func (s *MCPServer) addDiagnoseRoute() {
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "diagnose_route",
		Description: "Diagnose connectivity for a route: DNS, TCP, and HTTP checks",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input diagnoseRouteInput) (*mcp.CallToolResult, any, error) {
		out := diagnoseOutput{Domain: input.Domain}

		// 1. DNS lookup
		addrs, err := net.LookupHost(input.Domain)
		if err != nil {
			out.Checks = append(out.Checks, diagnoseCheck{
				Check:   "DNS lookup",
				Status:  "fail",
				Details: err.Error(),
			})
		} else {
			out.Checks = append(out.Checks, diagnoseCheck{
				Check:   "DNS lookup",
				Status:  "pass",
				Details: strings.Join(addrs, ", "),
			})
		}

		// 2. Find route in DB
		route, err := db.GetRouteByDomain(s.db, input.Domain)
		if err != nil {
			out.Checks = append(out.Checks, diagnoseCheck{
				Check:   "Route lookup",
				Status:  "fail",
				Details: err.Error(),
			})
			return nil, out, nil
		}
		if route == nil {
			out.Checks = append(out.Checks, diagnoseCheck{
				Check:   "Route lookup",
				Status:  "fail",
				Details: "no route configured for this domain",
			})
			return nil, out, nil
		}
		out.Checks = append(out.Checks, diagnoseCheck{
			Check:   "Route lookup",
			Status:  "pass",
			Details: fmt.Sprintf("upstream=%s tls=%v", route.Upstream, route.TLSEnabled),
		})

		// 3. TCP connect to upstream
		conn, err := net.DialTimeout("tcp", route.Upstream, 3*time.Second)
		if err != nil {
			out.Checks = append(out.Checks, diagnoseCheck{
				Check:   "TCP connect to upstream",
				Status:  "fail",
				Details: err.Error(),
			})
		} else {
			conn.Close()
			out.Checks = append(out.Checks, diagnoseCheck{
				Check:   "TCP connect to upstream",
				Status:  "pass",
				Details: route.Upstream,
			})
		}

		// 4. HTTP GET to upstream
		httpClient := &http.Client{Timeout: 3 * time.Second}
		resp, err := httpClient.Get("http://" + route.Upstream)
		if err != nil {
			out.Checks = append(out.Checks, diagnoseCheck{
				Check:   "HTTP GET upstream",
				Status:  "fail",
				Details: err.Error(),
			})
		} else {
			resp.Body.Close()
			out.Checks = append(out.Checks, diagnoseCheck{
				Check:   "HTTP GET upstream",
				Status:  "pass",
				Details: fmt.Sprintf("status %d", resp.StatusCode),
			})
		}

		return nil, out, nil
	})
}

// --- trust ---

func (s *MCPServer) addTrust() {
	s.server.AddTool(
		&mcp.Tool{
			Name:        "trust",
			Description: "Install macOS resolver files for all managed TLDs",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			tlds, err := db.ListTLDs(s.db)
			if err != nil {
				return errResult(err)
			}
			var installed []string
			var errors []string
			for _, t := range tlds {
				if err := resolver.Install(t.TLD); err != nil {
					errors = append(errors, fmt.Sprintf(".%s: %v", t.TLD, err))
				} else {
					installed = append(installed, "."+t.TLD)
					_ = db.UpdateTLDResolver(s.db, t.TLD, true)
				}
			}
			var sb strings.Builder
			if len(installed) > 0 {
				sb.WriteString("Installed resolvers for: " + strings.Join(installed, ", ") + "\n")
			}
			if len(errors) > 0 {
				sb.WriteString("Errors:\n")
				for _, e := range errors {
					sb.WriteString("  " + e + "\n")
				}
			}
			if len(installed) == 0 && len(errors) == 0 {
				sb.WriteString("No TLDs configured")
			}
			return textResult(sb.String())
		},
	)
}
