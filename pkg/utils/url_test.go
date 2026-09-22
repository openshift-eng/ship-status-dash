package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseHTTPURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantOK  bool
		wantURL string
	}{
		{
			name:    "valid http URL",
			input:   "http://localhost:9090",
			wantOK:  true,
			wantURL: "http://localhost:9090",
		},
		{
			name:    "valid https URL",
			input:   "https://prometheus.example.com",
			wantOK:  true,
			wantURL: "https://prometheus.example.com",
		},
		{
			name:    "valid https URL with path",
			input:   "https://prometheus.example.com/api/v1",
			wantOK:  true,
			wantURL: "https://prometheus.example.com/api/v1",
		},
		{
			name:    "trims surrounding whitespace",
			input:   "  https://example.com  ",
			wantOK:  true,
			wantURL: "https://example.com",
		},
		{
			name:   "invalid no scheme",
			input:  "localhost:9090",
			wantOK: false,
		},
		{
			name:   "invalid not http or https",
			input:  "ftp://example.com",
			wantOK: false,
		},
		{
			name:   "invalid empty string",
			input:  "",
			wantOK: false,
		},
		{
			name:   "invalid cluster name",
			input:  "app.ci",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, ok := ParseHTTPURL(tt.input)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantURL, parsed.String())
			} else {
				assert.Nil(t, parsed)
			}
		})
	}
}
