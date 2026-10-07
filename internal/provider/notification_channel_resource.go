package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*notificationChannelResource)(nil)
	_ resource.ResourceWithConfigure   = (*notificationChannelResource)(nil)
	_ resource.ResourceWithImportState = (*notificationChannelResource)(nil)
)

var guidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

var channelTypes = []string{"Webhook", "Telegram", "Email", "PagerDuty", "Teams", "Discord", "Jira", "IncidentIo", "JsmOps"}

// NewNotificationChannelResource is the flare_notification_channel factory.
func NewNotificationChannelResource() resource.Resource { return &notificationChannelResource{} }

type notificationChannelResource struct{ client *client.Client }

// notificationChannelModel is shared by the resource and (a subset of) the data source.
type notificationChannelModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Description         types.String `tfsdk:"description"`
	Type                types.String `tfsdk:"type"`
	WebhookURL          types.String `tfsdk:"webhook_url"`
	TelegramBotToken    types.String `tfsdk:"telegram_bot_token"`
	TelegramChatID      types.String `tfsdk:"telegram_chat_id"`
	EmailTo             types.String `tfsdk:"email_to"`
	PagerDutyRoutingKey types.String `tfsdk:"pagerduty_routing_key"`
	SendResolved        types.Bool   `tfsdk:"send_resolved"`
	JiraBaseURL         types.String `tfsdk:"jira_base_url"`
	JiraEmail           types.String `tfsdk:"jira_email"`
	JiraAPIToken        types.String `tfsdk:"jira_api_token"`
	JiraProjectKey      types.String `tfsdk:"jira_project_key"`
	JiraIssueType       types.String `tfsdk:"jira_issue_type"`
	IncidentIoToken     types.String `tfsdk:"incident_io_token"`
	JsmOpsAPIKey        types.String `tfsdk:"jsm_ops_api_key"`
}

func (r *notificationChannelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notification_channel"
}

func (r *notificationChannelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	secret := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{
			Optional: true, Sensitive: true,
			Description: desc + " Write-only: Flare never returns it, so changes made outside Terraform to this value are not detected.",
		}
	}
	plain := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Optional: true, Description: desc}
	}

	resp.Schema = schema.Schema{
		Description: "A saved notification destination that alert rules reference by name. Which destination fields apply depends on `type`; Flare validates the combination.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, Description: "Flare's id for the channel.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Unique (case-insensitive) name. Renaming updates the channel in place.",
			},
			"description": plain("Free-text description."),
			"type": schema.StringAttribute{
				Required:    true,
				Description: "One of: " + joinQuoted(channelTypes) + ".",
				Validators:  []validator.String{stringvalidator.OneOf(channelTypes...)},
			},
			"webhook_url":           secret("Destination URL for Webhook, Teams, Discord and IncidentIo channels. Treated as a secret because the URL usually embeds a token."),
			"telegram_bot_token":    secret("Telegram bot token."),
			"telegram_chat_id":      plain("Telegram chat id."),
			"email_to":              plain("Recipient address(es) for Email channels."),
			"pagerduty_routing_key": secret("PagerDuty Events API v2 routing key."),
			"send_resolved": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(true),
				Description: "Whether a rule's recovery sends a \"Resolved\" notification through this channel. Defaults to true.",
			},
			"jira_base_url":     plain("Jira Cloud site root, e.g. https://acme.atlassian.net."),
			"jira_email":        plain("Atlassian account email for the API token."),
			"jira_api_token":    secret("Atlassian API token."),
			"jira_project_key":  plain("Key of the Jira project issues are created in."),
			"jira_issue_type":   plain("Jira issue type name; empty means Task."),
			"incident_io_token": secret("Bearer token of the incident.io HTTP alert source."),
			"jsm_ops_api_key":   secret("JSM Operations integration API key."),
		},
	}
}

func (r *notificationChannelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (m notificationChannelModel) toAPI() client.NotificationChannel {
	sendResolved := m.SendResolved.ValueBool()
	return client.NotificationChannel{
		Name:                m.Name.ValueString(),
		Description:         strPtr(m.Description),
		Type:                m.Type.ValueString(),
		WebhookURL:          strPtr(m.WebhookURL),
		TelegramBotToken:    strPtr(m.TelegramBotToken),
		TelegramChatID:      strPtr(m.TelegramChatID),
		EmailTo:             strPtr(m.EmailTo),
		PagerDutyRoutingKey: strPtr(m.PagerDutyRoutingKey),
		SendResolved:        &sendResolved,
		JiraBaseURL:         strPtr(m.JiraBaseURL),
		JiraEmail:           strPtr(m.JiraEmail),
		JiraAPIToken:        strPtr(m.JiraAPIToken),
		JiraProjectKey:      strPtr(m.JiraProjectKey),
		JiraIssueType:       strPtr(m.JiraIssueType),
		IncidentIoToken:     strPtr(m.IncidentIoToken),
		JsmOpsAPIKey:        strPtr(m.JsmOpsAPIKey),
	}
}

// fromAPI folds a server response into the model. Credentials are deliberately left as they were: the API
// returns them masked, and the configured value is the source of truth (ADR-0146).
func (m *notificationChannelModel) fromAPI(api client.NotificationChannel) {
	m.ID = types.StringValue(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Type = types.StringValue(api.Type)
	m.Description = readString(api.Description, m.Description)
	m.TelegramChatID = readString(api.TelegramChatID, m.TelegramChatID)
	m.EmailTo = readString(api.EmailTo, m.EmailTo)
	m.JiraBaseURL = readString(api.JiraBaseURL, m.JiraBaseURL)
	m.JiraEmail = readString(api.JiraEmail, m.JiraEmail)
	m.JiraProjectKey = readString(api.JiraProjectKey, m.JiraProjectKey)
	m.JiraIssueType = readString(api.JiraIssueType, m.JiraIssueType)
	if api.SendResolved != nil {
		m.SendResolved = types.BoolValue(*api.SendResolved)
	}
}

func (r *notificationChannelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan notificationChannelModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.client.CreateNotificationChannel(ctx, plan.toAPI())
	if err != nil {
		resp.Diagnostics.AddError("Creating notification channel", err.Error())
		return
	}
	plan.fromAPI(created)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *notificationChannelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state notificationChannelModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetNotificationChannel(ctx, state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx) // deleted outside Terraform: plan will recreate it
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading notification channel", err.Error())
		return
	}
	state.fromAPI(got)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *notificationChannelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state notificationChannelModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	updated, err := r.client.UpdateNotificationChannel(ctx, state.ID.ValueString(), plan.toAPI())
	if err != nil {
		resp.Diagnostics.AddError("Updating notification channel", err.Error())
		return
	}
	plan.fromAPI(updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *notificationChannelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state notificationChannelModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteNotificationChannel(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Deleting notification channel", err.Error())
	}
}

// ImportState accepts a channel id or its name. Credentials can't be imported (Flare never returns
// them); set them in configuration and the next apply writes them.
func (r *notificationChannelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID
	if !guidPattern.MatchString(id) {
		found, err := r.client.FindNotificationChannelByName(ctx, id)
		if err != nil {
			resp.Diagnostics.AddError("Importing notification channel", err.Error())
			return
		}
		id = found.ID
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func joinQuoted(values []string) string {
	out := ""
	for i, v := range values {
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("`%s`", v)
	}
	return out
}
