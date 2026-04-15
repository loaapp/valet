package dns

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	mdns "github.com/miekg/dns"
)

// QueryLog represents a single DNS query event.
type QueryLog struct {
	Timestamp float64 `json:"ts"`
	Domain    string  `json:"domain"`
	Type      string  `json:"type"`   // "A", "AAAA", etc.
	Action    string  `json:"action"` // "local", "forwarded"
	Result    string  `json:"result"` // "127.0.0.1", upstream IP, or "error"
}

// QueryBuffer is a ring buffer for DNS query logs.
type QueryBuffer struct {
	mu      sync.RWMutex
	entries []QueryLog
	size    int
	head    int
	count   int
}

func NewQueryBuffer(size int) *QueryBuffer {
	return &QueryBuffer{
		entries: make([]QueryLog, size),
		size:    size,
	}
}

func (b *QueryBuffer) Push(entry QueryLog) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries[b.head] = entry
	b.head = (b.head + 1) % b.size
	if b.count < b.size {
		b.count++
	}
}

func (b *QueryBuffer) Last(n int) []QueryLog {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if n > b.count {
		n = b.count
	}
	if n == 0 {
		return []QueryLog{}
	}
	result := make([]QueryLog, n)
	start := (b.head - n + b.size) % b.size
	for i := 0; i < n; i++ {
		result[i] = b.entries[(start+i)%b.size]
	}
	return result
}

// Server is an embedded DNS server that resolves known route domains to 127.0.0.1
// and forwards all other queries to upstream DNS.
type Server struct {
	mu      sync.RWMutex
	domains map[string]bool // exact domains with routes
	tlds    map[string]bool // managed TLDs for wildcard resolution

	queryLog  *QueryBuffer
	udpServer *mdns.Server
	tcpServer *mdns.Server
}

func NewServer() *Server {
	return &Server{
		domains:  make(map[string]bool),
		tlds:     make(map[string]bool),
		queryLog: NewQueryBuffer(5000),
	}
}

// QueryLogs returns the DNS query log buffer.
func (s *Server) QueryLogs() *QueryBuffer {
	return s.queryLog
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

// Start begins listening for DNS queries on the given address.
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
		qtype := mdns.TypeToString[q.Qtype]
		clean := strings.ToLower(strings.TrimSuffix(q.Name, "."))

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
				s.queryLog.Push(QueryLog{
					Timestamp: float64(time.Now().UnixMilli()) / 1000.0,
					Domain:    clean,
					Type:      qtype,
					Action:    "local",
					Result:    "127.0.0.1",
				})
			} else {
				// Forward and log
				result := s.forwardAndLog(w, r, clean, qtype)
				if result != "" {
					return // already sent response
				}
			}
		}
	}

	w.WriteMsg(m)
}

// shouldResolveLocally checks if a query should resolve to 127.0.0.1.
func (s *Server) shouldResolveLocally(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clean := strings.ToLower(strings.TrimSuffix(name, "."))

	if s.domains[clean] {
		return true
	}

	parts := strings.Split(clean, ".")
	if len(parts) >= 2 {
		tld := parts[len(parts)-1]
		if s.tlds[tld] {
			return true
		}
	}

	return false
}

// forwardAndLog forwards a query and logs the result. Returns non-empty if response was sent.
func (s *Server) forwardAndLog(w mdns.ResponseWriter, r *mdns.Msg, domain, qtype string) string {
	upstream := "8.8.8.8:53"

	client := new(mdns.Client)
	resp, _, err := client.Exchange(r, upstream)
	if err != nil {
		log.Printf("DNS forward error: %v", err)
		m := new(mdns.Msg)
		m.SetRcode(r, mdns.RcodeServerFailure)
		w.WriteMsg(m)
		s.queryLog.Push(QueryLog{
			Timestamp: float64(time.Now().UnixMilli()) / 1000.0,
			Domain:    domain,
			Type:      qtype,
			Action:    "forwarded",
			Result:    "error: " + err.Error(),
		})
		return "sent"
	}

	// Extract resolved IP for logging
	result := "forwarded"
	for _, ans := range resp.Answer {
		if a, ok := ans.(*mdns.A); ok {
			result = a.A.String()
			break
		}
		if aaaa, ok := ans.(*mdns.AAAA); ok {
			result = aaaa.AAAA.String()
			break
		}
	}

	s.queryLog.Push(QueryLog{
		Timestamp: float64(time.Now().UnixMilli()) / 1000.0,
		Domain:    domain,
		Type:      qtype,
		Action:    "forwarded",
		Result:    result,
	})

	w.WriteMsg(resp)
	return "sent"
}
