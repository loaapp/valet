package dns

import "testing"

func TestGetTarget_EntryPriority(t *testing.T) {
	s := NewServer()

	// Set up all three resolution sources
	s.SetEntries([]DNSEntry{{Domain: "app.test", Target: "10.0.0.1"}})
	s.SetDomains([]string{"app.test"})
	s.SetTLDs([]string{"test"})

	// dns_entries should take priority
	target := s.getTarget("app.test")
	if target != "10.0.0.1" {
		t.Errorf("getTarget = %q, want %q (entry priority)", target, "10.0.0.1")
	}
}

func TestGetTarget_RouteDomain(t *testing.T) {
	s := NewServer()
	s.SetDomains([]string{"myapp.test"})

	target := s.getTarget("myapp.test")
	if target != "127.0.0.1" {
		t.Errorf("getTarget = %q, want %q", target, "127.0.0.1")
	}
}

func TestGetTarget_TLDWildcard(t *testing.T) {
	s := NewServer()
	s.SetTLDs([]string{"test"})

	// Any subdomain under .test should resolve
	target := s.getTarget("anything.test")
	if target != "127.0.0.1" {
		t.Errorf("getTarget = %q, want %q", target, "127.0.0.1")
	}

	// Multi-level subdomain
	target = s.getTarget("deep.sub.test")
	if target != "127.0.0.1" {
		t.Errorf("deep subdomain: getTarget = %q, want %q", target, "127.0.0.1")
	}
}

func TestGetTarget_NoMatch(t *testing.T) {
	s := NewServer()
	s.SetTLDs([]string{"test"})
	s.SetDomains([]string{"myapp.test"})

	target := s.getTarget("google.com")
	if target != "" {
		t.Errorf("getTarget for unmanaged domain = %q, want empty", target)
	}
}

func TestGetTarget_CaseInsensitive(t *testing.T) {
	s := NewServer()
	s.SetDomains([]string{"MyApp.Test"})

	target := s.getTarget("myapp.test")
	if target != "127.0.0.1" {
		t.Errorf("getTarget case-insensitive = %q, want %q", target, "127.0.0.1")
	}
}

func TestGetTarget_CNAMEEntry(t *testing.T) {
	s := NewServer()
	s.SetEntries([]DNSEntry{{Domain: "app.test", Target: "other-host.example.com"}})

	target := s.getTarget("app.test")
	if target != "other-host.example.com" {
		t.Errorf("getTarget CNAME = %q, want %q", target, "other-host.example.com")
	}
}

func TestGetTarget_SinglePartDomain(t *testing.T) {
	s := NewServer()
	s.SetTLDs([]string{"test"})

	// Single-part domain should NOT match TLD wildcard (need at least domain.tld)
	target := s.getTarget("test")
	if target != "" {
		t.Errorf("single-part domain should not match TLD wildcard, got %q", target)
	}
}

func TestSetDomains_Replaces(t *testing.T) {
	s := NewServer()
	s.SetDomains([]string{"old.test"})
	s.SetDomains([]string{"new.test"})

	if s.getTarget("old.test") != "" {
		t.Error("old domain should be cleared after SetDomains")
	}
	if s.getTarget("new.test") != "127.0.0.1" {
		t.Error("new domain should be set")
	}
}

func TestSetEntries_Replaces(t *testing.T) {
	s := NewServer()
	s.SetEntries([]DNSEntry{{Domain: "old.test", Target: "1.1.1.1"}})
	s.SetEntries([]DNSEntry{{Domain: "new.test", Target: "2.2.2.2"}})

	if s.getTarget("old.test") != "" {
		t.Error("old entry should be cleared after SetEntries")
	}
	if s.getTarget("new.test") != "2.2.2.2" {
		t.Error("new entry should be set")
	}
}

func TestSetTLDs_StripsDot(t *testing.T) {
	s := NewServer()
	s.SetTLDs([]string{".test"})

	target := s.getTarget("myapp.test")
	if target != "127.0.0.1" {
		t.Errorf("TLD with leading dot: getTarget = %q, want %q", target, "127.0.0.1")
	}
}

func TestShouldResolveLocally(t *testing.T) {
	s := NewServer()
	s.SetDomains([]string{"myapp.test"})

	if !s.shouldResolveLocally("myapp.test.") {
		t.Error("should resolve locally for known domain (with trailing dot)")
	}
	if s.shouldResolveLocally("unknown.com.") {
		t.Error("should NOT resolve locally for unknown domain")
	}
}
