package client

import (
	"context"
	"fmt"
	"net/http"
)

// StatusPageComponent mirrors the API's StatusPageComponent (kind is "Monitor" or "Slo").
type StatusPageComponent struct {
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	RefID string `json:"refId"`
}

// StatusPage mirrors the API's StatusPage / StatusPageRequest (ADR-0158).
type StatusPage struct {
	ID          string                `json:"id,omitempty"`
	Slug        string                `json:"slug"`
	Title       string                `json:"title"`
	Description *string               `json:"description,omitempty"`
	Enabled     *bool                 `json:"enabled,omitempty"`
	Components  []StatusPageComponent `json:"components"`
	// SubscriberChannelIDs are notification channels told about every incident (ADR-0161). Always sent, so an
	// empty list clears them; the API treats an absent field as "leave as is".
	SubscriberChannelIDs []string `json:"subscriberChannelIds"`
}

type statusPageList struct {
	Pages []StatusPage `json:"pages"`
}

func (c *Client) CreateStatusPage(ctx context.Context, in StatusPage) (StatusPage, error) {
	var out StatusPage
	err := c.do(ctx, http.MethodPost, "/api/status-pages", in, &out)
	return out, err
}

func (c *Client) GetStatusPage(ctx context.Context, id string) (StatusPage, error) {
	var out StatusPage
	err := c.do(ctx, http.MethodGet, "/api/status-pages/"+id, nil, &out)
	return out, err
}

func (c *Client) UpdateStatusPage(ctx context.Context, id string, in StatusPage) (StatusPage, error) {
	var out StatusPage
	err := c.do(ctx, http.MethodPut, "/api/status-pages/"+id, in, &out)
	return out, err
}

func (c *Client) DeleteStatusPage(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/status-pages/"+id, nil, nil)
}

// FindStatusPageBySlug resolves a page by its (unique) slug.
func (c *Client) FindStatusPageBySlug(ctx context.Context, slug string) (StatusPage, error) {
	var out statusPageList
	if err := c.do(ctx, http.MethodGet, "/api/status-pages", nil, &out); err != nil {
		return StatusPage{}, err
	}
	for _, p := range out.Pages {
		if p.Slug == slug {
			return p, nil
		}
	}
	return StatusPage{}, &APIError{Status: http.StatusNotFound, Detail: fmt.Sprintf("no status page with slug %q", slug)}
}
