package client

import (
	"context"
	"fmt"
	"net/http"
)

// ForwardingTarget mirrors the API's ForwardingTarget / ForwardingTargetRequest (ADR-0157). Header values
// come back masked. Empty Signals / Services / IngestKeyIDs mean "all" / "every service" / "any request".
type ForwardingTarget struct {
	ID           string            `json:"id,omitempty"`
	Name         string            `json:"name"`
	Enabled      *bool             `json:"enabled,omitempty"`
	Endpoint     string            `json:"endpoint"`
	Headers      map[string]string `json:"headers"`
	Signals      []string          `json:"signals"`
	Services     []string          `json:"services"`
	IngestKeyIDs []string          `json:"ingestKeyIds"`
	Gzip         *bool             `json:"gzip,omitempty"`
}

type forwardingTargetList struct {
	Targets []ForwardingTarget `json:"targets"`
}

// The API has no GET-by-id for targets, so reads go through the list.

func (c *Client) CreateForwardingTarget(ctx context.Context, in ForwardingTarget) (ForwardingTarget, error) {
	var out ForwardingTarget
	err := c.do(ctx, http.MethodPost, "/api/forwarding/targets", in, &out)
	return out, err
}

func (c *Client) UpdateForwardingTarget(ctx context.Context, id string, in ForwardingTarget) (ForwardingTarget, error) {
	var out ForwardingTarget
	err := c.do(ctx, http.MethodPut, "/api/forwarding/targets/"+id, in, &out)
	return out, err
}

func (c *Client) DeleteForwardingTarget(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/forwarding/targets/"+id, nil, nil)
}

func (c *Client) ListForwardingTargets(ctx context.Context) ([]ForwardingTarget, error) {
	var out forwardingTargetList
	if err := c.do(ctx, http.MethodGet, "/api/forwarding/targets", nil, &out); err != nil {
		return nil, err
	}
	return out.Targets, nil
}

// GetForwardingTarget returns a 404 APIError when no target has the id.
func (c *Client) GetForwardingTarget(ctx context.Context, id string) (ForwardingTarget, error) {
	all, err := c.ListForwardingTargets(ctx)
	if err != nil {
		return ForwardingTarget{}, err
	}
	for _, t := range all {
		if equalFold(t.ID, id) {
			return t, nil
		}
	}
	return ForwardingTarget{}, &APIError{Status: http.StatusNotFound, Detail: fmt.Sprintf("no forwarding target with id %q", id)}
}

// FindForwardingTargetByName resolves a target name; an ambiguous name is an error, as for channels.
func (c *Client) FindForwardingTargetByName(ctx context.Context, name string) (ForwardingTarget, error) {
	all, err := c.ListForwardingTargets(ctx)
	if err != nil {
		return ForwardingTarget{}, err
	}
	var matches []ForwardingTarget
	for _, t := range all {
		if equalFold(t.Name, name) {
			matches = append(matches, t)
		}
	}
	switch len(matches) {
	case 0:
		return ForwardingTarget{}, &APIError{Status: http.StatusNotFound, Detail: fmt.Sprintf("no forwarding target named %q", name)}
	case 1:
		return matches[0], nil
	default:
		return ForwardingTarget{}, fmt.Errorf("%d forwarding targets are named %q; rename the duplicates in Flare", len(matches), name)
	}
}

// ArchiveSettings mirrors the API's ArchiveSettings / ArchiveSettingsRequest. There is one per Flare
// instance; Saved is false while the archive still follows the worker's configuration. Keys come back masked.
type ArchiveSettings struct {
	Enabled   *bool    `json:"enabled,omitempty"`
	Endpoint  string   `json:"endpoint"`
	AccessKey *string  `json:"accessKey,omitempty"`
	SecretKey *string  `json:"secretKey,omitempty"`
	Prefix    *string  `json:"prefix,omitempty"`
	Format    *string  `json:"format,omitempty"`
	Signals   []string `json:"signals"`
	Saved     bool     `json:"saved,omitempty"`
}

func (c *Client) GetArchiveSettings(ctx context.Context) (ArchiveSettings, error) {
	var out ArchiveSettings
	err := c.do(ctx, http.MethodGet, "/api/archive/settings", nil, &out)
	return out, err
}

func (c *Client) SaveArchiveSettings(ctx context.Context, in ArchiveSettings) (ArchiveSettings, error) {
	var out ArchiveSettings
	err := c.do(ctx, http.MethodPut, "/api/archive/settings", in, &out)
	return out, err
}

func (c *Client) DeleteArchiveSettings(ctx context.Context) error {
	return c.do(ctx, http.MethodDelete, "/api/archive/settings", nil, nil)
}
