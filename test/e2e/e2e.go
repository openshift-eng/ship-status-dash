package e2e

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
)

// HTTP client helper with X-Forwarded-User header and GAP-Signature for mutating requests
type TestHTTPClient struct {
	serverURL  string
	client     *http.Client
	hmacSecret []byte
}

func NewTestHTTPClient(serverURL string, hmacSecretFile string) (*TestHTTPClient, error) {
	hmacSecret, err := os.ReadFile(hmacSecretFile)
	if err != nil {
		return nil, err
	}

	return &TestHTTPClient{
		serverURL:  serverURL,
		client:     &http.Client{},
		hmacSecret: hmacSecret,
	}, nil
}

func (c *TestHTTPClient) computeSignature(user string) string {
	mac := hmac.New(sha256.New, c.hmacSecret)
	mac.Write([]byte(user))
	return hex.EncodeToString(mac.Sum(nil))
}

func (c *TestHTTPClient) setAuthHeaders(req *http.Request) {
	user := "test-user"
	req.Header.Set("X-Forwarded-User", user)
	req.Header.Set("GAP-Signature", c.computeSignature(user))
}

func (c *TestHTTPClient) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.serverURL+url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Forwarded-User", "test-user")
	return c.client.Do(req)
}

func (c *TestHTTPClient) Post(url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest("POST", c.serverURL+url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(req)
	return c.client.Do(req)
}

func (c *TestHTTPClient) Put(url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest("PUT", c.serverURL+url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(req)
	return c.client.Do(req)
}

func (c *TestHTTPClient) Patch(url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest("PATCH", c.serverURL+url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(req)
	return c.client.Do(req)
}

func (c *TestHTTPClient) Delete(url string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", c.serverURL+url, nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeaders(req)
	return c.client.Do(req)
}
