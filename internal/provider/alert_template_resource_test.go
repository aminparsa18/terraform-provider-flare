package provider

import (
	"context"
	"testing"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestAlertTemplateToAPI(t *testing.T) {
	ctx := context.Background()
	bodies, _ := types.MapValueFrom(ctx, types.StringType, map[string]string{"Telegram": "short"})
	m := alertTemplateModel{
		Name: types.StringValue("pager"), Description: types.StringValue(""), IsDefault: types.BoolValue(true),
		TitleTemplate: types.StringValue("{{rule_name}}"), BodyTemplate: types.StringValue(""), ResolvedBodyTemplate: types.StringValue(""),
		ChannelBodies: bodies,
	}
	got, diags := m.toAPI(ctx)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if got.IsDefault == nil || !*got.IsDefault || got.ChannelBodies["Telegram"] != "short" || got.TitleTemplate == nil || *got.TitleTemplate != "{{rule_name}}" {
		t.Fatalf("unexpected mapping: %+v", got)
	}

	// No overrides must send an explicit empty map so an update clears any stored ones.
	m.ChannelBodies = types.MapNull(types.StringType)
	got, _ = m.toAPI(ctx)
	if got.ChannelBodies == nil || len(got.ChannelBodies) != 0 {
		t.Fatalf("null channel_bodies must send {}, got %#v", got.ChannelBodies)
	}
}

func TestAlertTemplateFromAPIKeepsNullChannelBodies(t *testing.T) {
	api := client.AlertTemplate{ID: "id", Name: "pager"}

	m := alertTemplateModel{ChannelBodies: types.MapNull(types.StringType)}
	m.fromAPI(api)
	if !m.ChannelBodies.IsNull() || m.IsDefault.ValueBool() || m.BodyTemplate.ValueString() != "" {
		t.Fatalf("omitted channel_bodies must stay null and omitted texts read back empty: %+v", m)
	}

	empty, _ := types.MapValueFrom(context.Background(), types.StringType, map[string]string{})
	m = alertTemplateModel{ChannelBodies: empty}
	m.fromAPI(api)
	if m.ChannelBodies.IsNull() {
		t.Fatalf("an empty configured map must read back empty, not null")
	}
}

func TestAlertRuleTemplateResolvesByName(t *testing.T) {
	dir := channelDirectory{
		byName: map[string]string{}, byID: map[string]string{},
		templateByName: map[string]string{"pager": "tpl-1"}, templateByID: map[string]string{"tpl-1": "Pager"},
	}
	m := alertRuleModel{ConditionKind: types.StringValue("LogCount"), Threshold: types.Float64Value(1), NotificationTemplate: types.StringValue(" PAGER ")}
	rule, err := m.toAPI(dir)
	if err != nil {
		t.Fatal(err)
	}
	if rule.NotificationTemplateID == nil || *rule.NotificationTemplateID != "tpl-1" {
		t.Fatalf("template name must resolve case-insensitively to its id: %+v", rule.NotificationTemplateID)
	}

	m.NotificationTemplate = types.StringValue("missing")
	if _, err := m.toAPI(dir); err == nil {
		t.Fatal("an unknown template name must be an error")
	}

	id := "tpl-1"
	var back alertRuleModel
	back.fromAPI(client.AlertRule{ID: "r", Name: "n", NotificationTemplateID: &id}, dir)
	if back.NotificationTemplate.ValueString() != "Pager" {
		t.Fatalf("id must read back as the template's name, got %v", back.NotificationTemplate)
	}
}
