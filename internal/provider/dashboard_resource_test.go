package provider

import (
	"encoding/json"
	"testing"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestCheckLayout(t *testing.T) {
	for _, ok := range []string{`{"panels":[]}`, `{"panels":[{"id":"a"}],"variables":[]}`} {
		if err := checkLayout(ok); err != nil {
			t.Errorf("%s rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{``, `[]`, `{}`, `{"panels":{}}`, `not json`} {
		if checkLayout(bad) == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestLayoutKeepsConfiguredTextWhenSameJSON(t *testing.T) {
	prior := types.StringValue("{\n  \"variables\": [],\n  \"panels\": []\n}")
	if got := layoutFromAPI(json.RawMessage(`{"panels":[],"variables":[]}`), prior); got.ValueString() != prior.ValueString() {
		t.Fatalf("equal JSON must keep the configured text, got %s", got.ValueString())
	}
	if got := layoutFromAPI(json.RawMessage(` {"panels": [{"id":"x"}]} `), prior); got.ValueString() != `{"panels":[{"id":"x"}]}` {
		t.Fatalf("different JSON must take the API's (compacted), got %s", got.ValueString())
	}
}

func TestDashboardProjectAndTagsMapping(t *testing.T) {
	m := dashboardModel{Name: types.StringValue("d"), Description: types.StringValue(""), Tags: types.SetNull(types.StringType),
		ProjectID: types.StringNull(), LayoutJSON: types.StringValue(`{"panels":[]}`)}

	in, _ := m.toAPI(false)
	if in.ProjectID != nil || in.Tags == nil || len(in.Tags) != 0 {
		t.Fatalf("no project is omitted and null tags are sent as [] so Terraform stays authoritative: %+v", in)
	}
	in, _ = m.toAPI(true)
	if in.ProjectID == nil || *in.ProjectID != emptyGUID {
		t.Fatal("dropping a project must send the empty id, since an omitted one keeps the old project")
	}

	api := client.Dashboard{ID: "id", Name: "d", LayoutJSON: json.RawMessage(`{"panels":[]}`), Tags: []string{}}
	m.fromAPI(api)
	if !m.Tags.IsNull() || !m.ProjectID.IsNull() {
		t.Fatal("omitted tags and project must read back as null")
	}
}
