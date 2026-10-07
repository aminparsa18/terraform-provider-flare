package provider

import (
	"context"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*notificationChannelDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*notificationChannelDataSource)(nil)
)

// NewNotificationChannelDataSource is the flare_notification_channel data source factory.
func NewNotificationChannelDataSource() datasource.DataSource {
	return &notificationChannelDataSource{}
}

type notificationChannelDataSource struct{ client *client.Client }

type notificationChannelDataModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	Type         types.String `tfsdk:"type"`
	SendResolved types.Bool   `tfsdk:"send_resolved"`
}

func (d *notificationChannelDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notification_channel"
}

func (d *notificationChannelDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up a notification channel Terraform doesn't own, by name. Credentials are never exposed.",
		Attributes: map[string]schema.Attribute{
			"name":          schema.StringAttribute{Required: true, Description: "Channel name (case-insensitive)."},
			"id":            schema.StringAttribute{Computed: true},
			"description":   schema.StringAttribute{Computed: true},
			"type":          schema.StringAttribute{Computed: true},
			"send_resolved": schema.BoolAttribute{Computed: true},
		},
	}
}

func (d *notificationChannelDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *notificationChannelDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg notificationChannelDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ch, err := d.client.FindNotificationChannelByName(ctx, cfg.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Looking up notification channel", err.Error())
		return
	}
	cfg.ID = types.StringValue(ch.ID)
	cfg.Name = types.StringValue(ch.Name)
	cfg.Type = types.StringValue(ch.Type)
	cfg.Description = types.StringValue("")
	if ch.Description != nil {
		cfg.Description = types.StringValue(*ch.Description)
	}
	cfg.SendResolved = types.BoolValue(ch.SendResolved == nil || *ch.SendResolved)
	resp.Diagnostics.Append(resp.State.Set(ctx, cfg)...)
}
