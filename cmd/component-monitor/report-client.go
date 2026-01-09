package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"ship-status-dash/pkg/types"
)

// ReportClient is a REST client for communicating with the dashboard API.
type ReportClient struct {
	baseURL    string
	name       string
	authToken  string
	httpClient *http.Client
}

// NewReportClient creates a new ReportClient with the specified base URL.
func NewReportClient(baseURL, name, authToken string) *ReportClient {
	return &ReportClient{
		baseURL:   baseURL,
		name:      name,
		authToken: authToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SendReport sends a component monitor report to the dashboard API.
func (c *ReportClient) SendReport(results []types.ComponentMonitorReportComponentStatus) error {
	url := c.baseURL + "/api/component-monitor/report"

	req := types.ComponentMonitorReportRequest{
		ComponentMonitor: c.name,
		Statuses:         results,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.authToken)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("unexpected status code: %d, failed to read response body: %w", resp.StatusCode, err)
		}
		return fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
