package client

import (
	"context"
	"fmt"
	"net/http"
)

// AlertTemplate mirrors the API's AlertTemplate / AlertTemplateRequest (camelCase JSON). One struct serves both
// directions; timestamps are response-only and not modelled.
type AlertTemplate struct {
	ID                   string            `json:"id,omitempty"`
	Name                 string            `json:"name"`
	Description          *string           `json:"description,omitempty"`
	IsDefault            *bool             `json:"isDefault,omitempty"`
	TitleTemplate        *string           `json:"titleTemplate,omitempty"`
	BodyTemplate         *string           `json:"bodyTemplate,omitempty"`
	ResolvedBodyTemplate *string           `json:"resolvedBodyTemplate,omitempty"`
	ChannelBodies        map[string]string `json:"channelBodies,omitempty"`
}

func (c *Client) CreateAlertTemplate(ctx context.Context, in AlertTemplate) (AlertTemplate, error) {
	var out AlertTemplate
	err := c.do(ctx, http.MethodPost, "/api/alert-templates", in, &out)
	return out, err
}

func (c *Client) GetAlertTemplate(ctx context.Context, id string) (AlertTemplate, error) {
	var out AlertTemplate
	err := c.do(ctx, http.MethodGet, "/api/alert-templates/"+id, nil, &out)
	return out, err
}

func (c *Client) UpdateAlertTemplate(ctx context.Context, id string, in AlertTemplate) (AlertTemplate, error) {
	var out AlertTemplate
	err := c.do(ctx, http.MethodPut, "/api/alert-templates/"+id, in, &out)
	return out, err
}

func (c *Client) DeleteAlertTemplate(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/alert-templates/"+id, nil, nil)
}

// ListAlertTemplates reads the API's bare JSON array.
func (c *Client) ListAlertTemplates(ctx context.Context) ([]AlertTemplate, error) {
	var out []AlertTemplate
	if err := c.do(ctx, http.MethodGet, "/api/alert-templates", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FindAlertTemplateByName resolves a template name; an ambiguous name is an error, as for channels.
func (c *Client) FindAlertTemplateByName(ctx context.Context, name string) (AlertTemplate, error) {
	all, err := c.ListAlertTemplates(ctx)
	if err != nil {
		return AlertTemplate{}, err
	}
	var matches []AlertTemplate
	for _, t := range all {
		if equalFold(t.Name, name) {
			matches = append(matches, t)
		}
	}
	switch len(matches) {
	case 0:
		return AlertTemplate{}, &APIError{Status: http.StatusNotFound, Detail: fmt.Sprintf("no alert template named %q", name)}
	case 1:
		return matches[0], nil
	default:
		return AlertTemplate{}, fmt.Errorf("%d alert templates are named %q; rename the duplicates in Flare", len(matches), name)
	}
}
