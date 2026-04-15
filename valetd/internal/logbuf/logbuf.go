package logbuf

import (
	"sync"
)

// LogEntry represents a parsed access log entry.
type LogEntry struct {
	Timestamp  float64 `json:"ts"`
	Level      string  `json:"level"`
	Host       string  `json:"host"`
	Method     string  `json:"method"`
	URI        string  `json:"uri"`
	Status     int     `json:"status"`
	Duration   float64 `json:"duration"`
	Size       int     `json:"size"`
	RemoteAddr string  `json:"remoteAddr"`
}

// RingBuffer is a fixed-size circular buffer of log entries.
type RingBuffer struct {
	mu      sync.RWMutex
	entries []LogEntry
	size    int
	head    int
	count   int
}

// New creates a new ring buffer with the given capacity.
func New(size int) *RingBuffer {
	return &RingBuffer{
		entries: make([]LogEntry, size),
		size:    size,
	}
}

// Push adds an entry to the ring buffer.
func (b *RingBuffer) Push(entry LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.entries[b.head] = entry
	b.head = (b.head + 1) % b.size
	if b.count < b.size {
		b.count++
	}
}

// Last returns the most recent n entries, oldest first.
func (b *RingBuffer) Last(n int) []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if n > b.count {
		n = b.count
	}
	if n == 0 {
		return nil
	}

	result := make([]LogEntry, n)
	start := (b.head - n + b.size) % b.size
	for i := 0; i < n; i++ {
		result[i] = b.entries[(start+i)%b.size]
	}
	return result
}

// Since returns all entries with timestamp greater than ts, oldest first.
func (b *RingBuffer) Since(ts float64) []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return nil
	}

	// Walk from oldest to newest, collect entries after ts
	var result []LogEntry
	start := (b.head - b.count + b.size) % b.size
	for i := 0; i < b.count; i++ {
		entry := b.entries[(start+i)%b.size]
		if entry.Timestamp > ts {
			result = append(result, entry)
		}
	}
	return result
}
