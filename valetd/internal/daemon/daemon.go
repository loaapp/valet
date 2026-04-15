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

	"github.com/loaapp/valet/valetd/internal/caddy"
	"github.com/loaapp/valet/valetd/internal/certs"
	"github.com/loaapp/valet/valetd/internal/db"
	"github.com/loaapp/valet/valetd/internal/dns"
	"github.com/loaapp/valet/valetd/internal/logbuf"
	"github.com/loaapp/valet/valetd/internal/mcpserver"
	"github.com/loaapp/valet/valetd/internal/metrics"
	"github.com/loaapp/valet/valetd/internal/routes"
	"github.com/loaapp/valet/valetd/internal/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Config struct {
	APIAddr string // e.g., ":7800"
	DNSAddr string // e.g., ":53"
	MCPAddr string // e.g., ":7801" — MCP HTTP endpoint
}

type Daemon struct {
	config    Config
	database  *db.AppDB
	dnsServer *dns.Server
	apiServer *server.Server
	routeMgr  *routes.Manager
	certMgr   *certs.Manager
	mcpHTTP   *http.Server
	collector *metrics.Collector
	logBuf    *logbuf.RingBuffer
	tailer    *logbuf.Tailer
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

	// Route manager
	d.routeMgr = routes.NewManager(database.DB, certMgr, d.dnsServer, dataDir)

	// Sync DNS TLDs from database
	if err := d.routeMgr.SyncDNS(); err != nil {
		log.Printf("Warning: failed to sync DNS TLDs: %v", err)
	}

	// Start DNS server
	if d.config.DNSAddr != "" {
		if err := d.dnsServer.Start(d.config.DNSAddr); err != nil {
			log.Printf("Warning: DNS server failed to start on %s: %v (may need sudo)", d.config.DNSAddr, err)
			// Non-fatal — DNS is optional if using /etc/hosts
		}
	}

	// Load initial Caddy config from existing routes with combined cert
	routeList, err := db.ListRoutes(database.DB)
	if err != nil {
		return fmt.Errorf("list routes: %w", err)
	}
	combinedCert, combinedKey := certMgr.CombinedCertPath()
	if combinedCert == "" && len(routeList) > 0 && certs.MkcertAvailable() {
		var tlsDomains []string
		for _, r := range routeList {
			if r.TLSEnabled {
				tlsDomains = append(tlsDomains, r.Domain)
			}
		}
		if len(tlsDomains) > 0 {
			combinedCert, combinedKey, _ = certMgr.GenerateCombinedCert(tlsDomains)
		}
	}
	// Redirect stderr to access.log so Caddy's structured logs are captured
	accessLogPath := filepath.Join(dataDir, "access.log")
	accessLogFile, err := os.OpenFile(accessLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("Warning: cannot open access log: %v", err)
	} else {
		os.Stderr = accessLogFile
	}

	if err := caddy.Reload(routeList, combinedCert, combinedKey, dataDir); err != nil {
		return fmt.Errorf("initial caddy load: %w", err)
	}

	// Start metrics collector
	d.collector = metrics.NewCollector(database.DB)
	d.collector.Start()

	// Start log buffer and tailer
	d.logBuf = logbuf.New(10000)
	d.tailer = logbuf.NewTailer(filepath.Join(dataDir, "access.log"), d.logBuf)
	d.tailer.Start()

	// Start API server
	d.apiServer = server.New(d.config.APIAddr, database.DB, d.routeMgr, d.certMgr, d.collector, d.logBuf, d.dnsServer)
	if err := d.apiServer.Start(); err != nil {
		return fmt.Errorf("start API server: %w", err)
	}

	// MCP HTTP server
	mcpAddr := d.config.MCPAddr
	if mcpAddr == "" {
		mcpAddr = ":7801"
	}
	mcpSrv := mcpserver.New(d.database.DB, d.routeMgr, d.certMgr)
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
