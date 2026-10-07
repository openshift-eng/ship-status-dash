package seed

import (
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ship-status-dash/pkg/slo"
	payloadv1 "ship-status-dash/pkg/slo/payloadstreams/v1"
	"ship-status-dash/pkg/types"
)

func TestSLOComponentUsesListedSlug(t *testing.T) {
	cfg := &types.DashboardConfig{
		Components: []*types.Component{
			{
				Name: "TRT Incidents", ShipTeam: "Nope", SLOComponent: true,
				Subcomponents: []types.SubComponent{{Name: "Incidents"}},
			},
			{
				Name: "Other", ShipTeam: "TRT", SLOComponent: true,
				Subcomponents: []types.SubComponent{{Name: "Ignored"}},
			},
		},
		TeamSLOs: []types.TeamSLOConfig{{
			Team:          "TRT",
			SLOComponents: []string{"trt-incidents"},
		}},
	}
	cfg.AssignSlugs()

	tests := []struct {
		team    string
		want    string
		wantSub string
		wantErr bool
	}{
		{team: "TRT", want: "trt-incidents", wantSub: "incidents"},
		{team: "Nope", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.team, func(t *testing.T) {
			component, sub, err := SLOComponent(cfg, tt.team)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, component)
			assert.Equal(t, tt.wantSub, sub)
		})
	}
}

func TestTRTPayloadSettingsUsesConfig(t *testing.T) {
	raw, err := json.Marshal(payloadv1.Settings{
		Window: "24h", MinAccepted: 1, RecentPayloads: 2,
		Streams: []payloadv1.Stream{{ReleaseController: "arm64", Name: "renamed-nightly"}},
	})
	require.NoError(t, err)
	cfg := &types.DashboardConfig{
		TeamSLOs: []types.TeamSLOConfig{{
			Team: "Widget",
			SLOs: []types.NamedSLO{{
				Name:   "payloads",
				Source: payloadv1.Source,
				Workspace: &types.SLOWorkspace{
					Kind:          payloadv1.Kind,
					SchemaVersion: payloadv1.SchemaVersion,
					Spec:          raw,
				},
			}},
		}},
	}
	team, settings, err := TRTPayloadSettings(cfg)
	require.NoError(t, err)
	assert.Equal(t, "Widget", team)
	require.Len(t, settings.Streams, 1)
	assert.Equal(t, "renamed-nightly", settings.Streams[0].Name)
	assert.Equal(t, "arm64", settings.Streams[0].ReleaseController)
}

func TestRoleAt(t *testing.T) {
	tests := []struct {
		n        int
		index    int
		wantRole string
		wantMiss int
	}{
		{n: 2, index: 0, wantRole: RoleRecentReject, wantMiss: 1},
		{n: 2, index: 1, wantRole: RoleMiss, wantMiss: 1},
		{n: 4, index: 2, wantRole: RoleMiss, wantMiss: 2},
	}
	for _, tt := range tests {
		t.Run(tt.wantRole, func(t *testing.T) {
			assert.Equal(t, tt.wantRole, RoleAt(tt.n, tt.index))
			assert.Equal(t, tt.wantMiss, MissIndex(tt.n))
		})
	}
	assert.Equal(t, []int{0, 1}, JiraStreamIndexes(2))
}

func TestRecentRejectStaysMetWhenRecentListIsShort(t *testing.T) {
	now := time.Date(2026, 9, 29, 16, 0, 0, 0, time.UTC)
	cases := [][]payloadv1.Stream{
		{
			{ReleaseController: "amd64", Name: "5.1.0-0.nightly"},
			{ReleaseController: "amd64", Name: "5.1.0-0.ci"},
		},
		{
			{ReleaseController: "amd64", Name: "5.1.0-0.nightly"},
			{ReleaseController: "amd64", Name: "5.1.0-0.ci"},
			{ReleaseController: "amd64", Name: "5.0.0-0.nightly"},
			{ReleaseController: "amd64", Name: "5.0.0-0.ci"},
		},
	}
	for _, streams := range cases {
		t.Run(streams[len(streams)-1].Name, func(t *testing.T) {
			seeded, err := buildSeed(now, "TRT", streams, 2)
			require.NoError(t, err)
			items := make([]types.SLOWorkspaceItem, 0, len(seeded))
			for _, row := range seeded {
				items = append(items, row.item)
			}
			assertConsistentStreaks(t, items)

			spec, err := json.Marshal(payloadv1.Settings{Window: "24h", MinAccepted: 1, RecentPayloads: 2, Streams: streams})
			require.NoError(t, err)
			team := &types.TeamSLOConfig{
				Team: "TRT",
				SLOs: []types.NamedSLO{{
					Name:   "accepted-payload-per-day",
					Source: payloadv1.Source,
					Workspace: &types.SLOWorkspace{
						Kind:          payloadv1.Kind,
						SchemaVersion: payloadv1.SchemaVersion,
						Spec:          spec,
					},
				}},
			}
			got, err := slo.Evaluate(now, team, items)
			require.NoError(t, err)
			require.Len(t, got, 1)
			assert.False(t, got[0].Met)
			var result payloadv1.Result
			require.NoError(t, json.Unmarshal(got[0].Result, &result))

			byStream := map[string]payloadv1.GroupEval{}
			for _, group := range result.Groups {
				byStream[group.Key] = group
			}
			recent := streams[0].Name
			assert.True(t, byStream[recent].Met)
			assert.GreaterOrEqual(t, byStream[recent].Accepted, 1)
			rows := itemsForStream(items, recent)
			require.GreaterOrEqual(t, len(rows), 3)
			assert.Equal(t, "Rejected", rows[0].Outcome)
			assert.Equal(t, "Rejected", rows[1].Outcome)
			assert.Len(t, jobsOf(t, rows[1]), 4)
			assert.Equal(t, "Accepted", rows[2].Outcome)

			miss := streams[MissIndex(len(streams))].Name
			assert.False(t, byStream[miss].Met)
			assert.Equal(t, 0, byStream[miss].Accepted)
			require.NotNil(t, byStream[miss].LastAcceptedAt)
		})
	}
}

func TestBuildSeed(t *testing.T) {
	now := time.Date(2026, 9, 29, 16, 0, 0, 0, time.UTC)
	streams := []payloadv1.Stream{
		{ReleaseController: "amd64", Name: "5.1.0-0.nightly"},
		{ReleaseController: "amd64", Name: "5.1.0-0.ci"},
		{ReleaseController: "amd64", Name: "5.0.0-0.nightly"},
		{ReleaseController: "amd64", Name: "5.0.0-0.ci"},
		{ReleaseController: "arm64", Name: "renamed-0.nightly"},
	}

	seeded, err := buildSeed(now, "Widget", streams, 5)
	require.NoError(t, err)
	require.Len(t, seeded, len(streams)*5+1)
	attachSampleLinks(seeded, streams, "/trt-incidents/incidents/outages/7", 7)

	items := make([]types.SLOWorkspaceItem, 0, len(seeded))
	jiraLinks := 0
	outageLinks := 0
	seenKey := map[string]bool{}
	for _, row := range seeded {
		item := row.item
		assert.Equal(t, "Widget", item.Team)
		assert.Equal(t, payloadv1.Kind, item.Kind)
		assert.Equal(t, payloadv1.SchemaVersion, item.SchemaVersion)
		assert.Equal(t, UpdatedBy, item.UpdatedBy)
		assert.False(t, seenKey[item.ItemKey])
		seenKey[item.ItemKey] = true
		require.NoError(t, payloadv1.ValidateDetails(item.Details))
		if item.ItemKey == PruneCandidateItemKey {
			assert.Equal(t, streams[0].Name, item.GroupKey)
			assert.Equal(t, "Rejected", item.Outcome)
			assert.True(t, item.OccurredAt.Equal(now.Add(-PruneCandidateAgo)))
		} else {
			assert.Contains(t, string(item.Details), item.ItemKey)
		}
		items = append(items, item)
		for _, link := range row.links {
			switch link.LinkType {
			case "jira":
				jiraLinks++
				assert.Equal(t, JiraURL, link.URL)
			case "outage":
				outageLinks++
				assert.Equal(t, "/trt-incidents/incidents/outages/7", link.URL)
				require.NotNil(t, link.OutageID)
				assert.Equal(t, uint(7), *link.OutageID)
			default:
				t.Fatalf("unexpected link type %s", link.LinkType)
			}
		}
	}
	assert.Equal(t, 2, jiraLinks)
	assert.Equal(t, 1, outageLinks)
	assertConsistentStreaks(t, items)

	spec, err := json.Marshal(payloadv1.Settings{Window: "24h", MinAccepted: 1, RecentPayloads: 5, Streams: streams})
	require.NoError(t, err)
	team := &types.TeamSLOConfig{
		Team: "Widget",
		SLOs: []types.NamedSLO{{
			Name:   "accepted-payload-per-day",
			Source: payloadv1.Source,
			Workspace: &types.SLOWorkspace{
				Kind:          payloadv1.Kind,
				SchemaVersion: payloadv1.SchemaVersion,
				Spec:          spec,
			},
		}},
	}
	got, err := slo.Evaluate(now, team, items)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.False(t, got[0].Met)
	var result payloadv1.Result
	require.NoError(t, json.Unmarshal(got[0].Result, &result))

	byStream := map[string]payloadv1.GroupEval{}
	for _, group := range result.Groups {
		byStream[group.Key] = group
	}
	assert.True(t, byStream[streams[0].Name].Met)
	assert.Equal(t, "Rejected", newest(items, streams[0].Name).Outcome)
	assert.Contains(t, string(newest(items, streams[0].Name).Details), "https://amd64.ocp.releases.ci.openshift.org/")
	noted := itemsWithPayloadNotes(t, items)
	require.Len(t, noted, 2)
	assert.Equal(t, streams[0].Name, noted[0].GroupKey)
	assert.Equal(t, streams[0].Name, noted[1].GroupKey)
	assertSeededNotes(t, noted[0], []string{noteDisruptionID}, []string{noteDisruptionID})
	assert.NotEmpty(t, noted[0].Notes)
	assert.Empty(t, noted[1].Notes)
	assertSeededNotes(t, noted[1], []string{noteDisruptionID})
	assertCauseLinks(t, noted[0], "Jira", "incident")
	assertCauseLinks(t, noted[1], "Jira")
	assert.Equal(t, "/trt-incidents/incidents/outages/7", causeLink(t, noted[0], "incident"))
	assertLaterPass(t, noted[0], upgradeJob, true)
	assertLaterPass(t, noted[0], metalJob, false)
	assertLaterPass(t, noted[1], upgradeJob, false)
	var crowded []types.SLOWorkspaceItem
	for _, item := range items {
		if len(jobsOf(t, item)) > 3 {
			crowded = append(crowded, item)
		}
	}
	require.Len(t, crowded, 1)
	assert.Equal(t, streams[0].Name, crowded[0].GroupKey)
	assert.Len(t, jobsOf(t, crowded[0]), 4)
	visible := itemsForStream(items, streams[0].Name)
	require.GreaterOrEqual(t, len(visible), 2)
	assert.Equal(t, crowded[0].ItemKey, visible[1].ItemKey)
	assert.True(t, byStream[streams[1].Name].Met)
	assert.True(t, byStream[streams[3].Name].Met)
	assert.True(t, byStream[streams[4].Name].Met)
	assert.Contains(t, string(newest(items, streams[4].Name).Details), "https://arm64.ocp.releases.ci.openshift.org/")
	assert.False(t, byStream[streams[2].Name].Met)
	assert.Equal(t, 0, byStream[streams[2].Name].Accepted)
	require.NotNil(t, byStream[streams[2].Name].LastAcceptedAt)
	assert.True(t, byStream[streams[2].Name].LastAcceptedAt.Equal(now.Add(-32*time.Hour)))
	assert.True(t, byStream[streams[0].Name].LastAcceptedAt.Equal(now.Add(-6*time.Hour)))
	assert.True(t, byStream[streams[1].Name].LastAcceptedAt.Equal(now.Add(-4*time.Hour)))
	assert.True(t, byStream[streams[3].Name].LastAcceptedAt.Equal(now.Add(-8*time.Hour)))

	for i := range items {
		items[i].ID = uint(i + 1)
	}
	var candidateID uint
	for _, item := range items {
		if item.ItemKey == PruneCandidateItemKey {
			candidateID = item.ID
		}
	}
	require.NotZero(t, candidateID)
	settings := payloadv1.Settings{Window: "24h", MinAccepted: 1, RecentPayloads: 5, Streams: streams}
	assert.Contains(t, payloadv1.PruneIDs(now, settings, items), candidateID)
	assert.NotContains(t, itemKeysOf(payloadv1.DisplayItems(settings, items)), PruneCandidateItemKey)
	// Twelve hours later the candidate is still a day outside the window.
	assert.Contains(t, payloadv1.PruneIDs(now.Add(12*time.Hour), settings, items), candidateID)
}

func itemKeysOf(items []types.SLOWorkspaceItem) []string {
	keys := make([]string, len(items))
	for i, item := range items {
		keys[i] = item.ItemKey
	}
	return keys
}

func assertConsistentStreaks(t *testing.T, items []types.SLOWorkspaceItem) {
	t.Helper()
	byStream := map[string][]types.SLOWorkspaceItem{}
	for _, item := range items {
		byStream[item.GroupKey] = append(byStream[item.GroupKey], item)
	}
	for stream, rows := range byStream {
		sort.Slice(rows, func(i, j int) bool {
			return rows[i].OccurredAt.After(rows[j].OccurredAt)
		})
		if rows[0].Outcome == "Accepted" {
			for _, row := range rows {
				for _, job := range jobsOf(t, row) {
					assert.Nil(t, job.RecurringCount, "%s %s keeps a streak after an accepted payload", stream, row.ItemKey)
				}
			}
		}
		for i, row := range rows {
			for _, job := range jobsOf(t, row) {
				if job.RecurringCount == nil || *job.RecurringCount < 2 {
					continue
				}
				for k := 0; k < *job.RecurringCount; k++ {
					require.Less(t, i+k, len(rows), "%s streak walks off the stream", stream)
					assert.NotEqual(t, "Accepted", rows[i+k].Outcome)
					assert.True(t, failedJobNamed(jobsOf(t, rows[i+k]), job.Name), "%s streak includes a payload that did not fail %s", stream, job.Name)
				}
			}
		}
	}
}

func itemsWithPayloadNotes(t *testing.T, items []types.SLOWorkspaceItem) []types.SLOWorkspaceItem {
	t.Helper()
	var noted []types.SLOWorkspaceItem
	for _, item := range items {
		var doc payloadv1.PayloadDetails
		require.NoError(t, json.Unmarshal(item.Details, &doc))
		if len(doc.SharedCauses) > 0 {
			noted = append(noted, item)
		}
	}
	sort.Slice(noted, func(i, j int) bool {
		return noted[i].OccurredAt.After(noted[j].OccurredAt)
	})
	return noted
}

func assertSeededNotes(t *testing.T, item types.SLOWorkspaceItem, jobNoteIDs ...[]string) {
	t.Helper()
	var doc payloadv1.PayloadDetails
	require.NoError(t, json.Unmarshal(item.Details, &doc))
	assert.Equal(t, item.OccurredAt.Format(time.RFC3339), doc.FinishedAt)
	require.Len(t, doc.Jobs, len(jobNoteIDs))
	seen := map[string]payloadv1.SharedCause{}
	for _, note := range doc.SharedCauses {
		seen[note.ID] = note
		assert.NotEmpty(t, note.Text)
	}
	if cause, ok := seen[noteDisruptionID]; ok {
		require.NotEmpty(t, cause.Links)
		assert.Equal(t, "Jira", cause.Links[0].Label)
		assert.Equal(t, JiraURL, cause.Links[0].URL)
	}
	for i, ids := range jobNoteIDs {
		assert.Equal(t, ids, doc.Jobs[i].NoteIDs)
		for _, id := range ids {
			_, ok := seen[id]
			assert.True(t, ok, "job %s references missing note %s", doc.Jobs[i].Name, id)
		}
	}
}

func assertCauseLinks(t *testing.T, item types.SLOWorkspaceItem, labels ...string) {
	t.Helper()
	cause := noteByID(t, item, noteDisruptionID)
	require.Len(t, cause.Links, len(labels))
	for i, label := range labels {
		assert.Equal(t, label, cause.Links[i].Label)
		assert.NotEmpty(t, cause.Links[i].URL)
	}
}

func causeLink(t *testing.T, item types.SLOWorkspaceItem, label string) string {
	t.Helper()
	for _, link := range noteByID(t, item, noteDisruptionID).Links {
		if link.Label == label {
			return link.URL
		}
	}
	t.Fatalf("cause %s has no %s link", noteDisruptionID, label)
	return ""
}

func assertLaterPass(t *testing.T, item types.SLOWorkspaceItem, jobName string, want bool) {
	t.Helper()
	var doc payloadv1.PayloadDetails
	require.NoError(t, json.Unmarshal(item.Details, &doc))
	var job *payloadv1.PayloadJob
	for i := range doc.Jobs {
		if doc.Jobs[i].Name == jobName {
			job = &doc.Jobs[i]
		}
	}
	require.NotNil(t, job)
	if !want {
		assert.Nil(t, job.LaterPass)
		return
	}
	require.NotNil(t, job.LaterPass)
	assert.NotEmpty(t, job.LaterPass.Tag)
	assert.Contains(t, job.LaterPass.Tag, item.GroupKey)
	assert.NotEqual(t, item.ItemKey, job.LaterPass.Tag)
	assert.Contains(t, job.LaterPass.URL, job.LaterPass.Tag)
}

func noteByID(t *testing.T, item types.SLOWorkspaceItem, id string) payloadv1.SharedCause {
	t.Helper()
	var doc payloadv1.PayloadDetails
	require.NoError(t, json.Unmarshal(item.Details, &doc))
	for _, note := range doc.SharedCauses {
		if note.ID == id {
			return note
		}
	}
	t.Fatalf("payload %s has no note %s", item.ItemKey, id)
	return payloadv1.SharedCause{}
}

func jobsOf(t *testing.T, item types.SLOWorkspaceItem) []payloadv1.PayloadJob {
	t.Helper()
	var doc payloadv1.PayloadDetails
	require.NoError(t, json.Unmarshal(item.Details, &doc))
	return doc.Jobs
}

func failedJobNamed(jobs []payloadv1.PayloadJob, name string) bool {
	for _, job := range jobs {
		if job.Name == name && job.State == "failure" {
			return true
		}
	}
	return false
}

func itemsForStream(items []types.SLOWorkspaceItem, stream string) []types.SLOWorkspaceItem {
	var rows []types.SLOWorkspaceItem
	for _, item := range items {
		if item.GroupKey == stream && item.ItemKey != PruneCandidateItemKey {
			rows = append(rows, item)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].OccurredAt.After(rows[j].OccurredAt)
	})
	return rows
}

func newest(items []types.SLOWorkspaceItem, stream string) types.SLOWorkspaceItem {
	var latest types.SLOWorkspaceItem
	for _, item := range items {
		if item.GroupKey != stream {
			continue
		}
		if latest.ItemKey == "" || item.OccurredAt.After(latest.OccurredAt) {
			latest = item
		}
	}
	return latest
}
