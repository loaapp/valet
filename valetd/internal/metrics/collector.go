package metrics

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Collector periodically scrapes Caddy's Prometheus metrics and feeds the Store.
type Collector struct {
	store      *Store
	prevValues map[string]float64
	mu         sync.Mutex
	stopCh     chan struct{}
}

// NewCollector creates a new metrics collector backed by the given database.
func NewCollector(db *sql.DB) *Collector {
	return &Collector{
		store:      NewStore(db),
		prevValues: make(map[string]float64),
		stopCh:     make(chan struct{}),
	}
}

// Start begins periodic scraping (every 1s) and cleanup (every 10m).
func (c *Collector) Start() {
	go func() {
		scrapeTicker := time.NewTicker(1 * time.Second)
		cleanupTicker := time.NewTicker(10 * time.Minute)
		defer scrapeTicker.Stop()
		defer cleanupTicker.Stop()

		for {
			select {
			case <-c.stopCh:
				return
			case <-scrapeTicker.C:
				c.scrape()
			case <-cleanupTicker.C:
				if err := c.store.Cleanup(); err != nil {
					log.Printf("metrics cleanup error: %v", err)
				}
			}
		}
	}()
}

// Stop halts the collector.
func (c *Collector) Stop() {
	close(c.stopCh)
}

// Store returns the underlying metrics store.
func (c *Collector) Store() *Store {
	return c.store
}

func metricKey(name, host string) string {
	return name + "|" + host
}

// scrape fetches Prometheus metrics from Caddy and pushes deltas into the store.
// Uses simple text parsing instead of expfmt to avoid validation scheme issues.
func (c *Collector) scrape() {
	resp, err := http.Get("http://127.0.0.1:2019/metrics")
	if err != nil {
		return // Caddy admin not ready yet
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	// Parse Prometheus text format manually
	// Lines look like: metric_name{label="value",label2="value2"} 123.456
	type hostStats struct {
		requests    float64
		errors      float64
		durationSum float64
		durationCnt float64
		bytesOut    float64
	}
	current := make(map[string]*hostStats)

	getOrCreate := func(host string) *hostStats {
		if host == "" {
			return nil
		}
		s, ok := current[host]
		if !ok {
			s = &hostStats{}
			current[host] = s
		}
		return s
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line[0] == '#' {
			continue
		}

		name, labels, value := parsePromLine(line)
		if name == "" {
			continue
		}
		host := labels["host"]

		switch name {
		case "caddy_http_requests_total":
			s := getOrCreate(host)
			if s == nil {
				continue
			}
			s.requests += value
			code := labels["code"]
			if strings.HasPrefix(code, "4") || strings.HasPrefix(code, "5") {
				s.errors += value
			}

		case "caddy_http_request_duration_seconds_sum":
			s := getOrCreate(host)
			if s != nil {
				s.durationSum += value
			}

		case "caddy_http_request_duration_seconds_count":
			s := getOrCreate(host)
			if s != nil {
				s.durationCnt += value
			}

		case "caddy_http_response_size_bytes_sum":
			s := getOrCreate(host)
			if s != nil {
				s.bytesOut += value
			}
		}
	}

	now := time.Now().Unix()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Compute deltas and push to store — always push a point (even zeros)
	// so the chart gets a continuous time series
	newPrev := make(map[string]float64)
	for host, s := range current {
		reqKey := metricKey("requests", host)
		errKey := metricKey("errors", host)
		durSumKey := metricKey("duration_sum", host)
		durCntKey := metricKey("duration_cnt", host)
		bytesKey := metricKey("bytes_out", host)

		deltaReqs := s.requests - c.prevValues[reqKey]
		deltaErrs := s.errors - c.prevValues[errKey]
		deltaDurSum := s.durationSum - c.prevValues[durSumKey]
		deltaDurCnt := s.durationCnt - c.prevValues[durCntKey]
		deltaBytes := s.bytesOut - c.prevValues[bytesKey]

		newPrev[reqKey] = s.requests
		newPrev[errKey] = s.errors
		newPrev[durSumKey] = s.durationSum
		newPrev[durCntKey] = s.durationCnt
		newPrev[bytesKey] = s.bytesOut

		// Clamp negative deltas (counter reset)
		if deltaReqs < 0 { deltaReqs = 0 }
		if deltaErrs < 0 { deltaErrs = 0 }
		if deltaBytes < 0 { deltaBytes = 0 }

		var avgLatencyMs float64
		if deltaDurCnt > 0 {
			avgLatencyMs = (deltaDurSum / deltaDurCnt) * 1000.0
		}

		dp := DataPoint{
			Timestamp:  now,
			Requests:   int(deltaReqs),
			Errors:     int(deltaErrs),
			AvgLatency: avgLatencyMs,
			BytesOut:   int64(deltaBytes),
		}

		if err := c.store.Push(host, dp); err != nil {
			log.Printf("metrics push error for %s: %v", host, err)
		}
	}

	for k, v := range c.prevValues {
		if _, ok := newPrev[k]; !ok {
			newPrev[k] = v
		}
	}
	c.prevValues = newPrev
}

// parsePromLine parses a Prometheus text line like:
// metric_name{label="value",label2="value2"} 123.456
func parsePromLine(line string) (name string, labels map[string]string, value float64) {
	labels = make(map[string]string)

	// Split off value (last space-separated token)
	lastSpace := strings.LastIndex(line, " ")
	if lastSpace == -1 {
		return "", nil, 0
	}
	valStr := line[lastSpace+1:]
	metricPart := line[:lastSpace]

	v, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return "", nil, 0
	}
	value = v

	// Parse name and labels
	braceIdx := strings.Index(metricPart, "{")
	if braceIdx == -1 {
		name = metricPart
		return
	}

	name = metricPart[:braceIdx]
	labelStr := metricPart[braceIdx+1:]
	labelStr = strings.TrimSuffix(labelStr, "}")

	// Parse label pairs: key="value",key2="value2"
	for _, pair := range splitLabels(labelStr) {
		eqIdx := strings.Index(pair, "=")
		if eqIdx == -1 {
			continue
		}
		k := pair[:eqIdx]
		v := strings.Trim(pair[eqIdx+1:], "\"")
		labels[k] = v
	}

	return
}

// splitLabels splits label pairs handling quoted values with commas
func splitLabels(s string) []string {
	var result []string
	var current strings.Builder
	inQuote := false
	for _, ch := range s {
		if ch == '"' {
			inQuote = !inQuote
			current.WriteRune(ch)
		} else if ch == ',' && !inQuote {
			result = append(result, current.String())
			current.Reset()
		} else {
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}

// FormatRange formats a range string for display.
func FormatRange(r string) string {
	switch r {
	case "5m":
		return "last 5 minutes"
	case "1h":
		return "last hour"
	case "24h":
		return "last 24 hours"
	default:
		return fmt.Sprintf("range=%s", r)
	}
}
