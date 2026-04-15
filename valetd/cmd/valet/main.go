package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"text/tabwriter"

	"github.com/loaapp/valet/pkg/client"
	"github.com/loaapp/valet/pkg/models"
	"github.com/loaapp/valet/valetd/internal/daemon"
	"github.com/spf13/cobra"
)

var c = client.New()

func main() {
	root := &cobra.Command{
		Use:   "valet",
		Short: "Local development reverse proxy manager",
	}

	root.AddCommand(upCmd())
	root.AddCommand(downCmd())
	root.AddCommand(statusCmd())
	root.AddCommand(addCmd())
	root.AddCommand(removeCmd())
	root.AddCommand(listCmd())
	root.AddCommand(tldCmd())
	root.AddCommand(trustCmd())
	root.AddCommand(logsCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func upCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Start the valetd daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, _ := daemon.ReadPID()
			if pid > 0 {
				fmt.Printf("valetd is already running (PID %d)\n", pid)
				return nil
			}

			valetdPath, err := exec.LookPath("valetd")
			if err != nil {
				exe, _ := os.Executable()
				valetdPath = filepath.Join(filepath.Dir(exe), "valetd")
			}

			proc := exec.Command(valetdPath)
			proc.SysProcAttr = &syscall.SysProcAttr{
				Setpgid: true,
			}
			if err := proc.Start(); err != nil {
				return fmt.Errorf("failed to start valetd: %w", err)
			}

			fmt.Printf("valetd started (PID %d)\n", proc.Process.Pid)
			return nil
		},
	}
}

func downCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Stop the valetd daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, _ := daemon.ReadPID()
			if pid == 0 {
				fmt.Println("valetd is not running")
				return nil
			}

			proc, err := os.FindProcess(pid)
			if err != nil {
				return fmt.Errorf("find process: %w", err)
			}
			if err := proc.Signal(syscall.SIGTERM); err != nil {
				return fmt.Errorf("send signal: %w", err)
			}
			fmt.Println("valetd stopping...")
			return nil
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := c.GetStatus()
			if err != nil {
				fmt.Println("valetd is not running")
				return nil
			}

			fmt.Println("valetd is running")
			fmt.Printf("  Routes: %d\n", status.Routes)
			fmt.Printf("  TLDs:   %d\n", status.TLDs)
			fmt.Printf("  mkcert: %v\n", status.Mkcert)
			return nil
		},
	}
}

func addCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <domain> <upstream>",
		Short: "Add a proxy route",
		Long:  "Add a proxy route (e.g., valet add myapp.test localhost:3000)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			tls := true
			route, err := c.CreateRoute(models.CreateRouteRequest{
				Domain:   args[0],
				Upstream: args[1],
				TLS:      &tls,
			})
			if err != nil {
				return err
			}

			proto := "https"
			if !route.TLSEnabled {
				proto = "http"
			}
			fmt.Printf("Route added: %s://%s -> %s\n", proto, route.Domain, route.Upstream)
			return nil
		},
	}
	return cmd
}

func removeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <domain>",
		Short: "Remove a proxy route",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := args[0]

			routes, err := c.ListRoutes()
			if err != nil {
				return err
			}

			var routeID string
			for _, r := range routes {
				if r.Domain == domain {
					routeID = r.ID
					break
				}
			}
			if routeID == "" {
				return fmt.Errorf("no route for %s", domain)
			}

			if err := c.DeleteRoute(routeID); err != nil {
				return err
			}
			fmt.Printf("Route removed: %s\n", domain)
			return nil
		},
	}
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all proxy routes",
		RunE: func(cmd *cobra.Command, args []string) error {
			routes, err := c.ListRoutes()
			if err != nil {
				return err
			}

			if len(routes) == 0 {
				fmt.Println("No routes configured")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "DOMAIN\tUPSTREAM\tTLS\tCREATED")
			for _, r := range routes {
				tls := "yes"
				if !r.TLSEnabled {
					tls = "no"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Domain, r.Upstream, tls, r.CreatedAt)
			}
			w.Flush()
			return nil
		},
	}
}

func tldCmd() *cobra.Command {
	tld := &cobra.Command{
		Use:   "tld",
		Short: "Manage TLDs",
	}

	tld.AddCommand(&cobra.Command{
		Use:   "add <tld>",
		Short: "Register a managed TLD (e.g., valet tld add test)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := c.CreateTLD(args[0]); err != nil {
				return err
			}
			fmt.Printf("TLD .%s registered. Run 'valet trust' to install the DNS resolver.\n", args[0])
			return nil
		},
	})

	tld.AddCommand(&cobra.Command{
		Use:   "remove <tld>",
		Short: "Unregister a managed TLD",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := c.DeleteTLD(args[0]); err != nil {
				return err
			}
			fmt.Printf("TLD .%s removed\n", args[0])
			return nil
		},
	})

	tld.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List managed TLDs",
		RunE: func(cmd *cobra.Command, args []string) error {
			tlds, err := c.ListTLDs()
			if err != nil {
				return err
			}

			if len(tlds) == 0 {
				fmt.Println("No managed TLDs")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "TLD\tRESOLVER\tCREATED")
			for _, t := range tlds {
				resolver := "not installed"
				if t.ResolverInstalled {
					resolver = "installed"
				}
				fmt.Fprintf(w, ".%s\t%s\t%s\n", t.TLD, resolver, t.CreatedAt)
			}
			w.Flush()
			return nil
		},
	})

	return tld
}

func trustCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "trust",
		Short: "Install DNS resolvers for managed TLDs (may require sudo)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := c.Trust(); err != nil {
				return fmt.Errorf("trust failed (you may need to run valetd with sudo for this operation): %w", err)
			}
			fmt.Println("DNS resolvers installed for all managed TLDs")
			return nil
		},
	}
}

func logsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs",
		Short: "Tail valetd logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			logPath := filepath.Join(home, ".valet", "valetd.log")

			tail := exec.Command("tail", "-f", logPath)
			tail.Stdout = os.Stdout
			tail.Stderr = os.Stderr
			return tail.Run()
		},
	}
}
