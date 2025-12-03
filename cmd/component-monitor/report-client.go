package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"ship-status-dash/pkg/types"
)

// ReportClient is a REST client for communicating with the dashboard API.
type ReportClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewReportClient creates a new ReportClient with the specified base URL.
func NewReportClient(baseURL string) *ReportClient {
	return &ReportClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ReportResponse represents the response from the report endpoint.
type ReportResponse struct {
	Status string `json:"status"`
}

// SendReport sends a component monitor report to the dashboard API.
func (c *ReportClient) SendReport(req *types.ComponentMonitorReportRequest) (*ReportResponse, error) {
	url := c.baseURL + "/api/component-monitor/report"

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var reportResp ReportResponse
	if err := json.NewDecoder(resp.Body).Decode(&reportResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &reportResp, nil
}
