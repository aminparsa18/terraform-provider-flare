package provider

import (
	"context"
	"testing"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestMaintenanceWindowToAPI(t *testing.T) {
	dir := ruleDirectory{byName: map[string]string{"high errors": "rule-1"}, byID: map[string]string{"rule-1": "High errors"}}
	days, _ := types.SetValueFrom(context.Background(), types.StringType, []string{"Sunday", "Saturday"})
	m := maintenanceWindowModel{
		Name: types.StringValue("w"), Description: types.StringValue(""),
		RuleNames: []types.String{types.StringValue("HIGH errors")},
		StartsAt:  types.StringValue("2026-11-01T02:00:00Z"), EndsAt: types.StringValue("2026-11-01T04:00:00Z"),
		Recurrence: types.StringValue("Weekly"), DaysOfWeek: days, RepeatUntil: types.StringNull(), TimeZone: types.StringValue("Europe/Berlin"),
		LabelMatchers: map[string]types.String{"team": types.StringValue("core")},
	}
	got, err := m.toAPI(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.RuleIDs) != 1 || got.RuleIDs[0] != "rule-1" || len(got.DaysOfWeek) != 2 || got.RepeatUntil != nil || got.LabelMatchers["team"] != "core" {
		t.Fatalf("unexpected mapping: %+v", got)
	}

	m.RuleNames = []types.String{types.StringValue("missing")}
	if _, err := m.toAPI(dir); err == nil {
		t.Fatal("an unknown rule name must be an error")
	}
	m.RuleNames = nil
	m.StartsAt = types.StringValue("tomorrow")
	if _, err := m.toAPI(dir); err == nil {
		t.Fatal("a non-RFC 3339 timestamp must be an error")
	}
}

func TestMaintenanceWindowTimestampsKeepConfiguredSpelling(t *testing.T) {
	api := client.MaintenanceWindow{ID: "id", Name: "w", StartsAt: "2026-11-01T02:00:00+00:00", EndsAt: "2026-11-01T04:30:00+00:00", Recurrence: "None", TimeZone: ptr("UTC")}
	m := maintenanceWindowModel{StartsAt: types.StringValue("2026-11-01T02:00:00Z"), EndsAt: types.StringValue("2026-11-01T05:00:00Z"), DaysOfWeek: types.SetNull(types.StringType)}
	m.fromAPI(api, ruleDirectory{})
	if m.StartsAt.ValueString() != "2026-11-01T02:00:00Z" {
		t.Fatalf("an equal instant must keep the configured spelling, got %s", m.StartsAt.ValueString())
	}
	if m.EndsAt.ValueString() != "2026-11-01T04:30:00Z" {
		t.Fatalf("a different instant is normalised to UTC, got %s", m.EndsAt.ValueString())
	}
	if !m.DaysOfWeek.IsNull() || !m.RepeatUntil.IsNull() || m.LabelMatchers != nil || m.RuleNames != nil {
		t.Fatal("omitted optional attributes must read back as null")
	}
}
