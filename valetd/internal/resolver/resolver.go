package resolver

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	resolverDir = "/etc/resolver"
	DNSPort     = 15353
)

// Install creates a resolver file for the given domain/TLD so that macOS
// sends DNS queries to 127.0.0.1 on the Valet DNS port.
// Requires root privileges.
func Install(tld string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("resolver files are only supported on macOS")
	}

	if err := os.MkdirAll(resolverDir, 0o755); err != nil {
		return fmt.Errorf("create resolver dir: %w", err)
	}

	content := fmt.Sprintf("nameserver 127.0.0.1\nport %d\n", DNSPort)
	path := filepath.Join(resolverDir, tld)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write resolver file for %s: %w", tld, err)
	}

	return nil
}

// Remove deletes the resolver file for the given TLD.
// Requires root privileges.
func Remove(tld string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("resolver files are only supported on macOS")
	}

	path := filepath.Join(resolverDir, tld)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove resolver file for .%s: %w", tld, err)
	}
	return nil
}

// IsInstalled checks if a resolver file exists for the given TLD.
func IsInstalled(tld string) bool {
	path := filepath.Join(resolverDir, tld)
	_, err := os.Stat(path)
	return err == nil
}
