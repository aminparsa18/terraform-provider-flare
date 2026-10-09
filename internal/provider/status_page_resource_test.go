package provider

import (
	"testing"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestStatusPageRoundTrip(t *testing.T) {
	m := statusPageModel{
		Slug: types.StringValue("acme"), Title: types.StringValue("Acme"), Description: types.StringValue(""), Enabled: types.BoolValue(true),
		Components:           []statusComponentModel{{Name: types.StringValue("API"), Kind: types.StringValue("Slo"), RefID: types.StringValue("id-1")}},
		SubscriberChannelIDs: stringSet([]string{"c1"}),
	}
	api, diags := m.toAPI()
	if diags.HasError() || len(api.SubscriberChannelIDs) != 1 || api.SubscriberChannelIDs[0] != "c1" {
		t.Fatalf("subscribers lost: %+v %v", api, diags)
	}
	if len(api.Components) != 1 || api.Components[0].Kind != "Slo" || api.Enabled == nil || !*api.Enabled {
		t.Fatalf("toAPI lost data: %+v", api)
	}
	var back statusPageModel
	back.fromAPI(client.StatusPage{ID: "p1", Slug: "acme", Title: "Acme", Components: api.Components, SubscriberChannelIDs: api.SubscriberChannelIDs})
	if len(back.SubscriberChannelIDs.Elements()) != 1 {
		t.Fatalf("subscribers not read back: %+v", back)
	}
	if back.Enabled.ValueBool() || len(back.Components) != 1 || back.Components[0].RefID.ValueString() != "id-1" {
		t.Fatalf("fromAPI wrong: %+v", back)
	}
	back.fromAPI(client.StatusPage{ID: "p1", Slug: "acme", Title: "Acme"})
	if back.Components != nil {
		t.Fatal("no components must read back as null")
	}
}
