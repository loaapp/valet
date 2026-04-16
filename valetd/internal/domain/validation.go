package domain

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	domainRe   = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?\.[a-z]+$`)
	hostPortRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+:\d+$`)
)

func ValidateDomain(domain string) (string, error) {
	domain = strings.TrimSpace(strings.ToLower(domain))
	if domain == "" {
		return "", fmt.Errorf("domain is required")
	}
	if strings.Contains(domain, " ") {
		return "", fmt.Errorf("domain must not contain spaces")
	}
	if strings.Contains(domain, "://") {
		return "", fmt.Errorf("do not include http:// or https://")
	}
	if strings.Contains(domain, "/") {
		return "", fmt.Errorf("do not include paths")
	}
	if strings.Contains(domain, ":") {
		return "", fmt.Errorf("do not include a port")
	}
	if !domainRe.MatchString(domain) {
		return "", fmt.Errorf("must be a valid domain (e.g., myapp.test)")
	}
	return domain, nil
}

func NormalizeUpstream(upstream string) string {
	upstream = strings.TrimSpace(upstream)
	upstream = strings.TrimPrefix(upstream, "http://")
	upstream = strings.TrimPrefix(upstream, "https://")
	if strings.HasPrefix(upstream, ":") {
		upstream = "localhost" + upstream
	}
	return upstream
}

func ValidateUpstream(upstream string) (string, error) {
	upstream = NormalizeUpstream(upstream)
	if upstream == "" {
		return "", nil // empty is OK for template mode
	}
	if strings.Contains(upstream, " ") {
		return "", fmt.Errorf("upstream must not contain spaces")
	}
	if !hostPortRe.MatchString(upstream) {
		return "", fmt.Errorf("upstream must be host:port format (e.g., localhost:3000)")
	}
	return upstream, nil
}
