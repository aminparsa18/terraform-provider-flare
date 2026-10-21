package provider

import (
	"testing"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestIngestKeyLimitsAndProjectMapping(t *testing.T) {
	m := ingestKeyModel{
		ProjectID: types.StringValue(""), LimitsEnabled: types.BoolValue(true),
		MaxEventsPerMinute: types.Int64Value(100), MaxBytesPerMinute: types.Int64Null(),
		MaxEventsPerDay: types.Int64Null(), MaxBytesPerDay: types.Int64Null(),
		MaxReplayBytesPerMinute: types.Int64Value(5000), MaxReplayBytesPerDay: types.Int64Null(),
	}
	if m.projectPtr() != nil {
		t.Fatal("an empty project id means instance-wide and must not be sent")
	}
	l := m.limits()
	if !l.LimitsEnabled || l.MaxEventsPerMinute == nil || *l.MaxEventsPerMinute != 100 || l.MaxBytesPerMinute != nil {
		t.Fatalf("limits mapped wrongly: %+v", l)
	}
	if l.MaxReplayBytesPerMinute == nil || *l.MaxReplayBytesPerMinute != 5000 || l.MaxReplayBytesPerDay != nil {
		t.Fatalf("replay caps mapped wrongly: %+v", l)
	}
	if !(ingestKeyModel{LimitsEnabled: types.BoolValue(false), MaxReplayBytesPerDay: types.Int64Value(1)}).hasLimits() {
		t.Fatal("a replay cap alone counts as a limit that must be sent on create")
	}
	if !m.hasLimits() || (ingestKeyModel{LimitsEnabled: types.BoolValue(false)}).hasLimits() {
		t.Fatal("hasLimits must be true only when something is set")
	}
}

func TestIngestKeyFromAPIKeepsSecretAndNullProject(t *testing.T) {
	zero := emptyGUID
	m := ingestKeyModel{RawKey: types.StringValue("secret"), ProjectID: types.StringNull()}
	m.fromAPI(client.IngestKey{ID: "id", Name: "k", ProjectID: &zero})
	if m.RawKey.ValueString() != "secret" {
		t.Fatal("the secret must survive a refresh")
	}
	if !m.ProjectID.IsNull() || !m.MaxEventsPerMinute.IsNull() || !m.MaxReplayBytesPerMinute.IsNull() || !m.MaxReplayBytesPerDay.IsNull() {
		t.Fatalf("an instance-wide key without caps must read back null: %+v", m)
	}
}
