package metrics

import (
	"math"
	"testing"
)

func TestParsePromLine_Basic(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantName   string
		wantLabels map[string]string
		wantValue  float64
	}{
		{
			name:       "simple metric",
			line:       "http_requests_total 42",
			wantName:   "http_requests_total",
			wantLabels: map[string]string{},
			wantValue:  42,
		},
		{
			name:     "metric with labels",
			line:     `caddy_http_requests_total{handler="reverse_proxy",host="myapp.test"} 123`,
			wantName: "caddy_http_requests_total",
			wantLabels: map[string]string{
				"handler": "reverse_proxy",
				"host":    "myapp.test",
			},
			wantValue: 123,
		},
		{
			name:       "float value",
			line:       "process_cpu_seconds_total 0.42",
			wantName:   "process_cpu_seconds_total",
			wantLabels: map[string]string{},
			wantValue:  0.42,
		},
		{
			name:     "single label",
			line:     `metric{key="value"} 1`,
			wantName: "metric",
			wantLabels: map[string]string{
				"key": "value",
			},
			wantValue: 1,
		},
		{
			name:       "scientific notation",
			line:       "metric 1.5e+06",
			wantName:   "metric",
			wantLabels: map[string]string{},
			wantValue:  1.5e+06,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, labels, value := parsePromLine(tt.line)
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if math.Abs(value-tt.wantValue) > 0.001 {
				t.Errorf("value = %f, want %f", value, tt.wantValue)
			}
			for k, v := range tt.wantLabels {
				if labels[k] != v {
					t.Errorf("label %q = %q, want %q", k, labels[k], v)
				}
			}
		})
	}
}

func TestParsePromLine_EdgeCases(t *testing.T) {
	// Empty line
	name, labels, _ := parsePromLine("")
	if name != "" {
		t.Errorf("empty line: name = %q, want empty", name)
	}

	// No space (invalid)
	name, labels, _ = parsePromLine("metricwithoutvalue")
	if name != "" {
		t.Errorf("no space: name = %q, want empty", name)
	}

	// Comment line (starts with #)
	name, _, _ = parsePromLine("# HELP http_requests_total Total requests")
	// parsePromLine doesn't skip comments — caller should. But it shouldn't crash.
	_ = name

	// Non-numeric value
	name, labels, _ = parsePromLine("metric notanumber")
	if name != "" || labels != nil {
		t.Errorf("non-numeric: name = %q, want empty", name)
	}
}

func TestSplitLabels(t *testing.T) {
	tests := []struct {
		name string
		input string
		want []string
	}{
		{
			name:  "simple pair",
			input: `key="value"`,
			want:  []string{`key="value"`},
		},
		{
			name:  "two pairs",
			input: `host="myapp.test",method="GET"`,
			want:  []string{`host="myapp.test"`, `method="GET"`},
		},
		{
			name:  "comma in quoted value",
			input: `path="/api/v1,v2",host="app.test"`,
			want:  []string{`path="/api/v1,v2"`, `host="app.test"`},
		},
		{
			name:  "empty",
			input: "",
			want:  nil,
		},
		{
			name:  "three pairs",
			input: `a="1",b="2",c="3"`,
			want:  []string{`a="1"`, `b="2"`, `c="3"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitLabels(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d; got %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFormatRange(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"5m", "last 5 minutes"},
		{"1h", "last hour"},
		{"24h", "last 24 hours"},
		{"7d", "range=7d"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := FormatRange(tt.input)
			if got != tt.want {
				t.Errorf("FormatRange(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
