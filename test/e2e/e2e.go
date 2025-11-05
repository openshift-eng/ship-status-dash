package e2e

import (
	"bytes"
	"net/http"
)

// HTTP client helper with X-Forwarded-User header and GAP-Signature for mutating requests
type TestHTTPClient struct {
	serverURL string
	client    *http.Client
}

func NewTestHTTPClient(serverURL string) (*TestHTTPClient, error) {
	return &TestHTTPClient{
		serverURL: serverURL,
		client:    &http.Client{},
	}, nil
}

func (c *TestHTTPClient) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.serverURL+url, nil)
	if err != nil {
		return nil, err
	}
	return c.client.Do(req)
}

func (c *TestHTTPClient) Post(url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest("POST", c.serverURL+url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.client.Do(req)
}

func (c *TestHTTPClient) Put(url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest("PUT", c.serverURL+url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.client.Do(req)
}

func (c *TestHTTPClient) Patch(url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest("PATCH", c.serverURL+url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.client.Do(req)
}

func (c *TestHTTPClient) Delete(url string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", c.serverURL+url, nil)
	if err != nil {
		return nil, err
	}
	return c.client.Do(req)
}
