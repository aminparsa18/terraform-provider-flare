package client

import (
	"context"
	"fmt"
	"net/http"
)

// MetricAttributeRule mirrors the API's MetricAttributeRule / MetricAttributeRuleRequest.
type MetricAttributeRule struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	Enabled     *bool    `json:"enabled,omitempty"`
	MetricName  string   `json:"metricName"`
	Mode        string   `json:"mode"`
	Attributes  []string `json:"attributes"`
}

type metricAttributeRuleList struct {
	Rules []MetricAttributeRule `json:"rules"`
}

func (c *Client) CreateMetricAttributeRule(ctx context.Context, in MetricAttributeRule) (MetricAttributeRule, error) {
	var out MetricAttributeRule
	err := c.do(ctx, http.MethodPost, "/api/metric-attribute-rules", in, &out)
	return out, err
}

func (c *Client) GetMetricAttributeRule(ctx context.Context, id string) (MetricAttributeRule, error) {
	var out MetricAttributeRule
	err := c.do(ctx, http.MethodGet, "/api/metric-attribute-rules/"+id, nil, &out)
	return out, err
}

func (c *Client) UpdateMetricAttributeRule(ctx context.Context, id string, in MetricAttributeRule) (MetricAttributeRule, error) {
	var out MetricAttributeRule
	err := c.do(ctx, http.MethodPut, "/api/metric-attribute-rules/"+id, in, &out)
	return out, err
}

func (c *Client) DeleteMetricAttributeRule(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/metric-attribute-rules/"+id, nil, nil)
}

func (c *Client) ListMetricAttributeRules(ctx context.Context) ([]MetricAttributeRule, error) {
	var out metricAttributeRuleList
	if err := c.do(ctx, http.MethodGet, "/api/metric-attribute-rules", nil, &out); err != nil {
		return nil, err
	}
	return out.Rules, nil
}

// FindMetricAttributeRuleByName resolves a rule name; an ambiguous name is an error, as for channels.
func (c *Client) FindMetricAttributeRuleByName(ctx context.Context, name string) (MetricAttributeRule, error) {
	all, err := c.ListMetricAttributeRules(ctx)
	if err != nil {
		return MetricAttributeRule{}, err
	}
	var matches []MetricAttributeRule
	for _, r := range all {
		if equalFold(r.Name, name) {
			matches = append(matches, r)
		}
	}
	switch len(matches) {
	case 0:
		return MetricAttributeRule{}, &APIError{Status: http.StatusNotFound, Detail: fmt.Sprintf("no metric attribute rule named %q", name)}
	case 1:
		return matches[0], nil
	default:
		return MetricAttributeRule{}, fmt.Errorf("%d metric attribute rules are named %q; rename the duplicates in Flare", len(matches), name)
	}
}
