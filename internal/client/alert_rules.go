package client

import (
	"context"
	"fmt"
	"net/http"
)

// AlertRule mirrors the API's AlertRule / AlertRuleRequest (camelCase JSON, enums as strings). One struct
// serves both directions: optional request fields are pointers or omitempty so an omitted value stays
// omitted, and response-only fields (timestamps) are simply not modelled.
type AlertRule struct {
	ID          string  `json:"id,omitempty"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`

	ConditionKind string         `json:"conditionKind,omitempty"`
	Condition     *LogFilter     `json:"condition,omitempty"`
	Threshold     AlertThreshold `json:"threshold"`
	WindowSeconds int            `json:"windowSeconds"`

	CooldownSeconds           *int     `json:"cooldownSeconds,omitempty"`
	EvaluationIntervalSeconds *int     `json:"evaluationIntervalSeconds,omitempty"`
	NoDataWindowSeconds       *int     `json:"noDataWindowSeconds,omitempty"`
	MinDataPoints             *int     `json:"minDataPoints,omitempty"`
	RecoveryThreshold         *float64 `json:"recoveryThreshold,omitempty"`
	Severity                  string   `json:"severity,omitempty"`
	ThresholdUnit             *string  `json:"thresholdUnit,omitempty"`
	MetricThresholdValue      *float64 `json:"metricThresholdValue,omitempty"`

	NotificationTitleTemplate *string           `json:"notificationTitleTemplate,omitempty"`
	NotificationBodyTemplate  *string           `json:"notificationBodyTemplate,omitempty"`
	NotificationTemplateID    *string           `json:"notificationTemplateId,omitempty"`
	Labels                    map[string]string `json:"labels,omitempty"`

	ChannelIDs                 []string `json:"channelIds,omitempty"`
	EscalateAfterMinutes       *int     `json:"escalateAfterMinutes,omitempty"`
	EscalationChannelIDs       []string `json:"escalationChannelIds,omitempty"`
	SecondEscalateAfterMinutes *int     `json:"secondEscalateAfterMinutes,omitempty"`
	SecondEscalationChannelIDs []string `json:"secondEscalationChannelIds,omitempty"`

	MetricCondition    *MetricAlertCondition `json:"metricCondition,omitempty"`
	ExceptionCondition *ExceptionCondition   `json:"exceptionCondition,omitempty"`
	AnomalyCondition   *AnomalyCondition     `json:"anomalyCondition,omitempty"`
	SloCondition       *SloBurnRateCondition `json:"sloCondition,omitempty"`
}

// AlertThreshold is the rule's count threshold. Metric, anomaly and SLO rules carry their own value and
// leave Count at 0.
type AlertThreshold struct {
	Count      uint64 `json:"count"`
	Comparator string `json:"comparator,omitempty"`
}

// LogFilter is the subset of the API's LogFilter that an alert condition can use meaningfully (the
// time range is set by the rule's window).
type LogFilter struct {
	Services        []string          `json:"services,omitempty"`
	SeverityNumbers []int             `json:"severityNumbers,omitempty"`
	Search          *string           `json:"search,omitempty"`
	ScopeNames      []string          `json:"scopeNames,omitempty"`
	Attributes      []AttributeFilter `json:"attributes,omitempty"`
}

type AttributeFilter struct {
	Bag   string `json:"bag,omitempty"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type MetricFilter struct {
	Services   []string   `json:"services,omitempty"`
	Attributes []KeyValue `json:"attributes,omitempty"`
}

type MetricAlertCondition struct {
	MetricName  string       `json:"metricName"`
	Type        string       `json:"type"`
	Filter      MetricFilter `json:"filter"`
	Aggregation string       `json:"aggregation,omitempty"`
}

type ExceptionFilter struct {
	Services           []string   `json:"services,omitempty"`
	ResourceAttributes []KeyValue `json:"resourceAttributes,omitempty"`
}

type ExceptionCondition struct {
	ExceptionType    string          `json:"exceptionType"`
	ExceptionMessage string          `json:"exceptionMessage,omitempty"`
	Filter           ExceptionFilter `json:"filter"`
}

type AnomalyCondition struct {
	Source          string  `json:"source,omitempty"`
	Seasonality     string  `json:"seasonality,omitempty"`
	BaselinePeriods int     `json:"baselinePeriods"`
	ZScoreThreshold float64 `json:"zScoreThreshold"`
	Direction       string  `json:"direction,omitempty"`
}

type SloBurnRateCondition struct {
	SloID              string  `json:"sloId"`
	LongWindowSeconds  int     `json:"longWindowSeconds"`
	ShortWindowSeconds int     `json:"shortWindowSeconds"`
	BurnRateThreshold  float64 `json:"burnRateThreshold"`
}

type alertRuleList struct {
	Rules []AlertRule `json:"rules"`
}

func (c *Client) CreateAlertRule(ctx context.Context, in AlertRule) (AlertRule, error) {
	var out AlertRule
	err := c.do(ctx, http.MethodPost, "/api/alerts", in, &out)
	return out, err
}

func (c *Client) GetAlertRule(ctx context.Context, id string) (AlertRule, error) {
	var out AlertRule
	err := c.do(ctx, http.MethodGet, "/api/alerts/"+id, nil, &out)
	return out, err
}

func (c *Client) UpdateAlertRule(ctx context.Context, id string, in AlertRule) (AlertRule, error) {
	var out AlertRule
	err := c.do(ctx, http.MethodPut, "/api/alerts/"+id, in, &out)
	return out, err
}

func (c *Client) DeleteAlertRule(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/alerts/"+id, nil, nil)
}

func (c *Client) ListAlertRules(ctx context.Context) ([]AlertRule, error) {
	var out alertRuleList
	if err := c.do(ctx, http.MethodGet, "/api/alerts", nil, &out); err != nil {
		return nil, err
	}
	return out.Rules, nil
}

// FindAlertRuleByName resolves a rule name; an ambiguous name is an error, as for channels.
func (c *Client) FindAlertRuleByName(ctx context.Context, name string) (AlertRule, error) {
	all, err := c.ListAlertRules(ctx)
	if err != nil {
		return AlertRule{}, err
	}
	var matches []AlertRule
	for _, r := range all {
		if equalFold(r.Name, name) {
			matches = append(matches, r)
		}
	}
	switch len(matches) {
	case 0:
		return AlertRule{}, &APIError{Status: http.StatusNotFound, Detail: fmt.Sprintf("no alert rule named %q", name)}
	case 1:
		return matches[0], nil
	default:
		return AlertRule{}, fmt.Errorf("%d alert rules are named %q; rename the duplicates in Flare", len(matches), name)
	}
}
