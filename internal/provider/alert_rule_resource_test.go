package provider

import (
	"encoding/json"
	"testing"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var testDirectory = channelDirectory{
	byName: map[string]string{"oncall-slack": "id-slack"},
	byID:   map[string]string{"id-slack": "oncall-slack"},
}

func baseModel() alertRuleModel {
	return alertRuleModel{
		Name: types.StringValue("r"), Description: types.StringValue(""), Enabled: types.BoolValue(true),
		ConditionKind: types.StringValue("LogCount"), Comparator: types.StringValue("GreaterThanOrEqual"),
		Threshold: types.Float64Value(5), ThresholdUnit: types.StringValue(""), RecoveryThreshold: types.Float64Null(),
		WindowSeconds: types.Int64Value(300), CooldownSeconds: types.Int64Value(300),
		EvaluationIntervalSeconds: types.Int64Value(0), NoDataWindowSeconds: types.Int64Value(0), MinDataPoints: types.Int64Value(0),
		Severity: types.StringValue("Critical"), NotificationTitleTemplate: types.StringValue(""), NotificationBodyTemplate: types.StringValue(""),
		EscalateAfterMinutes: types.Int64Value(0), SecondEscalateAfterMinutes: types.Int64Value(0),
		Channels: []types.String{types.StringValue("oncall-slack")},
	}
}

// roundTrip sends the model through the wire format and back, as Create does with the server's response.
func roundTrip(t *testing.T, m alertRuleModel) alertRuleModel {
	t.Helper()
	api, err := m.toAPI(testDirectory)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(api)
	if err != nil {
		t.Fatal(err)
	}
	var back client.AlertRule
	if err := json.Unmarshal(payload, &back); err != nil {
		t.Fatal(err)
	}
	back.ID = "rule-id"
	out := m
	out.fromAPI(back, testDirectory)
	return out
}

func TestLogCountRoundTripLeavesOmittedBlocksNull(t *testing.T) {
	out := roundTrip(t, baseModel())
	if out.LogCondition != nil || out.MetricCondition != nil || out.Labels != nil || out.Channels[0].ValueString() != "oncall-slack" {
		t.Fatalf("unexpected drift: %+v", out)
	}
	if out.Threshold.ValueFloat64() != 5 {
		t.Fatalf("threshold = %v", out.Threshold)
	}
}

func TestMetricRuleUsesMetricThresholdValue(t *testing.T) {
	m := baseModel()
	m.ConditionKind = types.StringValue("MetricThreshold")
	m.Threshold = types.Float64Value(0.75)
	m.MetricCondition = &metricConditionModel{
		MetricName: types.StringValue("cpu"), Type: types.StringValue("Gauge"), Aggregation: types.StringValue("Max"),
		Attributes: map[string]types.String{"host": types.StringValue("a")},
	}
	api, err := m.toAPI(testDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if api.MetricThresholdValue == nil || *api.MetricThresholdValue != 0.75 || api.Threshold.Count != 0 {
		t.Fatalf("api = %+v", api)
	}
	out := roundTrip(t, m)
	if out.Threshold.ValueFloat64() != 0.75 || out.MetricCondition.Attributes["host"].ValueString() != "a" {
		t.Fatalf("round trip = %+v", out)
	}
}

func TestThresholdValidation(t *testing.T) {
	m := baseModel()
	m.Threshold = types.Float64Value(1.5)
	if _, err := m.toAPI(testDirectory); err == nil {
		t.Fatal("fractional log count should be rejected")
	}
	m.Threshold = types.Float64Null()
	if _, err := m.toAPI(testDirectory); err == nil {
		t.Fatal("missing threshold should be rejected")
	}
	m.ConditionKind = types.StringValue("Anomaly")
	if _, err := m.toAPI(testDirectory); err != nil {
		t.Fatalf("anomaly needs no threshold: %v", err)
	}
}

func TestUnknownChannelNameIsAnError(t *testing.T) {
	m := baseModel()
	m.Channels = []types.String{types.StringValue("nope")}
	if _, err := m.toAPI(testDirectory); err == nil {
		t.Fatal("expected unknown channel error")
	}
}

func TestLogConditionRoundTrip(t *testing.T) {
	m := baseModel()
	m.LogCondition = &logConditionModel{
		Services: []types.String{types.StringValue("api")}, SeverityNumbers: []types.Int64{types.Int64Value(17)},
		Attributes: []attributeFilterModel{{Bag: types.StringValue("Resource"), Key: types.StringValue("k"), Value: types.StringValue("v")}},
		Search:     types.StringNull(),
	}
	out := roundTrip(t, m)
	if len(out.LogCondition.Attributes) != 1 || out.LogCondition.SeverityNumbers[0].ValueInt64() != 17 || !out.LogCondition.Search.IsNull() {
		t.Fatalf("round trip = %+v", out.LogCondition)
	}
}
