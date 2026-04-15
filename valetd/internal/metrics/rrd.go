package metrics

import (
	"sync"
	"time"
)

// DataPoint represents a single time-series data point.
type DataPoint struct {
	Timestamp  int64   `json:"ts"`
	Requests   int     `json:"reqs"`
	Errors     int     `json:"errs"`
	AvgLatency float64 `json:"latMs"`
	BytesOut   int64   `json:"bytesOut"`
}

// RingBuffer is a fixed-size circular buffer of DataPoints.
type RingBuffer struct {
	points []DataPoint
	size   int
	head   int
	count  int
}

// NewRingBuffer creates a ring buffer with the given capacity.
func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		points: make([]DataPoint, size),
		size:   size,
	}
}

// Push adds a data point to the ring buffer.
func (rb *RingBuffer) Push(dp DataPoint) {
	rb.points[rb.head] = dp
	rb.head = (rb.head + 1) % rb.size
	if rb.count < rb.size {
		rb.count++
	}
}

// Last returns the most recent n data points, oldest first.
func (rb *RingBuffer) Last(n int) []DataPoint {
	if n > rb.count {
		n = rb.count
	}
	if n == 0 {
		return nil
	}
	result := make([]DataPoint, n)
	start := (rb.head - n + rb.size) % rb.size
	for i := 0; i < n; i++ {
		result[i] = rb.points[(start+i)%rb.size]
	}
	return result
}

// All returns all data points in the buffer, oldest first.
func (rb *RingBuffer) All() []DataPoint {
	return rb.Last(rb.count)
}

// RRD provides per-route round-robin storage at three resolutions.
type RRD struct {
	mu      sync.RWMutex
	seconds map[string]*RingBuffer // 1s resolution, 300 points per route
	minutes map[string]*RingBuffer // 1min resolution, 60 points per route
	hours   map[string]*RingBuffer // 1hr resolution, 24 points per route
	totals  struct {
		seconds *RingBuffer
		minutes *RingBuffer
		hours   *RingBuffer
	}
}

// NewRRD creates a new RRD store.
func NewRRD() *RRD {
	r := &RRD{
		seconds: make(map[string]*RingBuffer),
		minutes: make(map[string]*RingBuffer),
		hours:   make(map[string]*RingBuffer),
	}
	r.totals.seconds = NewRingBuffer(300)
	r.totals.minutes = NewRingBuffer(60)
	r.totals.hours = NewRingBuffer(24)
	return r
}

func (r *RRD) getOrCreate(m map[string]*RingBuffer, route string, size int) *RingBuffer {
	buf, ok := m[route]
	if !ok {
		buf = NewRingBuffer(size)
		m[route] = buf
	}
	return buf
}

// Push adds a 1-second resolution data point for the given route and updates totals.
func (r *RRD) Push(route string, dp DataPoint) {
	r.mu.Lock()
	defer r.mu.Unlock()

	buf := r.getOrCreate(r.seconds, route, 300)
	buf.Push(dp)

	// Accumulate into totals
	totalBuf := r.totals.seconds
	// Try to merge with the current second if timestamps match
	if totalBuf.count > 0 {
		lastIdx := (totalBuf.head - 1 + totalBuf.size) % totalBuf.size
		last := &totalBuf.points[lastIdx]
		if last.Timestamp == dp.Timestamp {
			last.Requests += dp.Requests
			last.Errors += dp.Errors
			last.BytesOut += dp.BytesOut
			if last.Requests > 0 {
				// Weighted average
				last.AvgLatency = (last.AvgLatency*float64(last.Requests-dp.Requests) + dp.AvgLatency*float64(dp.Requests)) / float64(last.Requests)
			}
			return
		}
	}
	totalBuf.Push(dp)
}

// PushTotal pushes a data point directly to the totals buffer (used for zero-fill).
func (r *RRD) PushTotal(dp DataPoint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.totals.seconds.Push(dp)
}

// Rollup aggregates seconds into minutes, and minutes into hours.
func (r *RRD) Rollup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Roll up seconds → minutes for each route
	for route, secBuf := range r.seconds {
		minBuf := r.getOrCreate(r.minutes, route, 60)
		dp := aggregate(secBuf.Last(60))
		if dp.Requests > 0 || dp.Errors > 0 {
			minBuf.Push(dp)
		}
	}

	// Roll up seconds → minutes for totals
	dp := aggregate(r.totals.seconds.Last(60))
	if dp.Requests > 0 || dp.Errors > 0 {
		r.totals.minutes.Push(dp)
	}

	// Roll up minutes → hours for each route (every 60 minute-rollups)
	for route, minBuf := range r.minutes {
		if minBuf.count > 0 && minBuf.count%60 == 0 {
			hrBuf := r.getOrCreate(r.hours, route, 24)
			dp := aggregate(minBuf.Last(60))
			if dp.Requests > 0 || dp.Errors > 0 {
				hrBuf.Push(dp)
			}
		}
	}

	if r.totals.minutes.count > 0 && r.totals.minutes.count%60 == 0 {
		dp := aggregate(r.totals.minutes.Last(60))
		if dp.Requests > 0 || dp.Errors > 0 {
			r.totals.hours.Push(dp)
		}
	}
}

func aggregate(points []DataPoint) DataPoint {
	if len(points) == 0 {
		return DataPoint{}
	}
	var dp DataPoint
	dp.Timestamp = points[len(points)-1].Timestamp
	var totalLatency float64
	for _, p := range points {
		dp.Requests += p.Requests
		dp.Errors += p.Errors
		dp.BytesOut += p.BytesOut
		totalLatency += p.AvgLatency * float64(p.Requests)
	}
	if dp.Requests > 0 {
		dp.AvgLatency = totalLatency / float64(dp.Requests)
	}
	return dp
}

// GetHistory returns historical data points for a route at the appropriate resolution.
// rangeStr: "5m", "1h", "24h". Returns the points and the resolution string.
func (r *RRD) GetHistory(route string, rangeStr string) ([]DataPoint, string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	switch rangeStr {
	case "5m":
		if buf, ok := r.seconds[route]; ok {
			return buf.Last(300), "1s"
		}
		return nil, "1s"
	case "1h":
		if buf, ok := r.minutes[route]; ok {
			return buf.Last(60), "1m"
		}
		return nil, "1m"
	case "24h":
		if buf, ok := r.hours[route]; ok {
			return buf.Last(24), "1h"
		}
		return nil, "1h"
	default:
		// Default to 5m
		if buf, ok := r.seconds[route]; ok {
			return buf.Last(300), "1s"
		}
		return nil, "1s"
	}
}

// GetCurrent returns the latest data point per route.
func (r *RRD) GetCurrent() map[string]DataPoint {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]DataPoint, len(r.seconds))
	for route, buf := range r.seconds {
		points := buf.Last(1)
		if len(points) > 0 {
			result[route] = points[0]
		}
	}
	return result
}

// GetTotals returns aggregated totals at the appropriate resolution,
// filtered to only include points within the requested time range.
func (r *RRD) GetTotals(rangeStr string) ([]DataPoint, string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var points []DataPoint
	var resolution string
	var cutoff int64

	now := time.Now().Unix()

	switch rangeStr {
	case "5m":
		points = r.totals.seconds.Last(300)
		resolution = "1s"
		cutoff = now - 300
	case "1h":
		points = r.totals.minutes.Last(60)
		resolution = "1m"
		cutoff = now - 3600
	case "24h":
		points = r.totals.hours.Last(24)
		resolution = "1h"
		cutoff = now - 86400
	default:
		points = r.totals.seconds.Last(300)
		resolution = "1s"
		cutoff = now - 300
	}

	// Filter to requested time range
	filtered := make([]DataPoint, 0, len(points))
	for _, p := range points {
		if p.Timestamp >= cutoff {
			filtered = append(filtered, p)
		}
	}
	return filtered, resolution
}

// GetHistory returns historical data points for a route, filtered to the time range.
func (r *RRD) GetHistoryFiltered(route string, rangeStr string) ([]DataPoint, string) {
	points, resolution := r.GetHistory(route, rangeStr)
	now := time.Now().Unix()
	var cutoff int64
	switch rangeStr {
	case "5m":
		cutoff = now - 300
	case "1h":
		cutoff = now - 3600
	case "24h":
		cutoff = now - 86400
	default:
		cutoff = now - 300
	}
	filtered := make([]DataPoint, 0, len(points))
	for _, p := range points {
		if p.Timestamp >= cutoff {
			filtered = append(filtered, p)
		}
	}
	return filtered, resolution
}
