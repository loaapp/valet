package main

import (
	"context"
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
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "valetd",
		Short: "Valet daemon — local development reverse proxy",
	}

	root.AddCommand(serveCmd())
	root.AddCommand(mcpCmd())
	root.AddCommand(dnsCmd())

	// Default to serve if no subcommand given
	root.RunE = serveCmd().RunE

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func serveCmd() *cobra.Command {
	var apiAddr, dnsAddr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the daemon (default)",
		RunE: func(cmd *cobra.Command, args []string) error {
			setupLogging()

			d := daemon.New(daemon.Config{
				APIAddr: apiAddr,
				DNSAddr: dnsAddr,
			})

			if err := d.Start(); err != nil {
				log.Fatalf("Failed to start: %v", err)
			}

			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
			<-sig

			d.Stop()
			return nil
		},
	}
	cmd.Flags().StringVar(&apiAddr, "api", ":7800", "API listen address")
	cmd.Flags().StringVar(&dnsAddr, "dns", ":53", "DNS listen address (empty to disable)")
	return cmd
}

func mcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run MCP server on stdio (for Claude Code)",
		RunE: func(cmd *cobra.Command, args []string) error {
			database, err := db.Open()
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			certMgr, err := certs.NewManager()
			if err != nil {
				return fmt.Errorf("init cert manager: %w", err)
			}

			dataDir, err := db.DataDir()
			if err != nil {
				return fmt.Errorf("get data dir: %w", err)
			}

			dnsServer := dns.NewServer()
			routeMgr := routes.NewManager(database.DB, certMgr, dnsServer, dataDir)
			mcpSrv := mcpserver.New(database.DB, routeMgr, certMgr)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
			go func() { <-sig; cancel() }()

			session, err := mcpSrv.Server().Connect(ctx, &mcp.StdioTransport{}, nil)
			if err != nil {
				return fmt.Errorf("MCP connect: %w", err)
			}
			return session.Wait()
		},
	}
}

func dnsCmd() *cobra.Command {
	dnsRoot := &cobra.Command{
		Use:   "dns",
		Short: "Manage DNS resolvers",
	}

	dnsRoot.AddCommand(&cobra.Command{
		Use:   "install",
		Short: "Install /etc/resolver/ files for all managed TLDs (requires sudo)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withTLDs(func(database *db.AppDB, tlds []db.ManagedTLD) error {
				if len(tlds) == 0 {
					fmt.Println("No managed TLDs configured. Add one first with: valet tld add <tld>")
					return nil
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
				return nil
			})
		},
	})

	dnsRoot.AddCommand(&cobra.Command{
		Use:   "uninstall",
		Short: "Remove /etc/resolver/ files for all managed TLDs (requires sudo)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withTLDs(func(database *db.AppDB, tlds []db.ManagedTLD) error {
				for _, t := range tlds {
					if err := resolver.Remove(t.TLD); err != nil {
						fmt.Fprintf(os.Stderr, "Failed to remove resolver for .%s: %v\n", t.TLD, err)
					} else {
						db.UpdateTLDResolver(database.DB, t.TLD, false)
						fmt.Printf("Removed /etc/resolver/%s\n", t.TLD)
					}
				}
				fmt.Println("Done. DNS resolvers removed.")
				return nil
			})
		},
	})

	dnsRoot.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show DNS resolver status for all managed TLDs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withTLDs(func(database *db.AppDB, tlds []db.ManagedTLD) error {
				if len(tlds) == 0 {
					fmt.Println("No managed TLDs configured.")
					return nil
				}
				for _, t := range tlds {
					status := "not installed"
					if resolver.IsInstalled(t.TLD) {
						status = "installed"
					}
					fmt.Printf(".%-10s %s\n", t.TLD, status)
				}
				return nil
			})
		},
	})

	return dnsRoot
}

// withTLDs opens the database, loads TLDs, and calls fn.
func withTLDs(fn func(*db.AppDB, []db.ManagedTLD) error) error {
	database, err := db.Open()
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	tlds, err := db.ListTLDs(database.DB)
	if err != nil {
		return fmt.Errorf("list TLDs: %w", err)
	}
	return fn(database, tlds)
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
