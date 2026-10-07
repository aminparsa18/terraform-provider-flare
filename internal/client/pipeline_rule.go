package client

import (
	"context"
	"fmt"
	"net/http"
)

// PipelineRule mirrors the API's PipelineRule / PipelineRuleRequest.
type PipelineRule struct {
	ID          string           `json:"id,omitempty"`
	Name        string           `json:"name"`
	Description *string          `json:"description,omitempty"`
	Enabled     *bool            `json:"enabled,omitempty"`
	Condition   *LogFilter       `json:"condition,omitempty"`
	Actions     []PipelineAction `json:"actions"`
}

// PipelineAction is one step: Kind selects which of the three kind-specific groups is set.
type PipelineAction struct {
	Kind         string        `json:"kind"`
	ExtractRegex *ExtractRegex `json:"extractRegex,omitempty"`
	RedactRegex  *RedactRegex  `json:"redactRegex,omitempty"`
	ParseJSON    *ParseJSON    `json:"parseJson,omitempty"`
}

type ExtractRegex struct {
	SourceAttributeKey *string `json:"sourceAttributeKey,omitempty"`
	Pattern            string  `json:"pattern"`
}

type RedactRegex struct {
	SourceAttributeKey *string `json:"sourceAttributeKey,omitempty"`
	Pattern            string  `json:"pattern"`
	Replacement        *string `json:"replacement,omitempty"`
}

type ParseJSON struct {
	SourceAttributeKey *string `json:"sourceAttributeKey,omitempty"`
	KeyPrefix          *string `json:"keyPrefix,omitempty"`
	MaxDepth           *int    `json:"maxDepth,omitempty"`
	MaxKeys            *int    `json:"maxKeys,omitempty"`
}

type pipelineRuleList struct {
	Rules []PipelineRule `json:"rules"`
}

func (c *Client) CreatePipelineRule(ctx context.Context, in PipelineRule) (PipelineRule, error) {
	var out PipelineRule
	err := c.do(ctx, http.MethodPost, "/api/pipeline-rules", in, &out)
	return out, err
}

func (c *Client) GetPipelineRule(ctx context.Context, id string) (PipelineRule, error) {
	var out PipelineRule
	err := c.do(ctx, http.MethodGet, "/api/pipeline-rules/"+id, nil, &out)
	return out, err
}

func (c *Client) UpdatePipelineRule(ctx context.Context, id string, in PipelineRule) (PipelineRule, error) {
	var out PipelineRule
	err := c.do(ctx, http.MethodPut, "/api/pipeline-rules/"+id, in, &out)
	return out, err
}

func (c *Client) DeletePipelineRule(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/pipeline-rules/"+id, nil, nil)
}

func (c *Client) ListPipelineRules(ctx context.Context) ([]PipelineRule, error) {
	var out pipelineRuleList
	if err := c.do(ctx, http.MethodGet, "/api/pipeline-rules", nil, &out); err != nil {
		return nil, err
	}
	return out.Rules, nil
}

// FindPipelineRuleByName resolves a rule name; an ambiguous name is an error, as for channels.
func (c *Client) FindPipelineRuleByName(ctx context.Context, name string) (PipelineRule, error) {
	all, err := c.ListPipelineRules(ctx)
	if err != nil {
		return PipelineRule{}, err
	}
	var matches []PipelineRule
	for _, r := range all {
		if equalFold(r.Name, name) {
			matches = append(matches, r)
		}
	}
	switch len(matches) {
	case 0:
		return PipelineRule{}, &APIError{Status: http.StatusNotFound, Detail: fmt.Sprintf("no pipeline rule named %q", name)}
	case 1:
		return matches[0], nil
	default:
		return PipelineRule{}, fmt.Errorf("%d pipeline rules are named %q; rename the duplicates in Flare", len(matches), name)
	}
}
