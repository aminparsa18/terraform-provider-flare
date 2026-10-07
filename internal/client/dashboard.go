package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Dashboard mirrors the API's Dashboard / DashboardRequest. The layout is opaque to the server and to this
// provider: it is carried as raw JSON.
type Dashboard struct {
	ID          string          `json:"id,omitempty"`
	Name        string          `json:"name"`
	Description *string         `json:"description,omitempty"`
	LayoutJSON  json.RawMessage `json:"layoutJson"`
	Tags        []string        `json:"tags"`
	ProjectID   *string         `json:"projectId,omitempty"`
}

type dashboardList struct {
	Dashboards []Dashboard `json:"dashboards"`
}

func (c *Client) CreateDashboard(ctx context.Context, in Dashboard) (Dashboard, error) {
	var out Dashboard
	err := c.do(ctx, http.MethodPost, "/api/dashboards", in, &out)
	return out, err
}

func (c *Client) GetDashboard(ctx context.Context, id string) (Dashboard, error) {
	var out Dashboard
	err := c.do(ctx, http.MethodGet, "/api/dashboards/"+id, nil, &out)
	return out, err
}

func (c *Client) UpdateDashboard(ctx context.Context, id string, in Dashboard) (Dashboard, error) {
	var out Dashboard
	err := c.do(ctx, http.MethodPut, "/api/dashboards/"+id, in, &out)
	return out, err
}

func (c *Client) DeleteDashboard(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/dashboards/"+id, nil, nil)
}

func (c *Client) ListDashboards(ctx context.Context) ([]Dashboard, error) {
	var out dashboardList
	if err := c.do(ctx, http.MethodGet, "/api/dashboards", nil, &out); err != nil {
		return nil, err
	}
	return out.Dashboards, nil
}

// FindDashboardByName resolves a dashboard name. Names are unique per project, so the same name can exist in
// several projects; that is reported as ambiguous rather than guessed at.
func (c *Client) FindDashboardByName(ctx context.Context, name string) (Dashboard, error) {
	all, err := c.ListDashboards(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	var matches []Dashboard
	for _, d := range all {
		if equalFold(d.Name, name) {
			matches = append(matches, d)
		}
	}
	switch len(matches) {
	case 0:
		return Dashboard{}, &APIError{Status: http.StatusNotFound, Detail: fmt.Sprintf("no dashboard named %q", name)}
	case 1:
		return matches[0], nil
	default:
		ids := make([]string, len(matches))
		for i, m := range matches {
			ids[i] = m.ID
		}
		return Dashboard{}, fmt.Errorf("%d dashboards are named %q (in different projects); import by id instead: %s", len(matches), name, strings.Join(ids, ", "))
	}
}
