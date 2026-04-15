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
	"github.com/loaapp/valet/valetd/internal/resolver"
	"github.com/loaapp/valet/valetd/internal/routes"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	// Handle subcommands before flag parsing.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "mcp":
			runMCP()
			return
		case "dns":
			runDNS()
			return
		}
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

func runDNS() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: valetd dns <install|uninstall|status>")
		os.Exit(1)
	}

	database, err := db.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	tlds, err := db.ListTLDs(database.DB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list TLDs: %v\n", err)
		os.Exit(1)
	}

	switch os.Args[2] {
	case "install":
		if len(tlds) == 0 {
			fmt.Println("No managed TLDs configured. Add one first with: valet tld add <tld>")
			return
		}
		for _, t := range tlds {
			if err := resolver.Install(t.TLD); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to install resolver for .%s: %v\n", t.TLD, err)
			} else {
				db.UpdateTLDResolver(database.DB, t.TLD, true)
				fmt.Printf("Installed /etc/resolver/%s\n", t.TLD)
			}
		}
		fmt.Println("Done. DNS resolvers installed.")

	case "uninstall":
		for _, t := range tlds {
			if err := resolver.Remove(t.TLD); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to remove resolver for .%s: %v\n", t.TLD, err)
			} else {
				db.UpdateTLDResolver(database.DB, t.TLD, false)
				fmt.Printf("Removed /etc/resolver/%s\n", t.TLD)
			}
		}
		fmt.Println("Done. DNS resolvers removed.")

	case "status":
		if len(tlds) == 0 {
			fmt.Println("No managed TLDs configured.")
			return
		}
		for _, t := range tlds {
			installed := resolver.IsInstalled(t.TLD)
			status := "not installed"
			if installed {
				status = "installed"
			}
			fmt.Printf(".%-10s %s\n", t.TLD, status)
		}

	default:
		fmt.Fprintln(os.Stderr, "Usage: valetd dns <install|uninstall|status>")
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
