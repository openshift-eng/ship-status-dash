package utils

import (
	"net/url"
	"strings"
)

// ParseHTTPURL parses raw and returns the URL when the scheme is http or https.
func ParseHTTPURL(raw string) (*url.URL, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, false
	}
	switch parsed.Scheme {
	case "http", "https":
		if parsed.Host == "" {
			return nil, false
		}
		return parsed, true
	default:
		return nil, false
	}
}
