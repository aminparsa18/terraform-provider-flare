package provider

import (
	"context"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*alertTemplateResource)(nil)
	_ resource.ResourceWithConfigure   = (*alertTemplateResource)(nil)
	_ resource.ResourceWithImportState = (*alertTemplateResource)(nil)
)

// NewAlertTemplateResource is the flare_alert_template factory.
func NewAlertTemplateResource() resource.Resource { return &alertTemplateResource{} }

type alertTemplateResource struct{ client *client.Client }

type alertTemplateModel struct {
	ID                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	Description          types.String `tfsdk:"description"`
	IsDefault            types.Bool   `tfsdk:"is_default"`
	TitleTemplate        types.String `tfsdk:"title_template"`
	BodyTemplate         types.String `tfsdk:"body_template"`
	ResolvedBodyTemplate types.String `tfsdk:"resolved_body_template"`
	ChannelBodies        types.Map    `tfsdk:"channel_bodies"`
}

func (r *alertTemplateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert_template"
}

func (r *alertTemplateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	text := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: desc}
	}
	resp.Schema = schema.Schema{
		Description: "A shared alert notification template: named title and body texts (`{{placeholder}}` syntax) that alert rules reuse " +
			"through `notification_template`. Editing a template changes the next notification of every rule that uses it. A rule's own " +
			"`notification_title_template` / `notification_body_template` still override the template. At least one text is required.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, Description: "Flare's id for the template.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name":        schema.StringAttribute{Required: true, Description: "Unique (case-insensitive) name, at most 128 characters. Renaming updates in place."},
			"description": text("Free-text description."),
			"is_default": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Make this the instance default, used by rules that pick no template. Setting it clears any other default, so at most one template should set it.",
			},
			"title_template":         text("Notification title. Empty keeps each channel's built-in subject."),
			"body_template":          text("Body of a fired notification. Empty keeps the built-in wording."),
			"resolved_body_template": text("Body of a resolved notification. Empty falls back to `body_template`."),
			"channel_bodies": schema.MapAttribute{
				Optional: true, ElementType: types.StringType,
				Description: "Fired-body override per channel type (`Webhook`, `Telegram`, `Email`, `PagerDuty`, `Teams`, `Discord`, `Jira`, `IncidentIo`, `JsmOps`), e.g. a short one for Telegram. A type with no entry uses `body_template`.",
			},
		},
	}
}

func (r *alertTemplateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (m alertTemplateModel) toAPI(ctx context.Context) (client.AlertTemplate, diag.Diagnostics) {
	var diags diag.Diagnostics
	// A template with no overrides sends an explicit empty map so an update clears any stored ones.
	bodies := map[string]string{}
	if !m.ChannelBodies.IsNull() && !m.ChannelBodies.IsUnknown() {
		diags.Append(m.ChannelBodies.ElementsAs(ctx, &bodies, false)...)
	}
	return client.AlertTemplate{
		Name: m.Name.ValueString(), Description: strPtr(m.Description), IsDefault: boolPtr(m.IsDefault),
		TitleTemplate: strPtr(m.TitleTemplate), BodyTemplate: strPtr(m.BodyTemplate), ResolvedBodyTemplate: strPtr(m.ResolvedBodyTemplate),
		ChannelBodies: bodies,
	}, diags
}

func (m *alertTemplateModel) fromAPI(api client.AlertTemplate) {
	m.ID = types.StringValue(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Description = types.StringValue(deref(api.Description))
	m.IsDefault = types.BoolValue(api.IsDefault != nil && *api.IsDefault)
	m.TitleTemplate = types.StringValue(deref(api.TitleTemplate))
	m.BodyTemplate = types.StringValue(deref(api.BodyTemplate))
	m.ResolvedBodyTemplate = types.StringValue(deref(api.ResolvedBodyTemplate))
	switch {
	case len(api.ChannelBodies) > 0:
		m.ChannelBodies, _ = types.MapValueFrom(context.Background(), types.StringType, api.ChannelBodies)
	case !m.ChannelBodies.IsNull() && !m.ChannelBodies.IsUnknown():
		m.ChannelBodies, _ = types.MapValueFrom(context.Background(), types.StringType, map[string]string{})
	default:
		m.ChannelBodies = types.MapNull(types.StringType)
	}
}

func (r *alertTemplateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan alertTemplateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in, diags := plan.toAPI(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.client.CreateAlertTemplate(ctx, in)
	if err != nil {
		resp.Diagnostics.AddError("Creating alert template", err.Error())
		return
	}
	plan.fromAPI(created)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *alertTemplateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state alertTemplateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetAlertTemplate(ctx, state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading alert template", err.Error())
		return
	}
	state.fromAPI(got)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *alertTemplateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state alertTemplateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in, diags := plan.toAPI(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	updated, err := r.client.UpdateAlertTemplate(ctx, state.ID.ValueString(), in)
	if err != nil {
		resp.Diagnostics.AddError("Updating alert template", err.Error())
		return
	}
	plan.fromAPI(updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *alertTemplateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state alertTemplateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Flare refuses (409) while rules still reference the template; Terraform destroys dependants first, so
	// that only surfaces for rules managed outside this configuration, and the message names them.
	if err := r.client.DeleteAlertTemplate(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Deleting alert template", err.Error())
	}
}

// ImportState accepts a template id or its name.
func (r *alertTemplateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID
	if !guidPattern.MatchString(id) {
		found, err := r.client.FindAlertTemplateByName(ctx, id)
		if err != nil {
			resp.Diagnostics.AddError("Importing alert template", err.Error())
			return
		}
		id = found.ID
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
