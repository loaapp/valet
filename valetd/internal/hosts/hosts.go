package hosts

import (
	"fmt"
	"log"
	"os"
	"strings"
)

const (
	beginMarker = "# BEGIN VALET MANAGED BLOCK"
	endMarker   = "# END VALET MANAGED BLOCK"
	hostsFile   = "/etc/hosts"
)

// Sync updates /etc/hosts to include entries for the given domains.
// All domains resolve to 127.0.0.1. The managed block is replaced atomically.
func Sync(domains []string) error {
	content, err := os.ReadFile(hostsFile)
	if err != nil {
		return fmt.Errorf("read hosts file: %w", err)
	}

	existing := string(content)
	updated := replaceBlock(existing, domains)

	// Skip write if nothing changed (avoids needing sudo when unnecessary)
	if updated == existing {
		return nil
	}

	if err := os.WriteFile(hostsFile, []byte(updated), 0o644); err != nil {
		if os.IsPermission(err) {
			log.Printf("Warning: cannot update /etc/hosts (permission denied). Use 'sudo valetd tld add --tld <tld>' to install a resolver instead.")
			return nil
		}
		return fmt.Errorf("write hosts file: %w", err)
	}
	return nil
}

// Clear removes the Valet managed block from /etc/hosts.
func Clear() error {
	return Sync(nil)
}

func replaceBlock(content string, domains []string) string {
	// Remove existing block
	cleaned := removeBlock(content)

	if len(domains) == 0 {
		return cleaned
	}

	// Build new block
	var block strings.Builder
	block.WriteString(beginMarker + "\n")
	for _, d := range domains {
		block.WriteString(fmt.Sprintf("127.0.0.1  %s\n", d))
	}
	block.WriteString(endMarker + "\n")

	// Ensure trailing newline before appending
	if !strings.HasSuffix(cleaned, "\n") {
		cleaned += "\n"
	}

	return cleaned + block.String()
}

func removeBlock(content string) string {
	startIdx := strings.Index(content, beginMarker)
	if startIdx == -1 {
		return content
	}
	endIdx := strings.Index(content, endMarker)
	if endIdx == -1 {
		return content
	}
	endIdx += len(endMarker)
	// Also consume trailing newline
	if endIdx < len(content) && content[endIdx] == '\n' {
		endIdx++
	}

	return content[:startIdx] + content[endIdx:]
}

// ListManaged returns the domains currently in the Valet managed block.
func ListManaged() ([]string, error) {
	content, err := os.ReadFile(hostsFile)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	var domains []string
	inBlock := false
	for _, line := range lines {
		if strings.TrimSpace(line) == beginMarker {
			inBlock = true
			continue
		}
		if strings.TrimSpace(line) == endMarker {
			break
		}
		if inBlock {
			parts := strings.Fields(line)
			if len(parts) >= 2 && parts[0] == "127.0.0.1" {
				domains = append(domains, parts[1])
			}
		}
	}
	return domains, nil
}
