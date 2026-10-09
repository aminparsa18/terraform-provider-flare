package provider

import (
	"context"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*statusPageResource)(nil)
	_ resource.ResourceWithConfigure   = (*statusPageResource)(nil)
	_ resource.ResourceWithImportState = (*statusPageResource)(nil)
)

var statusComponentKinds = []string{"Monitor", "Slo"}

// NewStatusPageResource is the flare_status_page factory.
func NewStatusPageResource() resource.Resource { return &statusPageResource{} }

type statusPageResource struct{ client *client.Client }

type statusComponentModel struct {
	Name  types.String `tfsdk:"name"`
	Kind  types.String `tfsdk:"kind"`
	RefID types.String `tfsdk:"ref_id"`
}

type statusPageModel struct {
	ID          types.String           `tfsdk:"id"`
	Slug        types.String           `tfsdk:"slug"`
	Title       types.String           `tfsdk:"title"`
	Description types.String           `tfsdk:"description"`
	Enabled     types.Bool             `tfsdk:"enabled"`
	Components  []statusComponentModel `tfsdk:"components"`

	SubscriberChannelIDs types.Set `tfsdk:"subscriber_channel_ids"`
}

func (r *statusPageResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_status_page"
}

func (r *statusPageResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A public status page (ADR-0158) served at `/status/<slug>` while `enabled`. Each component is a public display name over a synthetic monitor or an SLO.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, Description: "Flare's id for the page.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"slug": schema.StringAttribute{
				Required: true, Description: "URL segment: 1-64 lowercase letters, digits or hyphens, not starting or ending with a hyphen. Unique across pages.",
			},
			"title": schema.StringAttribute{Required: true, Description: "Page heading, at most 120 characters."},
			"description": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Free text shown under the title, at most 1000 characters.",
			},
			"enabled": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Publishes the page to anyone with the URL. Defaults to false.",
			},
			"subscriber_channel_ids": schema.SetAttribute{
				Optional: true, Computed: true, ElementType: types.StringType, Default: emptyStringSet(),
				Description: "Notification channels (webhook, Slack, Telegram, email, Teams or Discord; at most 20) told about every incident on the page, e.g. `flare_notification_channel.oncall.id`. Empty (the default) tells none.",
			},
			"components": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Rows of the page, in order (at most 50).",
				Validators:  []validator.List{listvalidator.SizeAtMost(50)},
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{Required: true, Description: "Public display name; an internal monitor or SLO name is never shown."},
					"kind": schema.StringAttribute{
						Required: true, Description: "One of: " + joinQuoted(statusComponentKinds) + ".",
						Validators: []validator.String{stringvalidator.OneOf(statusComponentKinds...)},
					},
					"ref_id": schema.StringAttribute{Required: true, Description: "Id of the monitor or SLO, e.g. `flare_slo.api.id`."},
				}},
			},
		},
	}
}

func (r *statusPageResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (m statusPageModel) toAPI() (client.StatusPage, diag.Diagnostics) {
	var diags diag.Diagnostics
	channels := []string{}
	diags.Append(setToStrings(context.Background(), m.SubscriberChannelIDs, &channels)...)
	out := client.StatusPage{
		Slug: m.Slug.ValueString(), Title: m.Title.ValueString(), Description: strPtr(m.Description),
		Enabled: boolPtr(m.Enabled), Components: []client.StatusPageComponent{},
		SubscriberChannelIDs: channels,
	}
	for _, c := range m.Components {
		out.Components = append(out.Components, client.StatusPageComponent{
			Name: c.Name.ValueString(), Kind: c.Kind.ValueString(), RefID: c.RefID.ValueString(),
		})
	}
	return out, diags
}

func (m *statusPageModel) fromAPI(api client.StatusPage) {
	m.ID = types.StringValue(api.ID)
	m.Slug = types.StringValue(api.Slug)
	m.Title = types.StringValue(api.Title)
	m.Description = types.StringValue(deref(api.Description))
	m.Enabled = types.BoolValue(api.Enabled != nil && *api.Enabled)
	m.SubscriberChannelIDs = stringSet(api.SubscriberChannelIDs)
	// No components reads back as null so an omitted attribute stays a no-op diff.
	m.Components = nil
	for _, c := range api.Components {
		m.Components = append(m.Components, statusComponentModel{
			Name: types.StringValue(c.Name), Kind: types.StringValue(c.Kind), RefID: types.StringValue(c.RefID),
		})
	}
}

func (r *statusPageResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan statusPageModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, diags := plan.toAPI()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.client.CreateStatusPage(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Creating status page", err.Error())
		return
	}
	plan.fromAPI(created)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *statusPageResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state statusPageModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetStatusPage(ctx, state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading status page", err.Error())
		return
	}
	state.fromAPI(got)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *statusPageResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state statusPageModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, diags := plan.toAPI()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	updated, err := r.client.UpdateStatusPage(ctx, state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Updating status page", err.Error())
		return
	}
	plan.fromAPI(updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *statusPageResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state statusPageModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteStatusPage(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Deleting status page", err.Error())
	}
}

// ImportState accepts a page id or its slug.
func (r *statusPageResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID
	if !guidPattern.MatchString(id) {
		found, err := r.client.FindStatusPageBySlug(ctx, id)
		if err != nil {
			resp.Diagnostics.AddError("Importing status page", err.Error())
			return
		}
		id = found.ID
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
