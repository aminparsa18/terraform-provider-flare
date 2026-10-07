package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestSLOAvailabilityOmitsLatencyThreshold(t *testing.T) {
	m := sloModel{
		Name: types.StringValue("s"), Description: types.StringValue(""), Kind: types.StringValue("Availability"),
		ServiceName: types.StringValue("api"), OperationName: types.StringValue(""), TargetPercent: types.Float64Value(99.9),
		LatencyThresholdMs: types.Int64Value(0), WindowDays: types.Int64Value(30),
	}
	if m.toAPI().LatencyThresholdMs != nil {
		t.Fatal("availability SLO must not send latencyThresholdMs")
	}
	m.Kind = types.StringValue("Latency")
	m.LatencyThresholdMs = types.Int64Value(500)
	if p := m.toAPI().LatencyThresholdMs; p == nil || *p != 500 {
		t.Fatal("latency SLO must send its threshold")
	}
}
