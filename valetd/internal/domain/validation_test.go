package domain

import "testing"

func TestValidateDomain(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		// Valid domains
		{"simple", "myapp.test", "myapp.test", false},
		{"subdomain", "api.myapp.test", "api.myapp.test", false},
		{"uppercase normalized", "MyApp.Test", "myapp.test", false},
		{"leading/trailing spaces", "  myapp.test  ", "myapp.test", false},
		{"numeric", "app1.test", "app1.test", false},
		{"hyphens", "my-app.test", "my-app.test", false},
		{"multiple subdomains", "a.b.c.test", "a.b.c.test", false},

		// Invalid domains
		{"empty", "", "", true},
		{"spaces only", "   ", "", true},
		{"has protocol http", "http://myapp.test", "", true},
		{"has protocol https", "https://myapp.test", "", true},
		{"has path", "myapp.test/api", "", true},
		{"has port", "myapp.test:443", "", true},
		{"no tld", "myapp", "", true},
		{"starts with hyphen", "-myapp.test", "", true},
		{"starts with dot", ".myapp.test", "", true},
		{"ends with dot", "myapp.test.", "", true},
		{"double dot", "myapp..test", "", true},
		{"underscore", "my_app.test", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateDomain(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDomain(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ValidateDomain(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeUpstream(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain host:port", "localhost:3000", "localhost:3000"},
		{"strip http", "http://localhost:3000", "localhost:3000"},
		{"strip https", "https://localhost:3000", "localhost:3000"},
		{"port only", ":3000", "localhost:3000"},
		{"whitespace", "  localhost:3000  ", "localhost:3000"},
		{"empty", "", ""},
		{"ip address", "192.168.1.1:8080", "192.168.1.1:8080"},
		{"hostname", "myserver:9000", "myserver:9000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeUpstream(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeUpstream(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateUpstream(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		// Valid
		{"host:port", "localhost:3000", "localhost:3000", false},
		{"ip:port", "127.0.0.1:8080", "127.0.0.1:8080", false},
		{"normalizes http", "http://localhost:3000", "localhost:3000", false},
		{"normalizes port only", ":3000", "localhost:3000", false},
		{"empty allowed", "", "", false},

		// Invalid
		{"has spaces", "local host:3000", "", true},
		{"no port", "localhost", "", true},
		{"has path", "localhost:3000/api", "", true},
		{"just hostname", "myserver", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateUpstream(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUpstream(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ValidateUpstream(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
