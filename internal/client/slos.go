package client

import (
	"context"
	"fmt"
	"net/http"
)

// SLO mirrors the API's Slo / SloRequest (camelCase JSON, kind as a string).
type SLO struct {
	ID                 string  `json:"id,omitempty"`
	Name               string  `json:"name"`
	Description        *string `json:"description,omitempty"`
	Kind               string  `json:"kind"`
	ServiceName        string  `json:"serviceName"`
	OperationName      *string `json:"operationName,omitempty"`
	TargetPercent      float64 `json:"targetPercent"`
	LatencyThresholdMs *int    `json:"latencyThresholdMs,omitempty"`
	WindowDays         *int    `json:"windowDays,omitempty"`
}

type sloList struct {
	Slos []SLO `json:"slos"`
}

func (c *Client) CreateSLO(ctx context.Context, in SLO) (SLO, error) {
	var out SLO
	err := c.do(ctx, http.MethodPost, "/api/slos", in, &out)
	return out, err
}

func (c *Client) GetSLO(ctx context.Context, id string) (SLO, error) {
	var out SLO
	err := c.do(ctx, http.MethodGet, "/api/slos/"+id, nil, &out)
	return out, err
}

func (c *Client) UpdateSLO(ctx context.Context, id string, in SLO) (SLO, error) {
	var out SLO
	err := c.do(ctx, http.MethodPut, "/api/slos/"+id, in, &out)
	return out, err
}

func (c *Client) DeleteSLO(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/slos/"+id, nil, nil)
}

func (c *Client) ListSLOs(ctx context.Context) ([]SLO, error) {
	var out sloList
	if err := c.do(ctx, http.MethodGet, "/api/slos", nil, &out); err != nil {
		return nil, err
	}
	return out.Slos, nil
}

// FindSLOByName resolves an SLO name; an ambiguous name is an error, as for channels.
func (c *Client) FindSLOByName(ctx context.Context, name string) (SLO, error) {
	all, err := c.ListSLOs(ctx)
	if err != nil {
		return SLO{}, err
	}
	var matches []SLO
	for _, s := range all {
		if equalFold(s.Name, name) {
			matches = append(matches, s)
		}
	}
	switch len(matches) {
	case 0:
		return SLO{}, &APIError{Status: http.StatusNotFound, Detail: fmt.Sprintf("no SLO named %q", name)}
	case 1:
		return matches[0], nil
	default:
		return SLO{}, fmt.Errorf("%d SLOs are named %q; rename the duplicates in Flare", len(matches), name)
	}
}
