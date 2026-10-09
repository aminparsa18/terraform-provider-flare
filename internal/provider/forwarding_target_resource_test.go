package provider

import (
	"context"
	"testing"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestForwardingTargetToAPI(t *testing.T) {
	ctx := context.Background()
	headers, _ := types.MapValueFrom(ctx, types.StringType, map[string]string{"Authorization": "Bearer x"})
	m := forwardingTargetModel{
		Name: types.StringValue("t"), Enabled: types.BoolValue(true), Endpoint: types.StringValue("https://o.example.com"),
		Headers: headers, Signals: stringSet([]string{"Logs"}), Services: stringSet(nil), IngestKeyIDs: stringSet(nil), Gzip: types.BoolValue(false),
	}
	got, diags := m.toAPI(ctx)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if got.Headers["Authorization"] != "Bearer x" || len(got.Signals) != 1 || got.Gzip == nil || *got.Gzip {
		t.Fatalf("unexpected mapping: %+v", got)
	}

	// A target with no headers must send an explicit empty map so an update clears any stored ones.
	m.Headers = types.MapNull(types.StringType)
	got, _ = m.toAPI(ctx)
	if got.Headers == nil || len(got.Headers) != 0 {
		t.Fatalf("null headers must send {}, got %#v", got.Headers)
	}
}

func TestForwardingTargetFromAPIKeepsConfiguredHeaders(t *testing.T) {
	headers, _ := types.MapValueFrom(context.Background(), types.StringType, map[string]string{"Authorization": "Bearer x"})
	m := forwardingTargetModel{Headers: headers}
	m.fromAPI(client.ForwardingTarget{ID: "id", Name: "t", Endpoint: "https://o.example.com", Headers: map[string]string{"Authorization": "Bea***"}})
	if m.Headers.Elements()["Authorization"].String() != `"Bearer x"` {
		t.Fatalf("masked headers must not overwrite the configured values: %v", m.Headers)
	}
	if m.Signals.IsNull() || len(m.Signals.Elements()) != 0 || !m.Enabled.ValueBool() || !m.Gzip.ValueBool() {
		t.Fatalf("empty lists must read back as empty sets and omitted booleans as true: %+v", m)
	}
}
