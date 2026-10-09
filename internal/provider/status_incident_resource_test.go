package provider

import (
	"testing"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
)

func TestStatusIncidentFromAPI(t *testing.T) {
	var m statusIncidentModel
	m.fromAPI(client.StatusIncident{
		ID: "i1", PageID: "p1", Title: "Down", Status: "Monitoring",
		Updates:    []client.StatusIncidentUpdate{{Status: "Investigating", Message: "first"}, {Status: "Monitoring", Message: "latest"}},
		Components: []string{"AAAA"},
	})
	if m.Status.ValueString() != "Monitoring" || m.Message.ValueString() != "latest" {
		t.Fatalf("status/message must be the latest update's: %+v", m)
	}
	if len(m.Components.Elements()) != 1 {
		t.Fatalf("components lost: %+v", m.Components)
	}

	m.fromAPI(client.StatusIncident{ID: "i1", PageID: "p1", Title: "Down", Status: "Investigating"})
	if m.Components.IsNull() || len(m.Components.Elements()) != 0 {
		t.Fatal("no components must read back as an empty set, never null")
	}
}
