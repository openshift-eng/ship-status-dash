package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"ship-status-dash/pkg/types"
)

const (
	jiraSearchPath     = "/rest/api/3/search/jql"
	jiraSearchFields   = "summary"
	jiraMaxResults     = "100"
	jiraMaxSearchPages = 10
)

type JiraProber struct {
	componentSlug    string
	subComponentSlug string
	baseURL          string
	jql              string
	severity         types.Severity
	httpClient       *http.Client
}

type jiraSearchResponse struct {
	Issues        []jiraIssue `json:"issues"`
	NextPageToken string      `json:"nextPageToken"`
	IsLast        bool        `json:"isLast"`
}

type jiraIssue struct {
	Key    string          `json:"key"`
	Fields jiraIssueFields `json:"fields"`
}

type jiraIssueFields struct {
	Summary string `json:"summary"`
}

func NewJiraProber(componentSlug, subComponentSlug, baseURL, jql string, severity types.Severity, httpClient *http.Client) *JiraProber {
	if severity == "" {
		severity = types.SeverityDegraded
	}
	return &JiraProber{
		componentSlug:    componentSlug,
		subComponentSlug: subComponentSlug,
		baseURL:          strings.TrimRight(baseURL, "/"),
		jql:              strings.TrimSpace(jql),
		severity:         severity,
		httpClient:       httpClient,
	}
}

func (p *JiraProber) Probe(ctx context.Context, results chan<- ProbeResult) {
	results <- p.search(ctx)
}

func (p *JiraProber) formatErrorResult(err error) ProbeResult {
	return ProbeResult{
		ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
			ComponentSlug:    p.componentSlug,
			SubComponentSlug: p.subComponentSlug,
		},
		ProbeType: ProbeTypeJira,
		Error:     fmt.Errorf("error running Jira probe, for component: %s sub-component %s. error: %w", p.componentSlug, p.subComponentSlug, err),
	}
}

func (p *JiraProber) search(ctx context.Context) ProbeResult {
	issues, err := p.searchIssues(ctx)
	if err != nil {
		return p.formatErrorResult(err)
	}

	var reasons []types.Reason
	for _, issue := range issues {
		key := strings.TrimSpace(issue.Key)
		if key == "" {
			continue
		}
		reasons = append(reasons, types.Reason{
			Type:    types.CheckTypeJira,
			Check:   key,
			Results: issue.Fields.Summary,
			Links: []types.ReportedLink{{
				URL:      p.browseURL(key),
				LinkType: types.LinkTypeJira,
			}},
		})
	}

	status := types.StatusHealthy
	if len(reasons) > 0 {
		status = p.severity.ToStatus()
	}

	return ProbeResult{
		ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
			ComponentSlug:    p.componentSlug,
			SubComponentSlug: p.subComponentSlug,
			Status:           status,
			Reasons:          reasons,
		},
		ProbeType: ProbeTypeJira,
	}
}

func (p *JiraProber) browseURL(key string) string {
	return p.baseURL + "/browse/" + key
}

func (p *JiraProber) searchIssues(ctx context.Context) ([]jiraIssue, error) {
	var issues []jiraIssue
	nextPageToken := ""
	for page := 0; page < jiraMaxSearchPages; page++ {
		resp, err := p.getSearchPage(ctx, nextPageToken)
		if err != nil {
			return nil, err
		}
		issues = append(issues, resp.Issues...)
		if resp.NextPageToken == "" || resp.IsLast {
			return issues, nil
		}
		nextPageToken = resp.NextPageToken
	}
	return issues, fmt.Errorf("jira search exceeded %d pages", jiraMaxSearchPages)
}

func (p *JiraProber) getSearchPage(ctx context.Context, nextPageToken string) (*jiraSearchResponse, error) {
	query := url.Values{}
	query.Set("jql", p.jql)
	query.Set("fields", jiraSearchFields)
	query.Set("maxResults", jiraMaxResults)
	if nextPageToken != "" {
		query.Set("nextPageToken", nextPageToken)
	}

	reqURL := p.baseURL + jiraSearchPath + "?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jira search returned HTTP %d", resp.StatusCode)
	}

	var parsed jiraSearchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to decode jira search response: %w", err)
	}
	return &parsed, nil
}
