package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	promclientv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"

	"ship-status-dash/pkg/types"
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

func (p *PrometheusProber) Probe(ctx context.Context, results chan<- ProbeResult) {
	var successful, failed []types.PrometheusQuery
	for _, prometheusQuery := range p.prometheusQueries {
		result, err := p.runQuery(ctx, prometheusQuery.Query, prometheusQuery.Duration, prometheusQuery.Step)
		if err != nil {
			results <- p.createErrorResult(prometheusQuery.Query, err)
			// If any queries error, we want an error result to be returned so that the absent-report-checker can create an outage if it continues to error.
			return
		}

		if succeeded(result) {
			successful = append(successful, prometheusQuery)
		} else {
			failed = append(failed, prometheusQuery)
		}
	}
	results <- p.createStatusFromQueryResults(ctx, successful, failed)
}

var prometheusRetryDelays = []time.Duration{10 * time.Second, 30 * time.Second}

func (p *PrometheusProber) runQuery(ctx context.Context, query string, duration string, step string) (model.Value, error) {
	var dur, stepDuration time.Duration
	var isRange bool
	if duration != "" {
		var parseErr error
		dur, parseErr = time.ParseDuration(duration)
		if parseErr != nil {
			return nil, parseErr
		}
		stepDuration, parseErr = time.ParseDuration(step)
		if parseErr != nil {
			return nil, parseErr
		}
		isRange = true
	}

	var result model.Value
	var warnings promclientv1.Warnings
	var lastErr error
	maxAttempts := len(prometheusRetryDelays) + 1

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := prometheusRetryDelays[attempt-1]
			logrus.WithFields(logrus.Fields{
				"component_slug":     p.componentSlug,
				"sub_component_slug": p.subComponentSlug,
				"query":              query,
				"attempt":            attempt + 1,
				"max_attempts":       maxAttempts,
				"backoff":            backoff,
			}).Warn("Retrying Prometheus query after transient failure")
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		now := time.Now()
		var err error
		if isRange {
			r := promclientv1.Range{
				Start: now.Add(-dur),
				End:   now,
				Step:  stepDuration,
			}
			result, warnings, _, err = p.client.QueryRange(ctx, query, r)
		} else {
			result, warnings, _, err = p.client.Query(ctx, query, now)
		}

		if err == nil {
			lastErr = nil
			break
		}
		lastErr = err

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var promErr *promclientv1.Error
		if errors.As(err, &promErr) && (promErr.Type == promclientv1.ErrBadData || promErr.Type == promclientv1.ErrCanceled) {
			break
		}
	}

	if lastErr != nil {
		logFields := logrus.Fields{
			"component_slug":     p.componentSlug,
			"sub_component_slug": p.subComponentSlug,
			"query":              query,
			"duration":           duration,
			"step":               step,
			"error":              lastErr.Error(),
			"attempts":           maxAttempts,
		}

		// Try to extract response body from Prometheus client error
		if promErr, ok := lastErr.(*promclientv1.Error); ok {
			logFields["error_type"] = string(promErr.Type)
			if promErr.Detail != "" {
				logFields["response_body"] = promErr.Detail
			}
		}
		logrus.WithFields(logFields).Error("Prometheus query failed after retries")
		return nil, lastErr
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

func (p *PrometheusProber) createErrorResult(query string, err error) ProbeResult {
	return ProbeResult{
		ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
			ComponentSlug:    p.componentSlug,
			SubComponentSlug: p.subComponentSlug,
		},
		ProbeType: ProbeTypePrometheus,
		Error:     fmt.Errorf("error running Prometheus query, for component: %s sub-component %s. query: %s. error: %w", p.componentSlug, p.subComponentSlug, query, err),
	}
}

func (p *PrometheusProber) createStatusFromQueryResults(ctx context.Context, successfulQueries []types.PrometheusQuery, failedQueries []types.PrometheusQuery) ProbeResult {
	status := types.ComponentMonitorReportComponentStatus{
		ComponentSlug:    p.componentSlug,
		SubComponentSlug: p.subComponentSlug,
	}
	if len(failedQueries) == 0 {
		status.Status = types.StatusHealthy
		for _, query := range successfulQueries {
			status.Reasons = append(status.Reasons, types.Reason{
				Type:    types.CheckTypePrometheus,
				Check:   query.Query,
				Results: "query returned successfully",
			})
		}
	} else {
		var reasons []types.Reason
		var mostCriticalSeverity types.Severity
		for _, query := range failedQueries {
			resultStr := "query returned unsuccessful"
			if query.FailureQuery != "" {
				result, err := p.runQuery(ctx, query.FailureQuery, "", "")
				if err != nil {
					//This is best-effort to improve the outage description, if we have an error here, we just move on without
					logrus.WithError(err).WithField("failure_query", query.FailureQuery).Errorf("Failed to run failure query, will proceed without extra info in outage description")
				} else if result != nil {
					resultStr = extractValue(result)
				} else {
					resultStr = "no failure query result"
				}
			}
			reasons = append(reasons, types.Reason{
				Type:    types.CheckTypePrometheus,
				Check:   query.Query,
				Results: resultStr,
			})

			if types.GetSeverityLevel(query.Severity) > types.GetSeverityLevel(mostCriticalSeverity) {
				mostCriticalSeverity = query.Severity
			}
		}

		status.Status = mostCriticalSeverity.ToStatus()
		status.Reasons = reasons
	}

	return ProbeResult{ComponentMonitorReportComponentStatus: status, ProbeType: ProbeTypePrometheus}
}

func extractValue(result model.Value) string {
	if result == nil {
		return "no result"
	}

	switch v := result.(type) {
	case model.Vector:
		if len(v) == 0 {
			return "empty vector"
		}
		if len(v) == 1 {
			return v[0].Value.String()
		}
		var parts []string
		for _, sample := range v {
			var labelParts []string
			for name, value := range sample.Metric {
				labelParts = append(labelParts, fmt.Sprintf("%s=%s", name, value))
			}
			sort.Strings(labelParts)
			labelStr := strings.Join(labelParts, ",")
			valueStr := sample.Value.String()
			parts = append(parts, fmt.Sprintf("{%s}=%s", labelStr, valueStr))
		}
		return strings.Join(parts, ", ")
	case *model.Scalar:
		if v == nil {
			return "nil scalar"
		}
		return v.Value.String()
	case model.Matrix:
		if len(v) == 0 || len(v[0].Values) == 0 {
			return "empty matrix"
		}
		return v[0].Values[len(v[0].Values)-1].Value.String()
	case *model.String:
		if v == nil {
			return "nil string"
		}
		return v.Value
	default:
		return result.String()
	}
}

func succeeded(result model.Value) bool {
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
