package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReportedLink_Normalize(t *testing.T) {
	tests := []struct {
		name         string
		link         ReportedLink
		wantOK       bool
		wantURL      string
		wantLinkType LinkType
	}{
		{
			name:         "valid jira link",
			link:         ReportedLink{URL: "https://redhat.atlassian.net/browse/TRT-1", LinkType: LinkTypeJira},
			wantOK:       true,
			wantURL:      "https://redhat.atlassian.net/browse/TRT-1",
			wantLinkType: LinkTypeJira,
		},
		{
			name:         "empty link type defaults to other",
			link:         ReportedLink{URL: "https://example.com/runbook"},
			wantOK:       true,
			wantURL:      "https://example.com/runbook",
			wantLinkType: LinkTypeOther,
		},
		{
			name:   "invalid url",
			link:   ReportedLink{URL: "not-a-url", LinkType: LinkTypeJira},
			wantOK: false,
		},
		{
			name:   "empty url",
			link:   ReportedLink{URL: "   "},
			wantOK: false,
		},
		{
			name:   "invalid link type",
			link:   ReportedLink{URL: "https://example.com", LinkType: LinkType("bogus")},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, linkType, ok := tt.link.Normalize()
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantURL, url)
				assert.Equal(t, tt.wantLinkType, linkType)
			}
		})
	}
}
