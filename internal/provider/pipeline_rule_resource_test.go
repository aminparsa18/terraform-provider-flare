package provider

import (
	"testing"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestPipelineActionSendsOnlyItsKindsGroup(t *testing.T) {
	extract := pipelineActionModel{Kind: types.StringValue("ExtractRegex"), Pattern: types.StringValue("(?<id>\\d+)"), Replacement: types.StringValue("ignored")}.toAPI()
	if extract.ExtractRegex == nil || extract.RedactRegex != nil || extract.ParseJSON != nil {
		t.Fatalf("extract action sent the wrong groups: %+v", extract)
	}

	redact := pipelineActionModel{Kind: types.StringValue("RedactRegex"), Pattern: types.StringValue("\\d{16}"), Replacement: types.StringNull()}.toAPI()
	if redact.RedactRegex == nil || redact.RedactRegex.Replacement != nil {
		t.Fatalf("an omitted replacement must stay omitted so Flare applies its default: %+v", redact)
	}

	parse := pipelineActionModel{Kind: types.StringValue("ParseJson"), MaxDepth: types.Int64Value(3), MaxKeys: types.Int64Null()}.toAPI()
	if parse.ParseJSON == nil || parse.ParseJSON.MaxDepth == nil || *parse.ParseJSON.MaxDepth != 3 || parse.ParseJSON.MaxKeys != nil {
		t.Fatalf("parse action mapped wrongly: %+v", parse.ParseJSON)
	}
}

func TestPipelineRuleOmittedConditionAndDefaultsReadBackClean(t *testing.T) {
	enabled := true
	dflt := "***"
	api := client.PipelineRule{
		ID: "id", Name: "r", Enabled: &enabled, Condition: &client.LogFilter{},
		Actions: []client.PipelineAction{{Kind: "RedactRegex", RedactRegex: &client.RedactRegex{Pattern: "x", Replacement: &dflt}}},
	}
	m := pipelineRuleModel{Actions: []pipelineActionModel{{Replacement: types.StringNull(), SourceAttributeKey: types.StringNull()}}}
	m.fromAPI(api)
	if m.Condition != nil {
		t.Fatal("an empty condition must read back as null when the block was omitted")
	}
	if !m.Actions[0].Replacement.IsNull() || !m.Actions[0].SourceAttributeKey.IsNull() {
		t.Fatalf("omitted replacement/source key must stay null: %+v", m.Actions[0])
	}

	// An explicitly configured replacement, even Flare's default, is kept.
	m = pipelineRuleModel{Actions: []pipelineActionModel{{Replacement: types.StringValue("***")}}}
	m.fromAPI(api)
	if m.Actions[0].Replacement.ValueString() != "***" {
		t.Fatal("a configured replacement must be kept")
	}
}
