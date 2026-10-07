// Package client is a small typed client for the Flare HTTP API. It speaks JSON with a
// service-account access token (ADR-0082), so there is no session flow.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to one Flare.Api instance.
type Client struct {
	baseURL    string
	token      string
	userAgent  string
	httpClient *http.Client
}

// New returns a client for the API at endpoint (e.g. https://flare.example.com).
func New(endpoint, token, userAgent string) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("endpoint %q must be an absolute http(s) URL", endpoint)
	}
	return &Client{
		baseURL:    strings.TrimRight(endpoint, "/"),
		token:      token,
		userAgent:  userAgent,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// APIError is a non-2xx response. Flare returns RFC 7807 problem details; Detail carries the message.
type APIError struct {
	Status int
	Detail string
}

func (e *APIError) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("flare API returned HTTP %d", e.Status)
	}
	return fmt.Sprintf("flare API returned HTTP %d: %s", e.Status, e.Detail)
}

// IsNotFound reports whether err is a 404 from the API.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound
}

// do sends body (if non-nil) as JSON and decodes a JSON response into out (if non-nil).
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &APIError{Status: resp.StatusCode, Detail: problemDetail(data)}
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

func problemDetail(data []byte) string {
	var problem struct {
		Detail string `json:"detail"`
		Title  string `json:"title"`
	}
	if json.Unmarshal(data, &problem) == nil {
		if problem.Detail != "" {
			return problem.Detail
		}
		return problem.Title
	}
	return strings.TrimSpace(string(data))
}

// VersionInfo is the response of GET /api/version (ADR-0068).
type VersionInfo struct {
	Current string `json:"current"`
}

// Version returns the server's running version.
func (c *Client) Version(ctx context.Context) (VersionInfo, error) {
	var v VersionInfo
	err := c.do(ctx, http.MethodGet, "/api/version", nil, &v)
	return v, err
}

func equalFold(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
