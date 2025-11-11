package e2e

import (
	"bytes"
	"encoding/base64"
	"net/http"
)

// HTTP client helper with HTTP Basic Auth for mock oauth-proxy
type TestHTTPClient struct {
	serverURL string
	client    *http.Client
	username  string
	password  string
}

func NewTestHTTPClient(serverURL string) (*TestHTTPClient, error) {
	return &TestHTTPClient{
		serverURL: serverURL,
		client:    &http.Client{},
		username:  "developer",
		password:  "developer",
	}, nil
}

func (c *TestHTTPClient) setAuthHeader(req *http.Request) {
	if c.username != "" && c.password != "" {
		auth := c.username + ":" + c.password
		encoded := base64.StdEncoding.EncodeToString([]byte(auth))
		req.Header.Set("Authorization", "Basic "+encoded)
	}
}

func (c *TestHTTPClient) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.serverURL+url, nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeader(req)
	return c.client.Do(req)
}

func (c *TestHTTPClient) Post(url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest("POST", c.serverURL+url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeader(req)
	return c.client.Do(req)
}

func (c *TestHTTPClient) Put(url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest("PUT", c.serverURL+url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeader(req)
	return c.client.Do(req)
}

func (c *TestHTTPClient) Patch(url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest("PATCH", c.serverURL+url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeader(req)
	return c.client.Do(req)
}

func (c *TestHTTPClient) Delete(url string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", c.serverURL+url, nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeader(req)
	return c.client.Do(req)
}
