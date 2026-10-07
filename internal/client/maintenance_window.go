package client

import (
	"context"
	"fmt"
	"net/http"
)

// MaintenanceWindow mirrors the API's MaintenanceWindow / MaintenanceWindowRequest (camelCase JSON,
// enums as strings). Timestamps stay strings: the provider compares them as instants itself.
type MaintenanceWindow struct {
	ID            string            `json:"id,omitempty"`
	Name          string            `json:"name"`
	Description   *string           `json:"description,omitempty"`
	RuleIDs       []string          `json:"ruleIds,omitempty"`
	StartsAt      string            `json:"startsAt"`
	EndsAt        string            `json:"endsAt"`
	Recurrence    string            `json:"recurrence,omitempty"`
	DaysOfWeek    []string          `json:"daysOfWeek,omitempty"`
	RepeatUntil   *string           `json:"repeatUntil,omitempty"`
	TimeZone      *string           `json:"timeZone,omitempty"`
	LabelMatchers map[string]string `json:"labelMatchers,omitempty"`
}

type maintenanceWindowList struct {
	Windows []MaintenanceWindow `json:"windows"`
}

func (c *Client) CreateMaintenanceWindow(ctx context.Context, in MaintenanceWindow) (MaintenanceWindow, error) {
	var out MaintenanceWindow
	err := c.do(ctx, http.MethodPost, "/api/maintenance-windows", in, &out)
	return out, err
}

func (c *Client) GetMaintenanceWindow(ctx context.Context, id string) (MaintenanceWindow, error) {
	var out MaintenanceWindow
	err := c.do(ctx, http.MethodGet, "/api/maintenance-windows/"+id, nil, &out)
	return out, err
}

func (c *Client) UpdateMaintenanceWindow(ctx context.Context, id string, in MaintenanceWindow) (MaintenanceWindow, error) {
	var out MaintenanceWindow
	err := c.do(ctx, http.MethodPut, "/api/maintenance-windows/"+id, in, &out)
	return out, err
}

func (c *Client) DeleteMaintenanceWindow(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/maintenance-windows/"+id, nil, nil)
}

func (c *Client) ListMaintenanceWindows(ctx context.Context) ([]MaintenanceWindow, error) {
	var out maintenanceWindowList
	if err := c.do(ctx, http.MethodGet, "/api/maintenance-windows", nil, &out); err != nil {
		return nil, err
	}
	return out.Windows, nil
}

// FindMaintenanceWindowByName resolves a window name; an ambiguous name is an error, as for channels.
func (c *Client) FindMaintenanceWindowByName(ctx context.Context, name string) (MaintenanceWindow, error) {
	all, err := c.ListMaintenanceWindows(ctx)
	if err != nil {
		return MaintenanceWindow{}, err
	}
	var matches []MaintenanceWindow
	for _, w := range all {
		if equalFold(w.Name, name) {
			matches = append(matches, w)
		}
	}
	switch len(matches) {
	case 0:
		return MaintenanceWindow{}, &APIError{Status: http.StatusNotFound, Detail: fmt.Sprintf("no maintenance window named %q", name)}
	case 1:
		return matches[0], nil
	default:
		return MaintenanceWindow{}, fmt.Errorf("%d maintenance windows are named %q; rename the duplicates in Flare", len(matches), name)
	}
}
