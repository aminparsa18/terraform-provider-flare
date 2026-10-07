package provider

import (
	"context"
	"fmt"
	"math"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*alertRuleResource)(nil)
	_ resource.ResourceWithConfigure   = (*alertRuleResource)(nil)
	_ resource.ResourceWithImportState = (*alertRuleResource)(nil)
)

var (
	conditionKinds     = []string{"LogCount", "MetricThreshold", "ExceptionCount", "Anomaly", "SloBurnRate"}
	comparators        = []string{"GreaterThanOrEqual", "LessThan"}
	severities         = []string{"Critical", "Error", "Warning", "Info"}
	metricTypes        = []string{"Gauge", "Sum", "Histogram", "ExponentialHistogram"}
	metricAggregations = []string{"Value", "Count", "Sum", "P50", "P75", "P90", "P95", "P99", "MaxApprox", "Last", "Min", "Max"}
	anomalySources     = []string{"LogCount", "MetricThreshold", "ExceptionCount"}
	seasonalities      = []string{"Daily", "Weekly"}
	anomalyDirections  = []string{"Both", "Above", "Below"}
	attributeBags      = []string{"Log", "Resource", "Scope"}
)

// NewAlertRuleResource is the flare_alert_rule factory.
func NewAlertRuleResource() resource.Resource { return &alertRuleResource{} }

type alertRuleResource struct{ client *client.Client }

type alertRuleModel struct {
	ID                         types.String             `tfsdk:"id"`
	Name                       types.String             `tfsdk:"name"`
	Description                types.String             `tfsdk:"description"`
	Enabled                    types.Bool               `tfsdk:"enabled"`
	ConditionKind              types.String             `tfsdk:"condition_kind"`
	Comparator                 types.String             `tfsdk:"comparator"`
	Threshold                  types.Float64            `tfsdk:"threshold"`
	ThresholdUnit              types.String             `tfsdk:"threshold_unit"`
	RecoveryThreshold          types.Float64            `tfsdk:"recovery_threshold"`
	WindowSeconds              types.Int64              `tfsdk:"window_seconds"`
	CooldownSeconds            types.Int64              `tfsdk:"cooldown_seconds"`
	EvaluationIntervalSeconds  types.Int64              `tfsdk:"evaluation_interval_seconds"`
	NoDataWindowSeconds        types.Int64              `tfsdk:"no_data_window_seconds"`
	MinDataPoints              types.Int64              `tfsdk:"min_data_points"`
	Severity                   types.String             `tfsdk:"severity"`
	Labels                     map[string]types.String  `tfsdk:"labels"`
	NotificationTitleTemplate  types.String             `tfsdk:"notification_title_template"`
	NotificationBodyTemplate   types.String             `tfsdk:"notification_body_template"`
	Channels                   []types.String           `tfsdk:"channels"`
	EscalateAfterMinutes       types.Int64              `tfsdk:"escalate_after_minutes"`
	EscalationChannels         []types.String           `tfsdk:"escalation_channels"`
	SecondEscalateAfterMinutes types.Int64              `tfsdk:"second_escalate_after_minutes"`
	SecondEscalationChannels   []types.String           `tfsdk:"second_escalation_channels"`
	LogCondition               *logConditionModel       `tfsdk:"log_condition"`
	MetricCondition            *metricConditionModel    `tfsdk:"metric_condition"`
	ExceptionCondition         *exceptionConditionModel `tfsdk:"exception_condition"`
	AnomalyCondition           *anomalyConditionModel   `tfsdk:"anomaly_condition"`
	SloCondition               *sloConditionModel       `tfsdk:"slo_condition"`
}

type attributeFilterModel struct {
	Bag   types.String `tfsdk:"bag"`
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

type logConditionModel struct {
	Services        []types.String         `tfsdk:"services"`
	SeverityNumbers []types.Int64          `tfsdk:"severity_numbers"`
	Search          types.String           `tfsdk:"search"`
	ScopeNames      []types.String         `tfsdk:"scope_names"`
	Attributes      []attributeFilterModel `tfsdk:"attributes"`
}

type metricConditionModel struct {
	MetricName  types.String            `tfsdk:"metric_name"`
	Type        types.String            `tfsdk:"type"`
	Aggregation types.String            `tfsdk:"aggregation"`
	Services    []types.String          `tfsdk:"services"`
	Attributes  map[string]types.String `tfsdk:"attributes"`
}

type exceptionConditionModel struct {
	ExceptionType      types.String            `tfsdk:"exception_type"`
	ExceptionMessage   types.String            `tfsdk:"exception_message"`
	Services           []types.String          `tfsdk:"services"`
	ResourceAttributes map[string]types.String `tfsdk:"resource_attributes"`
}

type anomalyConditionModel struct {
	Source          types.String  `tfsdk:"source"`
	Seasonality     types.String  `tfsdk:"seasonality"`
	BaselinePeriods types.Int64   `tfsdk:"baseline_periods"`
	ZScoreThreshold types.Float64 `tfsdk:"z_score_threshold"`
	Direction       types.String  `tfsdk:"direction"`
}

type sloConditionModel struct {
	SloID              types.String  `tfsdk:"slo_id"`
	LongWindowSeconds  types.Int64   `tfsdk:"long_window_seconds"`
	ShortWindowSeconds types.Int64   `tfsdk:"short_window_seconds"`
	BurnRateThreshold  types.Float64 `tfsdk:"burn_rate_threshold"`
}

func (r *alertRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert_rule"
}

func (r *alertRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	oneOf := func(values []string) []validator.String { return []validator.String{stringvalidator.OneOf(values...)} }
	enum := func(desc string, values []string, def string) schema.StringAttribute {
		return schema.StringAttribute{
			Optional: true, Computed: true, Default: stringdefault.StaticString(def),
			Description: desc + " One of: " + joinQuoted(values) + ". Defaults to `" + def + "`.",
			Validators:  oneOf(values),
		}
	}
	intDefault := func(desc string, def int64) schema.Int64Attribute {
		return schema.Int64Attribute{
			Optional: true, Computed: true, Default: int64default.StaticInt64(def),
			Description: desc, Validators: []validator.Int64{int64validator.AtLeast(0)},
		}
	}
	strList := func(desc string) schema.ListAttribute {
		return schema.ListAttribute{Optional: true, ElementType: types.StringType, Description: desc}
	}
	kvMap := func(desc string) schema.MapAttribute {
		return schema.MapAttribute{Optional: true, ElementType: types.StringType, Description: desc}
	}

	resp.Schema = schema.Schema{
		Description: "An alert rule. `condition_kind` selects which condition block applies; Flare validates the combination. " +
			"Notification channels are referenced by name and resolved to ids at apply time.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, Description: "Flare's id for the rule.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{Required: true, Description: "Unique (case-insensitive) name. Renaming updates the rule in place."},
			"description": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Free-text description.",
			},
			"enabled": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(true), Description: "Whether the rule is evaluated. Defaults to true.",
			},
			"condition_kind": enum("What the rule evaluates.", conditionKinds, "LogCount"),
			"comparator":     enum("How the observed value is compared with `threshold`.", comparators, "GreaterThanOrEqual"),
			"threshold": schema.Float64Attribute{
				Optional: true,
				Description: "Value that fires the rule. A whole number of matching events for `LogCount` and `ExceptionCount`, a metric value for `MetricThreshold`. " +
					"Not used by `Anomaly` and `SloBurnRate`, whose thresholds live in their own blocks.",
			},
			"threshold_unit": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""),
				Description: "Unit `threshold` is written in for `MetricThreshold` rules (a time or byte unit such as `ms` or `MiBy`); empty means the metric's own unit.",
			},
			"recovery_threshold": schema.Float64Attribute{
				Optional:    true,
				Description: "Hysteresis: once firing, the rule only recovers when the value crosses this instead of `threshold`.",
			},
			"window_seconds": schema.Int64Attribute{Required: true, Description: "Evaluation window. For `SloBurnRate` it must equal the SLO condition's long window."},
			"cooldown_seconds": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(300),
				Description: "Minimum seconds between repeat notifications. Defaults to 300.", Validators: []validator.Int64{int64validator.AtLeast(0)},
			},
			"evaluation_interval_seconds":   intDefault("How often the rule is evaluated; 0 means every poll tick.", 0),
			"no_data_window_seconds":        intDefault("Fire when no data arrives for this long; 0 disables the check.", 0),
			"min_data_points":               intDefault("MetricThreshold only: skip evaluation until the window has at least this many points; 0 disables.", 0),
			"severity":                      enum("Severity attached to notifications.", severities, "Critical"),
			"labels":                        kvMap("Free-form labels carried on notifications."),
			"notification_title_template":   schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Custom notification title; empty uses the default."},
			"notification_body_template":    schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Custom notification body; empty uses the default."},
			"channels":                      strList("Names of `flare_notification_channel`s notified when the rule fires."),
			"escalate_after_minutes":        intDefault("Escalate if still unacknowledged after this many minutes; 0 disables escalation.", 0),
			"escalation_channels":           strList("Channel names notified on the first escalation."),
			"second_escalate_after_minutes": intDefault("Second escalation delay in minutes; 0 disables.", 0),
			"second_escalation_channels":    strList("Channel names notified on the second escalation."),

			"log_condition": logFilterAttribute("Filter for `LogCount` rules (and `Anomaly` rules whose source is `LogCount`)."),
			"metric_condition": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Required for `MetricThreshold` rules (and `Anomaly` rules whose source is `MetricThreshold`).",
				Attributes: map[string]schema.Attribute{
					"metric_name": schema.StringAttribute{Required: true},
					"type":        schema.StringAttribute{Required: true, Description: "Metric type. One of: " + joinQuoted(metricTypes) + ".", Validators: oneOf(metricTypes)},
					"aggregation": enum("How points in the window are reduced to one value.", metricAggregations, "Value"),
					"services":    strList("Only metrics from these services."),
					"attributes":  kvMap("Attribute equality filters, key to value."),
				},
			},
			"exception_condition": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Required for `ExceptionCount` rules (and `Anomaly` rules whose source is `ExceptionCount`).",
				Attributes: map[string]schema.Attribute{
					"exception_type": schema.StringAttribute{Required: true},
					"exception_message": schema.StringAttribute{
						Optional: true, Computed: true, Default: stringdefault.StaticString(""),
						Description: "Narrow to one message within the type; empty matches any.",
					},
					"services":            strList("Only exceptions from these services."),
					"resource_attributes": kvMap("Resource attribute equality filters, key to value."),
				},
			},
			"anomaly_condition": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Required for `Anomaly` rules: fires when the window deviates from the same window in earlier periods.",
				Attributes: map[string]schema.Attribute{
					"source":      enum("Series the baseline is built from.", anomalySources, "LogCount"),
					"seasonality": enum("Baseline period.", seasonalities, "Daily"),
					"baseline_periods": schema.Int64Attribute{
						Required: true, Description: "How many earlier periods form the baseline (3-12).",
						Validators: []validator.Int64{int64validator.Between(3, 12)},
					},
					"z_score_threshold": schema.Float64Attribute{Required: true, Description: "Standard deviations from the baseline mean that fire the rule (greater than 0, at most 10)."},
					"direction":         enum("Which deviations fire.", anomalyDirections, "Both"),
				},
			},
			"slo_condition": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Required for `SloBurnRate` rules: multi-window burn-rate alert on an SLO.",
				Attributes: map[string]schema.Attribute{
					"slo_id":               schema.StringAttribute{Required: true, Description: "Id of the SLO."},
					"long_window_seconds":  schema.Int64Attribute{Required: true, Description: "Long burn-rate window; must equal the rule's `window_seconds`."},
					"short_window_seconds": schema.Int64Attribute{Required: true, Description: "Short window, shorter than the long one."},
					"burn_rate_threshold":  schema.Float64Attribute{Required: true, Description: "Burn rate above which the rule fires (greater than 0, at most 1000)."},
				},
			},
		},
	}
}

func (r *alertRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// ---- model <-> API ----------------------------------------------------------------------------------

func stringSlice(in []types.String) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = v.ValueString()
	}
	return out
}

func stringValues(in []string, prior []types.String) []types.String {
	if len(in) == 0 {
		if prior != nil {
			return []types.String{}
		}
		return nil
	}
	out := make([]types.String, len(in))
	for i, v := range in {
		out[i] = types.StringValue(v)
	}
	return out
}

func keyValues(in map[string]types.String) []client.KeyValue {
	var out []client.KeyValue
	for k, v := range in {
		out = append(out, client.KeyValue{Key: k, Value: v.ValueString()})
	}
	return out
}

func keyValueMap(in []client.KeyValue, prior map[string]types.String) map[string]types.String {
	if len(in) == 0 {
		if prior != nil {
			return map[string]types.String{}
		}
		return nil
	}
	out := make(map[string]types.String, len(in))
	for _, kv := range in {
		out[kv.Key] = types.StringValue(kv.Value)
	}
	return out
}

// channelDirectory resolves channel names to ids and back using one list call.
type channelDirectory struct {
	byName map[string]string
	byID   map[string]string
}

func (r *alertRuleResource) channels(ctx context.Context) (channelDirectory, error) {
	all, err := r.client.ListNotificationChannels(ctx)
	if err != nil {
		return channelDirectory{}, err
	}
	d := channelDirectory{byName: map[string]string{}, byID: map[string]string{}}
	for _, ch := range all {
		d.byName[lower(ch.Name)] = ch.ID
		d.byID[ch.ID] = ch.Name
	}
	return d, nil
}

func (d channelDirectory) ids(names []types.String) ([]string, error) {
	var out []string
	for _, n := range names {
		id, ok := d.byName[lower(n.ValueString())]
		if !ok {
			return nil, fmt.Errorf("no notification channel named %q", n.ValueString())
		}
		out = append(out, id)
	}
	return out, nil
}

func (d channelDirectory) names(ids []string, prior []types.String) []types.String {
	var names []string
	for _, id := range ids {
		if n, ok := d.byID[id]; ok {
			names = append(names, n)
		}
		// A channel deleted out from under the rule drops out of state, so the plan re-adds the reference.
	}
	return stringValues(names, prior)
}

func (m alertRuleModel) toAPI(dir channelDirectory) (client.AlertRule, error) {
	kind := m.ConditionKind.ValueString()
	rule := client.AlertRule{
		Name:                       m.Name.ValueString(),
		Description:                strPtr(m.Description),
		Enabled:                    boolPtr(m.Enabled),
		ConditionKind:              kind,
		WindowSeconds:              int(m.WindowSeconds.ValueInt64()),
		CooldownSeconds:            intPtr(m.CooldownSeconds),
		EvaluationIntervalSeconds:  intPtr(m.EvaluationIntervalSeconds),
		NoDataWindowSeconds:        intPtr(m.NoDataWindowSeconds),
		MinDataPoints:              intPtr(m.MinDataPoints),
		Severity:                   m.Severity.ValueString(),
		ThresholdUnit:              strPtr(m.ThresholdUnit),
		NotificationTitleTemplate:  strPtr(m.NotificationTitleTemplate),
		NotificationBodyTemplate:   strPtr(m.NotificationBodyTemplate),
		EscalateAfterMinutes:       intPtr(m.EscalateAfterMinutes),
		SecondEscalateAfterMinutes: intPtr(m.SecondEscalateAfterMinutes),
		Threshold:                  client.AlertThreshold{Comparator: m.Comparator.ValueString()},
	}
	if !m.RecoveryThreshold.IsNull() {
		v := m.RecoveryThreshold.ValueFloat64()
		rule.RecoveryThreshold = &v
	}
	if m.Labels != nil {
		rule.Labels = map[string]string{}
		for k, v := range m.Labels {
			rule.Labels[k] = v.ValueString()
		}
	}

	// threshold: a count for log/exception rules, a double for metric rules, unused otherwise.
	switch kind {
	case "Anomaly", "SloBurnRate":
	default:
		if m.Threshold.IsNull() {
			return rule, fmt.Errorf("threshold is required when condition_kind is %s", kind)
		}
		v := m.Threshold.ValueFloat64()
		if kind == "MetricThreshold" {
			rule.MetricThresholdValue = &v
		} else {
			if v < 0 || v != math.Trunc(v) {
				return rule, fmt.Errorf("threshold must be a non-negative whole number when condition_kind is %s", kind)
			}
			rule.Threshold.Count = uint64(v)
		}
	}

	var err error
	if rule.ChannelIDs, err = dir.ids(m.Channels); err != nil {
		return rule, fmt.Errorf("channels: %w", err)
	}
	if rule.EscalationChannelIDs, err = dir.ids(m.EscalationChannels); err != nil {
		return rule, fmt.Errorf("escalation_channels: %w", err)
	}
	if rule.SecondEscalationChannelIDs, err = dir.ids(m.SecondEscalationChannels); err != nil {
		return rule, fmt.Errorf("second_escalation_channels: %w", err)
	}

	rule.Condition = m.LogCondition.toAPI()
	if c := m.MetricCondition; c != nil {
		rule.MetricCondition = &client.MetricAlertCondition{
			MetricName: c.MetricName.ValueString(), Type: c.Type.ValueString(), Aggregation: c.Aggregation.ValueString(),
			Filter: client.MetricFilter{Services: stringSlice(c.Services), Attributes: keyValues(c.Attributes)},
		}
	}
	if c := m.ExceptionCondition; c != nil {
		rule.ExceptionCondition = &client.ExceptionCondition{
			ExceptionType: c.ExceptionType.ValueString(), ExceptionMessage: c.ExceptionMessage.ValueString(),
			Filter: client.ExceptionFilter{Services: stringSlice(c.Services), ResourceAttributes: keyValues(c.ResourceAttributes)},
		}
	}
	if c := m.AnomalyCondition; c != nil {
		rule.AnomalyCondition = &client.AnomalyCondition{
			Source: c.Source.ValueString(), Seasonality: c.Seasonality.ValueString(), Direction: c.Direction.ValueString(),
			BaselinePeriods: int(c.BaselinePeriods.ValueInt64()), ZScoreThreshold: c.ZScoreThreshold.ValueFloat64(),
		}
	}
	if c := m.SloCondition; c != nil {
		rule.SloCondition = &client.SloBurnRateCondition{
			SloID: c.SloID.ValueString(), LongWindowSeconds: int(c.LongWindowSeconds.ValueInt64()),
			ShortWindowSeconds: int(c.ShortWindowSeconds.ValueInt64()), BurnRateThreshold: c.BurnRateThreshold.ValueFloat64(),
		}
	}
	return rule, nil
}

func (m *alertRuleModel) fromAPI(api client.AlertRule, dir channelDirectory) {
	m.ID = types.StringValue(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Description = types.StringValue(deref(api.Description))
	m.Enabled = types.BoolValue(api.Enabled == nil || *api.Enabled)
	m.ConditionKind = types.StringValue(orDefault(api.ConditionKind, "LogCount"))
	m.Comparator = types.StringValue(orDefault(api.Threshold.Comparator, "GreaterThanOrEqual"))
	switch m.ConditionKind.ValueString() {
	case "MetricThreshold":
		if api.MetricThresholdValue != nil {
			m.Threshold = types.Float64Value(*api.MetricThresholdValue)
		}
	case "Anomaly", "SloBurnRate":
		// no threshold of its own: keep what configuration said (null)
	default:
		m.Threshold = types.Float64Value(float64(api.Threshold.Count))
	}
	m.ThresholdUnit = types.StringValue(deref(api.ThresholdUnit))
	if api.RecoveryThreshold != nil {
		m.RecoveryThreshold = types.Float64Value(*api.RecoveryThreshold)
	} else {
		m.RecoveryThreshold = types.Float64Null()
	}
	m.WindowSeconds = types.Int64Value(int64(api.WindowSeconds))
	m.CooldownSeconds = types.Int64Value(int64(derefInt(api.CooldownSeconds, 300)))
	m.EvaluationIntervalSeconds = types.Int64Value(int64(derefInt(api.EvaluationIntervalSeconds, 0)))
	m.NoDataWindowSeconds = types.Int64Value(int64(derefInt(api.NoDataWindowSeconds, 0)))
	m.MinDataPoints = types.Int64Value(int64(derefInt(api.MinDataPoints, 0)))
	m.Severity = types.StringValue(orDefault(api.Severity, "Critical"))
	m.NotificationTitleTemplate = types.StringValue(deref(api.NotificationTitleTemplate))
	m.NotificationBodyTemplate = types.StringValue(deref(api.NotificationBodyTemplate))
	m.EscalateAfterMinutes = types.Int64Value(int64(derefInt(api.EscalateAfterMinutes, 0)))
	m.SecondEscalateAfterMinutes = types.Int64Value(int64(derefInt(api.SecondEscalateAfterMinutes, 0)))

	if len(api.Labels) == 0 {
		if m.Labels != nil {
			m.Labels = map[string]types.String{}
		}
	} else {
		m.Labels = make(map[string]types.String, len(api.Labels))
		for k, v := range api.Labels {
			m.Labels[k] = types.StringValue(v)
		}
	}

	m.Channels = dir.names(api.ChannelIDs, m.Channels)
	m.EscalationChannels = dir.names(api.EscalationChannelIDs, m.EscalationChannels)
	m.SecondEscalationChannels = dir.names(api.SecondEscalationChannelIDs, m.SecondEscalationChannels)

	m.LogCondition = logConditionFromAPI(api.Condition, m.LogCondition)
	if c := api.MetricCondition; c != nil {
		prior := m.MetricCondition
		if prior == nil {
			prior = &metricConditionModel{}
		}
		m.MetricCondition = &metricConditionModel{
			MetricName: types.StringValue(c.MetricName), Type: types.StringValue(c.Type),
			Aggregation: types.StringValue(orDefault(c.Aggregation, "Value")),
			Services:    stringValues(c.Filter.Services, prior.Services),
			Attributes:  keyValueMap(c.Filter.Attributes, prior.Attributes),
		}
	} else {
		m.MetricCondition = nil
	}
	if c := api.ExceptionCondition; c != nil {
		prior := m.ExceptionCondition
		if prior == nil {
			prior = &exceptionConditionModel{}
		}
		m.ExceptionCondition = &exceptionConditionModel{
			ExceptionType: types.StringValue(c.ExceptionType), ExceptionMessage: types.StringValue(c.ExceptionMessage),
			Services:           stringValues(c.Filter.Services, prior.Services),
			ResourceAttributes: keyValueMap(c.Filter.ResourceAttributes, prior.ResourceAttributes),
		}
	} else {
		m.ExceptionCondition = nil
	}
	if c := api.AnomalyCondition; c != nil {
		m.AnomalyCondition = &anomalyConditionModel{
			Source: types.StringValue(orDefault(c.Source, "LogCount")), Seasonality: types.StringValue(orDefault(c.Seasonality, "Daily")),
			BaselinePeriods: types.Int64Value(int64(c.BaselinePeriods)), ZScoreThreshold: types.Float64Value(c.ZScoreThreshold),
			Direction: types.StringValue(orDefault(c.Direction, "Both")),
		}
	} else {
		m.AnomalyCondition = nil
	}
	if c := api.SloCondition; c != nil {
		m.SloCondition = &sloConditionModel{
			SloID: types.StringValue(c.SloID), LongWindowSeconds: types.Int64Value(int64(c.LongWindowSeconds)),
			ShortWindowSeconds: types.Int64Value(int64(c.ShortWindowSeconds)), BurnRateThreshold: types.Float64Value(c.BurnRateThreshold),
		}
	} else {
		m.SloCondition = nil
	}
}

// logConditionFromAPI keeps log_condition null when the rule has no log filter: the API always returns a
// (possibly empty) filter, and a configuration that omitted the block must not see a diff.
func logConditionFromAPI(f *client.LogFilter, prior *logConditionModel) *logConditionModel {
	empty := f == nil || (len(f.Services) == 0 && len(f.SeverityNumbers) == 0 && deref(f.Search) == "" && len(f.ScopeNames) == 0 && len(f.Attributes) == 0)
	if empty && prior == nil {
		return nil
	}
	if f == nil {
		f = &client.LogFilter{}
	}
	if prior == nil {
		prior = &logConditionModel{}
	}
	out := &logConditionModel{
		Services:   stringValues(f.Services, prior.Services),
		ScopeNames: stringValues(f.ScopeNames, prior.ScopeNames),
		Search:     readString(f.Search, prior.Search),
	}
	if len(f.SeverityNumbers) > 0 {
		for _, n := range f.SeverityNumbers {
			out.SeverityNumbers = append(out.SeverityNumbers, types.Int64Value(int64(n)))
		}
	} else if prior.SeverityNumbers != nil {
		out.SeverityNumbers = []types.Int64{}
	}
	if len(f.Attributes) > 0 {
		for _, a := range f.Attributes {
			out.Attributes = append(out.Attributes, attributeFilterModel{
				Bag: types.StringValue(orDefault(a.Bag, "Log")), Key: types.StringValue(a.Key), Value: types.StringValue(a.Value),
			})
		}
	} else if prior.Attributes != nil {
		out.Attributes = []attributeFilterModel{}
	}
	return out
}

// ---- CRUD --------------------------------------------------------------------------------------------

func (r *alertRuleResource) build(ctx context.Context, m alertRuleModel, diags *diag.Diagnostics) (client.AlertRule, channelDirectory, bool) {
	dir, err := r.channels(ctx)
	if err != nil {
		diags.AddError("Listing notification channels", err.Error())
		return client.AlertRule{}, dir, false
	}
	rule, err := m.toAPI(dir)
	if err != nil {
		diags.AddError("Invalid alert rule", err.Error())
		return client.AlertRule{}, dir, false
	}
	return rule, dir, true
}

func (r *alertRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan alertRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rule, dir, ok := r.build(ctx, plan, &resp.Diagnostics)
	if !ok {
		return
	}
	created, err := r.client.CreateAlertRule(ctx, rule)
	if err != nil {
		resp.Diagnostics.AddError("Creating alert rule", err.Error())
		return
	}
	plan.fromAPI(created, dir)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *alertRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state alertRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetAlertRule(ctx, state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading alert rule", err.Error())
		return
	}
	dir, err := r.channels(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Listing notification channels", err.Error())
		return
	}
	state.fromAPI(got, dir)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *alertRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state alertRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rule, dir, ok := r.build(ctx, plan, &resp.Diagnostics)
	if !ok {
		return
	}
	updated, err := r.client.UpdateAlertRule(ctx, state.ID.ValueString(), rule)
	if err != nil {
		resp.Diagnostics.AddError("Updating alert rule", err.Error())
		return
	}
	plan.fromAPI(updated, dir)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *alertRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state alertRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteAlertRule(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Deleting alert rule", err.Error())
	}
}

// ImportState accepts a rule id or its name.
func (r *alertRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID
	if !guidPattern.MatchString(id) {
		found, err := r.client.FindAlertRuleByName(ctx, id)
		if err != nil {
			resp.Diagnostics.AddError("Importing alert rule", err.Error())
			return
		}
		id = found.ID
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
