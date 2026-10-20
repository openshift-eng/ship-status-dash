package v1

import (
	"time"

	"ship-status-dash/pkg/types"
)

// Target is this evaluator's goal.
type Target struct {
	MinAccepted int `json:"min_accepted"`
}

// StreamEval is one stream's met/miss result. Window and Target are set only
// when the stream overrides the workspace defaults.
type StreamEval struct {
	Key                  string     `json:"key"`
	ReleaseController    string     `json:"release_controller"`
	ReleaseControllerURL string     `json:"release_controller_url,omitempty"`
	Accepted             int        `json:"accepted"`
	Met                  bool       `json:"met"`
	LastAcceptedAt       *time.Time `json:"last_accepted_at,omitempty"`
	Window               string     `json:"window,omitempty"`
	Target               *Target    `json:"target,omitempty"`
}

// Result is the payload_streams v1 evaluation.
type Result struct {
	Window  string       `json:"window"`
	Target  Target       `json:"target"`
	Streams []StreamEval `json:"streams"`
	Met     bool         `json:"-"`
}

// Evaluate scores accepted payloads in the configured window.
func Evaluate(now time.Time, named types.NamedSLO, settings Settings, items []types.SLOWorkspaceItem) Result {
	defaultWindow := settings.WindowDuration()
	result := Result{
		Window:  settings.Window,
		Target:  Target{MinAccepted: settings.MinAccepted},
		Met:     true,
		Streams: []StreamEval{},
	}
	for _, stream := range settings.Streams {
		streamWindow, _ := time.ParseDuration(stream.Window)
		streamMin := *stream.MinAccepted
		cutoff := now.Add(-streamWindow)

		se := StreamEval{
			Key:                  stream.Name,
			ReleaseController:    stream.ReleaseController,
			ReleaseControllerURL: stream.ReleaseControllerURL,
			Met:                  false,
		}
		if streamWindow != defaultWindow {
			se.Window = stream.Window
		}
		if streamMin != settings.MinAccepted {
			se.Target = &Target{MinAccepted: streamMin}
		}
		var last *time.Time
		for i := range items {
			item := &items[i]
			if item.Kind != named.Workspace.Kind || item.GroupKey != stream.Name || item.Outcome != "Accepted" {
				continue
			}
			occurred := item.OccurredAt.UTC()
			if occurred.After(now) {
				continue
			}
			if last == nil || occurred.After(*last) {
				t := occurred
				last = &t
			}
			if !occurred.Before(cutoff) {
				se.Accepted++
			}
		}
		se.LastAcceptedAt = last
		se.Met = se.Accepted >= streamMin
		if !se.Met {
			result.Met = false
		}
		result.Streams = append(result.Streams, se)
	}
	if len(settings.Streams) == 0 {
		result.Met = false
	}
	return result
}
