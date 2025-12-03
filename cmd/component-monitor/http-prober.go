package main

import (
	"context"
	"fmt"
	"net/http"
	"ship-status-dash/pkg/types"
	"time"
)

type HTTPProber struct {
	componentSlug      string
	subComponentSlug   string
	url                string
	expectedStatusCode int
	confirmAfter       time.Duration
}

func NewHTTPProber(componentSlug string, subComponentSlug string, url string, expectedStatusCode int, confirmAfter time.Duration) *HTTPProber {
	return &HTTPProber{
		componentSlug:      componentSlug,
		subComponentSlug:   subComponentSlug,
		url:                url,
		expectedStatusCode: expectedStatusCode,
		confirmAfter:       confirmAfter,
	}
}

func (p *HTTPProber) makeStatus(statusCode int) types.ComponentMonitorReportComponentStatus {
	var status types.Status
	if statusCode == p.expectedStatusCode {
		status = types.StatusHealthy
	} else {
		status = types.StatusDown
	}

	return types.ComponentMonitorReportComponentStatus{
		ComponentSlug:    p.componentSlug,
		SubComponentSlug: p.subComponentSlug,
		Status:           status,
		Reason: types.Reason{
			Type:    types.CheckTypeHTTP,
			Check:   p.url,
			Results: fmt.Sprintf("Status code %d (expected %d)", statusCode, p.expectedStatusCode),
		},
	}
}

func (p *HTTPProber) makeErrorStatus(err error, context string) types.ComponentMonitorReportComponentStatus {
	return types.ComponentMonitorReportComponentStatus{
		ComponentSlug:    p.componentSlug,
		SubComponentSlug: p.subComponentSlug,
		Status:           types.StatusDown,
		Reason: types.Reason{
			Type:    types.CheckTypeHTTP,
			Check:   p.url,
			Results: fmt.Sprintf("%s: %s", context, err.Error()),
		},
	}
}

func (p *HTTPProber) Probe(ctx context.Context, results chan<- types.ComponentMonitorReportComponentStatus, errChan chan<- error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		errChan <- err
		return
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		results <- p.makeErrorStatus(err, "Network error")
		errChan <- err
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == p.expectedStatusCode {
		results <- p.makeStatus(resp.StatusCode)
		return
	}

	select {
	case <-ctx.Done():
		errChan <- ctx.Err()
		return
	case <-time.After(p.confirmAfter):
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		errChan <- err
		return
	}

	resp, err = client.Do(req)
	if err != nil {
		results <- p.makeErrorStatus(err, "Network error on retry")
		errChan <- err
		return
	}
	defer resp.Body.Close()

	results <- p.makeStatus(resp.StatusCode)
}
