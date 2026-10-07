package client

import (
	"context"
	"fmt"
	"net/http"
)

// IngestKey mirrors the API's IngestApiKeyDto. RawKey is only ever present on a create response.
type IngestKey struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	IsActive           bool    `json:"isActive"`
	LimitsEnabled      bool    `json:"limitsEnabled"`
	MaxEventsPerMinute *int64  `json:"maxEventsPerMinute,omitempty"`
	MaxBytesPerMinute  *int64  `json:"maxBytesPerMinute,omitempty"`
	MaxEventsPerDay    *int64  `json:"maxEventsPerDay,omitempty"`
	MaxBytesPerDay     *int64  `json:"maxBytesPerDay,omitempty"`
	ProjectID          *string `json:"projectId,omitempty"`
}

// IngestKeyLimits is the body of PUT /api/ingest-keys/{id}/limits, which replaces all five settings.
type IngestKeyLimits struct {
	LimitsEnabled      bool   `json:"limitsEnabled"`
	MaxEventsPerMinute *int64 `json:"maxEventsPerMinute,omitempty"`
	MaxBytesPerMinute  *int64 `json:"maxBytesPerMinute,omitempty"`
	MaxEventsPerDay    *int64 `json:"maxEventsPerDay,omitempty"`
	MaxBytesPerDay     *int64 `json:"maxBytesPerDay,omitempty"`
}

type createIngestKeyRequest struct {
	Name      string  `json:"name"`
	ProjectID *string `json:"projectId,omitempty"`
}

type createIngestKeyResponse struct {
	Key    IngestKey `json:"key"`
	RawKey string    `json:"rawKey"`
}

type ingestKeyList struct {
	Keys []IngestKey `json:"keys"`
}

// CreateIngestKey returns the key and its secret, which Flare never shows again.
func (c *Client) CreateIngestKey(ctx context.Context, name string, projectID *string) (IngestKey, string, error) {
	var out createIngestKeyResponse
	err := c.do(ctx, http.MethodPost, "/api/ingest-keys", createIngestKeyRequest{Name: name, ProjectID: projectID}, &out)
	return out.Key, out.RawKey, err
}

func (c *Client) ListIngestKeys(ctx context.Context) ([]IngestKey, error) {
	var out ingestKeyList
	if err := c.do(ctx, http.MethodGet, "/api/ingest-keys", nil, &out); err != nil {
		return nil, err
	}
	return out.Keys, nil
}

// GetIngestKey finds an *active* key by id; there is no get-by-id endpoint, and a revoked key is gone as far
// as callers are concerned.
func (c *Client) GetIngestKey(ctx context.Context, id string) (IngestKey, error) {
	all, err := c.ListIngestKeys(ctx)
	if err != nil {
		return IngestKey{}, err
	}
	for _, k := range all {
		if k.ID == id && k.IsActive {
			return k, nil
		}
	}
	return IngestKey{}, &APIError{Status: http.StatusNotFound, Detail: "ingest key not found or revoked"}
}

// FindIngestKeyByName resolves the active key with that name (names are unique among active keys).
func (c *Client) FindIngestKeyByName(ctx context.Context, name string) (IngestKey, error) {
	all, err := c.ListIngestKeys(ctx)
	if err != nil {
		return IngestKey{}, err
	}
	for _, k := range all {
		if k.IsActive && equalFold(k.Name, name) {
			return k, nil
		}
	}
	return IngestKey{}, &APIError{Status: http.StatusNotFound, Detail: fmt.Sprintf("no active ingest key named %q", name)}
}

func (c *Client) RenameIngestKey(ctx context.Context, id, name string) error {
	return c.do(ctx, http.MethodPut, "/api/ingest-keys/"+id+"/name", map[string]string{"name": name}, nil)
}

// SetIngestKeyProject moves the key to a project; nil makes it instance-wide.
func (c *Client) SetIngestKeyProject(ctx context.Context, id string, projectID *string) error {
	return c.do(ctx, http.MethodPut, "/api/ingest-keys/"+id+"/project", map[string]*string{"projectId": projectID}, nil)
}

func (c *Client) SetIngestKeyLimits(ctx context.Context, id string, limits IngestKeyLimits) error {
	return c.do(ctx, http.MethodPut, "/api/ingest-keys/"+id+"/limits", limits, nil)
}

// RevokeIngestKey is irreversible: the key stops authenticating and can't be reinstated.
func (c *Client) RevokeIngestKey(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/ingest-keys/"+id, nil, nil)
}
