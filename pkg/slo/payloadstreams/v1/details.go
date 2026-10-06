package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// PayloadDetails is the payload_streams schema version 1 document.
type PayloadDetails struct {
	PayloadURL   string        `json:"payload_url"`
	AnalysisURL  string        `json:"analysis_url,omitempty"`
	FinishedAt   string        `json:"finished_at,omitempty"`
	SharedCauses []SharedCause `json:"shared_causes,omitempty"`
	Jobs         []PayloadJob  `json:"jobs"`
}

// SharedCauseLink is one labeled URL on a shared cause.
type SharedCauseLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// SharedCause is one explanation shared by jobs on a payload.
type SharedCause struct {
	ID    string            `json:"id"`
	Text  string            `json:"text"`
	URL   string            `json:"url,omitempty"`
	Links []SharedCauseLink `json:"links,omitempty"`
}

// LaterPass is a newer payload where this job succeeded.
type LaterPass struct {
	Tag string `json:"tag"`
	URL string `json:"url"`
}

// PayloadJob is one blocking job on a payload_streams v1 row.
type PayloadJob struct {
	Name           string     `json:"name"`
	URL            string     `json:"url"`
	State          string     `json:"state"`
	Notes          string     `json:"notes,omitempty"`
	NoteIDs        []string   `json:"note_ids,omitempty"`
	LaterPass      *LaterPass `json:"later_pass,omitempty"`
	RecurringCount *int       `json:"recurring_count,omitempty"`
}

// DecodeDetails parses a payload_streams v1 document.
func DecodeDetails(details []byte) (PayloadDetails, error) {
	if len(bytes.TrimSpace(details)) == 0 {
		return PayloadDetails{}, fmt.Errorf("details is required")
	}
	dec := json.NewDecoder(bytes.NewReader(details))
	dec.DisallowUnknownFields()
	var doc PayloadDetails
	if err := dec.Decode(&doc); err != nil {
		return PayloadDetails{}, fmt.Errorf("invalid payload_streams v1 details: %w", err)
	}
	if dec.More() {
		return PayloadDetails{}, fmt.Errorf("invalid payload_streams v1 details: trailing data")
	}
	return doc, nil
}

// Validate checks a decoded payload_streams v1 document.
func (d PayloadDetails) Validate() error {
	if strings.TrimSpace(d.PayloadURL) == "" {
		return fmt.Errorf("payload_url is required")
	}
	if finished := strings.TrimSpace(d.FinishedAt); finished != "" {
		if _, err := time.Parse(time.RFC3339, finished); err != nil {
			return fmt.Errorf("finished_at must be RFC3339")
		}
	}
	if d.Jobs == nil {
		return fmt.Errorf("jobs is required")
	}
	noteIDs := make(map[string]struct{}, len(d.SharedCauses))
	for i, note := range d.SharedCauses {
		id := note.ID
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("shared_causes[%d].id is required", i)
		}
		if id != strings.TrimSpace(id) {
			return fmt.Errorf("shared_causes[%d].id must not have surrounding whitespace", i)
		}
		if strings.TrimSpace(note.Text) == "" {
			return fmt.Errorf("shared_causes[%d].text is required", i)
		}
		if _, ok := noteIDs[id]; ok {
			return fmt.Errorf("shared_causes[%d].id %q is duplicated", i, id)
		}
		noteIDs[id] = struct{}{}
		for j, link := range note.Links {
			if strings.TrimSpace(link.Label) == "" {
				return fmt.Errorf("shared_causes[%d].links[%d].label is required", i, j)
			}
			if strings.TrimSpace(link.URL) == "" {
				return fmt.Errorf("shared_causes[%d].links[%d].url is required", i, j)
			}
		}
	}
	for i, job := range d.Jobs {
		if strings.TrimSpace(job.Name) == "" {
			return fmt.Errorf("jobs[%d].name is required", i)
		}
		if strings.TrimSpace(job.URL) == "" {
			return fmt.Errorf("jobs[%d].url is required", i)
		}
		if strings.TrimSpace(job.State) == "" {
			return fmt.Errorf("jobs[%d].state is required", i)
		}
		if job.RecurringCount != nil && *job.RecurringCount < 0 {
			return fmt.Errorf("jobs[%d].recurring_count must be >= 0", i)
		}
		if job.LaterPass != nil {
			if strings.TrimSpace(job.LaterPass.Tag) == "" {
				return fmt.Errorf("jobs[%d].later_pass.tag is required", i)
			}
			if strings.TrimSpace(job.LaterPass.URL) == "" {
				return fmt.Errorf("jobs[%d].later_pass.url is required", i)
			}
		}
		for j, id := range job.NoteIDs {
			if strings.TrimSpace(id) == "" {
				return fmt.Errorf("jobs[%d].note_ids[%d] is required", i, j)
			}
			if id != strings.TrimSpace(id) {
				return fmt.Errorf("jobs[%d].note_ids[%d] must not have surrounding whitespace", i, j)
			}
			if _, ok := noteIDs[id]; !ok {
				return fmt.Errorf("jobs[%d].note_ids[%d] %q does not match a shared cause", i, j, id)
			}
		}
	}
	return nil
}

// ValidateDetails decodes details and checks the payload_streams v1 document.
func ValidateDetails(details []byte) error {
	doc, err := DecodeDetails(details)
	if err != nil {
		return err
	}
	return doc.Validate()
}
