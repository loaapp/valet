package dns

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	mdns "github.com/miekg/dns"
)

// Server is an embedded DNS server that resolves known route domains to 127.0.0.1
// and forwards all other queries to upstream DNS.
type Server struct {
	mu      sync.RWMutex
	domains map[string]bool // exact domains with routes (e.g., "foo.godaddy.com")
	tlds    map[string]bool // managed TLDs for wildcard resolution (e.g., "test")

	udpServer *mdns.Server
	tcpServer *mdns.Server
}

func NewServer() *Server {
	return &Server{
		domains: make(map[string]bool),
		tlds:    make(map[string]bool),
	}
}

// SetTLDs replaces the set of managed TLDs (wildcard: all *.tld → 127.0.0.1).
func (s *Server) SetTLDs(tlds []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tlds = make(map[string]bool, len(tlds))
	for _, tld := range tlds {
		s.tlds[strings.TrimPrefix(tld, ".")] = true
	}
}

// SetDomains replaces the set of known route domains (exact match → 127.0.0.1).
func (s *Server) SetDomains(domains []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.domains = make(map[string]bool, len(domains))
	for _, d := range domains {
		s.domains[strings.ToLower(d)] = true
	}
}

// Start begins listening for DNS queries on the given address (e.g., ":53").
func (s *Server) Start(addr string) error {
	mux := mdns.NewServeMux()
	mux.HandleFunc(".", s.handleDNS)

	s.udpServer = &mdns.Server{Addr: addr, Net: "udp", Handler: mux}
	s.tcpServer = &mdns.Server{Addr: addr, Net: "tcp", Handler: mux}

	errCh := make(chan error, 2)

	go func() {
		log.Printf("DNS server listening on %s (UDP)", addr)
		errCh <- s.udpServer.ListenAndServe()
	}()

	go func() {
		log.Printf("DNS server listening on %s (TCP)", addr)
		errCh <- s.tcpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("dns server failed: %w", err)
	default:
		return nil
	}
}

// Stop shuts down the DNS server.
func (s *Server) Stop() {
	if s.udpServer != nil {
		s.udpServer.Shutdown()
	}
	if s.tcpServer != nil {
		s.tcpServer.Shutdown()
	}
}

func (s *Server) handleDNS(w mdns.ResponseWriter, r *mdns.Msg) {
	m := new(mdns.Msg)
	m.SetReply(r)
	m.Authoritative = true
	m.Compress = true

	for _, q := range r.Question {
		if q.Qtype == mdns.TypeA || q.Qtype == mdns.TypeAAAA {
			if s.shouldResolveLocally(q.Name) {
				if q.Qtype == mdns.TypeA {
					m.Answer = append(m.Answer, &mdns.A{
						Hdr: mdns.RR_Header{
							Name:   q.Name,
							Rrtype: mdns.TypeA,
							Class:  mdns.ClassINET,
							Ttl:    60,
						},
						A: net.ParseIP("127.0.0.1"),
					})
				}
				// AAAA: return empty (no IPv6 loopback needed for local dev)
			} else {
				// Forward to upstream DNS
				s.forwardQuery(w, r)
				return
			}
		}
	}

	w.WriteMsg(m)
}

// shouldResolveLocally checks if a query should resolve to 127.0.0.1.
// Returns true if:
//  1. The domain exactly matches a known route, OR
//  2. The domain's TLD is a managed wildcard TLD
func (s *Server) shouldResolveLocally(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// name is FQDN with trailing dot, e.g., "foo.godaddy.com."
	clean := strings.ToLower(strings.TrimSuffix(name, "."))

	// Check exact domain match (route-based)
	if s.domains[clean] {
		return true
	}

	// Check wildcard TLD match
	parts := strings.Split(clean, ".")
	if len(parts) >= 2 {
		tld := parts[len(parts)-1]
		if s.tlds[tld] {
			return true
		}
	}

	return false
}

// forwardQuery forwards a DNS query to the system's upstream DNS resolver.
func (s *Server) forwardQuery(w mdns.ResponseWriter, r *mdns.Msg) {
	upstream := "8.8.8.8:53"

	client := new(mdns.Client)
	resp, _, err := client.Exchange(r, upstream)
	if err != nil {
		log.Printf("DNS forward error: %v", err)
		m := new(mdns.Msg)
		m.SetRcode(r, mdns.RcodeServerFailure)
		w.WriteMsg(m)
		return
	}
	w.WriteMsg(resp)
}
