package client

import (
	"context"
	"fmt"
	"net/http"
)

// StatusIncidentUpdate is one entry of an incident's timeline (oldest first).
type StatusIncidentUpdate struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// StatusIncident mirrors the API's StatusIncident (ADR-0159, ADR-0160). Status is the latest update's.
type StatusIncident struct {
	ID         string                 `json:"id"`
	PageID     string                 `json:"pageId"`
	Title      string                 `json:"title"`
	Status     string                 `json:"status"`
	Updates    []StatusIncidentUpdate `json:"updates"`
	Components []string               `json:"components"`
}

// StatusIncidentRequest is the body that opens an incident with its first update.
type StatusIncidentRequest struct {
	Title      string   `json:"title"`
	Status     string   `json:"status"`
	Message    string   `json:"message"`
	Components []string `json:"components"`
}

// StatusIncidentUpdateRequest appends an update; Components replaces the affected list when non-nil.
type StatusIncidentUpdateRequest struct {
	Status     string   `json:"status"`
	Message    string   `json:"message"`
	Components []string `json:"components"`
}

type statusIncidentList struct {
	Incidents []StatusIncident `json:"incidents"`
}

func incidentsPath(pageID string) string { return "/api/status-pages/" + pageID + "/incidents" }

func (c *Client) OpenStatusIncident(ctx context.Context, pageID string, in StatusIncidentRequest) (StatusIncident, error) {
	var out StatusIncident
	err := c.do(ctx, http.MethodPost, incidentsPath(pageID), in, &out)
	return out, err
}

func (c *Client) PostStatusIncidentUpdate(ctx context.Context, pageID, id string, in StatusIncidentUpdateRequest) (StatusIncident, error) {
	var out StatusIncident
	err := c.do(ctx, http.MethodPost, incidentsPath(pageID)+"/"+id+"/updates", in, &out)
	return out, err
}

func (c *Client) DeleteStatusIncident(ctx context.Context, pageID, id string) error {
	return c.do(ctx, http.MethodDelete, incidentsPath(pageID)+"/"+id, nil, nil)
}

// GetStatusIncident reads one incident; the API has no get-by-id, so it lists the page's incidents.
// A missing page or incident is a 404.
func (c *Client) GetStatusIncident(ctx context.Context, pageID, id string) (StatusIncident, error) {
	var out statusIncidentList
	if err := c.do(ctx, http.MethodGet, incidentsPath(pageID), nil, &out); err != nil {
		return StatusIncident{}, err
	}
	for _, i := range out.Incidents {
		if i.ID == id {
			return i, nil
		}
	}
	return StatusIncident{}, &APIError{Status: http.StatusNotFound, Detail: fmt.Sprintf("no incident %s on page %s", id, pageID)}
}
