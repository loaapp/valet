package daemon

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/loaapp/valet/valetd/internal/caddy"
	"github.com/loaapp/valet/valetd/internal/certs"
	"github.com/loaapp/valet/valetd/internal/db"
	"github.com/loaapp/valet/valetd/internal/dns"
	"github.com/loaapp/valet/valetd/internal/domain"
	"github.com/loaapp/valet/valetd/internal/logbuf"
	"github.com/loaapp/valet/valetd/internal/logstore"
	"github.com/loaapp/valet/valetd/internal/mcpserver"
	"github.com/loaapp/valet/valetd/internal/metrics"
	"github.com/loaapp/valet/valetd/internal/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Config struct {
	APIAddr string // e.g., ":7800"
	DNSAddr string // e.g., ":53"
	MCPAddr string // e.g., ":7801" — MCP HTTP endpoint
}

type Daemon struct {
	config      Config
	database    *db.AppDB
	dnsServer   *dns.Server
	apiServer   *server.Server
	routeSvc    *domain.RouteService
	tldSvc      *domain.TLDService
	dnsEntrySvc *domain.DNSEntryService
	certMgr     *certs.Manager
	collector   *metrics.Collector
	logStore    *logstore.Store
	tailer      *logbuf.Tailer
	mcpHTTP     *http.Server
}

func New(cfg Config) *Daemon {
	return &Daemon{config: cfg}
}

// Start initializes all components and starts serving.
func (d *Daemon) Start() error {
	log.Println("Starting valetd...")

	// Open database
	database, err := db.Open()
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	d.database = database

	// Resolve data directory
	dataDir, err := db.DataDir()
	if err != nil {
		return fmt.Errorf("data dir: %w", err)
	}

	// Cert manager
	certMgr, err := certs.NewManager()
	if err != nil {
		return fmt.Errorf("init cert manager: %w", err)
	}
	d.certMgr = certMgr

	// DNS server
	d.dnsServer = dns.NewServer()

	// Create domain services
	d.routeSvc = domain.NewRouteService(database.DB, certMgr, d.dnsServer, dataDir)
	d.tldSvc = domain.NewTLDService(database.DB, d.dnsServer)
	d.dnsEntrySvc = domain.NewDNSEntryService(database.DB, d.dnsServer)

	// Sync DNS TLDs from database
	d.tldSvc.SyncDNS()

	// Load dns_entries into DNS server
	d.dnsEntrySvc.SyncEntries()

	// Start DNS server
	if d.config.DNSAddr != "" {
		if err := d.dnsServer.Start(d.config.DNSAddr); err != nil {
			log.Printf("Warning: DNS server failed to start on %s: %v (may need sudo)", d.config.DNSAddr, err)
			// Non-fatal — DNS is optional if using /etc/hosts
		}
	}

	// Redirect stderr to access.log so Caddy's structured logs are captured
	logDir, err := db.LogDir()
	if err != nil {
		return fmt.Errorf("log dir: %w", err)
	}
	os.MkdirAll(logDir, 0o755)
	accessLogPath := filepath.Join(logDir, "access.log")
	accessLogFile, err := os.OpenFile(accessLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("Warning: cannot open access log: %v", err)
	} else {
		os.Stderr = accessLogFile
	}

	// Load initial Caddy config via route service sync
	if err := d.routeSvc.Sync(); err != nil {
		return fmt.Errorf("initial caddy load: %w", err)
	}

	// Start metrics collector
	d.collector = metrics.NewCollector(database.DB)
	d.collector.Start()

	// Start log store and tailer
	d.logStore = logstore.New(database.DB)
	d.tailer = logbuf.NewTailer(accessLogPath, d.logStore)
	d.tailer.Start()

	// Wire log store into DNS server
	d.dnsServer.SetLogStore(d.logStore)

	// Start periodic log cleanup (every 10 minutes, deletes entries older than 24h)
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if err := d.logStore.Cleanup(); err != nil {
				log.Printf("Warning: log cleanup error: %v", err)
			}
		}
	}()

	// Start API server
	d.apiServer = server.New(d.config.APIAddr, database.DB, d.routeSvc, d.tldSvc, d.dnsEntrySvc, d.certMgr, d.collector, d.logStore, d.dnsServer)
	if err := d.apiServer.Start(); err != nil {
		return fmt.Errorf("start API server: %w", err)
	}

	// MCP HTTP server
	mcpAddr := d.config.MCPAddr
	if mcpAddr == "" {
		mcpAddr = ":7801"
	}
	mcpSrv := mcpserver.New(d.routeSvc, d.tldSvc, d.dnsEntrySvc)
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return mcpSrv.Server()
	}, nil)
	d.mcpHTTP = &http.Server{
		Addr:    mcpAddr,
		Handler: handler,
	}
	go func() {
		log.Printf("MCP HTTP server listening on %s", mcpAddr)
		if err := d.mcpHTTP.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("MCP HTTP server error: %v", err)
		}
	}()

	// Write PID file
	if err := writePID(); err != nil {
		log.Printf("Warning: failed to write PID file: %v", err)
	}

	log.Println("valetd is running")
	return nil
}

// Stop gracefully shuts down all components.
func (d *Daemon) Stop() {
	log.Println("Stopping valetd...")

	if d.tailer != nil {
		d.tailer.Stop()
	}

	if d.collector != nil {
		d.collector.Stop()
	}

	if d.mcpHTTP != nil {
		d.mcpHTTP.Shutdown(context.Background())
	}

	if d.apiServer != nil {
		d.apiServer.Stop()
	}

	caddy.Stop()

	if d.dnsServer != nil {
		d.dnsServer.Stop()
	}

	if d.database != nil {
		d.database.Close()
	}

	removePID()
	log.Println("valetd stopped")
}

// PID file management

func PIDFile() (string, error) {
	dir, err := db.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "valetd.pid"), nil
}

func writePID() error {
	path, err := PIDFile()
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o644)
}

func removePID() {
	path, _ := PIDFile()
	if path != "" {
		os.Remove(path)
	}
}

// ReadPID returns the PID of a running valetd, or 0 if not running.
func ReadPID() (int, error) {
	path, err := PIDFile()
	if err != nil {
		return 0, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, nil
	}
	// Check if process is actually running
	proc, err := os.FindProcess(pid)
	if err != nil {
		return 0, nil
	}
	// On Unix, FindProcess always succeeds. Signal 0 checks existence.
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return 0, nil
	}
	return pid, nil
}
