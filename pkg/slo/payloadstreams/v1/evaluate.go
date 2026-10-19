package v1

import (
	"time"

	"ship-status-dash/pkg/types"
)

// Target is this evaluator's goal.
type Target struct {
	MinAccepted int `json:"min_accepted"`
}

// GroupEval is one stream's met/miss result. Window and Target are set only
// when the stream overrides the global defaults.
type GroupEval struct {
	Key            string     `json:"key"`
	Accepted       int        `json:"accepted"`
	Met            bool       `json:"met"`
	LastAcceptedAt *time.Time `json:"last_accepted_at,omitempty"`
	Window         string     `json:"window,omitempty"`
	Target         *Target    `json:"target,omitempty"`
}

// Result is the payload_streams v1 evaluation.
type Result struct {
	Window string      `json:"window"`
	Target Target      `json:"target"`
	Groups []GroupEval `json:"groups"`
	Met    bool        `json:"-"`
}

// Evaluate scores accepted payloads in the configured window.
func Evaluate(now time.Time, named types.NamedSLO, settings Settings, items []types.SLOWorkspaceItem) Result {
	globalWindow := settings.WindowDuration()
	result := Result{
		Window: settings.Window,
		Target: Target{MinAccepted: settings.MinAccepted},
		Met:    true,
		Groups: []GroupEval{},
	}
	for _, stream := range settings.Streams {
		streamWindow := stream.EffectiveWindowDuration(globalWindow)
		streamMin := stream.EffectiveMinAccepted(settings.MinAccepted)
		cutoff := now.Add(-streamWindow)

		group := GroupEval{Key: stream.Name, Met: false}
		if stream.Window != "" && streamWindow != globalWindow {
			group.Window = stream.Window
		}
		if stream.MinAccepted != nil && streamMin != settings.MinAccepted {
			group.Target = &Target{MinAccepted: streamMin}
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
				group.Accepted++
			}
		}
		group.LastAcceptedAt = last
		group.Met = group.Accepted >= streamMin
		if !group.Met {
			result.Met = false
		}
		result.Groups = append(result.Groups, group)
	}
	if len(settings.Streams) == 0 {
		result.Met = false
	}
	return result
}
