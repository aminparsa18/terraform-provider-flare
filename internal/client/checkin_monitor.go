package client

import (
	"context"
	"fmt"
	"net/http"
)

// CheckInMonitor mirrors the API's CheckInMonitor / CheckInMonitorRequest (camelCase JSON). The read-only
// `status` the list endpoint fills in is not modelled: it changes with every ping, so it has no place in state.
type CheckInMonitor struct {
	ID                string  `json:"id,omitempty"`
	Slug              string  `json:"slug,omitempty"`
	Name              string  `json:"name"`
	Description       *string `json:"description,omitempty"`
	Enabled           *bool   `json:"enabled,omitempty"`
	CronExpression    *string `json:"cronExpression,omitempty"`
	IntervalSeconds   *int    `json:"intervalSeconds,omitempty"`
	TimeZone          *string `json:"timeZone,omitempty"`
	GraceSeconds      *int    `json:"graceSeconds,omitempty"`
	MaxRuntimeSeconds *int    `json:"maxRuntimeSeconds,omitempty"`
}

type checkInMonitorList struct {
	Monitors []CheckInMonitor `json:"monitors"`
}

func (c *Client) CreateCheckInMonitor(ctx context.Context, in CheckInMonitor) (CheckInMonitor, error) {
	var out CheckInMonitor
	err := c.do(ctx, http.MethodPost, "/api/checkin-monitors", in, &out)
	return out, err
}

func (c *Client) GetCheckInMonitor(ctx context.Context, id string) (CheckInMonitor, error) {
	var out CheckInMonitor
	err := c.do(ctx, http.MethodGet, "/api/checkin-monitors/"+id, nil, &out)
	return out, err
}

func (c *Client) UpdateCheckInMonitor(ctx context.Context, id string, in CheckInMonitor) (CheckInMonitor, error) {
	var out CheckInMonitor
	err := c.do(ctx, http.MethodPut, "/api/checkin-monitors/"+id, in, &out)
	return out, err
}

func (c *Client) DeleteCheckInMonitor(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/checkin-monitors/"+id, nil, nil)
}

func (c *Client) ListCheckInMonitors(ctx context.Context) ([]CheckInMonitor, error) {
	var out checkInMonitorList
	if err := c.do(ctx, http.MethodGet, "/api/checkin-monitors", nil, &out); err != nil {
		return nil, err
	}
	return out.Monitors, nil
}

// FindCheckInMonitorBySlug resolves a monitor by its slug, the name a job puts in the ping URL.
func (c *Client) FindCheckInMonitorBySlug(ctx context.Context, slug string) (CheckInMonitor, error) {
	all, err := c.ListCheckInMonitors(ctx)
	if err != nil {
		return CheckInMonitor{}, err
	}
	for _, m := range all {
		if equalFold(m.Slug, slug) {
			return m, nil
		}
	}
	return CheckInMonitor{}, &APIError{Status: http.StatusNotFound, Detail: fmt.Sprintf("no check-in monitor with the slug %q", slug)}
}
