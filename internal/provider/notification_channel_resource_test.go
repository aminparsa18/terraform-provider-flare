package provider

import (
	"testing"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func ptr[T any](v T) *T { return &v }

func TestFromAPIKeepsConfiguredSecretsInsteadOfTheMaskedValue(t *testing.T) {
	m := notificationChannelModel{
		WebhookURL:       types.StringValue("https://hooks.example.com/real-token"),
		TelegramBotToken: types.StringValue("real-bot-token"),
	}
	m.fromAPI(client.NotificationChannel{
		ID: "1", Name: "n", Type: "Webhook",
		WebhookURL:       ptr("https://hooks.example.com/••••••••"),
		TelegramBotToken: ptr("••••••••oken"),
	})
	if m.WebhookURL.ValueString() != "https://hooks.example.com/real-token" || m.TelegramBotToken.ValueString() != "real-bot-token" {
		t.Fatalf("secrets overwritten by masked values: %+v", m)
	}
}

func TestFromAPIKeepsUnsetOptionalsNullSoThereIsNoDiff(t *testing.T) {
	m := notificationChannelModel{} // zero value: all attributes null
	m.fromAPI(client.NotificationChannel{ID: "1", Name: "n", Type: "Email", Description: ptr(""), EmailTo: ptr("a@b.c"), JiraEmail: ptr("")})
	if !m.Description.IsNull() || !m.JiraEmail.IsNull() {
		t.Fatalf("empty API strings should read back as null: %+v", m)
	}
	if m.EmailTo.ValueString() != "a@b.c" {
		t.Fatalf("EmailTo = %v", m.EmailTo)
	}
}

func TestFromAPIKeepsAnExplicitEmptyString(t *testing.T) {
	m := notificationChannelModel{Description: types.StringValue("")}
	m.fromAPI(client.NotificationChannel{ID: "1", Name: "n", Type: "Email", Description: ptr("")})
	if m.Description.IsNull() || m.Description.ValueString() != "" {
		t.Fatalf("Description = %v, want empty string", m.Description)
	}
}

func TestToAPIOmitsNullAndSendsConfigured(t *testing.T) {
	m := notificationChannelModel{
		Name: types.StringValue("n"), Type: types.StringValue("Email"),
		EmailTo: types.StringValue("a@b.c"), SendResolved: types.BoolValue(false),
		WebhookURL: types.StringNull(),
	}
	api := m.toAPI()
	if api.WebhookURL != nil || api.EmailTo == nil || *api.EmailTo != "a@b.c" || api.SendResolved == nil || *api.SendResolved {
		t.Fatalf("toAPI = %+v", api)
	}
}

func TestGUIDPattern(t *testing.T) {
	if !guidPattern.MatchString("0b6f0d1e-7c1a-4d6e-9a53-1f2e3d4c5b6a") || guidPattern.MatchString("oncall-slack") {
		t.Fatal("guidPattern misclassifies ids and names")
	}
}
