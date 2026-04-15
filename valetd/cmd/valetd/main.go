package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/loaapp/valet/valetd/internal/certs"
	"github.com/loaapp/valet/valetd/internal/daemon"
	"github.com/loaapp/valet/valetd/internal/db"
	"github.com/loaapp/valet/valetd/internal/dns"
	"github.com/loaapp/valet/valetd/internal/mcpserver"
	"github.com/loaapp/valet/valetd/internal/routes"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	// Handle "mcp" subcommand before flag parsing.
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		runMCP()
		return
	}

	apiAddr := flag.String("api", ":7800", "API listen address")
	dnsAddr := flag.String("dns", ":53", "DNS listen address (empty to disable)")
	flag.Parse()

	// Setup logging to file
	setupLogging()

	d := daemon.New(daemon.Config{
		APIAddr: *apiAddr,
		DNSAddr: *dnsAddr,
	})

	if err := d.Start(); err != nil {
		log.Fatalf("Failed to start: %v", err)
	}

	// Wait for shutdown signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	d.Stop()
}

func runMCP() {
	database, err := db.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	certMgr, err := certs.NewManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init cert manager: %v\n", err)
		os.Exit(1)
	}

	dataDir, err := db.DataDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get data dir: %v\n", err)
		os.Exit(1)
	}

	dnsServer := dns.NewServer()
	routeMgr := routes.NewManager(database.DB, certMgr, dnsServer, dataDir)

	mcpSrv := mcpserver.New(database.DB, routeMgr, certMgr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	session, err := mcpSrv.Server().Connect(ctx, &mcp.StdioTransport{}, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "MCP connect error: %v\n", err)
		os.Exit(1)
	}

	// Block until the session ends.
	if err := session.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "MCP session error: %v\n", err)
		os.Exit(1)
	}
}

func setupLogging() {
	dir, err := db.DataDir()
	if err != nil {
		return
	}
	os.MkdirAll(dir, 0o755)

	logPath := filepath.Join(dir, "valetd.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
