package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/loaapp/valet/valetd/internal/caddy"
	"github.com/loaapp/valet/valetd/internal/certs"
	"github.com/loaapp/valet/valetd/internal/db"
	"github.com/loaapp/valet/valetd/internal/dns"
	"github.com/loaapp/valet/valetd/internal/logbuf"
	"github.com/loaapp/valet/valetd/internal/metrics"
	"github.com/loaapp/valet/valetd/internal/resolver"
	"github.com/loaapp/valet/valetd/internal/routes"
	"github.com/loaapp/valet/valetd/internal/templates"
)

type Server struct {
	httpServer *http.Server
	database   *sql.DB
	routeMgr   *routes.Manager
	certMgr    *certs.Manager
	collector  *metrics.Collector
	logBuf     *logbuf.RingBuffer
	dnsServer  *dns.Server
}

func New(addr string, database *sql.DB, routeMgr *routes.Manager, certMgr *certs.Manager, collector *metrics.Collector, logBuf *logbuf.RingBuffer, dnsServer *dns.Server) *Server {
	s := &Server{
		database:  database,
		routeMgr:  routeMgr,
		certMgr:   certMgr,
		collector: collector,
		logBuf:    logBuf,
		dnsServer: dnsServer,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/status", s.handleStatus)
	mux.HandleFunc("GET /api/v1/routes", s.handleListRoutes)
	mux.HandleFunc("POST /api/v1/routes", s.handleAddRoute)
	mux.HandleFunc("GET /api/v1/routes/{id}", s.handleGetRoute)
	mux.HandleFunc("PUT /api/v1/routes/{id}", s.handleUpdateRoute)
	mux.HandleFunc("DELETE /api/v1/routes/{id}", s.handleDeleteRoute)
	mux.HandleFunc("GET /api/v1/tlds", s.handleListTLDs)
	mux.HandleFunc("POST /api/v1/tlds", s.handleAddTLD)
	mux.HandleFunc("DELETE /api/v1/tlds/{tld}", s.handleDeleteTLD)
	mux.HandleFunc("GET /api/v1/dns/status", s.handleDNSStatus)
	mux.HandleFunc("GET /api/v1/templates", s.handleListTemplates)
	mux.HandleFunc("POST /api/v1/routes/preview", s.handlePreviewRoute)
	mux.HandleFunc("GET /api/v1/metrics/current", s.handleMetricsCurrent)
	mux.HandleFunc("GET /api/v1/metrics/history", s.handleMetricsHistory)
	mux.HandleFunc("GET /api/v1/logs", s.handleLogs)
	mux.HandleFunc("GET /api/v1/dns/logs", s.handleDNSLogs)
	mux.HandleFunc("GET /api/v1/dns/entries", s.handleListDNSEntries)
	mux.HandleFunc("POST /api/v1/dns/entries", s.handleCreateDNSEntry)
	mux.HandleFunc("DELETE /api/v1/dns/entries/{domain...}", s.handleDeleteDNSEntry)
	mux.HandleFunc("GET /api/v1/settings", s.handleGetSettings)
	mux.HandleFunc("PUT /api/v1/settings/{key}", s.handleSetSetting)

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return s
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return err
	}
	go func() {
		log.Printf("API server listening on %s", s.httpServer.Addr)
		if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("API server error: %v", err)
		}
	}()
	return nil
}

func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

// --- Handlers ---

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	routeCount, _ := db.ListRoutes(s.database)
	tldCount, _ := db.ListTLDs(s.database)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "running",
		"routes":   len(routeCount),
		"tlds":     len(tldCount),
		"mkcert":   certs.MkcertAvailable(),
		"platform": "darwin",
	})
}

func (s *Server) handleListRoutes(w http.ResponseWriter, r *http.Request) {
	list, err := s.routeMgr.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if list == nil {
		list = []db.Route{}
	}
	writeJSON(w, http.StatusOK, list)
}

type addRouteRequest struct {
	Domain         string            `json:"domain"`
	Upstream       string            `json:"upstream"`
	TLS            *bool             `json:"tls"`
	Description    string            `json:"description"`
	Template       string            `json:"template"`
	TemplateParams map[string]string `json:"templateParams"`
	MatchConfig    string            `json:"matchConfig"`
	HandlerConfig  string            `json:"handlerConfig"`
}

func (req *addRouteRequest) hasTemplate() bool {
	return req.Template != ""
}

func (s *Server) resolveTemplate(req *addRouteRequest) error {
	if req.Template == "" {
		return nil
	}
	tmpl := templates.Get(req.Template)
	if tmpl == nil {
		return fmt.Errorf("unknown template: %s", req.Template)
	}
	params := make(map[string]string)
	if req.Domain != "" {
		params["domain"] = req.Domain
	}
	if req.Upstream != "" {
		params["upstream"] = req.Upstream
	}
	for k, v := range req.TemplateParams {
		params[k] = v
	}
	matchConfig, handlerConfig, err := tmpl.Apply(params)
	if err != nil {
		return fmt.Errorf("template %s: %w", req.Template, err)
	}
	if matchConfig != "" {
		req.MatchConfig = matchConfig
	}
	if handlerConfig != "" {
		req.HandlerConfig = handlerConfig
	}
	return nil
}

var domainRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?\.[a-z]+$`)
var hostPortRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+:\d+$`)

func normalizeUpstream(u string) string {
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	return u
}

func validateRouteRequest(req *addRouteRequest) error {
	// Domain validation
	req.Domain = strings.TrimSpace(req.Domain)
	if req.Domain == "" {
		return fmt.Errorf("invalid domain: domain is required")
	}
	if strings.Contains(req.Domain, " ") {
		return fmt.Errorf("invalid domain: must not contain spaces")
	}
	if strings.Contains(req.Domain, "://") {
		return fmt.Errorf("invalid domain: do not include http:// or https://")
	}
	if strings.Contains(req.Domain, "/") {
		return fmt.Errorf("invalid domain: do not include paths")
	}
	if strings.Contains(req.Domain, ":") {
		return fmt.Errorf("invalid domain: do not include a port")
	}
	if req.Domain != strings.ToLower(req.Domain) {
		return fmt.Errorf("invalid domain: must be lowercase")
	}
	if !domainRe.MatchString(req.Domain) {
		return fmt.Errorf("invalid domain: must be a valid domain (e.g., myapp.test)")
	}

	// Upstream validation — normalize first
	req.Upstream = strings.TrimSpace(req.Upstream)
	req.Upstream = normalizeUpstream(req.Upstream)
	if req.Upstream != "" {
		if strings.Contains(req.Upstream, " ") {
			return fmt.Errorf("invalid upstream: must not contain spaces")
		}
		if !hostPortRe.MatchString(req.Upstream) {
			return fmt.Errorf("invalid upstream: must be host:port format (e.g., localhost:3000)")
		}
	}

	// Template param validation
	if req.Template != "" {
		tmpl := templates.Get(req.Template)
		if tmpl != nil {
			for _, p := range tmpl.Params {
				if p.Required {
					v := req.TemplateParams[p.Key]
					if v == "" {
						return fmt.Errorf("invalid templateParams: required parameter %q is missing", p.Key)
					}
				}
			}
		}
	}

	return nil
}

func (s *Server) handleAddRoute(w http.ResponseWriter, r *http.Request) {
	var req addRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return
	}
	if err := validateRouteRequest(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if !req.hasTemplate() && req.Upstream == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid upstream: upstream is required when not using a template"))
		return
	}

	if err := s.resolveTemplate(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	tlsEnabled := true
	if req.TLS != nil {
		tlsEnabled = *req.TLS
	}

	route, err := s.routeMgr.Add(req.Domain, req.Upstream, tlsEnabled, req.MatchConfig, req.HandlerConfig, req.Template, req.Description)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, route)
}

func (s *Server) handleGetRoute(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	route, err := db.GetRoute(s.database, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if route == nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("route not found"))
		return
	}
	writeJSON(w, http.StatusOK, route)
}

func (s *Server) handleUpdateRoute(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	existing, err := db.GetRoute(s.database, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("route not found"))
		return
	}

	var req addRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return
	}

	// Normalize upstream at API level
	req.Upstream = strings.TrimSpace(req.Upstream)
	req.Upstream = normalizeUpstream(req.Upstream)

	// Validate any fields that were provided
	if req.Domain != "" {
		check := addRouteRequest{Domain: req.Domain, Upstream: "localhost:1"}
		if err := validateRouteRequest(&check); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	if req.Upstream != "" {
		if strings.Contains(req.Upstream, " ") {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid upstream: must not contain spaces"))
			return
		}
		if !hostPortRe.MatchString(req.Upstream) {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid upstream: must be host:port format (e.g., localhost:3000)"))
			return
		}
	}

	domain := existing.Domain
	if req.Domain != "" {
		domain = req.Domain
	}
	upstream := existing.Upstream
	upstreamChanged := false
	if req.Upstream != "" {
		upstream = req.Upstream
		upstreamChanged = true
	}
	tlsEnabled := existing.TLSEnabled
	if req.TLS != nil {
		tlsEnabled = *req.TLS
	}

	// Resolve template if provided
	if req.Template != "" {
		req.Domain = domain
		req.Upstream = upstream
		if err := s.resolveTemplate(&req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}

	matchConfig := existing.MatchConfig
	if req.MatchConfig != "" {
		matchConfig = req.MatchConfig
	}
	handlerConfig := existing.HandlerConfig
	if req.HandlerConfig != "" {
		handlerConfig = req.HandlerConfig
	} else if upstreamChanged && req.Template == "" {
		// If upstream changed on a simple (non-template) route, clear stale
		// handlerConfig so Caddy uses the updated upstream column directly.
		handlerConfig = ""
	}
	tmpl := existing.Template
	if req.Template != "" {
		tmpl = req.Template
	}
	description := existing.Description
	if req.Description != "" {
		description = req.Description
	}

	route, err := db.UpdateRoute(s.database, id, domain, upstream, tlsEnabled, existing.CertPath, existing.KeyPath, matchConfig, handlerConfig, tmpl, description)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Reload Caddy with updated config
	if err := s.routeMgr.Sync(); err != nil {
		log.Printf("Warning: failed to sync after route update: %v", err)
	}

	writeJSON(w, http.StatusOK, route)
}

func (s *Server) handleDeleteRoute(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	route, err := db.GetRoute(s.database, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if route == nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("route not found"))
		return
	}

	if err := s.routeMgr.Remove(route.Domain); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleListTLDs(w http.ResponseWriter, r *http.Request) {
	tlds, err := db.ListTLDs(s.database)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if tlds == nil {
		tlds = []db.ManagedTLD{}
	}
	writeJSON(w, http.StatusOK, tlds)
}

type addTLDRequest struct {
	TLD string `json:"tld"`
}

func (s *Server) handleAddTLD(w http.ResponseWriter, r *http.Request) {
	var req addTLDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return
	}

	tld := strings.TrimPrefix(req.TLD, ".")
	if tld == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("tld is required"))
		return
	}

	existing, _ := db.GetTLD(s.database, tld)
	if existing != nil {
		writeError(w, http.StatusConflict, fmt.Errorf("TLD .%s already managed", tld))
		return
	}

	result, err := db.CreateTLD(s.database, tld)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Update DNS server
	s.routeMgr.SyncDNS()

	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleDeleteTLD(w http.ResponseWriter, r *http.Request) {
	tld := r.PathValue("tld")

	if err := db.DeleteTLD(s.database, tld); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	resolver.Remove(tld)
	s.routeMgr.SyncDNS()

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- DNS Entries ---

func (s *Server) handleListDNSEntries(w http.ResponseWriter, r *http.Request) {
	tld := r.URL.Query().Get("tld")
	var entries []db.DNSEntry
	var err error
	if tld != "" {
		entries, err = db.ListDNSEntriesByTLD(s.database, tld)
	} else {
		entries, err = db.ListDNSEntries(s.database)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if entries == nil {
		entries = []db.DNSEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

type createDNSEntryRequest struct {
	Domain string `json:"domain"`
	TLD    string `json:"tld"`
	Target string `json:"target"`
}

func (s *Server) handleCreateDNSEntry(w http.ResponseWriter, r *http.Request) {
	var req createDNSEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return
	}

	req.Domain = strings.TrimSpace(strings.ToLower(req.Domain))
	req.TLD = strings.TrimSpace(strings.TrimPrefix(req.TLD, "."))
	req.Target = strings.TrimSpace(req.Target)

	if req.Domain == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("domain is required"))
		return
	}
	if req.TLD == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("tld is required"))
		return
	}
	if !domainRe.MatchString(req.Domain) {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid domain format"))
		return
	}
	if req.Target == "" {
		req.Target = "127.0.0.1"
	}

	entry, err := db.CreateDNSEntry(s.database, req.Domain, req.TLD, req.Target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Reload dns_entries into DNS server
	s.syncDNSEntries()

	writeJSON(w, http.StatusCreated, entry)
}

func (s *Server) handleDeleteDNSEntry(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	if domain == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("domain is required"))
		return
	}

	if err := db.DeleteDNSEntry(s.database, domain); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.syncDNSEntries()

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) syncDNSEntries() {
	entries, err := db.ListDNSEntries(s.database)
	if err != nil {
		log.Printf("Warning: failed to load dns_entries: %v", err)
		return
	}
	dnsEntries := make([]dns.DNSEntry, len(entries))
	for i, e := range entries {
		dnsEntries[i] = dns.DNSEntry{Domain: e.Domain, Target: e.Target}
	}
	s.dnsServer.SetEntries(dnsEntries)
}

// --- Settings ---

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	rows, err := s.database.Query(`SELECT key, value FROM settings`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		settings[k] = v
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleSetSetting(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	var body struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := db.SetSetting(s.database, key, body.Value); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Templates & Preview ---

func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	type paramOut struct {
		Key         string `json:"key"`
		Label       string `json:"label"`
		Placeholder string `json:"placeholder"`
		Required    bool   `json:"required"`
	}
	type templateOut struct {
		Slug        string     `json:"slug"`
		Name        string     `json:"name"`
		Description string     `json:"description"`
		Params      []paramOut `json:"params"`
	}

	out := make([]templateOut, 0, len(templates.Registry))
	for _, t := range templates.Registry {
		params := make([]paramOut, 0, len(t.Params))
		for _, p := range t.Params {
			params = append(params, paramOut{
				Key:         p.Key,
				Label:       p.Label,
				Placeholder: p.Placeholder,
				Required:    p.Required,
			})
		}
		out = append(out, templateOut{
			Slug:        t.Slug,
			Name:        t.Name,
			Description: t.Description,
			Params:      params,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handlePreviewRoute(w http.ResponseWriter, r *http.Request) {
	var req addRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return
	}
	if req.Domain == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("domain is required"))
		return
	}

	if err := s.resolveTemplate(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	tlsEnabled := true
	if req.TLS != nil {
		tlsEnabled = *req.TLS
	}

	fakeRoute := db.Route{
		ID:            "preview",
		Domain:        req.Domain,
		Upstream:      req.Upstream,
		TLSEnabled:    tlsEnabled,
		MatchConfig:   req.MatchConfig,
		HandlerConfig: req.HandlerConfig,
		Template:      req.Template,
		Description:   req.Description,
	}

	preview, err := caddy.BuildRoutePreview(fakeRoute)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(preview)
}

// --- Metrics & Logs ---

func (s *Server) handleMetricsCurrent(w http.ResponseWriter, r *http.Request) {
	if s.collector == nil {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	current, err := s.collector.Store().GetCurrent()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, current)
}

func (s *Server) handleMetricsHistory(w http.ResponseWriter, r *http.Request) {
	if s.collector == nil {
		writeJSON(w, http.StatusOK, map[string]any{"resolution": "1s", "routes": map[string]any{}, "totals": []any{}})
		return
	}

	rangeStr := r.URL.Query().Get("range")
	if rangeStr == "" {
		rangeStr = "5m"
	}

	store := s.collector.Store()
	totals, perRoute, err := store.GetHistory(rangeStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	var resolution string
	switch rangeStr {
	case "1h":
		resolution = "1m"
	case "24h":
		resolution = "1h"
	default:
		resolution = "1s"
	}

	if totals == nil {
		totals = []metrics.DataPoint{}
	}
	routeData := make(map[string]any, len(perRoute))
	for host, points := range perRoute {
		routeData[host] = points
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"resolution": resolution,
		"routes":     routeData,
		"totals":     totals,
	})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if s.logBuf == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 1000 {
		limit = 1000
	}

	route := r.URL.Query().Get("route")
	sinceStr := r.URL.Query().Get("since")

	var entries []logbuf.LogEntry
	if sinceStr != "" {
		ts, err := strconv.ParseFloat(sinceStr, 64)
		if err != nil || math.IsNaN(ts) {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid since parameter"))
			return
		}
		entries = s.logBuf.Since(ts)
	} else {
		entries = s.logBuf.Last(limit)
	}

	// Filter by route (host) if specified
	if route != "" {
		filtered := make([]logbuf.LogEntry, 0, len(entries))
		for _, e := range entries {
			if e.Host == route {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// Apply limit after filtering
	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}

	if entries == nil {
		entries = []logbuf.LogEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

// --- DNS ---

func (s *Server) handleDNSStatus(w http.ResponseWriter, r *http.Request) {
	tlds, err := db.ListTLDs(s.database)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	type tldStatus struct {
		TLD       string `json:"tld"`
		Installed bool   `json:"installed"`
	}

	var result []tldStatus
	for _, t := range tlds {
		result = append(result, tldStatus{
			TLD:       t.TLD,
			Installed: resolver.IsInstalled(t.TLD),
		})
	}
	if result == nil {
		result = []tldStatus{}
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleDNSLogs(w http.ResponseWriter, r *http.Request) {
	if s.dnsServer == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	entries := s.dnsServer.QueryLogs().Last(limit)
	writeJSON(w, http.StatusOK, entries)
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
