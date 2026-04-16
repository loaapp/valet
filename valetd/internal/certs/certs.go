package certs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/loaapp/valet/valetd/internal/db"
)

type Manager struct {
	certsDir string
}

func NewManager() (*Manager, error) {
	dataDir, err := db.DataDir()
	if err != nil {
		return nil, err
	}
	certsDir := filepath.Join(dataDir, "certs")
	if err := os.MkdirAll(certsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create certs dir: %w", err)
	}
	return &Manager{certsDir: certsDir}, nil
}

// MkcertAvailable checks if mkcert is installed.
func MkcertAvailable() bool {
	_, err := exec.LookPath(mkcertPath())
	return err == nil
}

// mkcertPath returns the full path to mkcert, checking Homebrew locations
// when running under launchd (which provides a minimal PATH).
func mkcertPath() string {
	if p, err := exec.LookPath("mkcert"); err == nil {
		return p
	}
	for _, p := range []string{"/opt/homebrew/bin/mkcert", "/usr/local/bin/mkcert"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "mkcert" // fall back to bare name
}

// InstallCA runs `mkcert -install` to install the local CA.
func InstallCA() error {
	cmd := exec.Command(mkcertPath(), "-install")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// GenerateCert creates a trusted certificate for the given domain.
// Returns (certPath, keyPath, error).
func (m *Manager) GenerateCert(domain string) (string, string, error) {
	certPath := filepath.Join(m.certsDir, domain+".pem")
	keyPath := filepath.Join(m.certsDir, domain+"-key.pem")

	cmd := exec.Command(mkcertPath(),
		"-cert-file", certPath,
		"-key-file", keyPath,
		domain,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("mkcert for %s: %w", domain, err)
	}

	return certPath, keyPath, nil
}

// GenerateCombinedCert creates a single certificate covering all given domains
// plus localhost and 127.0.0.1. This allows connections via any hostname.
// Returns (certPath, keyPath, error).
func (m *Manager) GenerateCombinedCert(domains []string) (string, string, error) {
	certPath := filepath.Join(m.certsDir, "_valet.pem")
	keyPath := filepath.Join(m.certsDir, "_valet-key.pem")

	args := []string{
		"-cert-file", certPath,
		"-key-file", keyPath,
		"localhost", "127.0.0.1", "::1",
	}
	args = append(args, domains...)

	cmd := exec.Command(mkcertPath(), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("mkcert combined cert: %w", err)
	}

	return certPath, keyPath, nil
}

// CombinedCertPath returns the path to the combined cert, or empty if it doesn't exist.
func (m *Manager) CombinedCertPath() (string, string) {
	certPath := filepath.Join(m.certsDir, "_valet.pem")
	keyPath := filepath.Join(m.certsDir, "_valet-key.pem")
	if _, err := os.Stat(certPath); err != nil {
		return "", ""
	}
	return certPath, keyPath
}

// CertExists checks if a certificate already exists for the domain.
func (m *Manager) CertExists(domain string) bool {
	certPath := filepath.Join(m.certsDir, domain+".pem")
	keyPath := filepath.Join(m.certsDir, domain+"-key.pem")
	_, errCert := os.Stat(certPath)
	_, errKey := os.Stat(keyPath)
	return errCert == nil && errKey == nil
}

// RemoveCert removes the certificate files for a domain.
func (m *Manager) RemoveCert(domain string) error {
	certPath := filepath.Join(m.certsDir, domain+".pem")
	keyPath := filepath.Join(m.certsDir, domain+"-key.pem")
	os.Remove(certPath)
	os.Remove(keyPath)
	return nil
}
