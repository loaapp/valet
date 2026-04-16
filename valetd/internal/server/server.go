package server

import (
	"context"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/loaapp/valet/valetd/internal/certs"
	"github.com/loaapp/valet/valetd/internal/db"
	"github.com/loaapp/valet/valetd/internal/dns"
	"github.com/loaapp/valet/valetd/internal/domain"
	"github.com/loaapp/valet/valetd/internal/logstore"
	"github.com/loaapp/valet/valetd/internal/metrics"
	"github.com/loaapp/valet/valetd/internal/resolver"
	"github.com/loaapp/valet/valetd/internal/templates"
)

type Server struct {
	httpServer  *http.Server
	database    *sql.DB
	routeSvc    *domain.RouteService
	tldSvc      *domain.TLDService
	dnsEntrySvc *domain.DNSEntryService
	certMgr     *certs.Manager
	collector   *metrics.Collector
	logStore    *logstore.Store
	dnsServer   *dns.Server
}

func New(addr string, database *sql.DB, routeSvc *domain.RouteService, tldSvc *domain.TLDService, dnsEntrySvc *domain.DNSEntryService, certMgr *certs.Manager, collector *metrics.Collector, logStore *logstore.Store, dnsServer *dns.Server) *Server {
	s := &Server{
		database:    database,
		routeSvc:    routeSvc,
		tldSvc:      tldSvc,
		dnsEntrySvc: dnsEntrySvc,
		certMgr:     certMgr,
		collector:   collector,
		logStore:    logStore,
		dnsServer:   dnsServer,
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
	mux.HandleFunc("GET /api/v1/certs/info", s.handleCertInfo)
	mux.HandleFunc("GET /api/v1/templates", s.handleListTemplates)
	mux.HandleFunc("POST /api/v1/routes/preview", s.handlePreviewRoute)
	mux.HandleFunc("GET /api/v1/metrics/current", s.handleMetricsCurrent)
	mux.HandleFunc("GET /api/v1/metrics/history", s.handleMetricsHistory)
	mux.HandleFunc("GET /api/v1/logs", s.handleLogs)
	mux.HandleFunc("DELETE /api/v1/logs", s.handleClearHTTPLogs)
	mux.HandleFunc("GET /api/v1/dns/logs", s.handleDNSLogs)
	mux.HandleFunc("DELETE /api/v1/dns/logs", s.handleClearDNSLogs)
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
	routeList, _ := s.routeSvc.List()
	tldList, _ := s.tldSvc.List()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "running",
		"routes":   len(routeList),
		"tlds":     len(tldList),
		"mkcert":   certs.MkcertAvailable(),
		"platform": "darwin",
	})
}

func (s *Server) handleListRoutes(w http.ResponseWriter, r *http.Request) {
	list, err := s.routeSvc.List()
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

func (s *Server) handleAddRoute(w http.ResponseWriter, r *http.Request) {
	var req addRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return
	}

	domainReq := domain.AddRouteRequest{
		Domain:         req.Domain,
		Upstream:       req.Upstream,
		Description:    req.Description,
		Template:       req.Template,
		TemplateParams: req.TemplateParams,
		MatchConfig:    req.MatchConfig,
		HandlerConfig:  req.HandlerConfig,
	}

	route, err := s.routeSvc.Add(domainReq)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, route)
}

func (s *Server) handleGetRoute(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	route, err := s.routeSvc.Get(id)
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

	var req addRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return
	}

	// Build UpdateRouteRequest with pointer fields for non-empty values
	updateReq := domain.UpdateRouteRequest{}
	if req.Domain != "" {
		updateReq.Domain = &req.Domain
	}
	if req.Upstream != "" {
		updateReq.Upstream = &req.Upstream
	}
	if req.Description != "" {
		updateReq.Description = &req.Description
	}
	if req.MatchConfig != "" {
		updateReq.MatchConfig = &req.MatchConfig
	}
	if req.HandlerConfig != "" {
		updateReq.HandlerConfig = &req.HandlerConfig
	}
	if req.Template != "" {
		updateReq.Template = &req.Template
	}

	route, err := s.routeSvc.Update(id, updateReq)
	if err != nil {
		if err.Error() == "route not found" {
			writeError(w, http.StatusNotFound, err)
		} else {
			writeError(w, http.StatusBadRequest, err)
		}
		return
	}

	writeJSON(w, http.StatusOK, route)
}

func (s *Server) handleDeleteRoute(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	route, err := s.routeSvc.Get(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if route == nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("route not found"))
		return
	}

	if err := s.routeSvc.Remove(route.Domain); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleListTLDs(w http.ResponseWriter, r *http.Request) {
	tlds, err := s.tldSvc.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if tlds == nil {
		tlds = []domain.TLDStatus{}
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

	result, err := s.tldSvc.Register(req.TLD)
	if err != nil {
		if fmt.Sprintf("%v", err) == fmt.Sprintf("TLD .%s already managed", req.TLD) {
			writeError(w, http.StatusConflict, err)
		} else {
			writeError(w, http.StatusBadRequest, err)
		}
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleDeleteTLD(w http.ResponseWriter, r *http.Request) {
	tld := r.PathValue("tld")

	if err := s.tldSvc.Unregister(tld); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- DNS Entries ---

func (s *Server) handleListDNSEntries(w http.ResponseWriter, r *http.Request) {
	tld := r.URL.Query().Get("tld")
	entries, err := s.dnsEntrySvc.List(tld)
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

	entry, err := s.dnsEntrySvc.Add(req.Domain, req.TLD, req.Target)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusCreated, entry)
}

func (s *Server) handleDeleteDNSEntry(w http.ResponseWriter, r *http.Request) {
	domainName := r.PathValue("domain")
	if domainName == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("domain is required"))
		return
	}

	if err := s.dnsEntrySvc.Remove(domainName); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
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

	domainReq := domain.AddRouteRequest{
		Domain:         req.Domain,
		Upstream:       req.Upstream,
		Template:       req.Template,
		TemplateParams: req.TemplateParams,
		MatchConfig:    req.MatchConfig,
		HandlerConfig:  req.HandlerConfig,
		Description:    req.Description,
	}

	preview, err := s.routeSvc.Preview(domainReq)
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
	if s.logStore == nil {
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

	var since float64
	if sinceStr != "" {
		ts, err := strconv.ParseFloat(sinceStr, 64)
		if err != nil || math.IsNaN(ts) {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid since parameter"))
			return
		}
		since = ts
	}

	entries, err := s.logStore.GetHTTPLogs(limit, since, route)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if entries == nil {
		entries = []logstore.HTTPLogEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

// --- Certs ---

func (s *Server) handleCertInfo(w http.ResponseWriter, r *http.Request) {
	certPath, _ := s.certMgr.CombinedCertPath()
	if certPath == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"exists":  false,
			"domains": []string{},
		})
		return
	}

	certData, err := os.ReadFile(certPath)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"exists": false, "domains": []string{}})
		return
	}

	block, _ := pem.Decode(certData)
	if block == nil {
		writeJSON(w, http.StatusOK, map[string]any{"exists": false, "domains": []string{}, "error": "failed to parse PEM"})
		return
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"exists": false, "domains": []string{}, "error": err.Error()})
		return
	}

	var domains []string
	for _, name := range cert.DNSNames {
		domains = append(domains, name)
	}
	for _, ip := range cert.IPAddresses {
		domains = append(domains, ip.String())
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"exists":    true,
		"domains":   domains,
		"notBefore": cert.NotBefore.Format("2006-01-02"),
		"notAfter":  cert.NotAfter.Format("2006-01-02"),
		"issuer":    cert.Issuer.CommonName,
	})
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
	if s.logStore == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	entries, err := s.logStore.GetDNSLogs(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if entries == nil {
		entries = []logstore.DNSLogEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) handleClearHTTPLogs(w http.ResponseWriter, r *http.Request) {
	if s.logStore == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if err := s.logStore.ClearHTTP(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

func (s *Server) handleClearDNSLogs(w http.ResponseWriter, r *http.Request) {
	if s.logStore == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if err := s.logStore.ClearDNS(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
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
