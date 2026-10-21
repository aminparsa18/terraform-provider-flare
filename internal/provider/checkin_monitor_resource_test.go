package provider

import (
	"testing"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestCheckInMonitorToAPI(t *testing.T) {
	m := checkInMonitorModel{
		Slug: types.StringValue("nightly-backup"), Name: types.StringValue("Nightly backup"), Description: types.StringValue(""),
		Enabled: types.BoolValue(true), CronExpression: types.StringValue("0 2 * * *"), IntervalSeconds: types.Int64Null(),
		TimeZone: types.StringValue("Europe/Berlin"), GraceSeconds: types.Int64Value(600), MaxRuntimeSeconds: types.Int64Value(0),
	}
	got := m.toAPI()
	if got.Slug != "nightly-backup" || deref(got.CronExpression) != "0 2 * * *" || got.IntervalSeconds != nil ||
		derefInt(got.GraceSeconds, -1) != 600 || derefInt(got.MaxRuntimeSeconds, -1) != 0 || got.Enabled == nil || !*got.Enabled {
		t.Fatalf("unexpected mapping: %+v", got)
	}
}

func TestCheckInMonitorFromAPIReadsUnusedScheduleAsNull(t *testing.T) {
	interval := client.CheckInMonitor{ID: "id", Slug: "poll", Name: "Poll", CronExpression: ptr(""), IntervalSeconds: ptr(900), TimeZone: ptr("UTC"), GraceSeconds: ptr(300)}
	var m checkInMonitorModel
	m.fromAPI(interval)
	if !m.CronExpression.IsNull() || m.IntervalSeconds.ValueInt64() != 900 || !m.Enabled.ValueBool() {
		t.Fatalf("an interval monitor must read cron as null: %+v", m)
	}

	cron := client.CheckInMonitor{ID: "id", Slug: "nightly", Name: "Nightly", CronExpression: ptr("@daily"), IntervalSeconds: ptr(0), Enabled: ptr(false)}
	m.fromAPI(cron)
	if !m.IntervalSeconds.IsNull() || m.CronExpression.ValueString() != "@daily" || m.Enabled.ValueBool() || m.TimeZone.ValueString() != "UTC" {
		t.Fatalf("a cron monitor must read interval as null and default the zone: %+v", m)
	}
}
