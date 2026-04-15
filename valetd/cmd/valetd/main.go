package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
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
	root.AddCommand(tldCmd())

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

func tldCmd() *cobra.Command {
	tldRoot := &cobra.Command{
		Use:   "tld",
		Short: "Manage TLDs and DNS resolvers",
	}

	addCmd := &cobra.Command{
		Use:   "add",
		Short: "Register a managed domain/TLD and install its DNS resolver (requires sudo)",
		Example: "  sudo valetd tld add --tld test          # all *.test → local DNS\n  sudo valetd tld add --tld godaddy.com   # all *.godaddy.com → local DNS",
		RunE: func(cmd *cobra.Command, args []string) error {
			tld, _ := cmd.Flags().GetString("tld")
			tld = strings.TrimPrefix(tld, ".")
			if tld == "" {
				return fmt.Errorf("--tld is required (e.g., --tld test)")
			}

			database, err := db.Open()
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			// Register in database
			existing, _ := db.GetTLD(database.DB, tld)
			if existing == nil {
				if _, err := db.CreateTLD(database.DB, tld); err != nil {
					return fmt.Errorf("register TLD: %w", err)
				}
				fmt.Printf("Registered .%s in database\n", tld)
			} else {
				fmt.Printf(".%s already registered in database\n", tld)
			}

			// Install resolver file
			if err := resolver.Install(tld); err != nil {
				return fmt.Errorf("install resolver for .%s: %w", tld, err)
			}
			db.UpdateTLDResolver(database.DB, tld, true)
			fmt.Printf("Installed /etc/resolver/%s\n", tld)
			fmt.Printf("Done. All *.%s domains will resolve to 127.0.0.1\n", tld)
			return nil
		},
	}
	addCmd.Flags().String("tld", "", "TLD or domain to manage (e.g., test, godaddy.com)")
	addCmd.MarkFlagRequired("tld")
	tldRoot.AddCommand(addCmd)

	removeCmd := &cobra.Command{
		Use:   "remove",
		Short: "Unregister a managed TLD and remove its DNS resolver (requires sudo)",
		Example: "  sudo valetd tld remove --tld test",
		RunE: func(cmd *cobra.Command, args []string) error {
			tld, _ := cmd.Flags().GetString("tld")
			tld = strings.TrimPrefix(tld, ".")
			if tld == "" {
				return fmt.Errorf("--tld is required")
			}

			database, err := db.Open()
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			// Remove resolver file
			if err := resolver.Remove(tld); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to remove resolver for .%s: %v\n", tld, err)
			} else {
				fmt.Printf("Removed /etc/resolver/%s\n", tld)
			}

			// Remove from database
			db.UpdateTLDResolver(database.DB, tld, false)
			if err := db.DeleteTLD(database.DB, tld); err != nil {
				return fmt.Errorf("remove TLD from database: %w", err)
			}
			fmt.Printf("Unregistered .%s\n", tld)
			return nil
		},
	}
	removeCmd.Flags().String("tld", "", "TLD to remove")
	removeCmd.MarkFlagRequired("tld")
	tldRoot.AddCommand(removeCmd)

	tldRoot.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Show all managed TLDs and resolver status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withTLDs(func(database *db.AppDB, tlds []db.ManagedTLD) error {
				if len(tlds) == 0 {
					fmt.Println("No managed TLDs configured.")
					fmt.Println("Add one with: sudo valetd tld add --tld test")
					return nil
				}
				fmt.Printf("%-12s %s\n", "TLD", "RESOLVER")
				for _, t := range tlds {
					status := "not installed"
					if resolver.IsInstalled(t.TLD) {
						status = "installed"
					}
					fmt.Printf(".%-11s %s\n", t.TLD, status)
				}
				return nil
			})
		},
	})

	return tldRoot
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
