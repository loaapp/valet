package logbuf

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

// caddyLogRaw matches Caddy's JSON access log structure.
type caddyLogRaw struct {
	Ts       float64 `json:"ts"`
	Level    string  `json:"level"`
	Logger   string  `json:"logger"`
	Status   int     `json:"status"`
	Duration float64 `json:"duration"`
	Size     int     `json:"size"`
	Request  struct {
		Method   string `json:"method"`
		Host     string `json:"host"`
		URI      string `json:"uri"`
		RemoteIP string `json:"remote_ip"`
	} `json:"request"`
}

// Tailer watches an access log file and pushes parsed entries into a RingBuffer.
type Tailer struct {
	path   string
	buf    *RingBuffer
	stopCh chan struct{}
}

// NewTailer creates a new log file tailer.
func NewTailer(path string, buf *RingBuffer) *Tailer {
	return &Tailer{
		path:   path,
		buf:    buf,
		stopCh: make(chan struct{}),
	}
}

// Start begins tailing the log file in a goroutine.
func (t *Tailer) Start() {
	go t.run()
}

// Stop halts the tailer.
func (t *Tailer) Stop() {
	close(t.stopCh)
}

func (t *Tailer) run() {
	for {
		select {
		case <-t.stopCh:
			return
		default:
		}

		f, err := os.Open(t.path)
		if err != nil {
			select {
			case <-t.stopCh:
				return
			case <-time.After(1 * time.Second):
				continue
			}
		}

		// Seek to end — only tail new entries
		f.Seek(0, io.SeekEnd)
		t.tail(f)
		f.Close()
	}
}

func (t *Tailer) tail(f *os.File) {
	reader := bufio.NewReader(f)

	for {
		select {
		case <-t.stopCh:
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// No new data — wait and retry
				select {
				case <-t.stopCh:
					return
				case <-time.After(200 * time.Millisecond):
					continue
				}
			}
			log.Printf("logbuf: read error: %v", err)
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var raw caddyLogRaw
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		// Only process access log entries
		if !strings.Contains(raw.Logger, "http.log.access") {
			continue
		}

		entry := LogEntry{
			Timestamp:  raw.Ts,
			Level:      raw.Level,
			Host:       raw.Request.Host,
			Method:     raw.Request.Method,
			URI:        raw.Request.URI,
			Status:     raw.Status,
			Duration:   raw.Duration,
			Size:       raw.Size,
			RemoteAddr: raw.Request.RemoteIP,
		}
		t.buf.Push(entry)
	}
}
