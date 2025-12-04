package main

import (
	"context"
	"fmt"
	"ship-status-dash/pkg/types"
	"strings"
	"time"

	promclientv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
)

type PrometheusProber struct {
	componentSlug     string
	subComponentSlug  string
	client            promclientv1.API
	prometheusQueries []types.PrometheusQuery
}

func NewPrometheusProber(componentSlug string, subComponentSlug string, client promclientv1.API, prometheusQueries []types.PrometheusQuery) *PrometheusProber {
	return &PrometheusProber{
		componentSlug:     componentSlug,
		subComponentSlug:  subComponentSlug,
		client:            client,
		prometheusQueries: prometheusQueries,
	}
}

func (p *PrometheusProber) Probe(ctx context.Context, results chan<- types.ComponentMonitorReportComponentStatus, errChan chan<- error) {
	var successful, failed []types.PrometheusQuery
	for _, prometheusQuery := range p.prometheusQueries {
		result, err := p.runQuery(ctx, prometheusQuery.Query)
		if err != nil {
			errChan <- err
		}

		if p.succeeded(result) {
			successful = append(successful, prometheusQuery)
		} else {
			failed = append(failed, prometheusQuery)
		}
	}
	results <- p.makeStatus(ctx, successful, failed)
}

func (p *PrometheusProber) runQuery(ctx context.Context, query string) (model.Value, error) {
	result, warnings, err := p.client.Query(ctx, query, time.Now())
	if err != nil {
		return nil, err
	}

	if len(warnings) > 0 {
		for _, warning := range warnings {
			logrus.WithFields(logrus.Fields{
				"component_slug":     p.componentSlug,
				"sub_component_slug": p.subComponentSlug,
				"query":              query,
			}).Warnf("Query warning: %s", warning)
		}
	}

	return result, nil
}

func (p *PrometheusProber) makeStatus(ctx context.Context, successfulQueries []types.PrometheusQuery, failedQueries []types.PrometheusQuery) types.ComponentMonitorReportComponentStatus {
	status := types.ComponentMonitorReportComponentStatus{
		ComponentSlug:    p.componentSlug,
		SubComponentSlug: p.subComponentSlug,
		Reason:           types.Reason{Type: types.CheckTypePrometheus},
	}
	if len(failedQueries) == 0 {
		status.Status = types.StatusHealthy
		//TODO: we will want to have a Reason added for each successful query, that way an outage can be properly cleared
		// either that or there is no need to send one at all when all queries are successful
		status.Reason.Check = summarizeQueries(successfulQueries)
		status.Reason.Results = "queries returned successfully"
	} else {
		//TODO: should the data model and API support multiple Reasons for an Outage? For now, let's aggregate all failures into one
		summary := p.summarizeFailures(ctx, failedQueries)

		if len(successfulQueries) == 0 {
			status.Status = types.StatusDown
			status.Reason.Check = summary
			status.Reason.Results = "all queries returned unsuccessful"
		} else {
			status.Status = types.StatusDegraded
			status.Reason.Check = summary
			status.Reason.Results = "some queries returned unsuccessful"
		}
	}

	return status
}

func (p *PrometheusProber) summarizeFailures(ctx context.Context, failedQueries []types.PrometheusQuery) string {
	var summaryParts []string
	for _, query := range failedQueries {
		var resultStr string
		if query.FailureQuery != "" {
			result, err := p.runQuery(ctx, query.FailureQuery)
			if err != nil {
				//This is best-effort to improve the outage description, if we have an error here, we just move on without
				logrus.WithError(err).WithField("failure_query", query.FailureQuery).Errorf("Failed to run failure query, will proceed without extra info in outage description")
			} else if result != nil {
				resultStr = result.String()
			} else {
				resultStr = "no result"
			}
			summaryParts = append(summaryParts, fmt.Sprintf("%s (failure query result: %s)", query.Query, resultStr))
		} else {
			summaryParts = append(summaryParts, query.Query)
		}
	}

	if len(summaryParts) == 1 {
		return summaryParts[0]
	}
	return strings.Join(summaryParts, "; ")
}

func summarizeQueries(queries []types.PrometheusQuery) string {
	if len(queries) == 0 {
		return ""
	}
	if len(queries) == 1 {
		return queries[0].Query
	}

	var b strings.Builder
	for i, q := range queries {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(q.Query)
	}
	return b.String()
}

func (p *PrometheusProber) succeeded(result model.Value) bool {
	if result == nil {
		return false
	}

	switch v := result.(type) {
	case model.Vector:
		return len(v) > 0
	case *model.Scalar:
		return v != nil
	case model.Matrix:
		return len(v) > 0 && len(v[0].Values) > 0
	default:
		return false
	}
}
