// Copyright 2025 Richard Clayton. All rights reserved.
// Proprietary — see LICENSE.md.

package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/loaapp/valet/valetd/internal/certs"
	"github.com/loaapp/valet/valetd/internal/domain"
	"github.com/loaapp/valet/valetd/internal/templates"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPServer wraps an MCP server with valetd tools.
type MCPServer struct {
	routeSvc    *domain.RouteService
	tldSvc      *domain.TLDService
	dnsEntrySvc *domain.DNSEntryService
	server      *mcp.Server
}

// New creates a new MCPServer with all tools registered.
func New(routeSvc *domain.RouteService, tldSvc *domain.TLDService, dnsEntrySvc *domain.DNSEntryService) *MCPServer {
	s := &MCPServer{
		routeSvc:    routeSvc,
		tldSvc:      tldSvc,
		dnsEntrySvc: dnsEntrySvc,
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
	s.addListTemplates()
	s.addPreviewRoute()
	s.addDiagnoseRoute()
	s.addListDNSEntries()
	s.addAddDNSEntry()
	s.addRemoveDNSEntry()
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
			routeList, err := s.routeSvc.List()
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
	Description    string            `json:"description,omitempty" jsonschema:"human-readable description"`
	Template       string            `json:"template,omitempty" jsonschema:"route template slug"`
	TemplateParams map[string]string `json:"templateParams,omitempty" jsonschema:"parameters for the template"`
}

func (s *MCPServer) addAddRoute() {
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "add_route",
		Description: "Add a new route mapping a domain to an upstream",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input addRouteInput) (*mcp.CallToolResult, any, error) {
		domainReq := domain.AddRouteRequest{
			Domain:         input.Domain,
			Upstream:       input.Upstream,
			Description:    input.Description,
			Template:       input.Template,
			TemplateParams: input.TemplateParams,
		}

		route, err := s.routeSvc.Add(domainReq)
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
		if err := s.routeSvc.Remove(input.Domain); err != nil {
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
	Description string `json:"description,omitempty" jsonschema:"new description"`
}

func (s *MCPServer) addUpdateRoute() {
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "update_route",
		Description: "Update an existing route by ID",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input updateRouteInput) (*mcp.CallToolResult, any, error) {
		updateReq := domain.UpdateRouteRequest{}
		if input.Domain != "" {
			updateReq.Domain = &input.Domain
		}
		if input.Upstream != "" {
			updateReq.Upstream = &input.Upstream
		}
		if input.Description != "" {
			updateReq.Description = &input.Description
		}

		updated, err := s.routeSvc.Update(input.ID, updateReq)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
				IsError: true,
			}, nil, nil
		}

		return nil, updated, nil
	})
}

// --- get_status ---

type statusOutput struct {
	RouteCount      int  `json:"routeCount"`
	TLDCount        int  `json:"tldCount"`
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
			routeList, err := s.routeSvc.List()
			if err != nil {
				return errResult(err)
			}
			tldList, err := s.tldSvc.List()
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
			tldList, err := s.tldSvc.List()
			if err != nil {
				return errResult(err)
			}
			return textResult(mustJSON(tldList))
		},
	)
}

// add_tld, remove_tld, trust removed — TLD management requires sudo via CLI (valetd tld add/remove)

// --- list_templates ---

type templateInfo struct {
	Slug        string          `json:"slug"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Params      []templateParam `json:"params,omitempty"`
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

func (s *MCPServer) addPreviewRoute() {
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "preview_route",
		Description: "Preview a route configuration without creating it",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input previewRouteInput) (*mcp.CallToolResult, any, error) {
		domainReq := domain.AddRouteRequest{
			Domain:         input.Domain,
			Upstream:       input.Upstream,
			Template:       input.Template,
			TemplateParams: input.TemplateParams,
		}

		preview, err := s.routeSvc.Preview(domainReq)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
				IsError: true,
			}, nil, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(preview)}},
		}, nil, nil
	})
}

// --- diagnose_route ---

type diagnoseRouteInput struct {
	Domain string `json:"domain" jsonschema:"domain to diagnose"`
}

func (s *MCPServer) addDiagnoseRoute() {
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "diagnose_route",
		Description: "Diagnose connectivity for a route: DNS, TCP, and HTTP checks",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input diagnoseRouteInput) (*mcp.CallToolResult, any, error) {
		result, err := s.routeSvc.Diagnose(input.Domain)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		return nil, result, nil
	})
}

// trust removed — resolver installation requires sudo via CLI (valetd tld add)

// --- list_dns_entries ---

func (s *MCPServer) addListDNSEntries() {
	s.server.AddTool(
		&mcp.Tool{
			Name:        "list_dns_entries",
			Description: "List all registered DNS entries (subdomains within managed TLDs)",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			entries, err := s.dnsEntrySvc.List("")
			if err != nil {
				return errResult(err)
			}
			return textResult(mustJSON(entries))
		},
	)
}

// --- add_dns_entry ---

type addDNSEntryInput struct {
	Domain string `json:"domain" jsonschema:"full domain name (e.g. app.example.com)"`
	TLD    string `json:"tld" jsonschema:"parent TLD (e.g. example.com)"`
	Target string `json:"target,omitempty" jsonschema:"IP address or hostname for CNAME (default 127.0.0.1)"`
}

func (s *MCPServer) addAddDNSEntry() {
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "add_dns_entry",
		Description: "Register a subdomain within a managed TLD. Resolves to 127.0.0.1 by default, or a custom IP/hostname for CNAME.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input addDNSEntryInput) (*mcp.CallToolResult, any, error) {
		entry, err := s.dnsEntrySvc.Add(input.Domain, input.TLD, input.Target)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		return nil, entry, nil
	})
}

// --- remove_dns_entry ---

type removeDNSEntryInput struct {
	Domain string `json:"domain" jsonschema:"domain to remove (e.g. app.example.com)"`
}

func (s *MCPServer) addRemoveDNSEntry() {
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "remove_dns_entry",
		Description: "Remove a registered DNS entry (subdomain)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input removeDNSEntryInput) (*mcp.CallToolResult, any, error) {
		if err := s.dnsEntrySvc.Remove(input.Domain); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
				IsError: true,
			}, nil, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("DNS entry for %s removed", input.Domain)}},
		}, nil, nil
	})
}
